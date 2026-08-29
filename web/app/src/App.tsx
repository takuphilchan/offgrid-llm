import { Component, useCallback, useEffect, useMemo, useRef, useState, type ErrorInfo, type FormEvent, type ReactNode } from 'react';
import { APIError, api, type Document, type Model, type PublicUser, type RunEvent, type RunSummary, type ToolApproval } from './api/client';
import { Icon } from './components/Icon';
import { ModelSelect } from './components/ModelSelect';
import { ChatPage } from './features/chat/ChatPage';
import { LoginPage } from './features/auth/LoginPage';
import { ModelsPage } from './features/models/ModelsPage';
import { useI18n, type LocaleCode } from './i18n';

type Page = 'chat' | 'knowledge' | 'agents' | 'models' | 'activity' | 'settings';
type Health = 'checking' | 'ready' | 'offline';

const pages: Page[] = ['chat', 'knowledge', 'agents', 'models', 'activity', 'settings'];

function pageFromLocation(): Page {
  const candidate = window.location.hash.replace(/^#\/?/, '').split('/')[0];
  return pages.includes(candidate as Page) ? candidate as Page : 'chat';
}

class PageBoundary extends Component<{ children: ReactNode; message: string; retry: string }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  componentDidCatch(error: Error, info: ErrorInfo) { console.error('Page render failed', error, info); }
  render() {
    if (this.state.failed) return <div className="page-failure" role="alert"><div><strong>{this.props.message}</strong><p>{this.props.retry}</p></div><button className="primary-button" onClick={() => this.setState({ failed: false })}>{this.props.retry}</button></div>;
    return this.props.children;
  }
}

export function App() {
  const { messages: text, locale, setLocale, available } = useI18n();
  const [page, setPage] = useState<Page>(pageFromLocation);
  const [health, setHealth] = useState<Health>('checking');
  const [models, setModels] = useState<Model[]>([]);
  const [model, setModel] = useState(localStorage.getItem('offgrid.model') ?? '');
  const [access, setAccess] = useState<'checking' | 'ready' | 'login'>('checking');
  const [authUser, setAuthUser] = useState<PublicUser | null>(null);
  const [loadError, setLoadError] = useState('');
  const [showOnboarding, setShowOnboarding] = useState(() => localStorage.getItem('offgrid.onboarding.complete') !== 'true');

  const refreshBase = useCallback(async () => {
    setLoadError('');
    const [healthResult, modelResult] = await Promise.allSettled([api.health(), api.models()]);
    setHealth(healthResult.status === 'fulfilled' ? 'ready' : 'offline');
    if (modelResult.status === 'fulfilled') {
      setAccess('ready');
      setModels(modelResult.value);
      setModel(current => {
        if (current && modelResult.value.some(item => item.id === current)) return current;
        return modelResult.value.find(item => item.type !== 'embedding')?.id ?? modelResult.value[0]?.id ?? '';
      });
      void api.currentUser().then(result => setAuthUser(result.user)).catch(reason => {
        if (reason instanceof APIError && reason.status === 401) setAccess('login');
      });
    } else if (modelResult.reason instanceof APIError && modelResult.reason.status === 401) {
      setAccess('login');
      setAuthUser(null);
    } else {
      setAccess('ready');
      setLoadError(modelResult.reason instanceof Error ? modelResult.reason.message : text.common.error);
    }
  }, [text.common.error]);

  useEffect(() => {
    if (!window.location.hash) window.history.replaceState(null, '', '#/chat');
    const syncPage = () => setPage(pageFromLocation());
    window.addEventListener('hashchange', syncPage);
    window.addEventListener('popstate', syncPage);
    return () => { window.removeEventListener('hashchange', syncPage); window.removeEventListener('popstate', syncPage); };
  }, []);
  useEffect(() => { void refreshBase(); const timer = window.setInterval(() => void api.health().then(() => setHealth('ready')).catch(() => setHealth('offline')), 15_000); return () => clearInterval(timer); }, [refreshBase]);
  useEffect(() => { if (model) localStorage.setItem('offgrid.model', model); }, [model]);

  const titles = {
    chat: [text.chat.title, text.chat.subtitle], knowledge: [text.knowledge.title, text.knowledge.subtitle],
    agents: [text.agents.title, text.agents.subtitle], models: [text.models.title, text.models.subtitle],
    activity: [text.activity.title, text.activity.subtitle], settings: [text.settings.title, text.settings.subtitle]
  };

  if (access === 'checking') return <div className="boot-screen"><div className="orb"><div /></div><span>{text.status.checking}</span></div>;
  if (access === 'login') return <LoginPage onAuthenticated={user => { setAuthUser(user); setAccess('ready'); void refreshBase(); }} />;

  const logout = async () => {
    try { await api.logout(); } finally { setAuthUser(null); await refreshBase(); }
  };

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark"><span /></div><div><strong>{text.product}</strong><small>{text.privateWorkspace}</small></div></div>
      <nav className="primary-nav" aria-label="Primary">
        {pages.map(item => <a key={item} href={`#/${item}`} className={page === item ? 'nav-link active' : 'nav-link'} aria-current={page === item ? 'page' : undefined}>
          <Icon name={item} /><span>{text.nav[item]}</span>
        </a>)}
      </nav>
      <div className="sidebar-foot">
        <div className={`service-state ${health}`}><i /> <span>{health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking}</span></div>
        <div className="privacy-note"><span>100%</span> {text.common.localProcessing}</div>
      </div>
    </aside>

    <main className="workspace">
      <header className="topbar">
        <div><h1>{titles[page][0]}</h1><p>{titles[page][1]}</p></div>
        <div className="topbar-actions">
          <label className="locale-picker"><span>{text.common.language}</span><select value={locale} onChange={event => setLocale(event.target.value as LocaleCode)}>{available.map(item => <option key={item.code} value={item.code}>{item.label}</option>)}</select></label>
          <button className="icon-button" onClick={() => void refreshBase()} aria-label={text.common.refresh}><Icon name="refresh" size={18} /></button>
          {authUser && <button className="text-button" onClick={() => void logout()}>{text.auth.signOut}</button>}
        </div>
      </header>
      {loadError && <div className="error-banner" role="alert"><span>{loadError}</span><button onClick={() => void refreshBase()}>{text.common.retry}</button></div>}
      <section className="page-content">
        <PageBoundary key={page} message={text.common.error} retry={text.common.retry}>
          {page === 'chat' && <ChatPage models={models} model={model} setModel={setModel} />}
          {page === 'knowledge' && <KnowledgePage />}
          {page === 'agents' && <AgentPage models={models} model={model} setModel={setModel} />}
          {page === 'models' && <ModelsPage models={models} selected={model} setSelected={setModel} onRefresh={refreshBase} />}
          {page === 'activity' && <ActivityPage health={health} />}
          {page === 'settings' && <SettingsPage health={health} onShowOnboarding={() => setShowOnboarding(true)} />}
        </PageBoundary>
      </section>
    </main>
    <nav className="mobile-nav" aria-label="Primary">{pages.map(item => <a key={item} href={`#/${item}`} className={page === item ? 'active' : ''} aria-current={page === item ? 'page' : undefined}><Icon name={item} size={19} /><span>{text.nav[item]}</span></a>)}</nav>
    {showOnboarding && <Onboarding health={health} models={models} onDone={() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); setShowOnboarding(false); }} />}
  </div>;
}

