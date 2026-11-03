import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { selectSpeech, type VoicePreferences } from '../../lib/voice-preferences';
import { writeDraft } from '../../lib/drafts';

export type TalkState = 'off' | 'checking' | 'listening' | 'transcribing' | 'thinking' | 'speaking' | 'needs_attention';

export function useTalkMode() {
  const [state, setState] = useState<TalkState>('off');
  const [cycle, setCycle] = useState(0);
  const active = useRef(false);
  const revision = useRef(0);
  const check = useRef<AbortController | null>(null);

  const reset = () => {
    check.current?.abort();
    active.current = false;
    revision.current++;
    setState('off');
  };
  useEffect(() => () => {
    check.current?.abort();
    active.current = false;
    revision.current++;
  }, []);

  const start = async ({ eligible, hasDraft, preferences, locale, unavailable, draftError, setError }: {
    eligible: boolean; hasDraft: boolean; preferences: VoicePreferences;
    locale: Parameters<typeof selectSpeech>[3]; unavailable: string; draftError: string;
    setError: (message: string) => void;
  }) => {
    if (state !== 'off' && state !== 'needs_attention') return;
    if (hasDraft) { setError(draftError); return; }
    if (!eligible) return;
    const pending = new AbortController();
    check.current?.abort(); check.current = pending;
    const current = ++revision.current;
    setState('checking'); setError('');
    try {
      const status = await api.audioStatus(pending.signal);
      if (pending.signal.aborted || current !== revision.current) return;
      selectSpeech(status, preferences, 'asr', locale);
      selectSpeech(status, preferences, 'tts', locale);
      active.current = true;
      setState('listening'); setCycle(value => value + 1);
    } catch (reason) {
      if (!pending.signal.aborted && current === revision.current) {
        setState('needs_attention');
        setError(reason instanceof Error ? reason.message : unavailable);
      }
    } finally { if (check.current === pending) check.current = null; }
  };

  const acceptTranscript = (value: string, draftKey: string, send: (value: string) => Promise<boolean>) => {
    if (!active.current || state !== 'listening' && state !== 'transcribing') return;
    const transcript = value.trim();
    if (!transcript) { active.current = false; setState('needs_attention'); return; }
    writeDraft(draftKey, transcript);
    setState('thinking');
    void send(transcript).then(sent => {
      if (!sent && active.current) { active.current = false; setState('needs_attention'); }
    });
  };

  const speakAgain = (busy: boolean, stopSpeech: () => void) => {
    if (!active.current || busy) return;
    revision.current++;
    stopSpeech();
    setState('listening'); setCycle(value => value + 1);
  };
  const fail = (message: string, setError: (message: string) => void) => {
    if (!active.current) return;
    active.current = false; setError(message); setState('needs_attention');
  };
  const capturePhase = (phase: string) => {
    if (active.current && phase === 'transcribing') setState('transcribing');
  };
  return { state, setState, cycle, nextCycle: () => setCycle(value => value + 1), active, revision, reset, start, acceptTranscript, speakAgain, fail, capturePhase };
}

export function useTalkIdentity(identity: string, cancel: () => void) {
  const previous = useRef(identity);
  useEffect(() => {
    if (previous.current !== identity) {
      previous.current = identity;
      cancel();
    }
  }, [identity]);
}
