import {useEffect, useRef, useState} from 'react';
import {api} from '../../api/client';
import type {components} from '../../api/schema.generated';
import {useI18n} from '../../i18n';
import {conversationWorkText} from '../../i18n/conversation-work';
import {taskSteering} from '../../i18n/task-steering';
import {draftKey, clearSubmittedDraft, useDraft} from '../../lib/drafts';
import {useDraftScope} from '../../lib/draft-scope';
import {pendingWorkerCommand, sendWorkerCommand} from './worker-command';
import {SavedResult} from './SavedResult';

type Worker = components['schemas']['WorkerSession'];
type Message = components['schemas']['WorkerMessage'];
const terminal = (status: string) => ['completed', 'failed', 'cancelled'].includes(status);

// Mounted only inside expanded worker activity. Observation cannot continue a
// worker, apply mailbox input or acquire execution authority.
export function WorkerControls({id, parent, runID, parentStatus, refresh}: {id: string; parent: string; runID: string; parentStatus: string; refresh: () => void}) {
  const {messages, locale} = useI18n(), copy = conversationWorkText[locale];
  const scope = useDraftScope(), draft = useDraft(scope, 'worker-message', id);
  const [worker, setWorker] = useState<Worker | null>(null), [mail, setMail] = useState<Message[]>([]);
  const [error, setError] = useState(''), [busy, setBusy] = useState(false), [version, setVersion] = useState(0);
  const commandKey = (kind: string) => draftKey(scope, `worker-command-${kind}`, id);
  const pendingCommand = ['stop', 'continue', 'message', 'answer'].map(kind => pendingWorkerCommand(commandKey(kind), id)).find(Boolean);
  const alive = useRef(true), locked = useRef(false);
  useEffect(() => {alive.current = true; return () => {alive.current = false;};}, []);
  useEffect(() => {
    let disposed = false, pending = false;
    const controller = new AbortController();
    const observe = async () => {
      if (pending) return;
      pending = true;
      try {
        const current = await api.conversations.worker(id, controller.signal);
        if (current.root_run_id !== parent && !current.run_ids.includes(runID)) throw new Error(messages.common.error);
        const entries: Message[] = [];
        for (let cursor = 0; ;) {
          const page = await api.conversations.workerMessages(id, cursor, 100, controller.signal);
          entries.push(...page.items);
          if (!page.has_more) break;
          if (page.next_cursor <= cursor) throw new Error(messages.common.error);
          cursor = page.next_cursor;
        }
        if (!disposed) {setWorker(current); setMail(entries);}
      } catch (reason) {if (!disposed) {setWorker(null); setMail([]); setError(reason instanceof Error ? reason.message : messages.common.error);}}
      finally {pending = false;}
    };
    void observe(); const timer = window.setInterval(() => void observe(), 3000);
    return () => {disposed = true; controller.abort(); window.clearInterval(timer);};
  }, [id, parent, runID, version]);
  const act = async (kind: 'message' | 'continue' | 'stop' | 'answer', retry = false) => {
    const saved = retry ? pendingWorkerCommand(commandKey(kind), id) : null;
    if ((!worker && !saved) || locked.current) return;
    locked.current = true; setBusy(true); setError('');
    const content = saved ? saved.payload.content : kind === 'stop' ? undefined : draft.value.trim(), submitted = draft.value;
    try {
      const questionID = kind === 'answer' ? saved?.payload.question_id ?? worker?.question?.id : undefined;
      const receipt = await sendWorkerCommand(commandKey(kind), saved ?? {request_id: crypto.randomUUID(), target_id: id, expected_revision: worker!.revision, kind, payload: {content, question_id: questionID}});
      if (kind !== 'stop' && content === submitted.trim() && (kind !== 'answer' || questionID === worker?.question?.id)) clearSubmittedDraft(draft.key, submitted);
      if (alive.current) {if (receipt.worker) setWorker(current => !current || receipt.worker!.revision >= current.revision ? receipt.worker! : current); setVersion(value => value + 1); refresh();}
    } catch (reason) {if (alive.current) setError(reason instanceof Error ? reason.message : messages.common.error);}
    finally {locked.current = false; if (alive.current) setBusy(false);}
  };
  const settled = worker && terminal(worker.status);
  const canContinue = settled && ['pending', 'running', 'waiting_for_children', 'completed', 'failed'].includes(parentStatus);
  const canMessage = worker && !worker.stopped && ['pending', 'running', 'interrupted'].includes(worker.status) && parentStatus === 'waiting_for_children';
  const canAnswer = worker && !worker.stopped && worker.status === 'waiting_for_input' && worker.question?.state === 'pending' && ['waiting_for_children', 'waiting_for_input'].includes(parentStatus);
  return <div className="worker-controls">
    {worker && !worker.closed && <>
      {canAnswer && <p className="worker-question" role="status">{worker.question!.content}</p>}
      {(canMessage || canContinue || canAnswer) && <label>{copy.update}<textarea value={draft.value} onChange={event => draft.setValue(event.target.value)} rows={2} maxLength={16000} disabled={busy} /></label>}
      <div className="button-row">
        {canAnswer && <button type="button" className="secondary-button" disabled={busy || !!pendingCommand || !draft.value.trim()} onClick={() => void act('answer')}>{copy.update}</button>}
        {canMessage && <button type="button" className="secondary-button" disabled={busy || !draft.value.trim()} onClick={() => void act('message')}>{copy.update}</button>}
        {canContinue && <button type="button" className="secondary-button" disabled={busy || !draft.value.trim()} onClick={() => void act('continue')}>{messages.models.resume}</button>}
        {!settled && !worker.stopped && <button type="button" className="secondary-button" disabled={busy} onClick={() => void act('stop')}>{messages.chat.stop}</button>}
      </div>
    </>}
    {mail.map(message => <p key={message.id} role="status">{message.content} — {message.state === 'applied' ? taskSteering[locale].applied : message.state === 'accepted' || message.state === 'delivered' ? taskSteering[locale].queued : message.resolution_reason ?? message.state}</p>)}
    {error && <p role="alert">{error}</p>}
    {pendingCommand && <button type="button" className="secondary-button" disabled={busy} onClick={() => void act(pendingCommand.kind as 'message' | 'continue' | 'stop' | 'answer', true)}>{messages.common.retry}</button>}
    {error && <button type="button" className="text-button" onClick={() => {setError(''); setVersion(value => value + 1);}}>{messages.common.refresh}</button>}
    {worker?.result_reference && <SavedResult key={worker.result_reference} reference={worker.result_reference} />}
    {worker && <details><summary>{copy.details}</summary><pre>{JSON.stringify({worker, messages: mail}, null, 2)}</pre></details>}
  </div>;
}
