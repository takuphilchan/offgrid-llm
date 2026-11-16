import {useEffect,useRef,useState} from 'react';
import {api,type AgentProfile} from '../../api/client';
import {useI18n} from '../../i18n';
import {profileDeletion} from '../../i18n/profile-deletion';
import {restoreFocus} from '../../lib/restore-focus';

export function ProfileDeleteDialog({profile,close,deleted}:{profile:AgentProfile;close:()=>void;deleted:()=>void}) {
 const {locale,language,messages}=useI18n(),copy=profileDeletion(locale);
 const ref=useRef<HTMLDialogElement>(null),lock=useRef(false);
 const [review,setReview]=useState<Awaited<ReturnType<typeof api.reviewProfileDeletion>>>(),[busy,setBusy]=useState(false),[error,setError]=useState(false),[reload,setReload]=useState(0);
 useEffect(()=>{const node=ref.current!,previous=document.activeElement as HTMLElement;node.showModal();return()=>{node.close();restoreFocus(previous);};},[]);
 useEffect(()=>{const abort=new AbortController();setReview(undefined);setError(false);void api.reviewProfileDeletion(profile.id,abort.signal).then(value=>{if(!abort.signal.aborted&&[value.jobs,value.memory,value.blocked_jobs,value.dependencies,value.revision].every(n=>Number.isSafeInteger(n)&&n>=0)&&/^[a-f0-9]{64}$/.test(value.token))setReview(value);else if(!abort.signal.aborted)setError(true);}).catch(()=>{if(!abort.signal.aborted)setError(true);});return()=>abort.abort();},[profile.id,reload]);
 const blocked=!!review&&(review.blocked_jobs>0||review.dependencies>0);
 const description=review?copy.body.replace('{jobs}',review.jobs.toLocaleString(language)).replace('{memory}',review.memory.toLocaleString(language)):'';
 const remove=async()=>{if(lock.current||!review||blocked)return;lock.current=true;setBusy(true);try{const result=await api.deleteAgentProfile(profile.id,review.revision,review.token);if(result.id!==profile.id||result.deleted!==true)throw Error();deleted();close();}catch{setReview(undefined);setError(true);}finally{lock.current=false;setBusy(false);}};
 return <dialog ref={ref} className="history-delete-dialog" aria-label={`${messages.models.delete}: ${profile.settings.name}`} onCancel={event=>{event.preventDefault();if(!busy)close();}}><h2>{messages.models.delete}: <bdi>{profile.settings.name}</bdi></h2>
 {review?<><p>{description}</p>{blocked&&<p role="status">{copy.blocked.replace('{jobs}',review.blocked_jobs.toLocaleString(language)).replace('{dependencies}',review.dependencies.toLocaleString(language))}</p>}</>:!error&&<p role="status">{messages.common.loading}</p>}
 {error&&<p role="alert">{messages.common.error}</p>}
 <div className="history-dialog-actions"><button autoFocus className="secondary-button" disabled={busy} onClick={close}>{messages.models.cancel}</button>{error||blocked?<button className="text-button" disabled={busy} onClick={()=>setReload(n=>n+1)}>{messages.common.refresh}</button>:<button className="danger-button" disabled={busy||!review} onClick={()=>void remove()}>{busy?messages.common.loading:messages.models.confirmDelete}</button>}</div>
 </dialog>;
}
