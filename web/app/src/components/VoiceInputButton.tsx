import { useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
import { Icon } from './Icon';
import { useI18n } from '../i18n';
import { voiceText } from '../i18n/voice';
import { spokenProse, stopSpeechEvent } from '../lib/response-speech';
import { selectSpeech, useVoicePreferences } from '../lib/voice-preferences';

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

type RecordingWork = { controller: AbortController; stream?: MediaStream; recorder?: MediaRecorder; timer?: number };

export function VoiceInputButton({ onTranscript, contextKey, disabled = false }: { onTranscript: (text: string) => void; contextKey: string; disabled?: boolean }) {
  const { preferences } = useVoicePreferences();
  const { locale } = useI18n();
  const copy = voiceText(locale);
  const [recording, setRecording] = useState(false);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState('');
  const current = useRef<RecordingWork | null>(null);
  // Use the current draft callback, but never deliver a result to another composer.
  const target = useRef({ contextKey, onTranscript });
  target.current = { contextKey, onTranscript };
  const cancel = () => {
    const work = current.current;
    current.current = null;
    if (!work) return;
    work.controller.abort();
    clearTimeout(work.timer);
    if (work.recorder?.state === 'recording') work.recorder.stop();
    work.stream?.getTracks().forEach(track => track.stop());
  };
  useEffect(() => {
    setRecording(false); setWorking(false); setError('');
    return cancel;
  }, [contextKey]);
  const stop = () => {
    const work = current.current;
    if (work?.recorder?.state === 'recording') {
      clearTimeout(work.timer);
      work.recorder.stop();
      work.stream?.getTracks().forEach(track => track.stop());
      setRecording(false);
    } else { cancel(); setWorking(false); setRecording(false); }
  };

  const start = async () => {
    if (current.current || disabled) return;
    setError('');
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === 'undefined') {
      setError(copy.unsupported);
      return;
    }
    const work: RecordingWork = { controller: new AbortController() };
    current.current = work;
    setWorking(true);
    const active = () => current.current === work && !work.controller.signal.aborted && target.current.contextKey === contextKey;
    try {
      // Check the service before requesting a device permission. This prevents
      // a confusing microphone prompt when no local ASR runtime is installed.
      const status = await api.audioStatus(work.controller.signal);
      if (!active()) return;
      const selection = selectSpeech(status, preferences, 'asr');
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
        if (bytes > 12 * 1024 * 1024) { cancel(); setRecording(false); setWorking(false); setError(copy.failed); return; }
        chunks.push(event.data);
      };
      next.onerror = () => { if (active()) { cancel(); setError(copy.failed); setRecording(false); setWorking(false); } };
      next.onstop = async () => {
        clearTimeout(work.timer);
        work.stream?.getTracks().forEach(track => track.stop());
        if (!active()) return;
        setRecording(false);
        setWorking(true);
        try {
          const blob = new Blob(chunks, { type: next.mimeType || 'audio/webm' });
          if (!blob.size) return;
          const wav = await toWav(blob);
          if (!active()) return;
          const result = await api.transcribeAudio(wav, 'offgrid-recording.wav', work.controller.signal, selection);
          if (active() && result.text.trim()) target.current.onTranscript(result.text.trim());
        } catch (reason) {
          if (active()) setError(reason instanceof Error ? reason.message : copy.failed);
        } finally { if (active()) { current.current = null; setWorking(false); } }
      };
      next.start(1000);
      work.timer = window.setTimeout(() => { if (active()) stop(); }, 120_000);
      setWorking(false);
      setRecording(true);
    } catch (reason) {
      if (!active()) return;
      cancel(); setWorking(false); setRecording(false);
      if ((reason as DOMException)?.name === 'NotAllowedError') setError(copy.permission);
      else setError(reason instanceof Error ? reason.message : copy.failed);
    }
  };

  return <span className="voice-input-control">
    <button type="button" className={recording ? 'voice-button recording' : 'voice-button'} disabled={disabled && !current.current} onClick={current.current ? stop : () => void start()} aria-label={recording || working ? copy.stop : copy.record} aria-pressed={recording} title={recording || working ? copy.stop : copy.record}>
      <Icon name="mic" size={18} />
      <span>{working ? copy.transcribing : recording ? copy.recording : copy.record}</span>
    </button>
    {error && <small className="voice-error" role="status">{error}</small>}
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
      const selection = selectSpeech(status, preferences, 'tts');
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
