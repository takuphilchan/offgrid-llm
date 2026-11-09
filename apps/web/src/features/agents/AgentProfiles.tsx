import { useEffect, useRef, useState } from 'react';
import { APIError, api, type AgentProfile, type AgentProfileSettings, type AgentCapabilityIdentity, type EffectiveAgentProfile, type Model } from '../../api/client';
import { ModelSelect } from '../../components/ModelSelect';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { SearchField } from '../../components/SearchField';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { modelProviderDetails } from '../../i18n/model-library';
import { agentProfilesText, profileSelectionPrompt } from '../../i18n/agent-profiles';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { EmptyState } from '../../components/WorkspacePresentation';
import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import './agent-profiles.css';
import { ExecutionPolicy } from './ExecutionPolicy';
import { AgentContextResources } from './AgentContextResources';
import { AgentPurpose } from './AgentPurpose';
import { ProfileCapability } from './ProfileCapability';
import { ProfileResources } from './ProfileResources';
import { supportsChat } from '../../api/model-capabilities';
import { useProfileResources } from './useProfileResources';

type Draft = { original: AgentProfile | null; settings: AgentProfileSettings; initial?: AgentProfileSettings };
const fresh = (model = ''): Draft => {
  const settings = { name: '', model, max_iterations: 10, tools: [], knowledge: [], delegates: [], computer: false };
  return { original: null, settings, initial: settings };
};
const toolKey = (tool: NonNullable<AgentProfileSettings['tools']>[number]) => JSON.stringify([tool.provider, tool.tool, tool.schema_revision, tool.location]);
const reference = (tool: AgentCapabilityIdentity): NonNullable<AgentProfileSettings['tools']>[number] => ({ provider: tool.source, tool: tool.name, schema_revision: tool.revision, location: tool.location });

