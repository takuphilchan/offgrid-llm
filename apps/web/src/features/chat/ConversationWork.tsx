import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
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
import { DocumentProgress } from '../agents/DocumentProgress';
import { BrowserActivity } from '../agents/BrowserActionSummary';
import { TaskContinuation } from '../agents/TaskContinuation';
import { useTaskStatus } from '../agents/useTaskStatus';
import { AgentProgress, AgentPreview } from '../agents/AgentProgress';
import { ExecutionActivity,ExecutionLocation } from '../agents/ExecutionActivity';
import { WorkRecovery } from '../agents/WorkRecovery';
import { SpecialistActivity } from '../agents/SpecialistActivity';
import {WorkChanges} from '../agents/WorkChanges';
import {takeoverOwnedComputer,useBrowserHandoff} from '../agents/browser-handoff';
import {browserManualText} from '../../i18n/browser-manual';

export const workSettled = (status: string) => ['completed', 'failed', 'cancelled'].includes(status);

/** A view of the canonical job, never a second runner or conversation store. */
export function ConversationWork({ run, refresh, connection, readError, outputSaved, recover, recoveryNeeded, liveSubmission = false, inspector = false }: {
  run: AgentRun; refresh: () => void; connection: string; readError: string; outputSaved: boolean;
  recover: () => Promise<void>; recoveryNeeded: boolean;
  liveSubmission?: boolean;
  inspector?: boolean;
}) {
  const { messages, locale } = useI18n(), copy = conversationWorkText[locale], presentation = taskPresentation(locale);
  const { workspace, scope, admin } = useWorkspace();
  const statusText = useTaskStatus();
  const handoff=useBrowserHandoff(run.computer_session),manualCopy=browserManualText(locale);
  const showHandoffSummary=!(run.status==='waiting_for_input'&&run.pending_input?.mode==='browser');
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const [stopping, setStopping] = useState<'idle' | 'pending' | 'unconfirmed'>('idle');
  const [report, setReport] = useState('');
  const [showPreview, setShowPreview] = useState(false);
  const lock = useRef(false), alive = useRef(true);
  const accessCancellation = useMemo(() => new AbortController(), [run.pending_input?.id]);
  const accessIntent = useRef(liveSubmission), [consentPending, setConsentPending] = useState(false);
  useEffect(() => {
    if (!accessIntent.current) return;
    if (!['pending', 'running', 'waiting_for_input'].includes(run.status)) { accessIntent.current = false; return; }
    // Models request installed applications with mode=app; desktop is the
    // pre-admission form. Both attach the same task-bound desktop grant and
    // reuse host-owned Full access. Never widen selected-window/browser work
    // or automatically reattach a lost session during recovery.
    if (run.status !== 'waiting_for_input' || !['desktop', 'app'].includes(run.pending_input?.mode ?? '') || run.pending_input?.previous_session || run.requested_computer_scope !== 'desktop' || window.electron?.desktopComputerProtocol !== 3) return;
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
      const serviceAction=()=>api.jobAction(run.run_id, action, {
          ...(['approve', 'deny'].includes(action) ? { approval_id: run.pending_approval?.id } : {}),
          ...(action === 'reconcile' ? { call_id: run.uncertain_call_id, result: report.trim() } : {}),
        });
      // Even a lost local acknowledgement must not prevent service revocation.
      const results=action==='takeover'
        ?[...await Promise.allSettled([takeoverOwnedComputer(run.computer_session,run.pending_input?.id)]),...await Promise.allSettled([serviceAction()])]
        :await Promise.allSettled([serviceAction(),stopping?stopOwnedComputer(run.computer_session,run.pending_input?.id):Promise.resolve()]);
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
    : statusText(run.status, run.outcome);
  const compactResult = !inspector && outputSaved && run.status === 'completed';
  return <section data-work-state={run.status} data-result-saved={compactResult || undefined} className={`conversation-work${inspector ? ' work-inspector' : ''}`} tabIndex={-1} aria-label={copy.activity}>
    <header><div><span className="work-status" role="status">{status}</span></div>{!inspector && <a className="utility-button" href={`#/agents/task/${encodeURIComponent(run.run_id)}`}>{copy.inspect}</a>}
    {!workSettled(run.status) && <div className="button-row"><TaskLifecycle run={run} busy={busy} act={action => void act(action)} />{run.resumable && !run.computer_session_expired && ['pending', 'interrupted'].includes(run.status) && <button type="button" className="secondary-button" disabled={busy} onClick={() => void act('resume')}>{messages.models.resume}</button>}</div>}</header>
    <DocumentProgress run={run} />
    {showHandoffSummary&&handoff.state==='manual'&&<p role="status">{manualCopy.manual} {manualCopy.hint}</p>}
    {showHandoffSummary&&['taking_over','resuming'].includes(handoff.state)&&<p role="status">{manualCopy.pending}</p>}
    <SpecialistActivity run={run} />
    {run.work_id && <WorkChanges id={run.work_id} revision={run.event_cursor} />}
    <TaskResult run={run} compact hideOutput={outputSaved} explainOutcome={inspector} />
    {inspector && run.output && <p className="workspace-secondary">{presentation.generated}</p>}
    {connection !== 'live' && !workSettled(run.status) && <p role="status">{messages.agentLive.reconnectHint}</p>}
    {stopping !== 'idle' && <ScopedNotice kind={stopping === 'unconfirmed' ? 'error' : 'status'}>{taskStopText(locale)[stopping]}</ScopedNotice>}
    {run.pending_input && run.status === 'waiting_for_input' && <TaskAccess key={run.pending_input.id} run={run} workspace={workspace} onResolved={refresh} cancelSignal={accessCancellation.signal} initialConsentPending={consentPending} />}
    {run.pending_approval && run.status === 'waiting_for_approval' && <TaskApproval key={run.pending_approval.id} run={run} busy={busy || connection !== 'live'} refresh={refresh} act={action => void act(action)} />}
    {run.status === 'uncertain' && <section className="conversation-work-recovery"><p>{copy.uncertain}</p><details><summary>{copy.manualReport}</summary>
      <p>{copy.reportHint}</p><pre>{JSON.stringify(run.uncertain_call, null, 2)}</pre>
      <label>{copy.manualReport}<textarea value={report} onChange={event => setReport(event.target.value)} rows={3} /></label>
      <button type="button" className="secondary-button" disabled={busy || !report.trim()} onClick={() => void act('reconcile')}>{copy.recordReport}</button>
    </details></section>}
    {run.pending_instructions?.length && !(inspector && admin) ? <p role="status">{taskSteering[locale].queued}</p> : null}
    {recoveryNeeded && <ScopedNotice action={<button className="secondary-button" disabled={busy} onClick={async () => { if (lock.current) return; lock.current = true; setBusy(true); try { await recover(); } catch (reason) { setError(reason instanceof Error ? reason.message : messages.common.error); } finally { lock.current = false; if (alive.current) setBusy(false); } }}>{copy.attach}</button>}>{copy.savedJob}</ScopedNotice>}
    <WorkRecovery run={run} readError={readError} actionError={error} refresh={refresh} />
    <details className="conversation-work-details"><summary>{copy.details}{run.steps.length ? ` · ${run.steps.length}` : ''}</summary>
      {run.profile && <small className="conversation-work-profile">{run.profile.name} · {agentProfilesText[locale].revision.replace('{revision}', String(run.profile.revision))}</small>}
      <ExecutionLocation run={run}/>
      <AgentProgress run={run} connection={connection === 'live' ? 'live' : connection === 'connecting' ? 'connecting' : 'reconnecting'} showPreview={showPreview} setShowPreview={setShowPreview} />
      <AgentPreview run={run} showPreview={showPreview} />
      <TaskEvidence run={run} />
      <TaskDetails run={run} onError={setError} />
      {run.computer_session ? <><BrowserActivity steps={run.steps} />{run.steps.filter(step=>step.tool_name?.startsWith('workspace_')).map((step,index)=><ExecutionActivity key={`${step.id??index}:${step.type}`} step={step}/>)}</> : run.steps.map((step, index) => step.tool_name?.startsWith('workspace_')?<ExecutionActivity key={`${step.id??index}:${step.type}`} step={step}/>:<details key={`${step.id ?? index}:${step.type}`}><summary>{step.tool_name || step.type}</summary><pre>{step.tool_result || step.content || presentation.unavailable}</pre></details>)}
      <p className="workspace-secondary">{messages.chat.model}: {run.model}</p>
    </details>
    {inspector && admin && <TaskContinuation key={run.run_id} run={run} scope={`${scope}:workspace:${workspace}`} refresh={refresh} />}
  </section>;
}
