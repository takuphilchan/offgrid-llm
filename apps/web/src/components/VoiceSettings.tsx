import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { api, type AudioStatus } from '../api/client';
import { useVoicePreferences } from '../lib/voice-preferences';
import { UtilityPopover } from './UtilityPopover';
import { useI18n } from '../i18n';
import { voiceSettingsText } from '../i18n/voice-settings';
import { Icon } from './Icon';
import { SpeechModelChoice } from './SpeechModelChoice';
import { voiceSetupText } from '../i18n/voice-setup';
import { voiceSetupActions } from '../i18n/voice-setup';
import { useWorkspaceState } from '../lib/workspace-context';
import type { ModelCategory } from '../api/client';

export function VoiceSettings({ children, compact = false, subtle = false, inline = false }: { children?: ReactNode; compact?: boolean; subtle?: boolean; inline?: boolean }) {
  const id = useId();
  const { locale, messages } = useI18n(), copy = voiceSettingsText(locale);
  const { preferences, setPreferences } = useVoicePreferences();
  const [, setCategory] = useWorkspaceState<ModelCategory>('models.category', 'language');
  const [, setView] = useWorkspaceState<'installed' | 'discover'>('models.view', 'installed');
  const setup = voiceSetupActions(locale);
  const [status, setStatus] = useState<AudioStatus>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  const refresh = async () => {
    request.current?.abort();
    const controller = new AbortController(); request.current = controller;
    setError(''); setLoading(true);
    try { const value = await api.audioStatus(controller.signal); if (!controller.signal.aborted) setStatus(value); }
    catch (reason) { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : copy.failed); }
    finally { if (!controller.signal.aborted) setLoading(false); }
  };
  // Inline preferences are mounted only while their containing utility is open.
  // Disposal cancels metadata reads just as dismissing the standalone menu does.
  useEffect(() => { if (inline) void refresh(); }, [inline]);
  const tts = status?.profiles?.find(p => preferences.tts ? `${p.id}@${p.revision}` === preferences.tts : p.available && p.capabilities.includes('speech_synthesis'));
  const content = <>
      {children}
      {status?.profiles?.some(profile => !profile.available) && <p className="model-search-hint">{voiceSetupText(locale)}</p>}
      {(['asr', 'tts'] as const).map(kind => {
        const profiles = status?.profiles?.filter(p => p.capabilities.includes(kind === 'asr' ? 'transcription' : 'speech_synthesis')) ?? [];
        return <section key={kind}>
          {status && !profiles.length && !status[kind]?.available ? <div className="field"><strong>{kind === 'asr' ? copy.recognition : copy.synthesis}</strong><p className="model-search-hint">{setup.missing}</p></div> : <SpeechModelChoice label={kind === 'asr' ? copy.recognition : copy.synthesis} profiles={profiles} value={preferences[kind]} disabled={loading || !!error}
          onChange={value => setPreferences({ ...preferences, [kind]: value, ...(kind === 'tts' ? { voice: '' } : {}) })} />}
          {status && !profiles.some(profile => profile.available) && !status[kind]?.available && <a className="text-button" href="#/models" onClick={() => { setCategory(kind === 'asr' ? 'speech_recognition' : 'speech_generation'); setView(profiles.length ? 'installed' : 'discover'); }}>{kind === 'asr' ? setup.recognition : setup.synthesis}</a>}
        </section>;
      })}
      {!!tts?.voices?.length && <div className="field"><label htmlFor={`${id}-voice`}>{copy.voice}</label><select id={`${id}-voice`} disabled={loading || !!error} value={preferences.voice} onChange={e => setPreferences({ ...preferences, voice: e.target.value })}>
        <option value="">{copy.defaultVoice}</option>{preferences.voice && !tts.voices.some(voice => voice.id === preferences.voice) && <option value={preferences.voice} disabled>{copy.voiceUnavailable}</option>}{tts.voices.map(voice => <option key={voice.id} value={voice.id}>{voice.id} · {voice.language}</option>)}
      </select></div>}
      <small>{copy.retention}</small>
      {loading && <small role="status">{messages.common.loading}</small>}
      {error && <div role="alert"><small>{error}</small><button className="secondary-button" onClick={() => void refresh()}>{messages.common.retry}</button></div>}
      <a className="secondary-button" href="#/models">{copy.manage}</a>
  </>;
  const changed = (expanded: boolean) => { if (expanded) void refresh(); else request.current?.abort(); };
  if (inline) return <section className="voice-settings-inline" aria-label={copy.title}><h3>{copy.title}</h3><div className="voice-settings-panel">{content}</div></section>;
  return <UtilityPopover label={copy.title} trigger={compact ? <Icon name="speaker" size={18} /> : undefined} triggerClassName={compact ? 'icon-button' : subtle ? 'utility-button' : 'text-button'} className="voice-settings" panelClassName="voice-settings-panel" onOpenChange={changed}>{content}</UtilityPopover>;
}
