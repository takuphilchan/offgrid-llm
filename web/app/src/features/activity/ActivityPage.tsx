import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState } from 'react';
import { api, type RunEvent, type RunSummary } from '../../api/client';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import {HistoryDeleteDialog, type HistoryItem} from '../../components/HistoryDeleteDialog';
import {Icon} from '../../components/Icon';
import { EmptyState, SectionHeading } from '../../components/WorkspacePresentation';
import { useKnownWork } from '../../lib/active-work';
import { WorkLinks } from '../../components/WorkLinks';
import { knownWorkText } from '../../i18n/known-work';
import { workspaceManagement } from '../../i18n/workspace-management';

type Health = 'checking' | 'ready' | 'offline';
type RuntimeStats = {
  server?: { uptime?: string; current_model?: string; version?: string };
  inference?: { aggregate?: { total_requests?: number } };
};

export function ActivityPage({ health }: { health: Health }) {
  const { messages: text, locale } = useI18n();
  const { admin } = useWorkspace();
  const known = useKnownWork(), management = workspaceManagement(locale);
  const [stats, setStats] = useState<RuntimeStats | null>(null);
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [selected, setSelected] = useWorkspaceState('activity.selected', '');
  const [error, setError] = useState('');
  const [statsError, setStatsError] = useState('');
  const [eventError, setEventError] = useState('');
  const requestRevision = useRef(0);
  const [loadingEvents, setLoadingEvents] = useState(false);
  const [loading, setLoading] = useState(true);
  const [deleting, setDeleting] = useState<HistoryItem[] | null>(null);
  const listRevision = useRef(0);
  useEffect(() => () => { requestRevision.current++; listRevision.current++; }, []);

  const refresh = async () => {
    const revision = ++listRevision.current;
    const selection = requestRevision.current;
    setLoading(true); setError('');
    try {
      const [nextStats, nextRuns] = await Promise.allSettled([api.stats(), admin ? api.runs() : Promise.resolve([])]);
      if (revision !== listRevision.current) return;
      setStats(nextStats.status === 'fulfilled' ? nextStats.value as RuntimeStats : null);
      setStatsError(nextStats.status === 'rejected' ? nextStats.reason instanceof Error ? nextStats.reason.message : text.common.error : '');
      if (nextRuns.status === 'fulfilled') setRuns(nextRuns.value);
      const current = nextRuns.status === 'fulfilled' ? nextRuns.value.find(run => run.id === selected) : undefined;
      if (current && selection === requestRevision.current) await inspect(current);
      else if (nextRuns.status === 'fulfilled' && selected && selection === requestRevision.current) { setSelected(''); setEvents([]); setEventError(''); }
      if (nextRuns.status === 'rejected') setError(nextRuns.reason instanceof Error ? nextRuns.reason.message : text.common.error);
    } catch (reason) { if (revision === listRevision.current) setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { if (revision === listRevision.current) setLoading(false); }
  };
  useEffect(() => { void refresh(); }, [text.common.error]);
  useWorkspaceRefresh(refresh);

  const inspect = async (run: RunSummary) => {
    const revision = ++requestRevision.current;
    setSelected(run.id);
    setEvents([]); setLoadingEvents(true); setEventError('');

    try { const next = await api.runEvents(run.id); if (revision === requestRevision.current) setEvents(next); }
    catch (reason) { if (revision === requestRevision.current) setEventError(reason instanceof Error ? reason.message : text.common.error); }
    finally { if (revision === requestRevision.current) setLoadingEvents(false); }
  };

  const server = stats?.server;
  const aggregate = stats?.inference?.aggregate;
  const current = known.entries.filter(entry => entry.kind === 'chat' || !runs.some(run => run.id === entry.id));
  const ordered = [...runs].sort((a, b) => {
    const active = (run: RunSummary) => !['completed', 'failed', 'cancelled'].includes(run.status);
    return Number(active(b)) - Number(active(a)) || Date.parse(b.updated_at) - Date.parse(a.updated_at);
  });
  const statusLabel = (value: string) => text.recovery[value as keyof typeof text.recovery] ?? value.replace(/[._-]/g, ' ');
  const eventLabel = (value: string) => {
    const suffix = value.split(/[._]/).pop() ?? value;
    return statusLabel(suffix === 'started' ? 'running' : suffix === 'finished' ? 'completed' : value in text.recovery ? value : value.replace(/[._-]/g, ' '));
  };
  return <div className="stack activity-page">
    {deleting && <HistoryDeleteDialog items={deleting} kind="tasks" remove={api.deleteJob} onClose={() => setDeleting(null)} onDeleted={ids => {
      listRevision.current++; requestRevision.current++;
      setRuns(current => current.filter(run => !ids.includes(run.id)));
      if (ids.includes(selected)) {setSelected('');setEvents([]);setEventError('');setLoadingEvents(false);}
      void refresh();
    }}/>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {admin && current.length > 0 && <section><SectionHeading title={knownWorkText[locale].title} /><WorkLinks entries={current} /></section>}
    <section className="stack">
      <SectionHeading title={text.activity.runs} actions={admin && runs.length > 0 && <div className="task-history-actions">
        <button className="secondary-button" disabled={loading || !runs.some(run => run.deletable)} onClick={() => setDeleting(runs.filter(run => run.deletable).map(run => ({id:run.id,label:String(run.data?.prompt ?? run.id)})))}>{text.history.clearTasks}</button>
      </div>} />
      {!admin && <p className="permission-notice">{interaction[locale].adminOnly}</p>}{!admin ? null : loading && runs.length === 0 ? <p role="status">{text.common.loading}</p> : error && runs.length === 0 ? null : runs.length === 0 ? <EmptyState title={text.activity.noRuns} /> : <div className="run-layout">
      <div className="run-list">{ordered.map(run => <div key={run.id} className="activity-history-row"><button className={selected === run.id ? 'run-row selected' : 'run-row'} aria-pressed={selected === run.id} onClick={() => void inspect(run)}><i className={run.status} /><span><strong>{String(run.data?.prompt ?? run.id)}</strong><small>{statusLabel(run.status)} · {new Date(run.updated_at).toLocaleString(locale)}</small></span></button><button className="icon-button task-delete" disabled={!run.deletable} aria-label={`${text.history.deleteTask}: ${String(run.data?.prompt ?? run.id)}`} title={run.deletable ? text.history.deleteTask : text.history.protectedTasks} onClick={() => setDeleting([{id:run.id,label:String(run.data?.prompt ?? run.id)}])}><Icon name="trash" size={16}/></button></div>)}</div>
      <div className="event-list" aria-busy={loadingEvents}>{eventError ? <p role="alert">{eventError}</p> : loadingEvents ? <p role="status">{text.common.loading}</p> : events.length === 0 ? <p>{selected ? interaction[locale].noEvents : text.activity.selectRun}</p> : events.map(event => <article key={event.id}><i /><div><strong className="event-label">{eventLabel(event.type)}</strong><small>#{event.sequence} · {new Date(event.time).toLocaleTimeString(locale)}</small>{typeof event.data?.output === 'string' && <div className="message-body agent-answer"><MarkdownMessage content={event.data.output} /></div>}{event.data && <details><summary>{presentation[locale].details}</summary><pre>{JSON.stringify({ type: event.type, ...event.data }, null, 2)}</pre></details>}</div></article>)}</div>
    </div>}
      {admin && selected && known.taskIDs.includes(selected) && <a className="secondary-button canonical-task-link" href={`#/agents/task/${encodeURIComponent(selected)}`}>{management.openTask}</a>}
    </section>
    <details className="activity-diagnostics"><summary>{management.diagnostics}</summary>
      {statsError && <p role="alert">{statsError}</p>}
      <p role="status">{health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking}</p>
      <div className="metric-grid">
        <Metric label={text.activity.uptime} value={server?.uptime ?? '—'} />
        <Metric label={text.activity.requests} value={String(aggregate?.total_requests ?? '—')} />
        <Metric label={text.activity.currentModel} value={!stats ? loading ? text.common.loading : presentation[locale].unknown : server?.current_model || text.status.noModel} />
        <Metric label={text.activity.version} value={server?.version ?? '—'} />
      </div>
    </details>
  </div>;
}

function Metric({ label, value }: { label: string; value: string }) {
  return <article className="metric"><span>{label}</span><strong>{value}</strong></article>;
}
