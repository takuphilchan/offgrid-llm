import { useEffect, useRef, useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskControls } from '../../i18n/task-controls';
import { taskPresentation } from '../../i18n/task-presentation';
import { VoiceInputButton } from '../../components/VoiceInputButton';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { draftKey, readDraft, writeDraft, clearSubmittedDraft, useDraft } from '../../lib/drafts';

export function TaskContinuation({ run, scope, refresh, newDraftExists, createDraft }: { run: AgentRun; scope: string; refresh: () => void; newDraftExists: boolean; createDraft: (text: string) => void }) {
  const { locale, messages } = useI18n(), copy = taskControls(locale), presentation = taskPresentation(locale);
  const draft = useDraft(scope, `task-instruction:${run.run_id}`);
  const key = draftKey(scope, `task-instruction-request:${run.run_id}`);
  const [saving, setSaving] = useState(false), [saved, setSaved] = useState(false);
  const [error, setError] = useState('');
  const lock = useRef(false), mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const canSteer = run.can_steer ?? (run.status === 'interrupted' && !run.uncertain_call_id && !run.children?.length);
  useEffect(() => {
    // An accepted instruction can outlive its HTTP acknowledgment. Inspect the
    // same saved job; never create a second job or silently send another request.
    try {
      const request = JSON.parse(readDraft(key));
      if (run.instructions?.some(item => item.request_id === request.id && item.text === request.instruction)) {
        clearSubmittedDraft(key, JSON.stringify(request));
        if (draft.value.trim() === request.instruction) clearSubmittedDraft(draft.key, draft.value);
        setSaved(true);
        setError('');
      }
    } catch { /* No pending request. */ }
  }, [run.instructions, key, draft.key, draft.value]);
  const steer = async () => {
    if (lock.current || !canSteer || !draft.value.trim()) return;
    lock.current = true; setSaving(true); setSaved(false); setError('');
    const instruction = draft.value.trim(), submitted = draft.value;
    let request: { id: string; instruction: string } | undefined;
    try { request = JSON.parse(readDraft(key)); } catch {}
    if (!request || request.instruction !== instruction) request = { id: crypto.randomUUID(), instruction };
    writeDraft(key, JSON.stringify(request));
    try {
      await api.jobAction(run.run_id, 'steer', { request_id: request.id, instruction });
      clearSubmittedDraft(key, JSON.stringify(request)); clearSubmittedDraft(draft.key, submitted);
      if (mounted.current) { setSaved(true); refresh(); }
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
      {saved && <p role="status">{copy.saved}</p>}
      {draft.unsaved && <p role="alert">{messages.recovery.draftWarning}</p>}
      {error && <ScopedNotice kind="error">{error}</ScopedNotice>}
    </> : <>
      <p>{presentation.newTaskHint}</p>
      <button className="secondary-button" disabled={newDraftExists} title={newDraftExists ? messages.history.draftProtected : undefined} onClick={() => createDraft([run.prompt, draft.value.trim()].filter(Boolean).join('\n\n'))}>{presentation.newTask}</button>
    </>}
  </section>;
}
