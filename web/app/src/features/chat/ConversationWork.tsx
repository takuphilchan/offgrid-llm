import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskControls } from '../../i18n/task-controls';
import { taskAccessText } from '../../i18n/task-access';
import { taskStopText, taskPresentation } from '../../i18n/task-presentation';
import { taskSteering } from '../../i18n/task-steering';
import { TaskAccess } from '../agents/TaskAccess';
import { TaskApproval } from '../agents/TaskApproval';
import { TaskLifecycle, TaskDetails, stopOwnedComputer, type TaskCommand } from '../agents/TaskControls';
import { TaskResult } from '../agents/TaskResult';
import { TaskEvidence } from '../agents/TaskEvidence';
import { useWorkspace } from '../../lib/workspace-context';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { conversationWorkText } from '../../i18n/conversation-work';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { taskEvidenceText } from '../../i18n/task-evidence';
import { grantTaskDesktop } from '../agents/TaskDesktopAccess';
import './conversation-work.css';

export const workSettled = (status: string) => ['completed', 'failed', 'cancelled'].includes(status);

/** A view of the canonical job, never a second runner or conversation store. */
export function ConversationWork({ run, refresh, connection, readError, outputSaved, recover, recoveryNeeded, liveSubmission = false }: {
  run: AgentRun; refresh: () => void; connection: string; readError: string; outputSaved: boolean;
  recover: () => Promise<void>; recoveryNeeded: boolean;
  liveSubmission?: boolean;
}) {
  const { messages, locale } = useI18n(), copy = conversationWorkText[locale], controls = taskControls(locale), presentation = taskPresentation(locale);
  const { workspace } = useWorkspace();
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const [stopping, setStopping] = useState<'idle' | 'pending' | 'unconfirmed'>('idle');
  const [report, setReport] = useState('');
  const lock = useRef(false), alive = useRef(true);
  const accessCancellation = useMemo(() => new AbortController(), [run.pending_input?.id]);
  const accessIntent = useRef(liveSubmission), [consentPending, setConsentPending] = useState(false);
  useEffect(() => {
    if (!accessIntent.current) return;
    if (!['pending', 'running', 'waiting_for_input'].includes(run.status)) { accessIntent.current = false; return; }
    if (run.pending_input?.mode !== 'desktop' || run.requested_computer_scope !== 'desktop' || window.electron?.desktopComputerProtocol !== 3) return;
    accessIntent.current = false; setConsentPending(true);
    void grantTaskDesktop(run, workspace, accessCancellation.signal).then(refresh)
      .catch(reason => { if (alive.current && !accessCancellation.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); })
      .finally(() => { if (alive.current) setConsentPending(false); });
  }, [run, workspace, accessCancellation]);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const act = async (action: TaskCommand) => {
    if (lock.current) return;
    lock.current = true; setBusy(true); setError('');
    const stopping = action === 'cancel' || action === 'takeover';
    if (stopping) { accessIntent.current = false; setStopping('pending'); accessCancellation.abort(); }
    try {
      const results = await Promise.allSettled([
        api.jobAction(run.run_id, action, {
          ...(['approve', 'deny'].includes(action) ? { approval_id: run.pending_approval?.id } : {}),
          ...(action === 'reconcile' ? { call_id: run.uncertain_call_id, result: report.trim() } : {}),
        }),
        stopping ? stopOwnedComputer(run.computer_session, run.pending_input?.id) : Promise.resolve(),
      ]);
      if (!alive.current) return;
      refresh();
      const failure = results.find((result): result is PromiseRejectedResult => result.status === 'rejected');
      if (failure) throw failure.reason;
      setStopping('idle');
    } catch (reason) {
      if (alive.current) { setError(reason instanceof Error ? reason.message : messages.common.error); if (stopping) setStopping('unconfirmed'); }
    } finally { lock.current = false; if (alive.current) setBusy(false); }
  };
  const status = run.status === 'completed' && run.verification?.state && run.verification.state !== 'checks_passed'
    ? taskEvidenceText(locale)[run.verification.state]
    : run.status === 'completed' && run.outcome === 'answer_only' ? taskAccessText(locale).answered
    : run.status === 'waiting_for_children' ? controls.waiting : messages.recovery[run.status as keyof typeof messages.recovery] ?? run.status;
  return <section className="conversation-work" aria-label={copy.activity}>
    <header><div><span role="status">{status}</span>{run.profile && <small className="conversation-work-profile">{run.profile.name} · {agentProfilesText[locale].revision.replace('{revision}', String(run.profile.revision))}</small>}</div><a className="secondary-button" href={`#/agents/task/${encodeURIComponent(run.run_id)}`}>{copy.inspect}</a></header>
    {!workSettled(run.status) && <div className="button-row"><TaskLifecycle run={run} busy={busy} act={action => void act(action)} />{run.resumable && run.status === 'interrupted' && <button type="button" className="secondary-button" disabled={busy} onClick={() => void act('resume')}>{messages.models.resume}</button>}</div>}
    {!!run.children?.length && <div className="conversation-work-delegates" aria-label={controls.children}><strong>{controls.children}</strong><ol>{run.children.map(child => <li key={child.id}><a href={`#/agents/task/${encodeURIComponent(child.id)}`}>{child.spec.goal}</a></li>)}</ol></div>}
    {connection !== 'live' && !workSettled(run.status) && <p role="status">{messages.agentLive.reconnectHint}</p>}
    {stopping !== 'idle' && <ScopedNotice kind={stopping === 'unconfirmed' ? 'error' : 'status'}>{taskStopText(locale)[stopping]}</ScopedNotice>}
    {run.pending_input && run.status === 'waiting_for_input' && <TaskAccess key={run.pending_input.id} run={run} workspace={workspace} onResolved={refresh} cancelSignal={accessCancellation.signal} initialConsentPending={consentPending} />}
    {run.pending_approval && run.status === 'waiting_for_approval' && <TaskApproval key={run.pending_approval.id} run={run} busy={busy || connection !== 'live'} refresh={refresh} act={action => void act(action)} />}
    {run.status === 'uncertain' && <section className="conversation-work-recovery"><p>{copy.uncertain}</p><details><summary>{copy.manualReport}</summary>
      <p>{copy.reportHint}</p><pre>{JSON.stringify(run.uncertain_call, null, 2)}</pre>
      <label>{copy.manualReport}<textarea value={report} onChange={event => setReport(event.target.value)} rows={3} /></label>
      <button type="button" className="secondary-button" disabled={busy || !report.trim()} onClick={() => void act('reconcile')}>{copy.recordReport}</button>
    </details></section>}
    {run.pending_instructions?.length ? <p role="status">{taskSteering[locale].queued}</p> : null}
    <TaskResult run={run} compact hideOutput={outputSaved} />
    {recoveryNeeded && <ScopedNotice action={<button className="secondary-button" disabled={busy} onClick={async () => { if (lock.current) return; lock.current = true; setBusy(true); try { await recover(); } catch (reason) { setError(reason instanceof Error ? reason.message : messages.common.error); } finally { lock.current = false; if (alive.current) setBusy(false); } }}>{copy.attach}</button>}>{copy.savedJob}</ScopedNotice>}
    {(run.error || error || readError) && <ScopedNotice kind="error" action={<button type="button" className="secondary-button" onClick={refresh}>{messages.common.refresh}</button>}>{error || readError || run.error}</ScopedNotice>}
    <details className="conversation-work-details"><summary>{copy.details}{run.steps.length ? ` · ${run.steps.length}` : ''}</summary>
      <TaskEvidence run={run} />
      <TaskDetails run={run} onError={setError} childStatus={() => controls.waiting} />
      {!!run.progress?.preview && <details><summary>{messages.agentLive.provisional}</summary><pre>{run.progress.preview}</pre></details>}
      {run.steps.map((step, index) => <details key={`${step.id ?? index}:${step.type}`}><summary>{step.tool_name || step.type}</summary><pre>{step.tool_result || step.content || presentation.unavailable}</pre></details>)}
    </details>
  </section>;
}
