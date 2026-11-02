import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { useI18n } from '../../i18n';
import { documentRetention } from '../../i18n/document-retention';
import './document-retention.css';

type Policy = Awaited<ReturnType<typeof api.documentRetention>>;

export function DocumentRetentionDialog({ id, name, close }: { id: string; name: string; close: () => void }) {
  const { locale, messages: text } = useI18n();
  const copy = documentRetention(locale);
  const dialog = useRef<HTMLDialogElement>(null);
  const alive = useRef(false);
  const [policy, setPolicy] = useState<Policy>();
  const [expiry, setExpiry] = useState('');
  const [keep, setKeep] = useState(true);
  const [hold, setHold] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const element = dialog.current!;
    alive.current = true;
    element.showModal();
    return () => { alive.current = false; element.close(); previous?.focus(); };
  }, []);
  useEffect(() => {
    let active = true;
    setPolicy(undefined); setError('');
    void api.documentRetention(id).then(value => {
      if (!active) return;
      setPolicy(value); setKeep(value.delete_after === 0); setHold(value.legal_hold);
      setExpiry(value.delete_after ? new Date(value.delete_after * 1000).toISOString().slice(0, 19) : '');
    }).catch(reason => { if (active) setError(reason instanceof Error ? reason.message : text.common.error); });
    return () => { active = false; };
  }, [id, reload, text.common.error]);
  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!policy || busy) return;
    const deleteAfter = keep ? 0 : Date.parse(`${expiry}Z`) / 1000;
    if (!Number.isSafeInteger(deleteAfter) || deleteAfter < 0 || deleteAfter > 253402300799) return;
    setBusy(true); setError('');
    try {
      await api.setDocumentRetention(id, { ...policy, delete_after: deleteAfter, legal_hold: hold });
      if (alive.current) close();
    } catch (reason) {
      if (alive.current) {
        // A lost update receipt is not permission to retry a mutation. Read the
        // current revision explicitly before another save, including conflicts.
        setPolicy(undefined); setError(reason instanceof Error ? reason.message : text.common.error);
      }
    } finally { if (alive.current) setBusy(false); }
  };
  return <dialog ref={dialog} className="document-retention-dialog" aria-labelledby="retention-title" onCancel={event => { event.preventDefault(); if (!busy) close(); }}>
    <header><h2 id="retention-title">{copy.title}</h2><p>{name}</p></header>
    {error && <div role="alert"><p>{error}</p><button type="button" className="text-button" disabled={busy} onClick={() => setReload(value => value + 1)}>{copy.reload}</button></div>}
    {!policy && !error && <p role="status">{text.common.loading}</p>}
    <form onSubmit={event => void save(event)}>
      <fieldset disabled={busy || !policy}>
        <label className="retention-check"><input type="checkbox" checked={keep} onChange={event => setKeep(event.target.checked)} />{copy.keep}</label>
        {!keep && <label className="field">{copy.expiry}<input type="datetime-local" step="1" min="1970-01-01T00:00:01" max="9999-12-31T23:59:59" required value={expiry} onChange={event => setExpiry(event.target.value)} /></label>}
        <label className="retention-check"><input type="checkbox" checked={hold} onChange={event => setHold(event.target.checked)} />{copy.hold}</label>
        <p className="workspace-secondary">{copy.holdHint}</p>
      </fieldset>
      <p className="workspace-secondary">{copy.effect}</p>
      <footer><button autoFocus type="button" className="secondary-button" disabled={busy} onClick={close}>{text.models.cancel}</button><button className="primary-button" type="submit" disabled={busy || !policy}>{busy ? text.common.loading : copy.save}</button></footer>
    </form>
  </dialog>;
}
