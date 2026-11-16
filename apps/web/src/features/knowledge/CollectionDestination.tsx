import {useEffect,useId,useRef,useState} from 'react';
import {api} from '../../api/client';
import type {CollectionPolicy} from '../../api/collections';
import {useI18n} from '../../i18n';
import {collectionDestination} from '../../i18n/collection-destination';
import {collectionErrorMessage} from '../../i18n/collection-errors';
import './collection-destination.css';
import {CollectionAudience} from './CollectionAudience';

// This review is local to one publication dialog. Never persist consent, infer
// it from a remembered destination, or refresh the audience behind a checkmark.
export function CollectionDestination({disabled,onChange,files=false}:{disabled:boolean;onChange:(policy:CollectionPolicy|undefined)=>void;files?:boolean}) {
 const {locale,messages}=useI18n(),copy=collectionDestination(locale),id=useId();
 const [policies,setPolicies]=useState<CollectionPolicy[]>([]),[selected,setSelected]=useState<CollectionPolicy>();
 const [loading,setLoading]=useState(true),[error,setError]=useState(''),[name,setName]=useState(''),[confirmed,setConfirmed]=useState(false);
 const operation=useRef<AbortController|null>(null),locked=useRef(false),mounted=useRef(true),actor=useRef('');
 const reset=()=>{setSelected(undefined);setConfirmed(false);onChange(undefined);};
 const writable=(p:CollectionPolicy)=>p.id==='workspace'||p.owner===actor.current||p.editors.includes(actor.current);
 const load=async()=>{
  if(disabled||locked.current)return;
  locked.current=true;reset();setLoading(true);setError('');
  operation.current?.abort();const abort=new AbortController();operation.current=abort;
  try {
   const [user,list]=await Promise.all([api.currentUser(),api.collections.list(abort.signal)]);
   if(abort.signal.aborted)return;
   actor.current=user.authenticated?user.user?.id??'':user.auth_required===false?'local':'';
   if(!actor.current)throw Error();
   setPolicies(list.filter(writable));
  }catch{if(!abort.signal.aborted){setPolicies([]);setError(collectionErrorMessage());}}
  finally{if(operation.current===abort){locked.current=false;if(!abort.signal.aborted)setLoading(false);}}
 };
 useEffect(()=>{mounted.current=true;locked.current=false;void load();return()=>{mounted.current=false;operation.current?.abort();};},[]);
 const choose=async(value:string)=>{
  if(disabled||locked.current)return;
  reset();if(!value)return;
  locked.current=true;setLoading(true);setError('');
  const abort=new AbortController();operation.current=abort;
  try{const current=await api.collections.get(value,abort.signal);if(!abort.signal.aborted){if(!writable(current))throw Error();setPolicies(items=>items.map(p=>p.id===current.id?current:p));setSelected(current);}}
  catch{if(!abort.signal.aborted)setError(collectionErrorMessage());}
  finally{locked.current=false;if(!abort.signal.aborted)setLoading(false);}
 };
 const create=async()=>{
  if(disabled||locked.current||error||!name.trim())return;
  locked.current=true;reset();setLoading(true);setError('');
  const abort=new AbortController();operation.current=abort;
  try{
   const current=await api.collections.create(name.trim(),abort.signal);
   if(abort.signal.aborted)return;
   if(current.owner!==actor.current)throw Error();
   setPolicies(items=>[...items,current]);setSelected(current);setName('');
  }catch{if(!abort.signal.aborted)setError(collectionErrorMessage());}
  finally{locked.current=false;if(mounted.current&&!abort.signal.aborted)setLoading(false);}
 };
 return <section className="collection-destination" aria-label={copy.destination} aria-busy={loading}>
  <label htmlFor={id}>{copy.destination}</label>
  <select id={id} disabled={disabled||loading} value={selected?.id??''} onChange={event=>void choose(event.target.value)}>
   <option value="">{copy.choose}</option>{policies.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}
  </select>
  {loading&&<p role="status">{messages.common.loading}</p>}
  {error&&<p role="alert">{error}</p>}
  {selected&&<div className="collection-audience">
   <CollectionAudience policy={selected}/>
   <label className="collection-consent"><input type="checkbox" checked={confirmed} disabled={disabled||loading} onChange={event=>{setConfirmed(event.target.checked);onChange(event.target.checked?selected:undefined);}}/><span>{files?copy.confirmFiles:copy.confirm}</span></label>
  </div>}
  <button type="button" className="text-button" disabled={disabled||loading} onClick={()=>void load()}>{copy.refresh}</button>
  <details><summary>{copy.create}</summary><div className="collection-create">
   <label htmlFor={`${id}-name`}>{copy.name}</label><input id={`${id}-name`} value={name} maxLength={256} disabled={disabled||loading} onChange={event=>setName(event.target.value)}/>
   <button type="button" className="secondary-button" disabled={disabled||loading||!!error||!name.trim()} onClick={()=>void create()}>{copy.create}</button>
  </div></details>
 </section>;
}
