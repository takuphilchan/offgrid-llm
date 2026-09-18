import { useEffect, useRef, useState } from 'react';
import { useI18n } from '../i18n';

export function ConfirmDialog({ title, body, confirm, close }: { title: string; body: string; confirm: () => Promise<void>; close: () => void }) {
 const {messages:text}=useI18n();const dialog=useRef<HTMLDialogElement>(null);const lock=useRef(false);
 const [busy,setBusy]=useState(false),[error,setError]=useState('');
 useEffect(()=>{const node=dialog.current!;const previous=document.activeElement as HTMLElement|null;node.showModal();return()=>{node.close();previous?.focus();};},[]);
 const run=async()=>{if(lock.current)return;lock.current=true;setBusy(true);setError('');try{await confirm();close();}catch(reason){setError(reason instanceof Error?reason.message:text.common.error);}finally{lock.current=false;setBusy(false);}};
 return <dialog ref={dialog} className="history-delete-dialog" aria-label={title} onCancel={event=>{event.preventDefault();if(!lock.current)close();}}><h2>{title}</h2><p>{body}</p>{error&&<p role="alert">{error}</p>}<div className="history-dialog-actions"><button autoFocus disabled={busy} className="secondary-button" onClick={close}>{text.models.cancel}</button><button disabled={busy} className="danger-button" onClick={()=>void run()}>{busy?text.common.loading:text.models.confirmDelete}</button></div></dialog>;
}
