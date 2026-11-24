import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { voiceText } from '../../i18n/voice';
import { ResponseSpeech, stopSpeechEvent, type SpeechState } from '../../lib/response-speech';
import { selectSpeech, type VoicePreferences } from '../../lib/voice-preferences';
import type {useTalkMode} from './useTalkMode';
import type {SpeechDelivery, SpeechOwner, SpeechSource} from '../../lib/speech-delivery';

export function useChatSpeech(preferences: VoicePreferences, locale: Parameters<typeof voiceText>[0]) {
  const [speakResponses, setSpeakResponses] = useState(false);
  const [speechState, setSpeechState] = useState<SpeechState>('idle');
  const [speechError, setSpeechError] = useState('');
  const [checkingSpeech, setCheckingSpeech] = useState(false);
  const speechCheck = useRef<AbortController | null>(null);
  const speech = useRef<ResponseSpeech | null>(null);
  const delivery = useRef<SpeechDelivery | null>(null);
  const epoch = useRef(0);
  const stopSpeech = () => { speech.current?.stop(); speech.current = null; epoch.current++; };
  // Capture before asynchronous admission. A later Stop or another audio owner
  // revokes this capability even when no ResponseSpeech exists yet.
  const retainSpeechIntent = () => { const generation = epoch.current; return () => epoch.current === generation; };
  const finishSpeech = (answer: string, source: SpeechSource) => {
    const active = speech.current;
    if (!active?.matchesSource(source) || !active.bindSource(source)) return false;
    active.finish(answer); return true;
  };
  const stopSpeechFor = (source: SpeechSource) => {
    if (!speech.current?.matchesSource(source)) return false;
    stopSpeech(); return true;
  };
  const beginSpeech = (fromTalk: boolean, talk: ReturnType<typeof useTalkMode>, mounted: {current: boolean}, owner: SpeechOwner, source?: SpeechSource) => {
    window.dispatchEvent(new Event(stopSpeechEvent));
    stopSpeech();
    const generation = ++epoch.current;
    const revision = talk.revision.current;
    const owns = () => epoch.current === generation;
    const current = () => owns() && fromTalk && talk.active.current && talk.revision.current === revision;
    speech.current = new ResponseSpeech(state => {
      if (!owns()) return;
      setSpeechState(state);
      if (current() && (state === 'preparing' || state === 'playing')) talk.setState('speaking');
    }, message => {
      if (!owns()) return;
      setSpeechError(message);
      if (current()) {talk.active.current = false; talk.setState('needs_attention');}
    }, preferences, locale, fromTalk ? () => {
      const receipt = delivery.current;
      window.setTimeout(() => {
        if (current() && mounted.current && receipt?.state === 'completed' && receipt.source && delivery.current?.playbackID === receipt.playbackID && delivery.current.state === 'completed') {talk.setState('listening'); talk.nextCycle();}
      }, 0);
    } : undefined, {owner, changed: value => {if (owns()) delivery.current = value;}});
    delivery.current = speech.current.delivery();
    if (source) speech.current.bindSource(source);
    return speech.current;
  };

  useEffect(() => {
    window.addEventListener(stopSpeechEvent, stopSpeech);
    return () => { window.removeEventListener(stopSpeechEvent, stopSpeech); speechCheck.current?.abort(); stopSpeech(); };
  }, []);

  const toggleSpeech = async (enabled: boolean) => {
    speechCheck.current?.abort();
    setSpeechError('');
    if (!enabled) { setSpeakResponses(false); stopSpeech(); return; }
    const check = new AbortController(); speechCheck.current = check;
    setCheckingSpeech(true);
    try {
      const status = await api.audioStatus(check.signal);
      if (check.signal.aborted) return;
      selectSpeech(status, preferences, 'tts', locale);
      setSpeakResponses(true);
    } catch (reason) { if (!check.signal.aborted) setSpeechError(reason instanceof Error ? reason.message : voiceText(locale).speechUnavailable); }
    finally { if (!check.signal.aborted) setCheckingSpeech(false); }
  };
  return { speakResponses, speechState, speechError, setSpeechError, checkingSpeech, speech, delivery, stopSpeech, stopSpeechFor, finishSpeech, toggleSpeech, beginSpeech, retainSpeechIntent };
}