function KnowledgePage() {
  const { messages: text } = useI18n();
  const [documents, setDocuments] = useState<Document[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState('');
  const [reindexing, setReindexing] = useState('');
  const input = useRef<HTMLInputElement | null>(null);
  const refresh = async () => { setBusy(true); setError(''); try { const [list, status] = await Promise.all([api.documents(), api.ragStatus()]); setDocuments(list.documents); setEnabled(status.enabled); } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); } finally { setBusy(false); } };
  useEffect(() => { void refresh(); }, []);
  const upload = async (file?: File) => { if (!file) return; setBusy(true); try { await api.ingest(file); await refresh(); } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); setBusy(false); } };
  const reindex = async (document: Document) => { setReindexing(document.id); setError(''); try { await api.reindexDocument(document.id); await refresh(); } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); } finally { setReindexing(''); } };
  return <div className="stack"><div className="section-actions"><div className={`notice ${enabled ? 'success' : 'warning'}`}><i />{enabled ? `${documents.length} ${text.knowledge.chunks}` : text.knowledge.disabled}</div><input ref={input} hidden type="file" onChange={event => void upload(event.target.files?.[0])} /><button className="primary-button" onClick={() => input.current?.click()} disabled={!enabled || busy}><Icon name="upload" size={17} />{text.knowledge.add}</button></div>
    {error && <div className="inline-error">{error}</div>}{busy && documents.length === 0 ? <SkeletonCards /> : documents.length === 0 ? <EmptyPanel text={text.knowledge.empty} /> : <div className="card-grid">{documents.map(document => <article className="resource-card" key={document.id}><div className="file-icon"><Icon name="knowledge" /></div><div className="resource-body"><h3>{document.name}</h3><p>{document.content_type || text.common.document} · {formatBytes(document.size)}</p><small>{document.chunk_count} {text.knowledge.chunks} · {document.index_status ?? 'ready'}</small><div className="resource-actions"><span className={document.source_retained ? 'source-state retained' : 'source-state'}>{document.source_retained ? text.knowledge.retained : text.knowledge.legacy}</span><button disabled={!document.source_retained || reindexing === document.id} onClick={() => void reindex(document)}>{reindexing === document.id ? text.knowledge.reindexing : text.knowledge.reindex}</button></div></div></article>)}</div>}
  </div>;
}

