import { useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskControls } from '../../i18n/task-controls';
import { draftKey, readDraft, writeDraft, clearSubmittedDraft, useDraft } from '../../lib/drafts';

export type TaskCommand = Parameters<typeof api.jobAction>[1];

// Server revocation is independent from host IPC. Never stop another task's
// desktop session just because this client has an Electron bridge.
export async function stopOwnedComputer(session?: string, requestId?: string) {
  if (window.electron?.stopComputerAccess) {
    const scopes: ({session:string}|{requestId:string})[] = [];
    if (requestId) scopes.push({requestId});
    if (session) scopes.push({session});
    const results = await Promise.allSettled(scopes.map(async scope => {
      const result = await window.electron!.stopComputerAccess!(scope);
      if (!['stopped', 'not_owned'].includes(result.state)) throw Error(result.code ?? 'computer_stop_unconfirmed');
    }));
    const failure = results.find((r): r is PromiseRejectedResult => r.status === 'rejected');
    if (failure) throw failure.reason;
    return;
  }
  if (!session || !window.electron?.getComputerStatus || !window.electron.stopComputerBrowser) return;
  const host = await window.electron.getComputerStatus();
  if (host.target?.id === session) {
    const result = await window.electron.stopComputerBrowser();
    if (result.state !== 'stopped') throw Error(result.code ?? 'computer_stop_unconfirmed');
  }
}

export function TaskLifecycle({run, busy, act}: {run: AgentRun; busy: boolean; act: (action: TaskCommand) => void}) {
  const {locale} = useI18n(), copy = taskControls(locale);
  const active = ['pending', 'running', 'waiting_for_approval', 'waiting_for_children'].includes(run.status);
  const stoppable = [...['pending', 'running', 'waiting_for_approval', 'waiting_for_children'], 'waiting_for_input', 'interrupted'].includes(run.status);
  return <>
    {active && <button className="secondary-button" disabled={busy} onClick={() => act('pause')}>{copy.pause}</button>}
    {run.computer_session && stoppable && !run.computer_session_expired && <button className="secondary-button" disabled={busy} onClick={() => act('takeover')}>{copy.takeover}</button>}
    {stoppable && <button className="secondary-button" disabled={busy} onClick={() => act('cancel')}>{copy.stop}</button>}
    {run.status === 'interrupted' && run.last_access && run.computer_session_expired && <button className="primary-button" disabled={busy} onClick={() => act('reconnect')}>{copy.reconnect}</button>}
  </>;
}

export function TaskDetails({run, scope, refresh, onError, childStatus}: {run: AgentRun; scope: string; refresh: () => void; onError: (message: string) => void; childStatus: (id: string) => string}) {
  const {locale, messages} = useI18n(), copy = taskControls(locale);
  const draft = useDraft(scope, `task-instruction:${run.run_id}`);
  const [saving, setSaving] = useState(false), [saved, setSaved] = useState(false);
  const steer = async () => {
    if (saving || !draft.value.trim()) return;
    setSaving(true); setSaved(false);
    const instruction = draft.value.trim(), submitted = draft.value;
    const key = draftKey(scope, `task-instruction-request:${run.run_id}`);
    let request: {id: string; instruction: string} | undefined;
    try { request = JSON.parse(readDraft(key)); } catch {}
    if (!request || request.instruction !== instruction) request = {id: crypto.randomUUID(), instruction};
    writeDraft(key, JSON.stringify(request));
    try {
      await api.jobAction(run.run_id, 'steer', {request_id: request.id, instruction});
      clearSubmittedDraft(key, JSON.stringify(request)); clearSubmittedDraft(draft.key, submitted);
      setSaved(true); refresh();
    } catch (e) { onError(e instanceof Error ? e.message : messages.common.error); }
    finally { setSaving(false); }
  };
  const exportEvidence = async () => {
    try {
      const value = await api.exportJob(run.run_id);
      const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], {type: 'application/json'}));
      const a = document.createElement('a'); a.href = url; a.download = `${run.run_id}.json`; a.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) { onError(e instanceof Error ? e.message : messages.common.error); }
  };
  return <>
    {!!run.artifacts?.length && <section className="task-artifacts"><ul>{run.artifacts.map((artifact, index) => <li key={`${artifact.sha256}:${index}`}><a download href={`/api/v2/jobs/${encodeURIComponent(run.run_id)}/artifact?digest=${encodeURIComponent(artifact.sha256)}`}>{artifact.name}</a><small> · {artifact.bytes.toLocaleString(locale)} B · SHA256: {artifact.sha256.slice(0,12)}</small></li>)}</ul></section>}
    {!!run.children?.length && <section aria-label={copy.children}>
      <h3>{copy.children}</h3>
      <ul className="task-children">{run.children.map(child => <li key={child.id}>
        <a href={`#/agents/task/${child.id}`}>{child.spec.goal}</a><small>{childStatus(child.id)}</small>
      </li>)}</ul>
    </section>}
    {run.parent_id && <a href={`#/agents/task/${run.parent_id}`}>{copy.plan}</a>}
    {!!run.plan?.length && <details><summary>{copy.plan}</summary><p>{copy.planHint}</p><ol>{run.plan.map(item => <li key={item.id}>{item.title} · {item.state === 'done' ? messages.recovery.completed : item.state === 'active' ? messages.recovery.running : messages.recovery.pending}{!!item.evidence_steps?.length && <small> · #{item.evidence_steps.join(', #')}</small>}</li>)}</ol></details>}
    {(run.can_steer ?? (run.status === 'interrupted' && !run.uncertain_call_id && !run.children?.length)) && <section className="task-steering">
      <label className="field"><span>{copy.instruction}</span><textarea rows={3} value={draft.value} maxLength={16000} onChange={e => {draft.setValue(e.target.value); setSaved(false);}} /></label>
      <button className="secondary-button" disabled={saving || !draft.value.trim()} onClick={() => void steer()}>{saving ? messages.common.loading : copy.save}</button>
      {saved && <p role="status">{copy.saved}</p>}
    </section>}
    {!!run.instructions?.length && <details><summary>{copy.instruction}</summary><ol>{run.instructions.map(item => <li key={item.request_id}>{item.text}</li>)}</ol></details>}
    <details><summary>{copy.context}</summary>
      {run.context && <dl><dt>{copy.tokens}</dt><dd>{run.context.estimated_tokens.toLocaleString(locale)} / {run.context.window.toLocaleString(locale)}</dd><dt>{copy.archived}</dt><dd>{run.context.archived_messages}</dd></dl>}
      <button className="secondary-button" onClick={() => void exportEvidence()}>{copy.exportEvidence}</button>
    </details>
  </>;
}
