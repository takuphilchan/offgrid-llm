import { useEffect, useRef, useState } from 'react';
import { api, type AgentProfile, type AgentResourceRestrictions } from '../../api/client';
import { UtilityPopover } from '../../components/UtilityPopover';
import { useI18n } from '../../i18n';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';

export function AgentSelector({ value, onChange, disabled }: { value: AgentProfile | null; onChange: (profile: AgentProfile | null) => void; disabled: boolean }) {
  const { locale, messages } = useI18n(), copy = agentProfilesText[locale];
  const [profiles, setProfiles] = useState<AgentProfile[]>([]), [supported, setSupported] = useState(false);
  const [error, setError] = useState(''), [loading, setLoading] = useState(true);
  const generation = useRef(0);
  const refresh = async () => {
    const current = ++generation.current; setLoading(true);
    try {
      const identity = await api.systemIdentity();
      if (current !== generation.current) return;
      const available = identity.capabilities?.includes('agent-conversations-v1') === true;
      setSupported(available);
      if (!available) { setProfiles([]); return; }
      const next = await api.agentProfiles();
      if (current === generation.current) { setProfiles(next); setError(''); }
    } catch (reason) { if (current === generation.current) setError(reason instanceof Error ? reason.message : messages.common.error); }
    finally { if (current === generation.current) setLoading(false); }
  };
  useEffect(() => { void refresh(); return () => { generation.current++; }; }, []);
  useWorkspaceRefresh(refresh);
  return <UtilityPopover label={copy.selector} trigger={<>{copy.selector}<span className="composer-choice">{value?.settings.name ?? copy.plain}</span></>} triggerClassName="secondary-button" className="chat-agent-selector" onOpenChange={open => { if (open) void refresh(); }}>
    <fieldset disabled={disabled}><legend>{copy.selector}</legend>
      <label className="profile-option"><input type="radio" name="chat-agent" checked={!value} onChange={() => onChange(null)} /><span>{copy.plain}</span></label>
      {profiles.map(profile => <label className="profile-option" key={profile.id}><input type="radio" name="chat-agent" checked={value?.id === profile.id && value.revision === profile.revision} onChange={() => onChange(profile)} /><span>{profile.settings.name}<small>{profile.settings.description || profile.settings.model}</small><small>{copy.revision.replace('{revision}', String(profile.revision))}</small></span></label>)}
      {value && !profiles.some(profile => profile.id === value.id && profile.revision === value.revision) && <p role="status">{value.settings.name} · {copy.revision.replace('{revision}', String(value.revision))} · {copy.review}</p>}
    </fieldset>
    <p>{copy.future}</p>
    {loading && <p role="status">{messages.common.loading}</p>}
    {!loading && !supported && <p role="status">{copy.unsupported}</p>}
    {error && <p role="alert">{error}</p>}
    <a className="secondary-button" href="#/agents/profiles">{copy.profiles}</a>
  </UtilityPopover>;
}

export function AgentTools({ profile, restrictions, onChange, disabled }: { profile: AgentProfile; restrictions: AgentResourceRestrictions; onChange: (value: AgentResourceRestrictions) => void; disabled: boolean }) {
  const { locale } = useI18n(), copy = agentProfilesText[locale];
  const tools = restrictions.tools ?? profile.settings.tools ?? [];
  const knowledge = restrictions.knowledge ?? profile.settings.knowledge ?? [];
  const [documentNames, setDocumentNames] = useState<Record<string, string>>({});
  useEffect(() => {
    let current = true;
    setDocumentNames({});
    if (profile.settings.knowledge?.length) void api.documents().then(result => { if (current) setDocumentNames(Object.fromEntries(result.documents.map(doc => [doc.id, doc.name]))); }).catch(() => {});
    return () => { current = false; };
  }, [profile.id, profile.revision]);
  return <UtilityPopover label={copy.resources} trigger={<>{copy.tools}<span className="composer-choice">{tools.length}</span></>} triggerClassName="secondary-button" className="chat-agent-tools">
    <p>{copy.future}</p><fieldset disabled={disabled}><legend>{copy.resources}</legend>
      {profile.settings.computer && <label className="profile-option"><input type="checkbox" checked={restrictions.computer !== false} onChange={event => onChange({ ...restrictions, computer: event.target.checked })} /><span>{copy.computer}<small>{copy.computerHint}</small></span></label>}
      {(profile.settings.tools ?? []).map(tool => <label className="profile-option" key={`${tool.provider}:${tool.tool}`}><input type="checkbox" checked={tools.some(value => value.provider === tool.provider && value.tool === tool.tool)} onChange={event => onChange({ ...restrictions, tools: event.target.checked ? [...tools, tool] : tools.filter(value => value.tool !== tool.tool || value.provider !== tool.provider) })} /><span>{tool.tool}<small>{tool.provider} · {tool.location}</small></span></label>)}
      {!!profile.settings.knowledge?.length && <details><summary>{copy.knowledge} · {knowledge.length}</summary>{profile.settings.knowledge.map(id => <label className="profile-option" key={id}><input type="checkbox" checked={knowledge.includes(id)} onChange={event => onChange({ ...restrictions, knowledge: event.target.checked ? [...knowledge, id] : knowledge.filter(value => value !== id) })} /><span>{documentNames[id] ?? id}</span></label>)}</details>}
    </fieldset><p>{copy.workspaceConnections}</p><a className="secondary-button" href="#/agents/profiles">{copy.edit}</a>
  </UtilityPopover>;
}
