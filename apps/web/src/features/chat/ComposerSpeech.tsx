import { VoiceSettings } from '../../components/VoiceSettings';
import { useI18n } from '../../i18n';
import { voiceText } from '../../i18n/voice';
import type { SpeechState } from '../../lib/response-speech';

export function ComposerSpeech({ enabled, checking, busy, state, error, toggle, stop, presentation = 'all' }: {
  enabled: boolean; checking: boolean; busy: boolean; state: SpeechState; error: string;
  toggle: (enabled: boolean) => Promise<void>; stop: () => void;
  presentation?: 'all' | 'settings' | 'status';
}) {
  const { locale } = useI18n(), copy = voiceText(locale);
  if (presentation === 'status' && !enabled && state === 'idle' && !error) return null;
  return <div className="composer-voice-toolbar chat-speech-controls">
    {presentation !== 'status' && <VoiceSettings subtle inline={presentation === 'settings'}><label className="switch"><input type="checkbox" checked={enabled} disabled={checking || (busy && !enabled)} onChange={event => void toggle(event.target.checked)} /><span />{copy.speakResponses}</label></VoiceSettings>}
    {presentation !== 'settings' && <>{enabled && <small>{copy.speakResponses}</small>}
      {state !== 'idle' && <button type="button" className="text-button" onClick={stop}>{copy.stopSpeaking}</button>}
      {state !== 'idle' && <small role="status">{state === 'waiting' ? copy.speechWaiting : state === 'preparing' ? copy.preparingHint : busy ? copy.provisionalSpeech : copy.speak}</small>}
      {error && <small role="alert">{error}</small>}</>}
  </div>;
}
