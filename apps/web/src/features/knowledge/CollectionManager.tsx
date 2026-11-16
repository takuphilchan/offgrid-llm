import {useEffect,useId,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import {api,type PublicUser} from '../../api/client';
import type {CollectionPolicy} from '../../api/collections';
import {useI18n} from '../../i18n';
import {collectionDestination} from '../../i18n/collection-destination';
import {collectionManagement} from '../../i18n/collection-management';
import {collectionErrorMessage} from '../../i18n/collection-errors';
import {sourceClose} from '../../i18n/knowledge-original';
import {restoreFocus} from '../../lib/restore-focus';
import {CollectionAudience} from './CollectionAudience';
import {Icon} from '../../components/Icon';
import './collection-management.css';

export function CollectionManager({close,changed,initial=''}:{close:()=>void;changed:()=>void;initial?:string}){
 const {locale,messages}=useI18n(),copy=collectionManagement(locale),labels=collectionDestination(locale),id=useId();
 const dialog=useRef<HTMLDialogElement>(null),opener=useRef(document.activeElement as HTMLElement|null),operation=useRef<AbortController|null>(null),lock=useRef(false);
 const [collections,setCollections]=useState<CollectionPolicy[]>([]),[people,setPeople]=useState<PublicUser[]>([]),[actor,setActor]=useState('');
 const [creating,setCreating]=useState(false);
 const [selected,setSelected]=useState<CollectionPolicy>(),[name,setName]=useState(''),[newName,setNewName]=useState('');
 const [personQuery,setPersonQuery]=useState(''),[peopleLimit,setPeopleLimit]=useState(40);
 const [readers,setReaders]=useState<string[]>([]),[editors,setEditors]=useState<string[]>([]),[review,setReview]=useState(false),[confirmed,setConfirmed]=useState(false);
 const [loading,setLoading]=useState(true),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState(''),[reload,setReload]=useState(0),[directory,setDirectory]=useState(false);
 const edit=(policy:CollectionPolicy|undefined)=>{setPersonQuery('');setPeopleLimit(40);setSelected(policy);setName(policy?.name??'');setReaders(policy?.readers??[]);setEditors(policy?.editors??[]);setReview(false);setConfirmed(false);setNotice('');};
 useEffect(()=>{const node=dialog.current!,previous=opener.current;node.showModal();return()=>{operation.current?.abort();node.close();restoreFocus(previous);};},[]);
 useEffect(()=>{
  const abort=new AbortController();operation.current=abort;setLoading(true);setError('');edit(undefined);
  void Promise.all([api.currentUser(),api.collections.list(abort.signal),api.collections.people(abort.signal).then(users=>({users,available:true})).catch(()=>({users:[],available:false}))]).then(([user,list,members])=>{
   if(abort.signal.aborted)return;const owner=user.authenticated?user.user?.id??'':user.auth_required===false?'local':'';if(!owner)throw Error();
   setActor(owner);setCollections(list);setPeople(members.users);setDirectory(members.available);edit(list.find(p=>p.id===initial)??list[0]);setCreating(list.length===0);
  }).catch(()=>{if(!abort.signal.aborted)setError(collectionErrorMessage());}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>abort.abort();
 },[reload]);
 const select=async(value:string)=>{
  if(lock.current||loading)return;edit(undefined);setError('');if(!value)return;setLoading(true);
  const abort=new AbortController();operation.current=abort;
  try{const policy=await api.collections.get(value,abort.signal);if(!abort.signal.aborted){edit(policy);setCollections(items=>items.map(p=>p.id===policy.id?policy:p));}}
  catch{if(!abort.signal.aborted)setError(collectionErrorMessage());}
  finally{if(!abort.signal.aborted)setLoading(false);}
 };
 const create=async()=>{
  if(lock.current||loading||error||!newName.trim())return;lock.current=true;setBusy(true);setError('');
  const abort=new AbortController();operation.current=abort;
  try{const policy=await api.collections.create(newName.trim(),abort.signal);if(abort.signal.aborted)return;if(policy.owner!==actor)throw Error();setCollections(items=>[...items,policy]);setNewName('');setCreating(false);edit(policy);setNotice(copy.saved);changed();}
  catch{if(!abort.signal.aborted)setError(collectionErrorMessage());}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const owned=selected?.id!=='workspace'&&selected?.owner===actor;
 const save=async()=>{
  if(lock.current||loading||error||!selected||!owned||!review||!confirmed||!name.trim())return;
  lock.current=true;setBusy(true);setError('');const abort=new AbortController();operation.current=abort;
  try{const policy=await api.collections.update(selected.id,{expected_revision:selected.revision,name:name.trim(),readers,editors},abort.signal);if(abort.signal.aborted)return;
   if(policy.owner!==selected.owner)throw Error();setCollections(items=>items.map(p=>p.id===policy.id?policy:p));edit(policy);setNotice(copy.saved);changed();
  }catch{if(!abort.signal.aborted){setError(collectionErrorMessage());setConfirmed(false);setReview(false);}}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const users=[...new Set([...people.map(p=>p.id),...readers,...editors])].filter(user=>user!==selected?.owner&&(`${user} ${people.find(person=>person.id===user)?.username??''}`).toLocaleLowerCase(locale).includes(personQuery.toLocaleLowerCase(locale)));
 return createPortal(<dialog ref={dialog} className="history-delete-dialog collection-manager" aria-label={copy.title} onCancel={event=>{event.preventDefault();if(!busy)close();}}>
  <header className="collection-manager-heading"><div><Icon name="collections" size={22}/><h2>{copy.title}</h2></div><button type="button" className="icon-button" disabled={busy} aria-label={sourceClose[locale]} onClick={close}><Icon name="close" size={20}/></button></header>
  {loading&&<p role="status">{messages.common.loading}</p>}{error&&<p role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
  <div className="collection-manager-layout"><nav className="collection-manager-list" aria-label={copy.title}>
   <ul>{collections.map(policy=><li key={policy.id}><button type="button" aria-current={!creating&&selected?.id===policy.id?'true':undefined} disabled={busy||loading} onClick={()=>{setCreating(false);void select(policy.id);}}><Icon name="folder" size={18}/><span>{policy.name}</span></button></li>)}</ul>
   <button type="button" className="text-button collection-new" disabled={busy||loading} aria-pressed={creating} onClick={()=>setCreating(true)}><Icon name="plus" size={18}/>{labels.create}</button>
  </nav><div className="collection-manager-content">
  {creating&&<form className="collection-create" onSubmit={event=>{event.preventDefault();void create();}}><h3>{labels.create}</h3><label htmlFor={id+'-new'}>{labels.name}</label><input id={id+'-new'} autoFocus maxLength={256} value={newName} disabled={busy||loading||!!error} onChange={event=>setNewName(event.target.value)}/><button type="submit" className="primary-button" disabled={busy||loading||!!error||!newName.trim()}>{labels.create}</button></form>}
  {selected&&!creating&&<section className="collection-editor">
   <div className="collection-audience"><CollectionAudience policy={review?{...selected,name,readers,editors}:selected}/></div>
   {!owned?<p>{copy.readOnly}</p>:<>
    <p>{copy.help}</p>
    {!review?<>
     <label htmlFor={`${id}-name`}>{copy.name}</label><input id={`${id}-name`} maxLength={256} value={name} disabled={busy||!!error} onChange={event=>setName(event.target.value)}/>
     <h3>{copy.people}</h3>{!directory&&<p role="status">{collectionErrorMessage()}</p>}
     <input aria-label={copy.people} value={personQuery} onChange={event=>{setPersonQuery(event.target.value);setPeopleLimit(40);}}/><ul className="collection-members">{users.slice(0,peopleLimit).map(user=><li key={user}><label htmlFor={`${id}-${user}`}><bdi>{people.find(person=>person.id===user)?.username??user}</bdi><small><bdi>{user}</bdi></small></label><select id={`${id}-${user}`} disabled={busy||!!error||!directory} value={editors.includes(user)?'editor':readers.includes(user)?'reader':'none'} onChange={event=>{setReaders(items=>[...items.filter(item=>item!==user),...(event.target.value==='reader'?[user]:[])]);setEditors(items=>[...items.filter(item=>item!==user),...(event.target.value==='editor'?[user]:[])]);}}><option value="none">{labels.none}</option><option value="reader">{labels.readers}</option><option value="editor">{labels.editors}</option></select></li>)}</ul>{users.length>peopleLimit&&<button type="button" className="text-button" onClick={()=>setPeopleLimit(value=>value+40)}>{messages.history.showMore}</button>}
     <button type="button" className="secondary-button" disabled={busy||!!error||!name.trim()||readers.length>128||editors.length>128} onClick={()=>{setReview(true);setConfirmed(false);}}>{copy.review}</button>
    </>:<>
     <h3><bdi>{name}</bdi></h3><label className="collection-consent"><input type="checkbox" checked={confirmed} disabled={busy} onChange={event=>setConfirmed(event.target.checked)}/><span>{labels.confirmFiles}</span></label>
     <div className="history-dialog-actions"><button type="button" className="secondary-button" disabled={busy} onClick={()=>{setReview(false);setConfirmed(false);}}>{messages.models.cancel}</button><button type="button" className="primary-button" disabled={busy||!confirmed} onClick={()=>void save()}>{copy.apply}</button></div>
    </>}
   </>}
  </section>}
  </div></div><footer className="collection-manager-footer"><button type="button" className="text-button" disabled={busy||loading} onClick={()=>setReload(value=>value+1)}><Icon name="refresh" size={16}/>{labels.refresh}</button></footer>
 </dialog>,document.body);
}
