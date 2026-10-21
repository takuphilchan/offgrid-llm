import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { useI18n } from '../../i18n';
import { workflow } from '../../i18n/workflow';

export function DocumentSourceDialog({ id, close }: { id: string; close: () => void }) {
 const {messages:text,locale}=useI18n();const copy=workflow[locale];const ref=useRef<HTMLDialogElement>(null);
 const [source,setSource]=useState<Awaited<ReturnType<typeof api.documentSource>>>();const [error,setError]=useState('');
 useEffect(()=>{const dialog=ref.current!;let disposed=false;const previous=document.activeElement as HTMLElement|null;dialog.showModal();void api.documentSource(id).then(value=>{if(!disposed)setSource(value);}).catch(reason=>{if(!disposed)setError(reason instanceof Error?reason.message:copy.sourceMissing);});return()=>{disposed=true;dialog.close();previous?.focus();};},[id]);
 return <dialog ref={ref} className="document-source-dialog" aria-label={copy.sourceTitle} onCancel={event=>{event.preventDefault();close();}}><header><h2>{source?.document.name??copy.sourceTitle}</h2><button autoFocus className="secondary-button" onClick={close}>{text.onboarding.close}</button></header>{error?<p role="alert">{error}</p>:!source?<p role="status">{text.common.loading}</p>:<><p>{copy.sourceTitle}{source.truncated?` · ${copy.sourceTruncated}`:''}</p><pre tabIndex={0}>{source.content}</pre></>}</dialog>;
}
