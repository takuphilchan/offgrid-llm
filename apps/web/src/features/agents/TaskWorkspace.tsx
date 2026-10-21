import { useEffect, useRef, useState } from 'react';
import { api, type AgentTask } from '../../api/client';
import { useI18n } from '../../i18n';
import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useWorkActions } from '../../lib/active-work';
import { HistoryDeleteDialog, type HistoryItem } from '../../components/HistoryDeleteDialog';
import { HistoryToolbar } from '../../components/HistoryToolbar';
import { HistoryItemActions } from '../../components/HistoryItemActions';
import { EmptyState, ScopedNotice } from '../../components/WorkspacePresentation';
import { Icon } from '../../components/Icon';
import { copyText } from '../../lib/clipboard';
import { interaction } from '../../i18n/interaction';
import { useTaskRun } from './useTaskRun';
import { ConversationWork } from '../chat/ConversationWork';
import { useTaskStatus } from './useTaskStatus';
import { taskPresentation } from '../../i18n/task-presentation';

export function taskFromLocation() {
  return window.location.hash.match(/^#\/(?:agents|activity)\/task\/(run-[a-f0-9]{32})$/)?.[1] ?? '';
}

/** One canonical inspector, shared with Chat. Navigation never dispatches work. */
export function TaskWorkspace({ scope, workspace }: { scope: string; workspace: string }) {
  const { locale, messages: text } = useI18n();
  const { admin } = useWorkspace();
  const statusText = useTaskStatus();
  const work = useWorkActions();
  const [id, setID] = useState(taskFromLocation);
  const { run, connection, error: readError, refresh } = useTaskRun(id);
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [historyError, setHistoryError] = useState('');
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useWorkspaceState(`task-history:${workspace}`, '');
  const [limit, setLimit] = useWorkspaceState(`task-limit:${workspace}`, 20);
  const [scroll, setScroll] = useWorkspaceState(`task-scroll:${workspace}`, 0);
  const [deleting, setDeleting] = useState<HistoryItem[] | null>(null);
  const [copied, setCopied] = useState(false), [error, setError] = useState('');
  const generation = useRef(0), heading = useRef<HTMLHeadingElement>(null);
  const currentID = useRef(id); currentID.current = id;
  const refreshHistory = async () => {
    if (!admin) { setLoading(false); return; }
    const attempt = ++generation.current;
    try {
      const next = await api.jobs();
      if (attempt !== generation.current) return;
      work.seedJobs(next);
      setTasks([...next].sort((a,b) => Date.parse(b.created_at) - Date.parse(a.created_at)));
      setHistoryError('');
    } catch (reason) {
      if (attempt === generation.current) setHistoryError(reason instanceof Error ? reason.message : text.common.error);
    } finally { if (attempt === generation.current) setLoading(false); }
  };
  useEffect(() => {
    void refreshHistory();
    const timer = setInterval(() => void refreshHistory(), 5000);
    return () => { generation.current++; clearInterval(timer); };
  }, [scope, workspace, admin]);
  useWorkspaceRefresh(refreshHistory);
  useEffect(() => {
    const changed = () => { setID(taskFromLocation()); setCopied(false); setError(''); };
    window.addEventListener('hashchange', changed);
    return () => window.removeEventListener('hashchange', changed);
  }, []);
  useEffect(() => {
    if (id) { window.scrollTo(0,0); heading.current?.focus(); }
    else if (!loading) requestAnimationFrame(() => window.scrollTo(0, scroll));
  }, [id, loading]);
  const select = (next: string) => {
    if (next) setScroll(window.scrollY);
    window.location.hash = next ? `#/activity/task/${next}` : '#/activity';
  };
  const filtered = tasks.filter(task =>
    (!task.parent_id || !tasks.some(parent => parent.id === task.parent_id)) &&
    `${task.prompt} ${task.id} ${task.status}`.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  const title = run?.prompt ?? tasks.find(task => task.id === id)?.prompt ?? text.agents.task;
  const status = (task: AgentTask) => statusText(task.status, task.outcome);
  return <div className="task-workspace">
    {id ? <section className="task-surface" aria-labelledby="task-title">
      <div className="task-inspector-toolbar">
        <a className="utility-button" href="#/activity">{text.nav.activity}</a>
        <div className="action-group">
          {run?.output && <button className="utility-button" onClick={() => void copyText(run.output!).then(() => setCopied(true)).catch(reason => setError(reason.message))}><Icon name="copy" size={16} />{copied ? text.chat.copied : text.history.copyResult}</button>}
          {admin && <HistoryItemActions title={title} label={text.history.deleteTask} disabled={!tasks.find(task => task.id === id)?.deletable} protection={text.history.protectedTasks} onDelete={() => setDeleting([{id,label:title}])} />}
        </div>
      </div>
      <h2 id="task-title" tabIndex={-1} ref={heading}>{title}</h2>
      {run ? <ConversationWork key={run.run_id} run={run} refresh={() => { refresh(); void refreshHistory(); }} connection={connection} readError={readError} outputSaved={false} recover={async () => {}} recoveryNeeded={false} inspector childStatus={childID => { const child = tasks.find(task => task.id === childID); return child ? status(child) : taskPresentation(locale).unavailable; }} />
        : readError ? <ScopedNotice kind="error" action={<button className="secondary-button" onClick={refresh}>{text.common.retry}</button>}>{readError}</ScopedNotice>
        : <p role="status">{text.common.loading}</p>}
    </section> : !admin ? <p className="permission-notice">{interaction[locale].adminOnly}</p> : <section className="task-list" aria-label={text.agentRuntime.history}>
      <h2>{text.agentRuntime.history}</h2>
      <HistoryToolbar query={query} onQuery={value => { setQuery(value); setLimit(20); setScroll(0); }} searchLabel={text.history.searchTasks} title={text.agentRuntime.history} count={tasks.length} refresh={() => void refreshHistory()} clearLabel={text.history.clearTasks} clearDisabled={!filtered.some(task => task.deletable)} clear={() => setDeleting(filtered.filter(task => task.deletable).map(task => ({id:task.id,label:task.prompt})))} protection={text.history.protectedTasks} />
      {historyError && <ScopedNotice kind="error" action={<button className="secondary-button" onClick={() => void refreshHistory()}>{text.common.retry}</button>}>{historyError}</ScopedNotice>}
      {!tasks.length && !historyError && (loading ? <p role="status">{text.common.loading}</p> : <EmptyState title={text.agentRuntime.noTasks} action={<a className="secondary-button" href="#/chat">{text.nav.chat}</a>} />)}
      {!!tasks.length && !filtered.length && <EmptyState title={text.history.noMatches} />}
      <ul>{filtered.slice(0,limit).map(task => <li key={task.id} className="task-history-row">
        <button className="task-list-item" onClick={() => select(task.id)} title={task.prompt}>
          <span>{task.prompt}</span><small>{status(task)}</small>
          <time dateTime={task.created_at}>{new Date(task.created_at).toLocaleDateString(locale)}</time>
        </button>
        <HistoryItemActions title={task.prompt} label={text.history.deleteTask} disabled={!task.deletable} protection={text.history.protectedTasks} onDelete={() => setDeleting([{id:task.id,label:task.prompt}])} />
      </li>)}</ul>
      {filtered.length > limit && <button className="secondary-button" onClick={() => setLimit(value => value + 20)}>{text.history.showMore}</button>}
    </section>}
    {error && <ScopedNotice kind="error">{error}</ScopedNotice>}
    {deleting && <HistoryDeleteDialog items={deleting} kind="tasks" remove={api.deleteJob} onClose={() => setDeleting(null)} onDeleted={ids => {
      setTasks(current => current.filter(task => !ids.includes(task.id)));
      if (ids.includes(currentID.current)) select('');
      void refreshHistory();
    }} />}
  </div>;
}
