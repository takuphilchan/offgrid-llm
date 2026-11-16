import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { api, APIError, type SessionAttachment } from '../../api/client';
import { useI18n } from '../../i18n';
import { fileKnowledge } from '../../i18n/file-knowledge';
import { knowledgeImport } from '../../i18n/knowledge-import';
import { documentRecovery } from '../../i18n/document-recovery';
import { restoreFocus } from '../../lib/restore-focus';
import { sourceClose } from '../../i18n/knowledge-original';
import { documentProgress } from '../../i18n/document-progress';
import { runDocumentImport } from '../knowledge/import-progress';
import {CollectionDestination} from '../knowledge/CollectionDestination';
import type {CollectionPolicy} from '../../api/collections';
import {collectionDestination} from '../../i18n/collection-destination';
import {collectionErrorMessage} from '../../i18n/collection-errors';

export function FileKnowledgeImport({ session, file, close }: { session: string; file: SessionAttachment; close: () => void }) {
 const { locale, messages } = useI18n(), copy = fileKnowledge(locale), imports = knowledgeImport(locale);
 const dialog = useRef<HTMLDialogElement>(null), lock = useRef(false), operation = useRef<AbortController | null>(null);
 // Capture before commit: the dialog's autoFocus button can take focus before
 // useEffect runs, otherwise cleanup tries to restore its own removed control.
 const opener = useRef(document.activeElement as HTMLElement | null);
 const [ready, setReady] = useState<boolean>(), [busy, setBusy] = useState(false), [result, setResult] = useState(''), [error, setError] = useState('');
 const [phase, setPhase] = useState('');
 const [destination,setDestination]=useState<CollectionPolicy>();
 useEffect(() => {
  const node = dialog.current!, previous = opener.current, abort = new AbortController();
  node.showModal();
  void api.ragStatus(abort.signal).then(status => { if (!abort.signal.aborted) setReady(status.enabled); }).catch(reason => { if (!abort.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); });
  return () => { abort.abort(); operation.current?.abort(); node.close(); restoreFocus(previous); };
 }, []);
 const run = async () => {
  if (!ready || !destination || lock.current || result) return;
  lock.current = true; setBusy(true); setError('');
  const abort = new AbortController(); operation.current = abort;
  try {
   const accepted = await api.collections.acceptAttachment(destination.id,destination.revision,session,file,abort.signal);
   if (abort.signal.aborted) return;
   if (!accepted.id || accepted.sha256 !== file.sha256) { setResult(imports.unknown); return; }
   // Only a fresh accepted source is eligible for this explicit operation.
   // Prior failed/uncertain attempts belong in the durable recovery screen.
   if (accepted.state === 'ready' && accepted.document_id) { setResult(imports.added); return; }
   if (accepted.state !== 'accepted') { setResult(imports.unknown); return; }
   const indexed = await runDocumentImport(accepted.id, abort.signal, state => setPhase(documentProgress(locale, state) ?? ''));
   if (!abort.signal.aborted) setResult(indexed.id===accepted.id&&indexed.sha256===file.sha256&&indexed.collection_id===destination.id&&indexed.collection_revision===destination.revision&&indexed.state === 'ready' && indexed.document_id ? imports.added : imports.unknown);
  } catch (reason) {
   if (abort.signal.aborted) return;
   const code = reason instanceof APIError ? reason.data?.error_code ?? reason.data?.error?.code : undefined;
   setError(code?.startsWith('collection_')?collectionErrorMessage():documentRecovery(locale, code) ?? (reason instanceof Error ? reason.message : messages.common.error));
   setResult(imports.unknown);
  } finally { lock.current = false; if (!abort.signal.aborted) setBusy(false); }
 };
 return createPortal(<dialog ref={dialog} className="history-delete-dialog" aria-label={copy.title} onCancel={event => { event.preventDefault(); if (!busy) close(); }}>
  <h2>{copy.title}</h2><p><strong><bdi>{file.name}</bdi></strong></p><p>{collectionDestination(locale).copy}</p>
  {ready === undefined && !error && <p role="status">{messages.common.loading}</p>}
  {ready === false && <p>{copy.setup}</p>}
  {ready===true&&!result&&<CollectionDestination disabled={busy} onChange={setDestination}/>}
  {error && <p role="alert">{error}</p>}{result && <p role="status">{result}</p>}
  {busy && phase && <p role="status">{phase}</p>}
  <div className="history-dialog-actions">
   <button className="secondary-button" autoFocus disabled={busy} onClick={close}>{result ? sourceClose[locale] : messages.models.cancel}</button>
   {(ready === false || result) ? <a className="primary-button" href="#/knowledge" onClick={close}>{messages.knowledge.title}</a> : <button className="primary-button" disabled={!ready || !destination || busy} onClick={() => void run()}>{busy ? imports.importing : copy.title}</button>}
  </div>
 </dialog>, document.body);
}
