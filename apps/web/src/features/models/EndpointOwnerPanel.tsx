import { useEffect, useRef, useState, type ReactNode } from 'react';
import { api } from '../../api/client';
import type { EndpointOwnerDraft, EndpointOwnerEntry, EndpointOwnerInput, EndpointOwnerRecord, EndpointOwnerSnapshot } from '../../api/endpoint-owner';
import { useI18n } from '../../i18n';
import { endpointOwner } from '../../i18n/endpoint-owner';
import { endpointMaintenance } from '../../i18n/endpoint-maintenance';
import { workspaceRecovery } from '../../i18n/workspace-recovery';
import { restoreFocus } from '../../lib/restore-focus';
import { EndpointOwnerForm } from './EndpointOwnerForm';
import { EndpointOwnerDetails } from './EndpointOwnerDetails';
import { EndpointOwnerHistory } from './EndpointOwnerHistory';
import './endpoint-owner.css';

function OwnerDialog({ title, busy, close, children }: { title: string; busy: boolean; close: () => void; children: ReactNode }) {
 const dialog = useRef<HTMLDialogElement>(null); const { messages } = useI18n();
 useEffect(() => { const node = dialog.current!; const previous = document.activeElement as HTMLElement | null; node.showModal(); return () => { node.close(); restoreFocus(previous); }; }, []);
 return <dialog ref={dialog} className="history-delete-dialog endpoint-owner-dialog" aria-label={title} aria-busy={busy} onCancel={event => { event.preventDefault(); if (!busy) close(); }}><h2>{title}</h2>{children}<div className="history-dialog-actions"><button className="secondary-button" disabled={busy} onClick={close}>{messages.models.cancel}</button></div></dialog>;
}

