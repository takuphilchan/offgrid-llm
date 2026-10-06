import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { api, type AudioStatus } from '../api/client';
import { useVoicePreferences } from '../lib/voice-preferences';

export function VoiceSettings() {
  const id = useId();
  const { preferences, setPreferences } = useVoicePreferences();
  const [status, setStatus] = useState<AudioStatus>();
  const [error, setError] = useState('');
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  useLayoutEffect(() => {
    if (!open) return;
    const position = () => {
      if (!trigger.current || !panel.current) return;
      const anchor = trigger.current.getBoundingClientRect();
      const width = Math.min(340, window.innerWidth - 32);
      const above = anchor.top - 24;
      const below = window.innerHeight - anchor.bottom - 24;
      const upwards = above >= Math.min(panel.current.scrollHeight, 360) || above > below;
      const start = getComputedStyle(trigger.current).direction === 'rtl' ? anchor.right - width : anchor.left;
      Object.assign(panel.current.style, {
        width: `${width}px`, left: `${Math.max(16, Math.min(start, window.innerWidth - width - 16))}px`,
        top: upwards ? 'auto' : `${anchor.bottom + 8}px`,
        bottom: upwards ? `${window.innerHeight - anchor.top + 8}px` : 'auto',
        maxHeight: `${Math.max(80, upwards ? above : below)}px`,
      });
    };
    position();
    window.addEventListener('resize', position);
    window.addEventListener('scroll', position, true);
    return () => { window.removeEventListener('resize', position); window.removeEventListener('scroll', position, true); };
  }, [open, status]);
  const refresh = async () => {
    request.current?.abort();
    const controller = new AbortController(); request.current = controller;
    setError('');
    try { const value = await api.audioStatus(controller.signal); if (!controller.signal.aborted) setStatus(value); }
    catch (reason) { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : 'Speech models are unavailable.'); }
  };
  const tts = status?.profiles?.find(p => `${p.id}@${p.revision}` === preferences.tts);
  return <div className="voice-settings">
    <button ref={trigger} type="button" className="text-button" popoverTarget={`${id}-panel`} aria-expanded={open} aria-controls={`${id}-panel`}>Voice settings</button>
    <div ref={panel} id={`${id}-panel`} className="voice-settings-panel" popover="auto" role="region" aria-label="Voice settings" onToggle={event => {
      const expanded = event.newState === 'open';
      setOpen(expanded);
      if (expanded) void refresh(); else request.current?.abort();
    }}>
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
  </div>;
}
