import { useEffect, useRef, useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskControls } from '../../i18n/task-controls';
import { taskSteering } from '../../i18n/task-steering';
import { taskPresentation } from '../../i18n/task-presentation';
import { VoiceInputButton } from '../../components/VoiceInputButton';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { draftKey, readDraft, writeDraft, clearSubmittedDraft, useDraft } from '../../lib/drafts';

export function TaskContinuation({ run, scope, refresh }: { run: AgentRun; scope: string; refresh: () => void }) {
  const { locale, messages } = useI18n(), copy = taskControls(locale), presentation = taskPresentation(locale);
  const draft = useDraft(scope, `task-instruction:${run.run_id}`);
  const key = draftKey(scope, `task-instruction-request:${run.run_id}`);
  const [saving, setSaving] = useState(false), [saved, setSaved] = useState(false);
  const [acceptedPending, setAcceptedPending] = useState<string | null>(null);
  const [error, setError] = useState('');
  const lock = useRef(false), mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const canSteer = run.can_steer ?? (run.status === 'interrupted' && !run.uncertain_call_id && !run.children?.length);
  useEffect(() => {
    if (acceptedPending && run.instructions?.some(item => item.request_id === acceptedPending)) setAcceptedPending(null);
    // An accepted instruction can outlive its HTTP acknowledgment. Inspect the
    // same saved job; never create a second job or silently send another request.
    try {
      const request = JSON.parse(readDraft(key));
      if ([...(run.instructions ?? []), ...(run.pending_instructions ?? [])].some(item => item.request_id === request.id && item.text === request.instruction)) {
        setAcceptedPending(run.pending_instructions?.some(item => item.request_id === request.id) ? request.id : null);
        clearSubmittedDraft(key, JSON.stringify(request));
        if (draft.value.trim() === request.instruction) clearSubmittedDraft(draft.key, draft.value);
        setSaved(true);
        setError('');
      }
    } catch { /* No pending request. */ }
  }, [run.instructions, run.pending_instructions, key, draft.key, draft.value, acceptedPending]);
  const steer = async () => {
    if (lock.current || !canSteer || !draft.value.trim()) return;
    lock.current = true; setSaving(true); setSaved(false); setError('');
    const instruction = draft.value.trim(), submitted = draft.value;
    let request: { id: string; instruction: string } | undefined;
    try { request = JSON.parse(readDraft(key)); } catch {}
    if (!request || request.instruction !== instruction) request = { id: crypto.randomUUID(), instruction };
    writeDraft(key, JSON.stringify(request));
    try {
      const accepted = await api.jobAction(run.run_id, 'steer', { request_id: request.id, instruction });
      clearSubmittedDraft(key, JSON.stringify(request)); clearSubmittedDraft(draft.key, submitted);
      if (mounted.current) { setAcceptedPending(accepted.pending_instructions?.some(item => item.request_id === request.id) ? request.id : null); setSaved(true); refresh(); }
    } catch (e) {
      if (mounted.current) { setError(e instanceof Error ? e.message : messages.common.error); refresh(); }
    } finally { lock.current = false; if (mounted.current) setSaving(false); }
  };
  if (!canSteer && !['completed', 'failed', 'cancelled', 'interrupted'].includes(run.status)) return null;
  return <section className="task-steering" aria-label={presentation.next}>
    <h3>{presentation.next}</h3>
    {canSteer ? <>
      <label className="field"><span>{copy.instruction}</span><textarea rows={3} value={draft.value} maxLength={16000} onChange={e => { draft.setValue(e.target.value); setSaved(false); }} /></label>
      <div className="button-row"><VoiceInputButton contextKey={draft.key} disabled={saving} onTranscript={value => { draft.setValue(draft.value ? `${draft.value} ${value}` : value); setSaved(false); }} /><button className="secondary-button" disabled={saving || !draft.value.trim()} onClick={() => void steer()}>{saving ? messages.common.loading : copy.save}</button></div>
      {(acceptedPending || !!run.pending_instructions?.length) && <p role="status">{taskSteering[locale].queued}</p>}
      {saved && !acceptedPending && !run.pending_instructions?.length && <p role="status">{run.status === 'interrupted' ? copy.saved : taskSteering[locale].applied}</p>}
      {draft.unsaved && <p role="alert">{messages.recovery.draftWarning}</p>}
      {error && <ScopedNotice kind="error">{error}</ScopedNotice>}
    </> : <>
      <p>{presentation.newTaskHint}</p>
      <a className="secondary-button" href="#/chat">{messages.nav.chat}</a>
    </>}
  </section>;
}
