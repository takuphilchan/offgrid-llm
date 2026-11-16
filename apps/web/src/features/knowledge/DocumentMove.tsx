import {useEffect,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import {api,type Document} from '../../api/client';
import type {CollectionPolicy} from '../../api/collections';
import {useI18n} from '../../i18n';
import {collectionManagement} from '../../i18n/collection-management';
import {collectionDestination} from '../../i18n/collection-destination';
import {collectionErrorMessage} from '../../i18n/collection-errors';
import {sourceClose} from '../../i18n/knowledge-original';
import {restoreFocus} from '../../lib/restore-focus';
import {CollectionAudience} from './CollectionAudience';
import {CollectionDestination} from './CollectionDestination';
import './collection-management.css';

export function DocumentMove({source,close,changed}:{source:Document;close:()=>void;changed:()=>void}){
 const {locale,messages}=useI18n(),copy=collectionManagement(locale),labels=collectionDestination(locale);
 const dialog=useRef<HTMLDialogElement>(null),opener=useRef(document.activeElement as HTMLElement|null),operation=useRef<AbortController|null>(null),lock=useRef(false);
 const [binding,setBinding]=useState<Awaited<ReturnType<typeof api.collections.document>>>(),[destination,setDestination]=useState<CollectionPolicy>();
 const [actor,setActor]=useState(''),[loading,setLoading]=useState(true),[busy,setBusy]=useState(false),[error,setError]=useState(''),[reload,setReload]=useState(0);
 useEffect(()=>{const node=dialog.current!,previous=opener.current;node.showModal();return()=>{operation.current?.abort();node.close();restoreFocus(previous);};},[]);
 useEffect(()=>{
  const abort=new AbortController();operation.current=abort;setLoading(true);setError('');setBinding(undefined);setDestination(undefined);
  void Promise.all([api.collections.document(source.id,abort.signal),api.currentUser()]).then(([current,user])=>{
   if(abort.signal.aborted)return;const id=user.authenticated?user.user?.id??'':user.auth_required===false?'local':'';
   if(!id)throw Error();setActor(id);setBinding(current);
  }).catch(()=>{if(!abort.signal.aborted)setError(collectionErrorMessage());}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>abort.abort();
 },[source.id,reload]);
 const allowed=!!binding&&(binding.collection.id==='workspace'||binding.collection.owner===actor);
 const move=async()=>{
  if(lock.current||!allowed||!binding||!destination||error||loading||destination.id===binding.collection.id)return;
  lock.current=true;setBusy(true);const abort=new AbortController();operation.current=abort;
  try{const result=await api.collections.move({document_id:source.id,expected_revision:binding.revision,expected_source_revision:binding.collection.revision,destination_id:destination.id,expected_destination_revision:destination.revision},abort.signal);
   if(result.revision!==binding.revision+1)throw Error();
   if(!abort.signal.aborted){changed();close();}
  }catch{if(!abort.signal.aborted){setError(collectionErrorMessage());setDestination(undefined);}}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 return createPortal(<dialog ref={dialog} className="history-delete-dialog collection-manager" aria-label={copy.move} onCancel={event=>{event.preventDefault();if(!busy)close();}}>
  <h2>{copy.move}</h2><p><bdi>{source.name}</bdi></p>
  {loading&&<p role="status">{messages.common.loading}</p>}{error&&<p role="alert">{error}</p>}
  {binding&&<section className="collection-audience"><h3>{copy.current}: <bdi>{binding.collection.name}</bdi></h3><CollectionAudience policy={binding.collection}/></section>}
  {binding&&!allowed&&<p>{copy.readOnly}</p>}
  {allowed&&!error&&<><CollectionDestination key={reload} disabled={busy} files onChange={setDestination}/></>}
  <div className="history-dialog-actions">
   <button type="button" className="secondary-button" disabled={busy} onClick={close}>{sourceClose[locale]}</button>
   {error&&<button type="button" className="secondary-button" disabled={busy||loading} onClick={()=>setReload(value=>value+1)}>{labels.refresh}</button>}
   <button type="button" className="primary-button" disabled={busy||loading||!!error||!allowed||!destination||destination.id===binding?.collection.id} onClick={()=>void move()}>{copy.move}</button>
  </div>
 </dialog>,document.body);
}
