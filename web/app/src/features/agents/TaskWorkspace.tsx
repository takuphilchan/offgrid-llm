import { useEffect, useMemo, useRef, useState } from "react";
import { api, type AgentTask } from "../../api/client";
import { useI18n } from "../../i18n";
import { taskWorkspaceText } from "../../i18n/task-workspace";
import { computerExperience } from "../../i18n/computer-experience";
import { useDraft } from "../../lib/drafts";
import { useWorkspaceRefresh } from "../../lib/workspace-refresh";
import {
  HistoryDeleteDialog,
  type HistoryItem,
} from "../../components/HistoryDeleteDialog";
import { copyText } from "../../lib/clipboard";
import { useTaskRun } from "./useTaskRun";
import { TaskAccess } from "./TaskAccess";
import { TaskLifecycle, TaskDetails, stopOwnedComputer, type TaskCommand } from './TaskControls';
import { taskControls } from '../../i18n/task-controls';
import { AgentNavigation } from './AgentNavigation';
import { Icon } from '../../components/Icon';
import { AgentPreview, AgentProgress } from "./AgentProgress";
import { BrowserActivity } from "./BrowserActionSummary";
import { TaskApproval } from './TaskApproval';
import { TaskEvidence } from './TaskEvidence';
import { DocumentProgress } from './DocumentProgress';
import { TaskResult } from './TaskResult';
import { TaskContinuation } from './TaskContinuation';
import { taskStopText } from '../../i18n/task-presentation';
import { EmptyState, ScopedNotice } from '../../components/WorkspacePresentation';
import { HistoryToolbar } from '../../components/HistoryToolbar';
import { useWorkspaceState } from '../../lib/workspace-context';
import { useWorkActions } from '../../lib/active-work';
import { taskAccessText } from '../../i18n/task-access';

