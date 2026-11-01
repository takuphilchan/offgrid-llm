import { VoiceSettings } from '../../components/VoiceSettings';
import { useI18n } from '../../i18n';
import { voiceText } from '../../i18n/voice';
import type { SpeechState } from '../../lib/response-speech';

export function ComposerSpeech({ enabled, checking, busy, state, error, toggle, stop }: {
  enabled: boolean; checking: boolean; busy: boolean; state: SpeechState; error: string;
  toggle: (enabled: boolean) => Promise<void>; stop: () => void;
}) {
  const { locale, messages } = useI18n(), copy = voiceText(locale);
  return <div className="composer-voice-toolbar chat-speech-controls">
    <VoiceSettings subtle><label className="switch"><input type="checkbox" checked={enabled} disabled={checking || (busy && !enabled)} onChange={event => void toggle(event.target.checked)} /><span />{copy.speakResponses}</label></VoiceSettings>
    {enabled && <small>{copy.speakResponses}</small>}
    {state !== 'idle' && <button type="button" className="text-button" onClick={stop}>{copy.stopSpeaking}</button>}
    {state !== 'idle' && <small role="status">{state === 'waiting' ? copy.speechWaiting : state === 'preparing' ? copy.preparingHint : busy ? copy.provisionalSpeech : copy.speak}</small>}
    {error && <small role="alert">{error} <a className="secondary-button" href="#/models">{messages.nav.models}</a></small>}
  </div>;
}
