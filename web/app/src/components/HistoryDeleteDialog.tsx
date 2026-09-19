import { useEffect, useId, useRef, useState } from 'react';
import { APIError } from '../api/client';
import { useI18n } from '../i18n';
import { interaction } from '../i18n/interaction';

export type HistoryItem = { id: string; label: string };

// Delete exactly the snapshot the user confirmed, not items added in another
// tab afterwards. Partial failure keeps only failed items selected for retry.
export function HistoryDeleteDialog({ items, kind, remove, onDeleted, onClose }: {
  items: HistoryItem[]; kind: 'chats' | 'tasks'; remove: (id: string) => Promise<unknown>;
  onDeleted: (ids: string[]) => void; onClose: () => void;
}) {
  const { messages: text, locale } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const locked = useRef(false);
  const stopRequested = useRef(false);
  const [stopping, setStopping] = useState(false);
  const [processed, setProcessed] = useState(0);
  const [remaining, setRemaining] = useState(items);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const id = useId();
  useEffect(() => {
    const element = dialog.current!;
    element.showModal(); cancel.current?.focus();
    return () => element.close();
  }, []);
  const confirm = async () => {
    if (locked.current) return;
    locked.current = true; stopRequested.current = false; setStopping(false); setProcessed(0); setBusy(true); setFailed(false);
    const removed: string[] = [], failures: HistoryItem[] = [];
    let failure = false;
    for (let index = 0; index < remaining.length; index++) {
      if (stopRequested.current) { failures.push(...remaining.slice(index)); break; }
      const item = remaining[index];
      try { await remove(item.id); removed.push(item.id); }
      catch (reason) {
        if (reason instanceof APIError && reason.status === 404) removed.push(item.id);
        else { failures.push(item); failure = true; }
      }
      setProcessed(index + 1);
    }
    onDeleted(removed);
    locked.current = false; setBusy(false);
    if (!failures.length) onClose();
    else { setRemaining(failures); setFailed(failure); }
  };
  return <dialog ref={dialog} className="history-delete-dialog" aria-labelledby={`${id}-title`} aria-describedby={`${id}-body`} onCancel={event => { event.preventDefault(); if (!locked.current) onClose(); }}>
    <h2 id={`${id}-title`}>{text.history.confirmTitle}</h2>
    <p id={`${id}-body`}>{(kind === 'chats' ? text.history.deleteChatsBody : text.history.deleteTasksBody).replace('{count}', String(remaining.length))}</p>
    <ul>{remaining.slice(0, 5).map(item => <li key={item.id}>{item.label}</li>)}{remaining.length > 5 && <li>+{remaining.length - 5}</li>}</ul>
    {failed && <p role="alert">{text.history.partialFailure.replace('{count}', String(remaining.length))}</p>}
    {busy && <p role="status">{interaction[locale].deleteProgress.replace('{done}', String(processed)).replace('{total}', String(remaining.length))}</p>}
    <div className="history-dialog-actions"><button ref={cancel} className="secondary-button" disabled={stopping && busy} onClick={() => { if (busy) { stopRequested.current = true; setStopping(true); } else onClose(); }}>{busy ? interaction[locale].stopAfterCurrent : text.models.cancel}</button><button className="danger-button" disabled={busy} onClick={() => void confirm()}>{busy ? text.history.deleting : text.models.confirmDelete}</button></div>
  </dialog>;
}