function taskFromLocation() {
  const part = window.location.hash.match(
    /^#\/agents\/task\/(run-[a-f0-9]{32})$/,
  );
  return part?.[1] ?? "";
}
export function TaskWorkspace({
  scope,
  workspace,
}: {
  scope: string;
  workspace: string;
}) {
  const { locale, messages: text } = useI18n(),
    copy = taskWorkspaceText(locale),
    experience = computerExperience(locale);
  const work = useWorkActions();
  const selected = useDraft(`${scope}:workspace:${workspace}`, "agent-run");
  const [id, setID] = useState(() => taskFromLocation() || (window.location.hash === '#/agents/workspace' ? selected.value : ''));
  const { run, connection, error: readError, refresh } = useTaskRun(id);
  // Only an explicit Stop aborts setup; navigation/reconnect never cancels work.
  const accessCancellation = useMemo(() => new AbortController(), [run?.pending_input?.id]);
  const [tasks, setTasks] = useState<AgentTask[]>([]),
    [historyError, setHistoryError] = useState("");
  const [historyLoading, setHistoryLoading] = useState(true);
  const [query, setQuery] = useWorkspaceState(`task-history:${workspace}`, ""),
    [limit, setLimit] = useState(20);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [preview, setPreview] = useState(false),
    [copied, setCopied] = useState(false);
  const [reconciliation, setReconciliation] = useState("");
  const [stopState, setStopState] = useState<'idle' | 'pending' | 'unconfirmed'>('idle');
  const [deleting, setDeleting] = useState<HistoryItem[] | null>(null);
  const lock = useRef(false),
    heading = useRef<HTMLHeadingElement>(null);
  const generation = useRef(0);
  const currentID = useRef(id);
  currentID.current = id;
  const refreshHistory = async () => {
    const attempt = ++generation.current;
    try {
      const next = await api.jobs();
      if (attempt === generation.current) {
        work.seedJobs(next);
        setTasks(
          next.sort(
            (a, b) => Date.parse(b.created_at) - Date.parse(a.created_at),
          ),
        );
        setHistoryError("");
      }
    } catch (e) {
      if (attempt === generation.current)
        setHistoryError(e instanceof Error ? e.message : text.common.error);
    } finally {
      if (attempt === generation.current) setHistoryLoading(false);
    }
  };
  useEffect(() => {
    void refreshHistory();
    const timer = setInterval(() => void refreshHistory(), 5000);
    return () => {
      generation.current++;
      clearInterval(timer);
    };
  }, []);
  useWorkspaceRefresh(refreshHistory);
  useEffect(() => {
    const change = () => {
      if (window.location.hash === "#/agents/new" || window.location.hash === "#/agents/workspace") setID("");
      else if (taskFromLocation()) setID(taskFromLocation());
    };
    window.addEventListener("hashchange", change);
    return () => window.removeEventListener("hashchange", change);
  }, []);
  useEffect(() => {
    selected.setValue(id);
    setError("");
    setReconciliation("");
    setCopied(false);
    setStopState('idle');
  }, [id]);
  const select = (next: string) => {
    setID(next);
    window.location.hash = next ? `#/agents/task/${next}` : "#/agents/workspace";
    requestAnimationFrame(() =>
      heading.current?.focus(),
    );
  };
  const act = async (
    action: TaskCommand,
  ) => {
    if (lock.current || !run) return;
    lock.current = true;
    setBusy(true);
    setError("");
    const taskID = run.run_id;
    const stopping = ['takeover', 'cancel'].includes(action);
    if (stopping) setStopState('pending');
    try {
      if (action === 'cancel') accessCancellation.abort();
      const operations = [api.jobAction(id, action, {
        ...(["approve", "deny"].includes(action)
          ? { approval_id: run.pending_approval?.id }
          : {}),
        ...(action === "reconcile"
          ? { call_id: run.uncertain_call_id, result: reconciliation.trim() }
          : {}),
      })];
      const localStop = ['takeover', 'cancel'].includes(action) ? stopOwnedComputer(run.computer_session, run.pending_input?.id) : Promise.resolve();
      const results = await Promise.allSettled([...operations, localStop]);
      if (currentID.current !== taskID) return;
      refresh();
      void refreshHistory();
      const failure = results.find((result): result is PromiseRejectedResult => result.status === 'rejected');
      if (failure) throw failure.reason;
      if (stopping) setStopState('idle');
    } catch (e) {
      if (currentID.current !== taskID) return;
      if (stopping) setStopState('unconfirmed');
      setError(e instanceof Error ? e.message : text.common.error);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  };
  const status = (value: string, outcome?: string) =>
    value === 'completed' && outcome === 'answer_only' ? taskAccessText(locale).answered : value === 'waiting_for_children' ? taskControls(locale).waiting : value === "waiting_for_input"
      ? copy.needsAccess
      : (text.recovery[value as keyof typeof text.recovery] ?? value);
  const filtered = tasks.filter((t) =>
    (!t.parent_id || !tasks.some(parent => parent.id === t.parent_id)) && `${t.prompt} ${t.id}`.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)),
  );
  const title =
    run?.prompt ?? tasks.find((t) => t.id === id)?.prompt ?? text.agents.task;
  const approval = run?.pending_approval;
  return (
    <div className="task-workspace">
      <AgentNavigation />
      <aside className="task-list" aria-label={text.agentRuntime.history}>
        <a className="secondary-button task-chat-link" href="#/chat">{text.nav.chat}</a>
        <HistoryToolbar query={query} onQuery={value => { setQuery(value); setLimit(20); }} searchLabel={text.history.searchTasks} title={text.agentRuntime.history} count={tasks.length} refresh={() => void refreshHistory()} clearLabel={text.history.clearTasks} clearDisabled={!filtered.some(t => t.deletable)} clear={() => setDeleting(filtered.filter(t => t.deletable).map(t => ({ id: t.id, label: t.prompt })))} protection={text.history.protectedTasks} />
        {historyError && <ScopedNotice kind="error" action={<button className="secondary-button" onClick={() => void refreshHistory()}>{text.common.retry}</button>}>{historyError}</ScopedNotice>}
        {!historyError && !tasks.length && (historyLoading ? <p role="status">{text.common.loading}</p> : <EmptyState title={text.agentRuntime.noTasks} />)}
        {!!tasks.length && !filtered.length && <EmptyState title={text.history.noMatches} />}
        <ul>
          {filtered.slice(0, limit).map((task) => (
            <li key={task.id}>
              <button
                className="task-list-item"
                aria-current={id === task.id ? "true" : undefined}
                onClick={() => select(task.id)}
              >
                <span>{task.prompt}</span>
                <small>{status(task.status, task.outcome)}</small>
              </button>
              <button className="task-delete icon-button" disabled={!task.deletable} aria-label={`${text.history.deleteTask}: ${task.prompt}`} title={task.deletable ? text.history.deleteTask : text.history.protectedTasks} onClick={() => setDeleting([{id:task.id,label:task.prompt}])}><Icon name="trash" size={16}/></button>
            </li>
          ))}
        </ul>
        {filtered.length > limit && (
          <button
            className="text-button"
            onClick={() => setLimit((n) => n + 20)}
          >
            {text.history.showMore}
          </button>
        )}
      </aside>
      <section className={id ? 'task-surface' : 'task-surface task-activity-entry'} aria-label={text.agentRuntime.history}>
        {!id ? (
          <div className="task-activity-empty">
            <h2 tabIndex={-1} ref={heading}>{text.agentRuntime.history}</h2>
            {!tasks.length && !historyLoading && <p>{text.agentRuntime.noTasks}</p>}
            <a className="secondary-button" href="#/chat">{text.nav.chat}</a>
          </div>
        ) : (
          <section className="task-detail" aria-labelledby="task-title">
            <header>
              <h2 id="task-title" tabIndex={-1} ref={heading}>
                {title}
              </h2>
              {run && (
                <>
                  <div className="task-state">
                    <span role="status">{status(run.status, run.outcome)}</span>
                    <small>{run.model}</small>
                  </div>
                  <div className="button-row">
                    <TaskLifecycle run={run} busy={busy} act={action => void act(action)} />
                    {run.output && (
                      <button
                        className="secondary-button"
                        onClick={() =>
                          void copyText(run.output)
                            .then(() => setCopied(true))
                            .catch((e) => setError(e.message))
                        }
                      >
                        {copied ? text.chat.copied : text.history.copyResult}
                      </button>
                    )}
                    {tasks.find((t) => t.id === id)?.deletable && (
                      <button
                        className="text-button"
                        onClick={() => setDeleting([{ id, label: title }])}
                      >
                        {text.history.deleteTask}
                      </button>
                    )}
                  </div>
                  <AgentProgress
                    run={run}
                    connection={connection}
                    showPreview={preview}
                    setShowPreview={setPreview}
                  />
                </>
              )}
            </header>
            {!run && !readError && <p role="status">{text.common.loading}</p>}
            {readError && (
              <p role="alert">
                {readError}{" "}
                <button className="text-button" onClick={refresh}>
                  {text.common.refresh}
                </button>
              </p>
            )}
            {stopState !== 'idle' && <ScopedNotice kind={stopState === 'unconfirmed' ? 'error' : 'status'}>{taskStopText(locale)[stopState]}</ScopedNotice>}
            {run?.pending_input && run.status === "waiting_for_input" && (
              <TaskAccess
                initialConsentPending={false}
                key={run.pending_input.id}
                run={run}
                workspace={workspace}
                onResolved={refresh}
                cancelSignal={accessCancellation.signal}
              />
            )}
            {approval && run.status === 'waiting_for_approval' && <TaskApproval key={`approval:${approval.id}`} run={run} busy={busy || connection !== 'live'} refresh={refresh} act={action => void act(action)} />}
            {run?.status === "uncertain" && (
              <section className="approval-card">
                <h3>{text.recovery.uncertain}</h3>
                <p>{text.recovery.verifyOutcome}</p>
                <details><summary>{experience.details}</summary><pre>{JSON.stringify(run.uncertain_call, null, 2)}</pre></details>
                <label className="field">
                  <span>{text.recovery.reconcile}</span>
                  <textarea
                    value={reconciliation}
                    onChange={(e) => setReconciliation(e.target.value)}
                  />
                </label>
                <button
                  className="primary-button"
                  disabled={busy || !reconciliation.trim()}
                  onClick={() => void act("reconcile")}
                >
                  {text.recovery.reconcile}
                </button>
              </section>
            )}
            {run &&
              ["pending", "interrupted"].includes(run.status) &&
              run.resumable && (
                <button
                  className="primary-button"
                  disabled={busy}
                  onClick={() => void act("resume")}
                >
                  {text.models.resume}
                </button>
              )}
            {run && <DocumentProgress run={run} />}
            {run && <TaskEvidence key={`evidence:${run.run_id}`} run={run} />}
            {run && <TaskResult key={`result:${run.run_id}`} run={run} />}
            {run?.error && <p role="alert">{run.error}</p>}
            {run && <TaskContinuation key={`continuation:${run.run_id}`} run={run} scope={`${scope}:workspace:${workspace}`} refresh={refresh} />}
            {run && <TaskDetails key={`details:${run.run_id}`} run={run} onError={setError} childStatus={child => status(tasks.find(t => t.id === child)?.status ?? 'pending')} />}
            {!!run?.steps.length && (
              <details className="task-activity">
                <summary>
                  {copy.activity} · {run.steps.length}
                </summary>
                {run.computer_session ? (
                  <BrowserActivity steps={run.steps} />
                ) : (
                  run.steps.map((step, index) => (
                    <article key={index}>
                      <strong>{step.tool_name || step.type}</strong>
                      <details>
                        <summary>{experience.details}</summary>
                        <pre>{step.tool_result || step.content}</pre>
                      </details>
                    </article>
                  ))
                )}
              </details>
            )}
            {run && <AgentPreview run={run} showPreview={preview} />}
          </section>
        )}
        {error && <ScopedNotice kind="error">{error}</ScopedNotice>}
      </section>
      {deleting && (
        <HistoryDeleteDialog
          items={deleting}
          kind="tasks"
          remove={api.deleteJob}
          onClose={() => setDeleting(null)}
          onDeleted={(ids) => {
            setTasks((current) => current.filter((t) => !ids.includes(t.id)));
            if (ids.includes(id)) select("");
            void refreshHistory();
          }}
        />
      )}
    </div>
  );
}