function AgentPage({ models, model, setModel }: { models: Model[]; model: string; setModel: (model: string) => void }) {
  const { messages: text } = useI18n(); const [task, setTask] = useState(''); const [busy, setBusy] = useState(false); const [result, setResult] = useState(''); const [error, setError] = useState('');
  const [approval, setApproval] = useState<ToolApproval | null>(null); const [grants, setGrants] = useState<ToolApproval[]>([]);
  const execute = async (approved: ToolApproval[]) => {
    if (!task.trim() || !model) return;
    setBusy(true); setResult(''); setError(''); setApproval(null);
    try { setResult((await api.runAgent(model, task.trim(), approved)).output); }
    catch (reason) {
      if (reason instanceof APIError && reason.status === 409 && reason.data?.run_id) {
        try {
          const events = await api.runEvents(String(reason.data.run_id));
          const request = [...events].reverse().find(item => item.type === 'approval.required');
          if (request?.data?.tool) { setApproval({ tool: String(request.data.tool), arguments: request.data.arguments ?? {} }); return; }
        } catch { /* Preserve the original policy error below. */ }
      }
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally { setBusy(false); }
  };
  const run = (event: FormEvent) => { event.preventDefault(); setGrants([]); void execute([]); };
  const approve = () => { if (!approval) return; const next = [...grants, approval]; setGrants(next); void execute(next); };
  const deny = () => { setApproval(null); setError(text.agents.denied); };
  return <div className="agent-workspace"><form className="task-card" onSubmit={run}><div className="form-row"><ModelSelect models={models} value={model} onChange={setModel} /></div><label><span>{text.agents.task}</span><textarea rows={7} value={task} onChange={event => setTask(event.target.value)} placeholder={text.agents.placeholder} /></label><button className="primary-button" disabled={busy || !task.trim() || !model}>{busy ? text.agents.running : text.agents.run}</button></form><section className="result-card"><span className="eyebrow">{text.agents.result}</span>{approval ? <div className="approval-card" role="alertdialog" aria-labelledby="approval-title"><span className="status-pill danger">{text.agents.approvalTitle}</span><h2 id="approval-title">{approval.tool}</h2><p>{text.agents.approvalBody}</p><pre>{JSON.stringify(approval.arguments, null, 2)}</pre><div><button className="danger-button" onClick={deny}>{text.agents.deny}</button><button className="primary-button" onClick={approve}>{text.agents.approve}</button></div></div> : error ? <div className="inline-error">{error}</div> : result ? <pre>{result}</pre> : <div className="quiet-state"><Icon name="agents" size={30} /><p>{text.agents.subtitle}</p></div>}</section></div>;
}

function ActivityPage({ health }: { health: Health }) {
  const { messages: text } = useI18n(); const [stats, setStats] = useState<Record<string, any> | null>(null); const [runs, setRuns] = useState<RunSummary[]>([]); const [events, setEvents] = useState<RunEvent[]>([]); const [selected, setSelected] = useState(''); const [error, setError] = useState('');
  useEffect(() => { Promise.all([api.stats(), api.runs()]).then(([nextStats, nextRuns]) => { setStats(nextStats); setRuns(nextRuns); }).catch(reason => setError(reason instanceof Error ? reason.message : text.common.error)); }, []);
  const inspect = async (run: RunSummary) => { setSelected(run.id); try { setEvents(await api.runEvents(run.id)); } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); } };
  const server = stats?.server ?? {}; const aggregate = stats?.inference?.aggregate ?? {};
  return <div className="stack">{error && <div className="inline-error">{error}</div>}<div className="metric-grid"><Metric label={text.activity.uptime} value={server.uptime ?? '—'} /><Metric label={text.activity.requests} value={String(aggregate.total_requests ?? '—')} /><Metric label={text.activity.currentModel} value={server.current_model || text.status.noModel} /><Metric label={text.activity.version} value={server.version ?? '—'} /></div><div className="health-panel"><div className={`health-visual ${health}`}><span /><span /><span /></div><div><span className="eyebrow">{text.common.runtime}</span><h2>{health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking}</h2><p>{text.activity.subtitle}</p></div></div><section><span className="eyebrow">{text.activity.runs}</span>{runs.length === 0 ? <EmptyPanel text={text.activity.noRuns} /> : <div className="run-layout"><div className="run-list">{runs.map(run => <button key={run.id} className={selected === run.id ? 'run-row selected' : 'run-row'} onClick={() => void inspect(run)}><i className={run.status} /><span><strong>{String(run.data?.prompt ?? run.id)}</strong><small>{run.status} · {new Date(run.updated_at).toLocaleString()}</small></span><b>{run.event_count}</b></button>)}</div><div className="event-list">{events.length === 0 ? <p>{text.activity.selectRun}</p> : events.map(event => <article key={event.id}><i /><div><strong>{event.type}</strong><small>#{event.sequence} · {new Date(event.time).toLocaleTimeString()}</small>{event.data && <pre>{JSON.stringify(event.data, null, 2)}</pre>}</div></article>)}</div></div>}</section></div>;
}

function SettingsPage({ health, onShowOnboarding }: { health: Health; onShowOnboarding: () => void }) {
  const { messages: text } = useI18n();
  const [details, setDetails] = useState<{ config?: Awaited<ReturnType<typeof api.systemConfig>>; rag?: Awaited<ReturnType<typeof api.ragStatus>>; computer?: Awaited<ReturnType<typeof api.computerStatus>> }>({});
  const [error, setError] = useState('');
  const refresh = async () => {
    setError('');
    const [config, rag, computer] = await Promise.allSettled([api.systemConfig(), api.ragStatus(), api.computerStatus()]);
    setDetails({ config: config.status === 'fulfilled' ? config.value : undefined, rag: rag.status === 'fulfilled' ? rag.value : undefined, computer: computer.status === 'fulfilled' ? computer.value : undefined });
    const failed = [config, rag, computer].find(item => item.status === 'rejected');
    if (failed?.status === 'rejected') setError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
  };
  useEffect(() => { void refresh(); }, []);
  const stopComputer = async () => { try { await api.emergencyStop(); await refresh(); } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); } };
  return <div className="stack">
    {error && <div className="inline-error">{error}</div>}
    <div className="metric-grid"><Metric label={text.settings.service} value={health === 'ready' ? text.status.ready : text.status.offline} /><Metric label={text.settings.version} value={details.config?.version ?? '—'} /><Metric label={text.settings.inferenceSlots} value={String(details.config?.inference_slots ?? '—')} /><Metric label={text.settings.knowledge} value={details.rag?.enabled ? text.settings.enabled : text.settings.disabled} /></div>
    <section className="settings-panel"><div><span className="eyebrow">{text.settings.safety}</span><h2>{text.settings.computerUse}</h2><p>{details.computer?.available ? text.settings.available : text.settings.unavailable}</p></div><div className="settings-actions"><span className={details.computer?.emergency_stop ? 'status-pill danger' : 'status-pill'}>{details.computer?.emergency_stop ? text.settings.stopped : `${details.computer?.active_sessions ?? 0} ${text.settings.sessions}`}</span><button className="danger-button" onClick={() => void stopComputer()}>{text.settings.emergencyStop}</button></div></section>
    <section className="settings-panel"><div><span className="eyebrow">{text.settings.setup}</span><h2>{text.onboarding.title}</h2><p>{text.onboarding.body}</p></div><button className="secondary-button" onClick={onShowOnboarding}>{text.settings.showGuide}</button></section>
  </div>;
}

