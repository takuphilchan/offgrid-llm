import { useEffect, useId, useRef, useState } from 'react';
import { api, type AudioStatus } from '../api/client';
import { useVoicePreferences } from '../lib/voice-preferences';

export function VoiceSettings() {
  const id = useId();
  const { preferences, setPreferences } = useVoicePreferences();
  const [status, setStatus] = useState<AudioStatus>();
  const [error, setError] = useState('');
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  const refresh = async () => {
    request.current?.abort();
    const controller = new AbortController(); request.current = controller;
    setError('');
    try { const value = await api.audioStatus(controller.signal); if (!controller.signal.aborted) setStatus(value); }
    catch (reason) { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : 'Speech models are unavailable.'); }
  };
  const tts = status?.profiles?.find(p => `${p.id}@${p.revision}` === preferences.tts);
  return <details className="voice-settings" onToggle={event => { if (event.currentTarget.open) void refresh(); }}>
    <summary>Voice settings</summary>
    <div className="voice-settings-panel">
      {(['asr', 'tts'] as const).map(kind => {
        const profiles = status?.profiles?.filter(p => p.capabilities.includes(kind === 'asr' ? 'transcription' : 'speech_synthesis')) ?? [];
        const selected = profiles.find(p => `${p.id}@${p.revision}` === preferences[kind]);
        return <div className="field" key={kind}><label htmlFor={`${id}-${kind}`}>{kind === 'asr' ? 'Recognition model' : 'Speech model'}</label>
          <select id={`${id}-${kind}`} value={preferences[kind]} onChange={e => setPreferences({ ...preferences, [kind]: e.target.value, ...(kind === 'tts' ? { voice: '' } : {}) })}>
            <option value="">Automatic{profiles.find(p => p.available) ? ` · ${profiles.find(p => p.available)!.name}` : ''}</option>
            {!!preferences[kind] && !selected && <option value={preferences[kind]} disabled>Selected model unavailable</option>}
            {profiles.map(p => <option key={`${p.id}@${p.revision}`} value={`${p.id}@${p.revision}`} disabled={!p.available}>{p.name}{!p.available ? ` · ${p.issue || 'Unavailable'}` : ''}</option>)}
          </select>
        </div>;
      })}
      {!!tts?.voices?.length && <div className="field"><label htmlFor={`${id}-voice`}>Voice</label><select id={`${id}-voice`} value={preferences.voice} onChange={e => setPreferences({ ...preferences, voice: e.target.value })}>
        <option value="">Default voice</option>{tts.voices.map(voice => <option key={voice.id} value={voice.id}>{voice.id} · {voice.language}</option>)}
      </select></div>}
      <small>Saved for this account in this browser. Changes apply to the next recording or playback.</small>
      {error && <small role="alert">{error}</small>}
      <a href="#/models">Manage speech models</a>
    </div>
  </details>;
}