/** This panel uses the owner protocol independently of the ordinary access-policy list. */
export function EndpointOwnerPanel({ onChanged }: { onChanged: () => Promise<void> }) {
 const { locale, messages } = useI18n(); const copy = endpointOwner[locale];
 const maintenanceCopy = endpointMaintenance[locale];
 const [open, setOpen] = useState(false), [snapshot, setSnapshot] = useState<EndpointOwnerSnapshot | null>(null);
 const [editor, setEditor] = useState<EndpointOwnerEntry | 'new' | null>(null), [draft, setDraft] = useState<EndpointOwnerDraft | null>(null);
 const [recovery, setRecovery] = useState<EndpointOwnerRecord | null>(null), [unknown, setUnknown] = useState('');
 const [maintenance, setMaintenance] = useState<{ kind: 'archive' | 'retire'; reviewed: EndpointOwnerSnapshot } | null>(null);
 const [maintenanceUnknown, setMaintenanceUnknown] = useState(false);
 const [busy, setBusy] = useState(false), [error, setError] = useState(''), [now, setNow] = useState(Date.now());
 const lock = useRef(false), live = useRef(true), generation = useRef(0);
 useEffect(() => { live.current = true; return () => { live.current = false; generation.current++; }; }, []);
 useEffect(() => { if (!draft) return; const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, [draft]);
 const refresh = async () => {
  const request = ++generation.current;
  try { const value = await api.endpointOwner.read(); if (live.current && request === generation.current) setSnapshot(value); }
  catch (reason) { if (live.current && request === generation.current) { setSnapshot(null); setError(reason instanceof Error ? reason.message : messages.common.error); } }
 };
 const operation = async (run: () => Promise<void>) => {
  if (lock.current) return; lock.current = true; setBusy(true); setError('');
  try { await run(); } catch (reason) { if (live.current) setError(reason instanceof Error ? reason.message : messages.common.error); }
  finally { lock.current = false; if (live.current) setBusy(false); }
 };
 const prepare = (input: EndpointOwnerInput) => operation(async () => { const value = await api.endpointOwner.prepare(input); if (live.current) { setNow(Date.now()); setDraft(value); } });
 const commit = () => operation(async () => {
  if (!draft || !snapshot || Date.now() >= Date.parse(draft.expires_at)) return;
  const reviewed = draft, actor = snapshot.actor, requestID = crypto.randomUUID();
  // Consume the local review before sending. A failed transport must never expose a retry-save button.
  setDraft(null); setEditor(null); setUnknown(requestID);
  try { await api.endpointOwner.commit(reviewed, requestID, actor); if (live.current) setUnknown(''); }
  finally { await refresh(); await onChanged(); }
 });
 const reconcile = () => operation(async () => {
  if (!recovery || !snapshot) return;
  const record = recovery, expected = snapshot.file_revision, actor = snapshot.actor; setRecovery(null);
  try { await api.endpointOwner.reconcile(record, expected, actor); if (live.current && unknown === record.request_id) setUnknown(''); }
  finally { await refresh(); await onChanged(); }
 });
 const closeEditor = () => { setEditor(null); setDraft(null); setError(''); };
 const maintain = () => operation(async () => {
  if (!maintenance) return;
  const reviewed = maintenance; setMaintenance(null); setMaintenanceUnknown(true);
  // Consume the displayed review once. Refresh inspects state; it never retries.
  try {
   if (reviewed.kind === 'archive') await api.endpointOwner.archive(reviewed.reviewed);
   else await api.endpointOwner.retireDecisions(reviewed.reviewed);
   if (live.current) setMaintenanceUnknown(false);
  } finally { await refresh(); await onChanged(); }
 });
 return <div className="endpoint-owner-panel">
  <button className="secondary-button" disabled={busy} aria-expanded={open} onClick={() => { setOpen(!open); if (!open) void operation(refresh); }}>{copy.configure}</button>
  {open && <div className="stack" aria-busy={busy}>
   <p>{copy.effects}</p>
   <a href="#/settings/recovery">{workspaceRecovery(locale).title}</a>
   {unknown && <p role="alert">{copy.unknown} <bdi>{unknown}</bdi></p>}
   {maintenanceUnknown && <p role="alert">{maintenanceCopy.unknown}</p>}
   {!editor && error && <p role="alert">{error}</p>}
   <div className="model-actions"><button disabled={busy || Boolean(editor) || Boolean(recovery)} onClick={() => void operation(refresh)}>{messages.common.refresh}</button><button disabled={busy || !snapshot?.write_available} onClick={() => { setError(''); setEditor('new'); }}>{copy.add}</button></div>
   {snapshot && <>
    {!snapshot.write_available && <p role="status">{copy.readonly} {snapshot.write_reason}</p>}
    {snapshot.entries.map(entry => <article className="endpoint-owner-entry" key={`${entry.id}@${entry.revision}`}><h3><bdi>{entry.id}@{entry.revision}</bdi></h3><details><summary>{copy.review}</summary><EndpointOwnerDetails entry={entry}/></details><button disabled={busy || !snapshot.write_available} onClick={() => { setError(''); setEditor(entry); }}>{copy.update}</button></article>)}
    <h3>{copy.history}</h3>
    <div className="model-actions endpoint-owner-maintenance">
     <button disabled={busy || !snapshot.write_available || !snapshot.journal_revision || snapshot.records.length === 0 || snapshot.records.some(record => !['committed','not_applied'].includes(record.state))} onClick={() => { setError(''); setMaintenance({ kind: 'archive', reviewed: structuredClone(snapshot) }); }}>{maintenanceCopy.archive}</button>
     <button disabled={busy || !snapshot.write_available || !snapshot.policy_maintenance?.retirable.length} onClick={() => { setError(''); setMaintenance({ kind: 'retire', reviewed: structuredClone(snapshot) }); }}>{maintenanceCopy.retire}</button>
    </div>
    {snapshot.records.map(record => <article className="endpoint-owner-record" key={record.request_id}><h4><bdi>{record.endpoint}</bdi></h4><p>{copy[record.state]}</p><p><bdi>{record.request_id}</bdi> · <bdi>{record.actor}</bdi> · <time dateTime={record.created_at}>{new Date(record.created_at).toLocaleString(locale)}</time></p><details><summary>{copy.review}</summary><p><bdi>{record.before}</bdi> → <bdi>{record.after}</bdi></p></details>{['pending','published','conflict'].includes(record.state) && <button disabled={busy} onClick={() => setRecovery(record)}>{copy.reconcile}</button>}</article>)}
    {snapshot.history_available && <EndpointOwnerHistory snapshot={snapshot} busy={busy}/>}
   </>}
  </div>}
  {editor && snapshot && <OwnerDialog title={draft ? copy.review : editor === 'new' ? copy.add : copy.update} busy={busy} close={closeEditor}>
   <p>{copy.effects}</p>{error && <p role="alert">{error}</p>}
   <div hidden={Boolean(draft)}><EndpointOwnerForm snapshot={snapshot} previous={editor === 'new' ? undefined : editor} busy={busy} prepare={prepare}/></div>
   {draft && <><EndpointOwnerDetails entry={draft.entry}/><p><bdi>{draft.expected_revision}</bdi></p>{now >= Date.parse(draft.expires_at) ? <p role="alert">{copy.expired}</p> : null}<div className="model-actions"><button disabled={busy} onClick={() => setDraft(null)}>{copy.update}</button><button className="primary-button" disabled={busy || now >= Date.parse(draft.expires_at)} onClick={() => void commit()}>{copy.publish}</button></div></>}
  </OwnerDialog>}
  {recovery && snapshot && <OwnerDialog title={copy.reconcile} busy={busy} close={() => setRecovery(null)}><p>{copy.effects}</p><p><bdi>{recovery.request_id}</bdi></p><p><bdi>{recovery.endpoint}</bdi></p><p><bdi>{snapshot.file_revision}</bdi></p><button className="primary-button" disabled={busy} onClick={() => void reconcile()}>{copy.reconcile}</button></OwnerDialog>}
  {maintenance && <OwnerDialog title={maintenance.kind === 'archive' ? maintenanceCopy.archive : maintenanceCopy.retire} busy={busy} close={() => setMaintenance(null)}>
   <p>{maintenance.kind === 'archive' ? maintenanceCopy.archiveHint : maintenanceCopy.retireHint}</p>
   <p><bdi>{maintenance.reviewed.actor}</bdi></p>
   <ul className="endpoint-maintenance-review" tabIndex={0} aria-label={maintenance.kind === 'archive' ? maintenanceCopy.archive : maintenanceCopy.retire}>{maintenance.kind === 'archive'
    ? maintenance.reviewed.records.map(record => <li key={record.request_id}><bdi>{record.endpoint}</bdi> · <bdi>{record.request_id}</bdi> · {copy[record.state]}</li>)
    : maintenance.reviewed.policy_maintenance!.retirable.map(key => <li key={key}><bdi>{key}</bdi></li>)}</ul>
   <button className="primary-button" disabled={busy} onClick={() => void maintain()}>{maintenance.kind === 'archive' ? maintenanceCopy.archive : maintenanceCopy.retire}</button>
  </OwnerDialog>}
 </div>;
}
