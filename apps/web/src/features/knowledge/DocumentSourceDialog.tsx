import { useEffect, useRef, useState } from 'react';
import { api, type DocumentCitation } from '../../api/client';
import { useI18n } from '../../i18n';
import { workflow } from '../../i18n/workflow';
import { knowledgeOriginal, sourceClose } from '../../i18n/knowledge-original';

export function DocumentSourceDialog({ id, citation, close }: { id: string; citation?: DocumentCitation; close: () => void }) {
 const { messages: text, locale } = useI18n();
 const copy = workflow[locale], original = knowledgeOriginal(locale);
 const ref = useRef<HTMLDialogElement>(null);
 const transfer = useRef<AbortController | null>(null);
 const objectURL = useRef('');
 const [source, setSource] = useState<Awaited<ReturnType<typeof api.documentSource>>>();
 const [error, setError] = useState('');
 const [downloadError, setDownloadError] = useState('');
 const [downloading, setDownloading] = useState(false);
 useEffect(() => {
  const dialog = ref.current!; let disposed = false;
  const previous = document.activeElement as HTMLElement | null;
  setSource(undefined); setError(''); setDownloadError(''); setDownloading(false);
  dialog.showModal();
  void api.documentSource(id, citation).then(value => {
   // Older services ignore citation parameters. Never label their document
   // prefix as the requested passage.
   if (citation && value.citation_verified !== true) throw new Error(copy.sourceMissing);
   if (!disposed) setSource(value);
  })
   .catch(reason => { if (!disposed) setError(reason instanceof Error ? reason.message : copy.sourceMissing); });
  return () => {
   disposed = true; transfer.current?.abort();
   if (objectURL.current) URL.revokeObjectURL(objectURL.current);
   objectURL.current = ''; dialog.close(); previous?.focus();
  };
 }, [id, citation]);
 const download = async () => {
  if (!source?.document.original_retained || downloading) return;
  const controller = new AbortController(); transfer.current = controller;
  setDownloading(true); setDownloadError('');
  try {
   const blob = await api.documentOriginal(id, controller.signal);
   if (controller.signal.aborted) return;
   if (objectURL.current) URL.revokeObjectURL(objectURL.current);
   objectURL.current = URL.createObjectURL(blob);
   const link = document.createElement('a'); link.href = objectURL.current;
   link.download = source.document.name; link.click();
  } catch (reason) {
   if (!controller.signal.aborted) setDownloadError(reason instanceof Error ? reason.message : text.common.error);
  } finally { if (!controller.signal.aborted) setDownloading(false); }
 };
 return <dialog ref={ref} className="document-source-dialog" aria-label={copy.sourceTitle} onCancel={event => { event.preventDefault(); close(); }}>
  <header><h2>{source?.document.name ?? copy.sourceTitle}</h2><button autoFocus className="secondary-button" onClick={close}>{sourceClose[locale]}</button></header>
  {error ? <p role="alert">{error}</p> : !source ? <p role="status">{text.common.loading}</p> : <>
   <p>{copy.sourceTitle}{source.truncated ? ` · ${copy.sourceTruncated}` : ''}</p>
   {source.document.original_retained ? <button className="text-button" disabled={downloading} onClick={() => void download()}>{downloading ? text.common.loading : original.download}</button> : <p className="workspace-secondary">{original.missing}</p>}
   {downloadError && <p role="alert">{downloadError}</p>}
   <pre tabIndex={0}>{source.content}</pre>
  </>}
 </dialog>;
}
