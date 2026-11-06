import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { api } from '../api/client';
import { Icon } from './Icon';
import { useI18n } from '../i18n';
import { voiceText, voiceSilenceText } from '../i18n/voice';
import { operationFeedback } from '../i18n/operation-feedback';
import { spokenProse, stopSpeechEvent } from '../lib/response-speech';
import { selectSpeech, SpeechSetupRequired, useVoicePreferences } from '../lib/voice-preferences';
import { VoiceSettings } from './VoiceSettings';
import { UtilityPopover } from './UtilityPopover';
import { voiceSettingsText } from '../i18n/voice-settings';
import { observeSpeechPause } from '../lib/speech-pause';

async function toWav(blob: Blob): Promise<Blob> {
  const AudioContextCtor = window.AudioContext ?? (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AudioContextCtor || blob.type.includes('wav')) return blob;
  const context = new AudioContextCtor();
  try {
    const decoded = await context.decodeAudioData(await blob.arrayBuffer());
    const channels = Math.min(decoded.numberOfChannels, 2);
    const frames = decoded.length;
    const bytes = new ArrayBuffer(44 + frames * channels * 2);
    const view = new DataView(bytes);
    const write = (offset: number, value: string) => [...value].forEach((char, index) => view.setUint8(offset + index, char.charCodeAt(0)));
    write(0, 'RIFF'); view.setUint32(4, 36 + frames * channels * 2, true); write(8, 'WAVE');
    write(12, 'fmt '); view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, channels, true);
    view.setUint32(24, decoded.sampleRate, true); view.setUint32(28, decoded.sampleRate * channels * 2, true);
    view.setUint16(32, channels * 2, true); view.setUint16(34, 16, true); write(36, 'data'); view.setUint32(40, frames * channels * 2, true);
    const input = Array.from({ length: channels }, (_, channel) => decoded.getChannelData(channel));
    let offset = 44;
    for (let frame = 0; frame < frames; frame++) for (let channel = 0; channel < channels; channel++) {
      const sample = Math.max(-1, Math.min(1, input[channel][frame]));
      view.setInt16(offset, sample < 0 ? sample * 0x8000 : sample * 0x7fff, true); offset += 2;
    }
    return new Blob([bytes], { type: 'audio/wav' });
  } finally { await context.close(); }
}

type RecordingWork = { controller: AbortController; stream?: MediaStream; recorder?: MediaRecorder; timer?: number; stopPause?: () => void };
type CapturePhase = 'idle' | 'checking' | 'permission' | 'recording' | 'transcribing';

