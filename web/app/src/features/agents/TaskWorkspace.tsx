import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { APIError, api, type AgentTask, type Model } from "../../api/client";
import { useI18n } from "../../i18n";
import { taskWorkspaceText } from "../../i18n/task-workspace";
import { computerExperience } from "../../i18n/computer-experience";
import {
  clearSubmittedDraft,
  draftKey,
  readDraft,
  useDraft,
  writeDraft,
} from "../../lib/drafts";
import { useWorkspaceRefresh } from "../../lib/workspace-refresh";
import { ModelSelect } from "../../components/ModelSelect";
import { MarkdownMessage } from "../../components/MarkdownMessage";
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
import { BrowserActionSummary, BrowserActivity } from "./BrowserActionSummary";

function taskFromLocation() {
  const part = window.location.hash.match(
    /^#\/agents\/task\/(run-[a-f0-9]{32})$/,
  );
  return part?.[1] ?? "";
}
export function TaskWorkspace({
  scope,
  workspace,
  models,
  model,
  setModel,
}: {
  scope: string;
  workspace: string;
  models: Model[];
  model: string;
  setModel: (value: string) => void;
}) {
  const { locale, messages: text } = useI18n(),
    copy = taskWorkspaceText(locale),
    experience = computerExperience(locale);
  const draft = useDraft(scope, "agent-task");
  const selected = useDraft(`${scope}:workspace:${workspace}`, "agent-run");
  const [id, setID] = useState(() =>
    window.location.hash === "#/agents/new"
      ? ""
      : taskFromLocation() || selected.value,
  );
  const { run, connection, error: readError, refresh } = useTaskRun(id);
  // Only an explicit Stop aborts setup; navigation/reconnect never cancels work.
  const accessCancellation = useMemo(() => new AbortController(), [run?.pending_input?.id]);
  const [tasks, setTasks] = useState<AgentTask[]>([]),
    [historyError, setHistoryError] = useState("");
  const [query, setQuery] = useState(""),
    [limit, setLimit] = useState(20);
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [preview, setPreview] = useState(false),
    [copied, setCopied] = useState(false);
  const [reconciliation, setReconciliation] = useState("");
  const [deleting, setDeleting] = useState<HistoryItem[] | null>(null);
  const lock = useRef(false),
    editor = useRef<HTMLTextAreaElement>(null),
    heading = useRef<HTMLHeadingElement>(null);
  const generation = useRef(0);
  const refreshHistory = async () => {
    const attempt = ++generation.current;
    try {
      const next = await api.jobs();
      if (attempt === generation.current) {
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
      if (window.location.hash === "#/agents/new") setID("");
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
  }, [id]);
  const select = (next: string) => {
    setID(next);
    window.location.hash = next ? `#/agents/task/${next}` : "#/agents/new";
    requestAnimationFrame(() =>
      next ? heading.current?.focus() : editor.current?.focus(),
    );
  };
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (lock.current || !draft.value.trim() || !model) return;
    lock.current = true;
    setBusy(true);
    setError("");
    const submitted = draft.value,
      prompt = submitted.trim();
    const key = draftKey(`${scope}:workspace:${workspace}`, "agent-submission");
    // A lost acknowledgment retries the same durable request, including reload.
    let request: { prompt: string; model: string; id: string } | undefined;
    try {
      request = JSON.parse(readDraft(key));
    } catch {}
    if (!request || request.prompt !== prompt || request.model !== model)
      request = { prompt, model, id: crypto.randomUUID() };
    writeDraft(key, JSON.stringify(request));
    try {
      const next = await api.submitJob(prompt, model, request.id);
      clearSubmittedDraft(draft.key, submitted);
      clearSubmittedDraft(key, JSON.stringify(request));
      select(next.run_id);
      void refreshHistory();
    } catch (e) {
      if (e instanceof APIError && typeof e.data?.run_id === "string")
        select(e.data.run_id);
      setError(e instanceof Error ? e.message : text.common.error);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  };
  const act = async (
    action: TaskCommand,
  ) => {
    if (lock.current || !run) return;
    lock.current = true;
    setBusy(true);
    setError("");
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
      refresh();
      void refreshHistory();
      const failure = results.find((result): result is PromiseRejectedResult => result.status === 'rejected');
      if (failure) throw failure.reason;
    } catch (e) {
      setError(e instanceof Error ? e.message : text.common.error);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  };
  const status = (value: string) =>
    value === 'waiting_for_children' ? taskControls(locale).waiting : value === "waiting_for_input"
      ? copy.needsAccess
      : (text.recovery[value as keyof typeof text.recovery] ?? value);
  const filtered = tasks.filter((t) =>
    (!t.parent_id || !tasks.some(parent => parent.id === t.parent_id)) && t.prompt.toLocaleLowerCase().includes(query.toLocaleLowerCase()),
  );
  const title =
    run?.prompt ?? tasks.find((t) => t.id === id)?.prompt ?? text.agents.task;
  const approval = run?.pending_approval;
  return (
    <div className="task-workspace">
      <AgentNavigation />
      <aside className="task-list" aria-label={text.agentRuntime.history}>
        <button className="primary-button" onClick={() => select("")}>
          {copy.newTask}
        </button>
        <label className="field">
          <span>{text.history.searchTasks}</span>
          <input
            type="search"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setLimit(20);
            }}
          />
        </label>
        <div className="task-history-actions">
          <button className="text-button" onClick={() => void refreshHistory()}>{text.common.refresh}</button>
          <button className="text-button" disabled={!filtered.some(t => t.deletable)} onClick={() => setDeleting(filtered.filter(t => t.deletable).map(t => ({id:t.id,label:t.prompt})))}>{text.history.clearTasks}</button>
        </div>
        {historyError && <p role="alert">{historyError}</p>}
        <ul>
          {filtered.slice(0, limit).map((task) => (
            <li key={task.id}>
              <button
                className="task-list-item"
                aria-current={id === task.id ? "true" : undefined}
                onClick={() => select(task.id)}
              >
                <span>{task.prompt}</span>
                <small>{status(task.status)}</small>
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
        <small className="task-history-hint">{text.history.protectedTasks}</small>
      </aside>
      <section className="task-surface" aria-label={text.agents.task}>
        {!id ? (
          <form className="task-composer" onSubmit={submit}>
            <h2>{copy.title}</h2>
            <label className="field">
              <span>{text.agents.task}</span>
              <textarea
                ref={editor}
                rows={6}
                value={draft.value}
                placeholder={text.agents.placeholder}
                onChange={(e) => draft.setValue(e.target.value)}
                aria-describedby="task-first-hint"
              />
            </label>
            <p id="task-first-hint">{copy.hint}</p>
            <details>
              <summary>
                {copy.settings}
                {model ? ` · ${model}` : ""}
              </summary>
              <ModelSelect models={models} value={model} onChange={setModel} />
            </details>
            {!model && (
              <p role="status">
                {copy.noModel} <a href="#/models">{text.nav.models}</a>
              </p>
            )}
            {draft.unsaved && <p role="alert">{text.recovery.draftWarning}</p>}
            <button
              className="primary-button"
              disabled={busy || !draft.value.trim() || !model}
            >
              {busy ? text.common.loading : copy.start}
            </button>
          </form>
        ) : (
          <section className="task-detail" aria-labelledby="task-title">
            <header>
              <h2 id="task-title" tabIndex={-1} ref={heading}>
                {title}
              </h2>
              {run && (
                <>
                  <div className="task-state">
                    <span role="status">{status(run.status)}</span>
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
            {run && (
              <button
                className="text-button task-reuse"
                disabled={!!draft.value.trim()}
                title={
                  draft.value.trim() ? text.history.draftProtected : undefined
                }
                onClick={() => {
                  draft.setValue(title);
                  select("");
                }}
              >
                {text.history.reuseTask}
              </button>
            )}
            {!run && !readError && <p role="status">{text.common.loading}</p>}
            {readError && (
              <p role="alert">
                {readError}{" "}
                <button className="text-button" onClick={refresh}>
                  {text.common.refresh}
                </button>
              </p>
            )}
            {run?.pending_input && run.status === "waiting_for_input" && (
              <TaskAccess
                key={run.pending_input.id}
                run={run}
                workspace={workspace}
                onResolved={refresh}
                cancelSignal={accessCancellation.signal}
              />
            )}
            {approval && (
              <section
                className="approval-card"
                aria-labelledby="task-approval-title"
              >
                <h3 id="task-approval-title">{text.agents.approvalTitle}</h3>
                {run.computer_session ? (
                  <BrowserActionSummary
                    tool={approval.tool}
                    args={approval.arguments as Record<string, unknown>}
                    steps={run.steps}
                  />
                ) : (
                  <pre>
                    {approval.canonical_arguments ??
                      JSON.stringify(approval.arguments, null, 2)}
                  </pre>
                )}
                <div className="button-row">
                  <button
                    className="secondary-button"
                    disabled={busy}
                    onClick={() => void act("deny")}
                  >
                    {text.agents.deny}
                  </button>
                  <button
                    className="primary-button"
                    disabled={busy}
                    onClick={() =>
                      void act(
                        Date.parse(approval.expires_at) <= Date.now()
                          ? "resume"
                          : "approve",
                      )
                    }
                  >
                    {Date.parse(approval.expires_at) <= Date.now()
                      ? text.common.refresh
                      : text.agents.approve}
                  </button>
                </div>
              </section>
            )}
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
            {run?.output && (
              <div className="markdown-body">
                <MarkdownMessage content={run.output} />
              </div>
            )}
            {run?.error && <p role="alert">{run.error}</p>}
            {run && <TaskDetails key={run.run_id} run={run} scope={`${scope}:workspace:${workspace}`} refresh={refresh} onError={setError} childStatus={child => status(tasks.find(t => t.id === child)?.status ?? 'pending')} />}
            {!!run?.steps.length && (
              <details
                className="task-activity"
                open={run.status === "running"}
              >
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
        {error && <p role="alert">{error}</p>}
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
