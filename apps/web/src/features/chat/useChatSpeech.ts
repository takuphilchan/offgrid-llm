import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { voiceText } from '../../i18n/voice';
import { ResponseSpeech, stopSpeechEvent, type SpeechState } from '../../lib/response-speech';
import { selectSpeech, type VoicePreferences } from '../../lib/voice-preferences';

export function useChatSpeech(preferences: VoicePreferences, locale: Parameters<typeof voiceText>[0]) {
  const [speakResponses, setSpeakResponses] = useState(false);
  const [speechState, setSpeechState] = useState<SpeechState>('idle');
  const [speechError, setSpeechError] = useState('');
  const [checkingSpeech, setCheckingSpeech] = useState(false);
  const speechCheck = useRef<AbortController | null>(null);
  const speech = useRef<ResponseSpeech | null>(null);
  const stopSpeech = () => { speech.current?.stop(); speech.current = null; };

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
  return { speakResponses, speechState, setSpeechState, speechError, setSpeechError, checkingSpeech, speech, stopSpeech, toggleSpeech };
}
