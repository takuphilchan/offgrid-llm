import { useEffect, useId, useRef, useState } from 'react';
import { APIError } from '../api/client';
import { useI18n } from '../i18n';

export type HistoryItem = { id: string; label: string };

// Delete exactly the snapshot the user confirmed, not items added in another
// tab afterwards. Partial failure keeps only failed items selected for retry.
export function HistoryDeleteDialog({ items, kind, remove, onDeleted, onClose }: {
  items: HistoryItem[]; kind: 'chats' | 'tasks'; remove: (id: string) => Promise<unknown>;
  onDeleted: (ids: string[]) => void; onClose: () => void;
}) {
  const { messages: text } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const locked = useRef(false);
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
    locked.current = true; setBusy(true); setFailed(false);
    const removed: string[] = [], failures: HistoryItem[] = [];
    for (const item of remaining) {
      try { await remove(item.id); removed.push(item.id); }
      catch (reason) {
        if (reason instanceof APIError && reason.status === 404) removed.push(item.id);
        else failures.push(item);
      }
    }
    onDeleted(removed);
    locked.current = false; setBusy(false);
    if (!failures.length) onClose();
    else { setRemaining(failures); setFailed(true); }
  };
  return <dialog ref={dialog} className="history-delete-dialog" aria-labelledby={`${id}-title`} aria-describedby={`${id}-body`} onCancel={event => { event.preventDefault(); if (!locked.current) onClose(); }}>
    <h2 id={`${id}-title`}>{text.history.confirmTitle}</h2>
    <p id={`${id}-body`}>{(kind === 'chats' ? text.history.deleteChatsBody : text.history.deleteTasksBody).replace('{count}', String(remaining.length))}</p>
    <ul>{remaining.slice(0, 5).map(item => <li key={item.id}>{item.label}</li>)}{remaining.length > 5 && <li>+{remaining.length - 5}</li>}</ul>
    {failed && <p role="alert">{text.history.partialFailure.replace('{count}', String(remaining.length))}</p>}
    {busy && <p role="status">{text.history.deleting}</p>}
    <div className="history-dialog-actions"><button ref={cancel} className="secondary-button" disabled={busy} onClick={onClose}>{text.models.cancel}</button><button className="danger-button" disabled={busy} onClick={() => void confirm()}>{busy ? text.history.deleting : text.models.confirmDelete}</button></div>
  </dialog>;
}