function Onboarding({ health, models, onDone }: { health: Health; models: Model[]; onDone: () => void }) {
  const { messages: text } = useI18n();
  return <div className="modal-backdrop" role="presentation"><section className="onboarding" role="dialog" aria-modal="true" aria-labelledby="onboarding-title"><div className="orb"><div /></div><span className="eyebrow">{text.privateWorkspace}</span><h2 id="onboarding-title">{text.onboarding.title}</h2><p>{text.onboarding.body}</p><div className="readiness-list"><div><i className={health === 'ready' ? 'ready' : ''} /><span>{text.onboarding.service}</span><strong>{health === 'ready' ? text.status.ready : text.status.offline}</strong></div><div><i className={models.length > 0 ? 'ready' : ''} /><span>{text.onboarding.models}</span><strong>{models.length}</strong></div><div><i className="ready" /><span>{text.onboarding.privacy}</span><strong>{text.onboarding.local}</strong></div></div><button className="primary-button" onClick={onDone}>{text.onboarding.continue}</button></section></div>;
}

function Metric({ label, value }: { label: string; value: string }) { return <article className="metric"><span>{label}</span><strong>{value}</strong></article>; }
function EmptyPanel({ text }: { text: string }) { return <div className="empty-panel"><div className="empty-lines"><i /><i /><i /></div><p>{text}</p></div>; }
function SkeletonCards() { return <div className="card-grid">{[1, 2, 3].map(item => <div className="skeleton-card" key={item}><i /><div><span /><span /></div></div>)}</div>; }
function formatBytes(bytes: number) { if (!bytes) return '—'; const units = ['B', 'KB', 'MB', 'GB', 'TB']; const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`; }
