import {useEffect, useRef, useState, type ComponentType} from 'react';
import {api} from '../api/client';
import {useI18n} from '../i18n';
import {voiceText} from '../i18n/voice';
import {selectSpeech, useVoicePreferences} from '../lib/voice-preferences';
import {VoiceCapture, type ContinuousCapturePhase, type ContinuousVoiceConfig, type VoiceCaptureEvent} from '../lib/voice-capture';
import {Icon} from './Icon';
import type {VoiceInputProps} from './VoiceInputButton';

export function ContinuousVoiceInput(props: VoiceInputProps & {continuous: ContinuousVoiceConfig; fallback: ComponentType<VoiceInputProps>}) {
  const {locale, messages} = useI18n(), {preferences} = useVoicePreferences();
  const copy = voiceText(locale);
  const latest = useRef(props); latest.current = props;
  const work = useRef<VoiceCapture | null>(null);
  const [phase, setPhase] = useState<ContinuousCapturePhase | 'checking'>('checking');
  const reportedPhase = useRef<ContinuousCapturePhase | 'checking'>('checking');
  const [fallback, setFallback] = useState(false);
  const fallbackOwner = useRef<VoiceCaptureEvent | undefined>(undefined);
  const fallbackID = useRef(crypto.randomUUID());
  const alive = useRef(false);
  const [fallbackCycle, setFallbackCycle] = useState(0);
  const fallbackTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const rearm = () => { clearTimeout(fallbackTimer.current); fallbackTimer.current = setTimeout(() => setFallbackCycle(value => value + 1), 750); };

  useEffect(() => {
    alive.current = true;
    let mounted = true;
    const pending = new AbortController();
    setFallback(false); setPhase('checking'); reportedPhase.current = 'checking';
    latest.current.onPhaseChange?.('checking');
    const dispose = () => { pending.abort(); clearTimeout(fallbackTimer.current); work.current?.close(); work.current = null; fallbackOwner.current = undefined; };
    const visibility = () => { if (document.hidden) dispose(); };
    window.addEventListener('offgrid-desktop-dismiss', dispose);
    document.addEventListener('visibilitychange', visibility);
    void (async () => {
      try {
        const status = await api.audioStatus(pending.signal);
        if (pending.signal.aborted || !mounted) return;
        const selection = selectSpeech(status, preferences, 'asr', locale);
        const capture = new VoiceCapture(props.continuous.sessionID, () => ({...latest.current.continuous, onCapability: value => {
          if (!mounted || pending.signal.aborted) return;
          latest.current.continuous.onCapability?.(value);
        }}),
          async (blob, signal) => (await api.transcribeAudio(blob, 'offgrid-recording.wav', signal, selection)).text,
          (next, level) => {
            if (!mounted || pending.signal.aborted) return;
            setPhase(next);
            // Match dictation's display scale so ordinary quiet speech visibly
            // responds. This affects the orb only, never detection thresholds.
            latest.current.onLevel?.(Math.min(1, Math.sqrt(Math.max(0, level)) * 2.5));
            if (reportedPhase.current !== next) {
              reportedPhase.current = next;
              latest.current.onPhaseChange?.(next === 'closed' ? 'idle' : next);
            }
          }, reason => {
            if (mounted && !pending.signal.aborted) latest.current.onFailure?.((reason as DOMException)?.name === 'NotAllowedError' ? copy.permission : copy.failed);
          });
        work.current = capture;
        const started = await capture.start();
        if (!started && mounted && !pending.signal.aborted && capture.unsupported) setFallback(true);
      } catch (reason) {
        if (mounted && !pending.signal.aborted) latest.current.onFailure?.(reason instanceof Error ? reason.message : copy.failed);
      }
    })();
    return () => { mounted = false; alive.current = false; dispose(); window.removeEventListener('offgrid-desktop-dismiss', dispose); document.removeEventListener('visibilitychange', visibility); };
  }, [props.continuous.sessionID, props.contextKey]);

  if (fallback) {
    const Manual = props.fallback;
    return <Manual key={fallbackCycle} {...props} continuous={undefined} autoStart={props.continuous.monitoring && props.continuous.fallbackListening !== false && !props.continuous.playbackActive}
      disabled={props.disabled || props.continuous.playbackActive || !props.continuous.monitoring || props.continuous.fallbackListening === false}
      onCaptureStart={() => {
        clearTimeout(fallbackTimer.current);
        const owner = {sessionID: props.continuous.sessionID, captureID: fallbackID.current, utteranceID: crypto.randomUUID()};
        if (!props.continuous.onSpeechStart(owner)) return false;
        fallbackOwner.current = owner; return true;
      }}
      onTranscript={text => {
        const owner = fallbackOwner.current; fallbackOwner.current = undefined;
        if (owner && owner.sessionID === latest.current.continuous.sessionID) void latest.current.continuous.onTranscript(text, owner).then(accepted => {
          if (!alive.current || owner.sessionID !== latest.current.continuous.sessionID) return;
          latest.current.continuous.onSettled?.(owner);
          if (accepted) {
            // Re-arm only when this fallback is still waiting for a recording.
            // A later explicit Speak again already starts its own recorder;
            // an old admission timer must not replace it mid-utterance.
            if (latest.current.continuous.fallbackListening !== false) rearm();
          } else latest.current.onFailure?.(copy.failed);
        }, () => { if (alive.current && owner.sessionID === latest.current.continuous.sessionID) latest.current.onFailure?.(copy.failed); });
      }} onSilence={() => {const owner = fallbackOwner.current; fallbackOwner.current = undefined; if (owner) props.continuous.onSettled?.(owner); rearm();}}/>;
  }
  return <span className="voice-input-control voice-input-compact" data-capture-phase={phase}>
    {phase === 'recording' && <>
      <button type="button" className="voice-button" title={copy.stop} aria-label={copy.stop} onClick={() => work.current?.finish()}><Icon name="check" size={18}/></button>
      <button type="button" className="voice-discard" title={messages.models.cancel} aria-label={messages.models.cancel} onClick={() => work.current?.discard()}><Icon name="trash" size={17}/></button>
    </>}
  </span>;
}
