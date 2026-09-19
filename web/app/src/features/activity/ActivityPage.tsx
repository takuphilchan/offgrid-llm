import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState } from 'react';
import { api, type RunEvent, type RunSummary } from '../../api/client';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import { MarkdownMessage } from '../../components/MarkdownMessage';

type Health = 'checking' | 'ready' | 'offline';
type RuntimeStats = {
  server?: { uptime?: string; current_model?: string; version?: string };
  inference?: { aggregate?: { total_requests?: number } };
};

export function ActivityPage({ health }: { health: Health }) {
  const { messages: text, locale } = useI18n();
  const { admin } = useWorkspace();
  const [stats, setStats] = useState<RuntimeStats | null>(null);
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [selected, setSelected] = useWorkspaceState('activity.selected', '');
  const [error, setError] = useState('');
  const [eventError, setEventError] = useState('');
  const requestRevision = useRef(0);
  const [loadingEvents, setLoadingEvents] = useState(false);
  const [loading, setLoading] = useState(true);
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
      if (nextRuns.status === 'fulfilled') setRuns(nextRuns.value);
      const current = nextRuns.status === 'fulfilled' ? nextRuns.value.find(run => run.id === selected) : undefined;
      if (current && selection === requestRevision.current) await inspect(current);
      else if (nextRuns.status === 'fulfilled' && selected && selection === requestRevision.current) { setSelected(''); setEvents([]); setEventError(''); }
      const failure = [nextStats, nextRuns].find(item => item.status === 'rejected');
      if (failure?.status === 'rejected') setError(failure.reason instanceof Error ? failure.reason.message : text.common.error);
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
  const statusLabel = (value: string) => text.recovery[value as keyof typeof text.recovery] ?? value.replace(/[._-]/g, ' ');
  const eventLabel = (value: string) => {
    const suffix = value.split(/[._]/).pop() ?? value;
    return statusLabel(suffix === 'started' ? 'running' : suffix === 'finished' ? 'completed' : value in text.recovery ? value : value.replace(/[._-]/g, ' '));
  };
  return <div className="stack activity-page">
    {error && <div className="inline-error" role="alert">{error}</div>}
    <div className="metric-grid">
      <Metric label={text.activity.uptime} value={server?.uptime ?? '—'} />
      <Metric label={text.activity.requests} value={String(aggregate?.total_requests ?? '—')} />
      <Metric label={text.activity.currentModel} value={!stats ? loading ? text.common.loading : presentation[locale].unknown : server?.current_model || text.status.noModel} />
      <Metric label={text.activity.version} value={server?.version ?? '—'} />
    </div>
    <section className="runtime-summary"><div className={`runtime-indicator ${health}`} /><div><span className="eyebrow">{text.common.runtime}</span><h2>{health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking}</h2><p>{text.activity.subtitle}</p></div></section>
    <section className="stack"><span className="eyebrow">{text.activity.runs}</span>{!admin && <p className="permission-notice">{interaction[locale].adminOnly}</p>}{!admin ? null : loading && runs.length === 0 ? <p role="status">{text.common.loading}</p> : error && runs.length === 0 ? null : runs.length === 0 ? <div className="empty-panel"><p>{text.activity.noRuns}</p></div> : <div className="run-layout">
      <div className="run-list">{runs.map(run => <button key={run.id} className={selected === run.id ? 'run-row selected' : 'run-row'} onClick={() => void inspect(run)}><i className={run.status} /><span><strong>{String(run.data?.prompt ?? run.id)}</strong><small>{statusLabel(run.status)} · {new Date(run.updated_at).toLocaleString(locale)}</small></span><b>{run.event_count}</b></button>)}</div>
      <div className="event-list" aria-busy={loadingEvents}>{eventError ? <p role="alert">{eventError}</p> : loadingEvents ? <p role="status">{text.common.loading}</p> : events.length === 0 ? <p>{selected ? interaction[locale].noEvents : text.activity.selectRun}</p> : events.map(event => <article key={event.id}><i /><div><strong className="event-label">{eventLabel(event.type)}</strong><small>#{event.sequence} · {new Date(event.time).toLocaleTimeString(locale)}</small>{typeof event.data?.output === 'string' && <div className="message-body agent-answer"><MarkdownMessage content={event.data.output} /></div>}{event.data && <details><summary>{presentation[locale].details}</summary><pre>{JSON.stringify({ type: event.type, ...event.data }, null, 2)}</pre></details>}</div></article>)}</div>
    </div>}</section>
  </div>;
}

function Metric({ label, value }: { label: string; value: string }) {
  return <article className="metric"><span>{label}</span><strong>{value}</strong></article>;
}
