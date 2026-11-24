import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { selectSpeech, type VoicePreferences } from '../../lib/voice-preferences';
import { writeDraft } from '../../lib/drafts';

export type TalkState = 'off' | 'checking' | 'listening' | 'transcribing' | 'thinking' | 'speaking' | 'needs_attention';

export function useTalkMode() {
  const [state, setState] = useState<TalkState>('off');
  const [cycle, setCycle] = useState(0);
  const active = useRef(false);
  const session = useRef('');
  const revision = useRef(0);
  const submission = useRef(0);
  const capture = useRef({accepted: false});
  const captureOwner = capture.current;
  const nextCycle = () => { capture.current = {accepted: false}; setCycle(value => value + 1); };
  const check = useRef<AbortController | null>(null);
  const silenceTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const reset = () => {
    clearTimeout(silenceTimer.current);
    check.current?.abort();
    active.current = false;
    session.current = '';
    revision.current++;
    setState('off');
  };
  useEffect(() => () => {
    clearTimeout(silenceTimer.current);
    check.current?.abort();
    active.current = false;
    session.current = '';
    revision.current++;
  }, []);

  // Silence is not a failed turn. Re-arm capture, never replay transcription or
  // send an empty message. Ownership invalidation also cancels this short delay.
  const silence = () => {
    if (!active.current) return;
    clearTimeout(silenceTimer.current);
    const current = revision.current;
    setState('listening');
    silenceTimer.current = setTimeout(() => {
      if (active.current && current === revision.current) nextCycle();
    }, 750);
  };

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
    session.current = crypto.randomUUID();
    setState('checking'); setError('');
    try {
      const status = await api.audioStatus(pending.signal);
      if (pending.signal.aborted || current !== revision.current) return;
      selectSpeech(status, preferences, 'asr', locale);
      selectSpeech(status, preferences, 'tts', locale);
      active.current = true;
      setState('listening'); nextCycle();
    } catch (reason) {
      if (!pending.signal.aborted && current === revision.current) {
        setState('needs_attention');
        setError(reason instanceof Error ? reason.message : unavailable);
      }
    } finally { if (check.current === pending) check.current = null; }
  };

  const acceptTranscript = (value: string, draftKey: string, send: (value: string) => Promise<boolean>) => {
    if (!active.current || capture.current !== captureOwner || captureOwner.accepted || state !== 'listening' && state !== 'transcribing') return;
    const transcript = value.trim();
    if (!transcript) { silence(); return; }
    captureOwner.accepted = true;
    const current = revision.current;
    const utterance = ++submission.current;
    const failed = () => {
      // Both a restarted session and its next utterance have distinct owners.
      if (active.current && revision.current === current && submission.current === utterance) { active.current = false; setState('needs_attention'); }
    };
    writeDraft(draftKey, transcript);
    setState('thinking');
    void send(transcript).then(sent => {
      if (!sent) failed();
    }, failed);
  };

  const speakAgain = (busy: boolean, stopSpeech: () => void) => {
    if (!active.current || busy) return;
    revision.current++;
    stopSpeech();
    setState('listening'); nextCycle();
  };
  const resumeAfterInput = (continuous = false) => {
    // A worker answer has no foreground reply to play. Wait until admission's
    // busy cleanup settles, and never re-arm a reset or superseding capture.
    const current = revision.current;
    setTimeout(() => {
      if (active.current && revision.current === current && capture.current === captureOwner && (captureOwner.accepted || continuous)) {setState('listening'); nextCycle();}
    }, 0);
  };
  const fail = (message: string, setError: (message: string) => void) => {
    if (!active.current) return;
    active.current = false; setError(message); setState('needs_attention');
  };
  const capturePhase = (phase: string) => {
    if (active.current && capture.current === captureOwner && !captureOwner.accepted && phase === 'transcribing') setState('transcribing');
  };
  return { state, setState, cycle, nextCycle, active, session, revision, reset, start, acceptTranscript, speakAgain, resumeAfterInput, fail, silence, capturePhase };
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

export function useTalkDismiss(reset: () => void, stopSpeech: () => void) {
  useEffect(() => {
    const dismiss = () => {reset(); stopSpeech();};
    const visibility = () => {if (document.hidden) dismiss();};
    window.addEventListener('offgrid-desktop-dismiss', dismiss);
    document.addEventListener('visibilitychange', visibility);
    return () => {window.removeEventListener('offgrid-desktop-dismiss', dismiss); document.removeEventListener('visibilitychange', visibility);};
  }, []);
}
