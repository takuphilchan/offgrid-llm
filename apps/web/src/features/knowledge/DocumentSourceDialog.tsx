import {useSetupHandoff} from '../../lib/setup-handoff';
import {useChatCapabilities} from '../chat/useChatCapabilities';
import {useDraftScope} from '../../lib/draft-scope';
import {knowledgeUnitLabels} from '../../i18n/knowledge-locators';
import { useEffect, useMemo, useRef, useState } from 'react';
import {DocumentReader} from '../chat/DocumentReader';
import type {ReaderSource} from '../chat/DocumentWorkspace';
import { api, type DocumentCitation } from '../../api/client';
import { useI18n } from '../../i18n';
import { workflow } from '../../i18n/workflow';
import { knowledgeOriginal, sourceClose } from '../../i18n/knowledge-original';
import { DocumentEvidenceNotice } from '../../components/DocumentEvidenceNotice';
import { DocumentReview } from '../chat/DocumentReview';
import { documentReview } from '../../i18n/document-review';

export function DocumentSourceDialog({ id, citation, close }: { id: string; citation?: DocumentCitation; close: () => void }) {
 const { messages: text, locale } = useI18n();
 const handoff=useSetupHandoff(),{sourceScopes}=useChatCapabilities(useDraftScope());
 const ask=(revision:string,page?:number,block?:import('../../api/client').DocumentBlock)=>{if(!source||!sourceScopes)return;handoff.askKnowledge([{id,name:source.document.name,revision,page,block_ids:block?[block.id]:undefined,scopeLabel:page?knowledgeUnitLabels[locale].page+' '+page:block?.text.slice(0,64)}]);close();};
 const copy = workflow[locale], original = knowledgeOriginal(locale);
 const ref = useRef<HTMLDialogElement>(null);
 const transfer = useRef<AbortController | null>(null);
 const objectURL = useRef('');
 const [source, setSource] = useState<Awaited<ReturnType<typeof api.documentSource>>>();
 const [error, setError] = useState('');
 const [downloadError, setDownloadError] = useState('');
 const [downloading, setDownloading] = useState(false);
 const [review, setReview] = useState(false), [reviewOpened, setReviewOpened] = useState(false);
 useEffect(() => {
  const dialog = ref.current!; let disposed = false;
  const previous = document.activeElement as HTMLElement | null;
  setSource(undefined); setError(''); setDownloadError(''); setDownloading(false); setReview(false); setReviewOpened(false);
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
 const readerSource=useMemo<ReaderSource|undefined>(()=>source?.document.original_retained&&source.document.extraction_revision?{name:source.document.name,path:'/v1/documents/source?'+new URLSearchParams({id,format:'preview'}),documentId:id,extractionRevision:source.document.extraction_revision,sha256:citation?.source_sha256,citation,verifiedText:citation?source.content:undefined,original:()=>api.documentOriginal(id)}:undefined,[source,id,citation]);
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
 return <dialog ref={ref} className={`document-source-dialog${readerSource?' document-source-reader-dialog':''}`} aria-label={copy.sourceTitle} onCancel={event => { event.preventDefault(); close(); }}>
  {readerSource?<DocumentReader key={JSON.stringify([id,readerSource.extractionRevision,citation])} source={readerSource} close={close} selectPage={sourceScopes?page=>ask(readerSource.extractionRevision!,page):undefined} selectBlock={sourceScopes?(revision,block)=>ask(revision,undefined,block):undefined}/>:<>
  <header><h2>{source?.document.name ?? copy.sourceTitle}</h2><button autoFocus className="secondary-button" onClick={close}>{sourceClose[locale]}</button></header>
  {error ? <p role="alert">{error}</p> : !source ? <p role="status">{text.common.loading}</p> : <>
   <p>{copy.sourceTitle}{source.truncated ? ` · ${copy.sourceTruncated}` : ''}</p>
   <DocumentEvidenceNotice evidence={source.document.evidence}/>
   {source.document.original_retained ? <button className="text-button" disabled={downloading} onClick={() => void download()}>{downloading ? text.common.loading : original.download}</button> : <p className="workspace-secondary">{original.missing}</p>}
   {downloadError && <p role="alert">{downloadError}</p>}
   {source.document.extraction_revision && <div className="document-review-tabs" role="group" aria-label={copy.sourceTitle}>
    <button className="text-button" aria-pressed={!review} onClick={() => setReview(false)}>{copy.sourceTitle}</button>
    <button className="text-button" aria-pressed={review} onClick={() => { setReview(true); setReviewOpened(true); }}>{documentReview(locale).title}</button>
   </div>}
   <pre tabIndex={0} hidden={review}>{source.content}</pre>
   {reviewOpened && <div hidden={!review}><DocumentReview key={id + source.document.extraction_revision} id={id} revision={source.document.extraction_revision} highlight={citation ? source.content : ''} selectBlock={sourceScopes?(revision,block)=>ask(revision,undefined,block):undefined}/></div>}
  </>}
  </>}
 </dialog>;
}