export function AgentProfiles({ models, preferredModel = '' }: { models: Model[]; preferredModel?: string }) {
  const { locale, messages } = useI18n(), copy = agentProfilesText[locale];
  const { knowledge: knowledgePermitted,scope,workspace } = useWorkspace();
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const resourceState = useProfileResources(knowledgePermitted, messages.common.error);
  const { resources, documents } = resourceState;
  const [draft, setDraft] = useWorkspaceState<Draft | null>('agent.profileDraft', null);
  const [chatAgent, setChatAgent] = useWorkspaceState<AgentProfile | null>('chat.agent', null);
  const [query, setQuery] = useWorkspaceState('agent.profileQuery', '');
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState(false);
  const [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [issues, setIssues] = useState<EffectiveAgentProfile['issues']>([]);
  const [conflict, setConflict] = useState(false);
  const [discardTo, setDiscardTo] = useState<Draft | null>(null);
  const [archiving, setArchiving] = useState<AgentProfile | null>(null);
  const generation = useRef(0), lock = useRef(false), heading = useRef<HTMLHeadingElement>(null);
  const dirty = !!draft && JSON.stringify(draft.settings) !== JSON.stringify(draft.original?.settings ?? draft.initial ?? fresh().settings);
  const eligible = models.filter(model => supportsChat(model) && model.runtime_ready !== false);
  // Suggest only at explicit new-draft creation. Reloads, model inventory changes
  // and saved-profile edits never replace the user's choice or load a model.
  const suggestedModel = eligible.find(model => model.id === preferredModel)?.id || eligible.find(model => model.loaded)?.id || eligible[0]?.id || '';
  const refresh = async () => {
    const current = ++generation.current;
    setLoading(true);
    const [result] = await Promise.allSettled([api.agentProfiles(), resourceState.refresh()]);
    if (current !== generation.current) return;
    if (result.status === 'fulfilled') setProfiles(result.value);
    setError(result.status === 'rejected' ? (result.reason instanceof Error ? result.reason.message : messages.common.error) : '');
    setLoading(false);
  };
  useEffect(() => { void refresh(); return () => { generation.current++; }; }, [knowledgePermitted]);
  useWorkspaceRefresh(refresh);
  useEffect(() => {
    if (!dirty) return;
    const beforeUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener('beforeunload', beforeUnload);
    return () => window.removeEventListener('beforeunload', beforeUnload);
  }, [dirty]);
  useEffect(() => {
    setIssues([]); setConflict(false);
    if (!draft?.original) return;
    const check = new AbortController();
    void api.effectiveAgentProfile(draft.original.id, draft.original.revision, check.signal)
      .then(result => { if (!check.signal.aborted) setIssues(result.issues); })
      .catch(reason => { if (!check.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); });
    return () => check.abort();
  }, [draft?.original?.id, draft?.original?.revision]);
  const choose = (next: Draft) => {
    if (busy) return;
    if (dirty) { setDiscardTo(next); return; }
    setDraft(next); setNotice(''); setError(''); setConflict(false);
    requestAnimationFrame(() => heading.current?.focus());
  };
  const update = (patch: Partial<AgentProfileSettings>) => setDraft(current => current && ({ ...current, settings: { ...current.settings, ...patch } }));
  const toggle = (field: 'knowledge' | 'delegates', id: string, enabled: boolean) => {
    const current = draft?.settings[field] ?? [];
    update({ [field]: enabled ? [...current, id] : current.filter(value => value !== id) });
  };
  const save = async (asCopy = false) => {
    if (!draft || lock.current) return;
    lock.current = true; setBusy(true); setError(''); setNotice(''); setIssues([]);
    try {
      const saved = draft.original && !asCopy
        ? await api.updateAgentProfile(draft.original.id, draft.original.revision, draft.settings)
        : await api.createAgentProfile(draft.settings);
      setDraft({ original: saved, settings: saved.settings }); setConflict(false); setNotice(copy.saved);
      setProfiles(current => [saved, ...current.filter(item => item.id !== saved.id)]);
      // Saving a new revision of the profile already selected in Chat should
      // update the next turn's selection. Existing jobs keep their pinned model.
      if (chatAgent?.id === saved.id) setChatAgent(saved);
    } catch (reason) {
      if (reason instanceof APIError && reason.status === 409) { setConflict(true); setError(copy.conflict); }
      else if (reason instanceof APIError && Array.isArray(reason.data?.issues)) { setIssues(reason.data.issues); setError(copy.review); }
      else setError(reason instanceof Error ? reason.message : messages.common.error);
    } finally { lock.current = false; setBusy(false); }
  };
  const loadLatest = async () => {
    if (!draft?.original || busy) return;
    setBusy(true);
    try { const latest = await api.agentProfile(draft.original.id); setDiscardTo({ original: latest, settings: latest.settings }); }
    catch (reason) { setError(reason instanceof Error ? reason.message : messages.common.error); }
    finally { setBusy(false); }
  };
  const selectedTools = draft?.settings.tools ?? [];
  const availableTools = Object.values(resources?.tools ?? {}).sort((a, b) => a.name.localeCompare(b.name));
  const unavailableTools = selectedTools.filter(tool => !availableTools.some(value => toolKey(reference(value)) === toolKey(tool)));
  const modelMissing = !!draft?.settings.model && !models.some(item => item.id === draft.settings.model);
  const selectedModel = models.find(item => item.id === draft?.settings.model);
  const providerCopy = modelProviderDetails(locale);
  const visibleProfiles = profiles.filter(item => `${item.settings.name} ${item.settings.description ?? ''}`.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  return <section className="agent-profiles" aria-label={copy.profiles}>
    <header className="profile-library-header"><div><h2>{copy.profiles}</h2><p>{messages.guidance.profiles}</p></div><button type="button" className="secondary-button library-create-action" disabled={busy} onClick={() => choose(fresh(suggestedModel))}><Icon name="plus" size={17}/>{copy.create}</button></header>
    {error && <div className="inline-error" role="alert"><p>{error}</p>{!conflict && <button type="button" className="secondary-button" onClick={() => void refresh()} disabled={loading || busy}>{messages.common.refresh}</button>}</div>}
    {notice && <p role="status">{notice}</p>}
    <div className={`profile-library-layout${profiles.length ? '' : ' profile-library-empty'}`}>
      {(profiles.length > 0 || loading) && <aside className="profile-library-list"><SearchField id="agent-profile-search" className="profile-search" label={copy.search} value={query} onChange={setQuery} />
        {loading && <p role="status">{messages.common.loading}</p>}
        {profiles.length > 0 && !visibleProfiles.length && <EmptyState title={messages.history.noMatches} action={<button type="button" className="text-button" onClick={() => { setQuery(''); document.getElementById('agent-profile-search')?.focus(); }}>{workspaceRefinement[locale].clear}</button>} />}
        <ul>{visibleProfiles.map(item => <li key={item.id}>
          <button type="button" className="profile-list-item" aria-current={draft?.original?.id === item.id ? 'true' : undefined} disabled={busy} onClick={() => choose({ original: item, settings: item.settings })}><strong>{item.settings.name}</strong><span>{item.settings.description || item.settings.model}</span></button>
        </li>)}</ul>
      </aside>}
      {draft ? <form className="profile-editor" onSubmit={event => { event.preventDefault(); void save(); }}>
        <header><h3 ref={heading} tabIndex={-1}>{draft.original ? copy.edit : copy.create}</h3>{draft.original && <small>{copy.revision.replace('{revision}', String(draft.original.revision))}</small>}</header>
        {issues.length > 0 && <section className="profile-issues" aria-label={copy.review}><h4>{copy.review}</h4><ul>{issues.map((issue, index) => <li key={`${issue.code}:${issue.reference}:${index}`}><strong>{issue.reference}</strong>: {issue.message}</li>)}</ul></section>}
        {conflict && <div className="profile-actions"><button type="button" className="secondary-button" disabled={busy} onClick={() => void loadLatest()}>{copy.loadLatest}</button><button type="button" className="secondary-button" disabled={busy} onClick={() => void save(true)}>{copy.saveCopy}</button></div>}
        <fieldset disabled={busy} className="profile-fields">
          <div className="profile-basics">
          <div className="profile-identity">
          <label>{copy.name}<input required value={draft.settings.name} maxLength={128} onChange={event => update({ name: event.target.value })} /></label>
          </div>
          <AgentPurpose value={draft.settings.instructions ?? ''} onChange={instructions => update({ instructions })} />
          <ModelSelect models={models} value={draft.settings.model} onChange={model => update({ model })} />
          {modelMissing && <p role="status">{draft.settings.model}: {copy.modelMissing}</p>}
          </div>
          <ProfileResources key={`${draft.original?.id ?? 'new'}-resources`} title={copy.resources} onOpen={() => void resourceState.load()} count={selectedTools.length + (draft.settings.knowledge?.length ?? 0) + (draft.settings.skills?.length ?? 0) + (draft.settings.delegates?.length ?? 0) + Number(!!draft.settings.computer)}>
            {resourceState.loading && <p role="status">{messages.common.loading}</p>}
            {resourceState.error && <div className="inline-error" role="alert"><p>{resourceState.error}</p><button type="button" className="text-button" disabled={resourceState.loading} onClick={() => void resourceState.load()}>{messages.common.retry}</button></div>}
            <label className="profile-option"><input type="checkbox" checked={draft.settings.computer ?? false} disabled={!resources?.computer_permitted && !draft.settings.computer} onChange={event => update({ computer: event.target.checked })} /><span>{copy.computer}<small>{copy.computerHint}</small></span></label>
            <ProfileCapability key={`${draft.original?.id ?? 'new'}-tools`} title={copy.tools} count={selectedTools.length}><p className="profile-hint">{copy.workspaceConnections}</p><div className="profile-options">
              {availableTools.map(tool => { const ref = reference(tool), key = toolKey(ref); return <label className="profile-option" key={key}><input type="checkbox" checked={selectedTools.some(value => toolKey(value) === key)} onChange={event => update({ tools: event.target.checked ? [...selectedTools, ref] : selectedTools.filter(value => toolKey(value) !== key) })} /><span>{tool.name}<small>{tool.description}</small><small>{tool.source} · {tool.location} · {tool.kind}</small></span></label>; })}
              {unavailableTools.map(tool => <label className="profile-option" key={toolKey(tool)}><input type="checkbox" checked onChange={() => update({ tools: selectedTools.filter(value => toolKey(value) !== toolKey(tool)) })} /><span>{tool.tool}<small>{copy.unavailable}</small></span></label>)}
              {!availableTools.length && !unavailableTools.length && <p>{copy.noResources}</p>}
            </div></ProfileCapability>
            <ProfileCapability key={`${draft.original?.id ?? 'new'}-knowledge`} title={copy.knowledge} count={draft.settings.knowledge?.length ?? 0}><div className="profile-options">
              {documents.map(doc => <label className="profile-option" key={doc.id}><input type="checkbox" checked={draft.settings.knowledge?.includes(doc.id) ?? false} onChange={event => toggle('knowledge', doc.id, event.target.checked)} /><span>{doc.name}</span></label>)}
              {(draft.settings.knowledge ?? []).filter(id => !documents.some(doc => doc.id === id)).map(id => <label className="profile-option" key={id}><input type="checkbox" checked onChange={() => toggle('knowledge', id, false)} /><span>{id}<small>{copy.unavailable}</small></span></label>)}
              {!documents.length && !draft.settings.knowledge?.length && <p>{copy.noResources}</p>}
            </div></ProfileCapability>
            <AgentContextResources key={JSON.stringify([scope,workspace,draft.original?.id??'new'])} profile={draft.original?.id} selected={draft.settings.skills??[]} onChange={skills=>update({skills})} disabled={busy}/>
            <ProfileCapability key={`${draft.original?.id ?? 'new'}-delegates`} title={copy.delegates} count={draft.settings.delegates?.length ?? 0}><p className="profile-hint">{copy.delegateHint}</p><div className="profile-options">
              {profiles.filter(item => item.id !== draft.original?.id && !item.settings.delegates?.length).map(item => { const selected = draft.settings.delegates?.includes(item.id) ?? false; return <label className="profile-option" key={item.id}><input type="checkbox" checked={selected} disabled={!selected && (draft.settings.delegates?.length ?? 0) >= 4} onChange={event => toggle('delegates', item.id, event.target.checked)} /><span>{item.settings.name}</span></label>; })}
              {(draft.settings.delegates ?? []).filter(id => !profiles.some(item => item.id === id && !item.settings.delegates?.length)).map(id => <label className="profile-option" key={id}><input type="checkbox" checked onChange={() => toggle('delegates', id, false)} /><span>{id}<small>{copy.unavailable}</small></span></label>)}
            </div></ProfileCapability>
          </ProfileResources>
          <details className="profile-advanced"><summary>{copy.advanced}</summary>
            <label className="profile-description">{copy.description}<input value={draft.settings.description ?? ''} maxLength={512} onChange={event => update({ description: event.target.value })} /></label>
            <label>{copy.iterations}<input type="number" required min={1} max={50} value={draft.settings.max_iterations} onChange={event => update({ max_iterations: event.target.valueAsNumber || 1 })} /></label>
            {selectedModel && <p className="profile-hint">{[selectedModel.engine || 'llama_cpp', selectedModel.format, selectedModel.backend].filter(Boolean).join(' · ')}{selectedModel.context_window ? ` · ${providerCopy.context}: ${selectedModel.context_window.toLocaleString(locale)}` : ''} · {providerCopy.qualified}: {selectedModel.qualified ? providerCopy.yes : providerCopy.unmeasured}</p>}
            <ExecutionPolicy value={draft.settings.execution} onChange={execution=>update({execution})} />

          </details>
        </fieldset>
        <footer className="profile-actions"><button type="submit" className="primary-button" disabled={busy || conflict || !draft.settings.name.trim() || !draft.settings.model || modelMissing}>{busy ? messages.common.loading : copy.save}</button>
          {draft.original && <><button type="button" className="secondary-button" disabled={busy || dirty || issues.length > 0} onClick={() => { setChatAgent(draft.original); window.location.hash = '#/chat'; }}>{copy.use}</button><button type="button" className="secondary-button" disabled={busy} onClick={() => choose({ original: null, settings: { ...draft.settings, name: copy.duplicateName.replace('{name}', draft.settings.name) } })}>{copy.duplicate}</button><button type="button" className="secondary-button" disabled={busy} onClick={() => setArchiving(draft.original)}>{copy.archive}</button></>}
        </footer>
      </form> : !loading && !error ? <div className="profile-empty"><p>{profiles.length ? profileSelectionPrompt[locale] : copy.empty}</p></div> : null}
    </div>
    {discardTo && <ConfirmDialog title={copy.discard} body={copy.discardBody} confirmLabel={copy.discard} close={() => setDiscardTo(null)} confirm={async () => { setDraft(discardTo); setConflict(false); setError(''); setNotice(''); }} />}
    {archiving && <ConfirmDialog title={`${copy.archive}: ${archiving.settings.name}`} body={copy.archiveBody} confirmLabel={copy.archive} close={() => setArchiving(null)} confirm={async () => {
      await api.archiveAgentProfile(archiving.id, archiving.revision);
      setProfiles(current => current.filter(item => item.id !== archiving.id));
      if (draft?.original?.id === archiving.id) setDraft(null);
      setNotice(copy.archived);
    }} />}
  </section>;
}