export function VoiceInputButton({ onTranscript, onFailure, onPhaseChange, autoStart = false, contextKey, disabled = false, compact = false }: { onTranscript: (text: string) => void; onFailure?: (message: string) => void; onPhaseChange?: (phase: 'idle' | 'checking' | 'permission' | 'recording' | 'transcribing') => void; autoStart?: boolean; contextKey: string; disabled?: boolean; compact?: boolean }) {
  const { preferences } = useVoicePreferences();
  const { locale, messages } = useI18n();
  const copy = { ...voiceText(locale), silence: voiceSilenceText[locale] };
  const feedback = operationFeedback(locale);
  const [phase, setPhase] = useState<CapturePhase>('idle');
  const [levels, setLevels] = useState<number[]>(Array(12).fill(0));
  const [seconds, setSeconds] = useState(0);
  useEffect(() => {
    if (phase !== 'recording') return;
    const started = performance.now();
    setSeconds(0);
    const timer = window.setInterval(() => setSeconds(Math.floor((performance.now() - started) / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, [phase]);
  useEffect(() => onPhaseChange?.(phase), [phase, onPhaseChange]);
  const [failure, setFailure] = useState<{ key?: 'failed' | 'permission' | 'unsupported' | 'silence'; message?: string } | null>(null);
  const error = failure?.key ? copy[failure.key] : failure?.message ?? '';
  const currentCopy = useRef(copy); currentCopy.current = copy;
  const [setupRequired, setSetupRequired] = useState(false);
  const current = useRef<RecordingWork | null>(null);
  // Use the current draft callback, but never deliver a result to another composer.
  const target = useRef({ contextKey, onTranscript, onFailure });
  target.current = { contextKey, onTranscript, onFailure };
  const fail = (message: string) => { setFailure({ message }); target.current.onFailure?.(message); };
  const failKnown = (key: 'failed' | 'permission' | 'unsupported' | 'silence') => { setFailure({ key }); target.current.onFailure?.(currentCopy.current[key]); };
  const cancel = () => {
    const work = current.current;
    current.current = null;
    if (!work) return;
    work.controller.abort();
    clearTimeout(work.timer);
    work.stopPause?.();
    if (work.recorder?.state === 'recording') work.recorder.stop();
    work.stream?.getTracks().forEach(track => track.stop());
  };
  useEffect(() => {
    setPhase('idle'); setFailure(null); setSetupRequired(false);
    return cancel;
  }, [contextKey]);
  const stop = () => {
    const work = current.current;
    if (work?.recorder?.state === 'recording') {
      clearTimeout(work.timer);
      work.stopPause?.();
      setPhase('transcribing');
      work.recorder.stop();
      work.stream?.getTracks().forEach(track => track.stop());
    } else { cancel(); setPhase('idle'); }
  };

  const start = async () => {
    if (current.current || disabled) return;
    setFailure(null);
    setSetupRequired(false);
    setLevels(Array(12).fill(0));
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
      failKnown('unsupported');
      return;
    }
    const work: RecordingWork = { controller: new AbortController() };
    current.current = work;
    setPhase('checking');
    const active = () => current.current === work && !work.controller.signal.aborted && target.current.contextKey === contextKey;
    try {
      // Check the service before requesting a device permission. This prevents
      // a confusing microphone prompt when no local ASR runtime is installed.
      const status = await api.audioStatus(work.controller.signal);
      if (!active()) return;
      const selection = selectSpeech(status, preferences, 'asr', locale);
      setPhase('permission');
      const input = await navigator.mediaDevices.getUserMedia({ audio: true });
      if (!active()) { input.getTracks().forEach(track => track.stop()); return; }
      work.stream = input;
      const chunks: Blob[] = [];
      let bytes = 0;
      const mime = ['audio/webm;codecs=opus', 'audio/webm', 'audio/ogg'].find(value => MediaRecorder.isTypeSupported(value));
      const next = new MediaRecorder(input, mime ? { mimeType: mime } : undefined);
      work.recorder = next;
      next.ondataavailable = event => {
        if (!active() || !event.data.size) return;
        bytes += event.data.size;
        if (bytes > 12 * 1024 * 1024) { cancel(); setPhase('idle'); failKnown('failed'); return; }
        chunks.push(event.data);
      };
      next.onerror = () => { if (active()) { cancel(); failKnown('failed'); setPhase('idle'); } };
      next.onstop = async () => {
        clearTimeout(work.timer);
        work.stopPause?.();
        work.stream?.getTracks().forEach(track => track.stop());
        if (!active()) return;
        setPhase('transcribing');
        try {
          const blob = new Blob(chunks, { type: next.mimeType || 'audio/webm' });
          if (!blob.size) { failKnown('silence'); return; }
          const wav = await toWav(blob);
          if (!active()) return;
          const result = await api.transcribeAudio(wav, 'offgrid-recording.wav', work.controller.signal, selection);
          if (active() && result.text.trim()) target.current.onTranscript(result.text.trim());
          else if (active()) failKnown('silence');
        } catch (reason) {
          if (active()) fail(reason instanceof Error ? reason.message : currentCopy.current.failed);
        } finally { if (active()) { current.current = null; setPhase('idle'); } }
      };
      next.start(1000);
      work.stopPause = observeSpeechPause(input, () => { if (active()) stop(); }, level => {
        if (active()) setLevels(previous => [...previous.slice(1), Math.min(1, Math.sqrt(Math.max(0, level)) * 2.5)]);
      });
      work.timer = window.setTimeout(() => { if (active()) stop(); }, 120_000);
      setPhase('recording');
    } catch (reason) {
      if (!active()) return;
      cancel(); setPhase('idle');
      setSetupRequired(reason instanceof SpeechSetupRequired);
      if ((reason as DOMException)?.name === 'NotAllowedError') failKnown('permission');
      else fail(reason instanceof Error ? reason.message : currentCopy.current.failed);
    }
  };
  useEffect(() => { if (autoStart) void start(); }, [autoStart, contextKey]);

  const label = phase === 'recording' ? copy.stop : phase === 'transcribing' ? feedback.cancelTranscription : phase === 'idle' ? copy.record : feedback.cancelVoice;
  const status = phase === 'checking' ? feedback.checkingVoice : phase === 'permission' ? feedback.permissionPending : phase === 'transcribing' ? copy.transcribing : '';
  if (compact && error) return <span className="voice-input-control voice-input-compact" data-capture-phase={phase}>
    <UtilityPopover defaultOpen label={voiceSettingsText(locale).title} trigger={<Icon name="mic" size={18} />} triggerClassName="icon-button voice-needs-setup" className="composer-voice-recovery" align="end">
      <p className="voice-error" role="status">{error}</p>
      {setupRequired && <VoiceSettings inline />}
      <button type="button" className="secondary-button" disabled={disabled} onClick={() => void start()}>{copy.record}</button>
    </UtilityPopover>
  </span>;
  return <span className={`voice-input-control${compact ? ' voice-input-compact' : ''}`} data-capture-phase={phase}>
    {compact && phase !== 'idle' && <span className="voice-capture-feedback">
      <span className="speech-orb" data-phase={phase} aria-hidden="true" style={{ '--speech-scale': 1 + (levels.at(-1) ?? 0) * .4 } as CSSProperties}>{phase !== 'recording' && <span className="voice-progress" />}</span>
      {phase === 'recording' && <span className="voice-wave" aria-hidden="true">{levels.map((level, index) => <i key={index} style={{ height: `${3 + level * 21}px` }} />)}</span>}
      <span role="status">{phase === 'recording' ? copy.recording : status}</span>
      {phase === 'recording' && <span className="voice-duration" aria-hidden="true">{Math.floor(seconds / 60)}:{String(seconds % 60).padStart(2, '0')}</span>}
    </span>}
    {compact && phase === 'recording' && <button type="button" className="voice-discard" aria-label={messages.models.cancel} title={messages.models.cancel} onClick={() => { cancel(); setPhase('idle'); }}><Icon name="close" size={17} /></button>}
    <button type="button" className={phase === 'recording' ? 'voice-button recording' : 'voice-button'} disabled={disabled && !current.current} onClick={current.current ? stop : () => void start()} aria-label={label} aria-pressed={phase === 'recording'} aria-busy={phase === 'checking' || phase === 'permission' || phase === 'transcribing'} title={label}>
      <Icon name={phase === 'recording' ? 'check' : phase !== 'idle' ? 'close' : 'mic'} size={18} />
      <span className={compact ? 'sr-only' : undefined}>{phase === 'recording' ? copy.recording : label}</span>
    </button>
    {status && !compact && <small role="status">{status}</small>}
    {error && <small className="voice-error" role="status">{error}</small>}
    {setupRequired && <VoiceSettings subtle />}
  </span>;
}

// Deterministic cleanup speaks visible prose, not code, raw Markdown or URLs.
// Small bounded chunks reduce time to first playback without simultaneous jobs.
export function speechChunks(text: string): string[] {
  const plain = spokenProse(text);
  const chunks: string[] = [];
  let rest = plain;
  while (rest.length) {
    const limit = chunks.length ? 280 : 160;
    if (rest.length <= limit) { chunks.push(rest); break; }
    const head = rest.slice(0, limit);
    const sentence = [...head.matchAll(/[.!?。！？](?:\s|$)/g)].pop();
    const end = sentence && sentence.index! > 35 ? sentence.index! + sentence[0].length : head.lastIndexOf(' ');
    const cut = end > 0 ? end : limit;
    chunks.push(rest.slice(0, cut).trim()); rest = rest.slice(cut).trim();
  }
  return chunks;
}

const playbackEvent = stopSpeechEvent;

export function ReadAloudButton({ text }: { text: string }) {
  const { preferences } = useVoicePreferences();
  const { locale } = useI18n();
  const copy = voiceText(locale);
  const [state, setState] = useState<'idle' | 'preparing' | 'playing'>('idle');
  const [error, setError] = useState('');
  const current = useRef<{ controller: AbortController; player: HTMLAudioElement; url?: string } | null>(null);

  const dispose = () => {
    const work = current.current;
    current.current = null;
    if (!work) return;
    work.controller.abort();
    work.player.pause();
    work.player.removeAttribute('src');
    work.player.load();
    if (work.url) URL.revokeObjectURL(work.url);
  };
  const stop = () => { dispose(); setState('idle'); };
  useEffect(() => {
    window.addEventListener(playbackEvent, stop);
    return () => { window.removeEventListener(playbackEvent, stop); dispose(); };
  }, []);
  useEffect(() => { stop(); setError(''); }, [text]);

  const speak = async () => {
    if (current.current) { stop(); return; }
    window.dispatchEvent(new Event(playbackEvent));
    setError('');
    setState('preparing');
    const work = { controller: new AbortController(), player: new Audio(), url: undefined as string | undefined };
    current.current = work;
    const timer = window.setTimeout(() => {
      if (current.current === work) { stop(); setError(copy.speechTimeout); }
    }, 300_000);
    try {
      const status = await api.audioStatus(work.controller.signal);
      if (current.current !== work) return;
      const selection = selectSpeech(status, preferences, 'tts', locale);
      const chunks = speechChunks(text);
      if (!chunks.length) throw new Error(copy.noSpeech);
      // At most one look-ahead request; errors are settled immediately so a
      // cancelled prefetch cannot produce an unhandled rejection or stale audio.
      const prepare = (chunk: string) => api.synthesizeSpeech(chunk, work.controller.signal, selection)
        .then(blob => ({ blob, error: null as unknown }), error => ({ blob: null, error }));
      let pending = prepare(chunks[0]);
      for (let index = 0; index < chunks.length; index++) {
        if (work.controller.signal.aborted) break;
        setState('preparing');
        const result = await pending;
        if (result.error) throw result.error;
        if (current.current !== work) break;
        if (!result.blob) throw new Error(copy.speechFailed);
        work.url = URL.createObjectURL(result.blob);
        work.player.src = work.url;
        if (index + 1 < chunks.length) pending = prepare(chunks[index + 1]);
        await new Promise<void>((resolve, reject) => {
          const cleanup = () => { work.player.onended = null; work.player.onerror = null; work.controller.signal.removeEventListener('abort', aborted); };
          const ended = () => { cleanup(); resolve(); };
          const aborted = () => { cleanup(); reject(new DOMException('Playback stopped', 'AbortError')); };
          work.player.onended = ended;
          work.player.onerror = () => { cleanup(); reject(new Error(copy.speechFailed)); };
          work.controller.signal.addEventListener('abort', aborted, { once: true });
          void work.player.play().then(() => { if (current.current === work) setState('playing'); }).catch(reason => { cleanup(); reject(reason); });
        });
        if (work.url) { URL.revokeObjectURL(work.url); work.url = undefined; }
      }
    } catch (reason) {
      if (current.current === work && !work.controller.signal.aborted) setError(reason instanceof Error ? reason.message : copy.speechFailed);
    } finally {
      window.clearTimeout(timer);
      if (current.current === work) stop();
    }
  };
  return <span className="voice-input-control">
    <button type="button" className="message-copy voice-speak-button" onClick={() => void speak()} aria-label={state === 'idle' ? copy.speak : copy.stopSpeaking} aria-pressed={state !== 'idle'}>
      {state === 'preparing' ? copy.preparing : state === 'playing' ? copy.stopSpeaking : copy.speak}
    </button>
    {state === 'preparing' && <small role="status">{copy.preparingHint}</small>}
    {error && <small className="voice-error" role="alert">{error}</small>}
  </span>;
}
