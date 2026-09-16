import { useEffect, useState } from 'react';
import { api, type RunEvent, type RunSummary } from '../../api/client';
import { useI18n } from '../../i18n';

type Health = 'checking' | 'ready' | 'offline';
type RuntimeStats = {
  server?: { uptime?: string; current_model?: string; version?: string };
  inference?: { aggregate?: { total_requests?: number } };
};

export function ActivityPage({ health }: { health: Health }) {
  const { messages: text, locale } = useI18n();
  const [stats, setStats] = useState<RuntimeStats | null>(null);
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [selected, setSelected] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    void Promise.all([api.stats(), api.runs()])
      .then(([nextStats, nextRuns]) => { setStats(nextStats as RuntimeStats); setRuns(nextRuns); })
      .catch(reason => setError(reason instanceof Error ? reason.message : text.common.error));
  }, [text.common.error]);

  const inspect = async (run: RunSummary) => {
    setSelected(run.id);
    setError('');
    try { setEvents(await api.runEvents(run.id)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
  };

  const server = stats?.server;
  const aggregate = stats?.inference?.aggregate;
  return <div className="stack activity-page">
    {error && <div className="inline-error" role="alert">{error}</div>}
    <div className="metric-grid">
      <Metric label={text.activity.uptime} value={server?.uptime ?? '—'} />
      <Metric label={text.activity.requests} value={String(aggregate?.total_requests ?? '—')} />
      <Metric label={text.activity.currentModel} value={server?.current_model || text.status.noModel} />
      <Metric label={text.activity.version} value={server?.version ?? '—'} />
    </div>
    <section className="runtime-summary"><div className={`runtime-indicator ${health}`} /><div><span className="eyebrow">{text.common.runtime}</span><h2>{health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking}</h2><p>{text.activity.subtitle}</p></div></section>
    <section className="stack"><span className="eyebrow">{text.activity.runs}</span>{runs.length === 0 ? <div className="empty-panel"><p>{text.activity.noRuns}</p></div> : <div className="run-layout">
      <div className="run-list">{runs.map(run => <button key={run.id} className={selected === run.id ? 'run-row selected' : 'run-row'} onClick={() => void inspect(run)}><i className={run.status} /><span><strong>{String(run.data?.prompt ?? run.id)}</strong><small>{run.status} · {new Date(run.updated_at).toLocaleString(locale)}</small></span><b>{run.event_count}</b></button>)}</div>
      <div className="event-list">{events.length === 0 ? <p>{text.activity.selectRun}</p> : events.map(event => <article key={event.id}><i /><div><strong>{event.type}</strong><small>#{event.sequence} · {new Date(event.time).toLocaleTimeString(locale)}</small>{event.data && <pre>{JSON.stringify(event.data, null, 2)}</pre>}</div></article>)}</div>
    </div>}</section>
  </div>;
}

function Metric({ label, value }: { label: string; value: string }) {
  return <article className="metric"><span>{label}</span><strong>{value}</strong></article>;
}
