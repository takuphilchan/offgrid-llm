import { useEffect, useRef, useState } from 'react';
import { api, APIError, type Model, type InferenceEndpointManagement, type InferenceEndpointStatus } from '../../api/client';
import { chatAvailable } from '../../api/model-capabilities';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { SectionHeading } from '../../components/WorkspacePresentation';
import { useI18n } from '../../i18n';
import { inferenceEndpoints } from '../../i18n/inference-endpoints';
import { workspaceOrder } from '../../i18n/workspace-order';
import { workspaceAudit } from '../../i18n/workspace-audit';
import { useWorkspace } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';

export function InferenceEndpointModels({ models, filter, selected, setSelected, onChanged, refreshToken = 0 }: {models:Model[];filter:string;selected:string;setSelected:(value:string)=>void;onChanged:()=>Promise<void>;refreshToken?:number}) {
 const {admin}=useWorkspace(); const {locale,messages:text}=useI18n(); const copy=inferenceEndpoints[locale];
 const [snapshot,setSnapshot]=useState<InferenceEndpointManagement|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false),[success,setSuccess]=useState('');
 const [unsupported,setUnsupported]=useState(false);
 const [review,setReview]=useState<{item:InferenceEndpointStatus;action:'enable'|'disable'|'probe';policy:string}|null>(null);
 const lock=useRef(false);
 const load=async(signal?:AbortSignal)=>{
  if(!admin)return;
  try{const result=await api.inferenceEndpoints(signal);if(!signal?.aborted){setSnapshot(result);setUnsupported(false);setError('');}}
  catch(reason){if(!signal?.aborted){setSnapshot(null);const missing=reason instanceof APIError&&reason.status===404;setUnsupported(missing);setError(missing?'':reason instanceof Error?reason.message:text.common.error);}}
 };
 useEffect(()=>{const controller=new AbortController();void load(controller.signal);return()=>controller.abort();},[admin,refreshToken]);
 useWorkspaceRefresh(load);
 const perform=async()=>{
  if(!review||lock.current)return;lock.current=true;setBusy(true);setError('');setSuccess('');
  const input={id:review.item.id,revision:review.item.revision,config_pin:review.item.config_pin,policy_revision:review.policy};
  try{
   if(review.action==='probe'){await api.probeInferenceEndpoint(input);setSuccess(copy.success);}
   else {setSnapshot(await api.reviewInferenceEndpoint({...input,enabled:review.action==='enable'}));if(review.action==='disable'&&selected===review.item.model_id)setSelected('');await onChanged();}
   setReview(null);
  }catch(reason){setError(reason instanceof Error?reason.message:text.common.error);await load();throw reason;}
  finally{lock.current=false;setBusy(false);}
 };
 const query=filter.trim().toLocaleLowerCase(locale);
 const publicModels=models.filter(model=>model.id.startsWith('endpoint:')&&model.id.toLocaleLowerCase(locale).includes(query));
 const items=(snapshot?.items??[]).filter(item=>item.model_id.toLocaleLowerCase(locale).includes(query));
 if(!admin&&!publicModels.length)return null;
 const select=(id:string)=>{const model=models.find(value=>value.id===id);return model&&<button className="text-button" disabled={!chatAvailable(model)} aria-pressed={selected===id} onClick={()=>setSelected(id)}>{selected===id?workspaceAudit(locale).selected:workspaceOrder(locale).selectModel}</button>;};
 return <section className="model-section inference-endpoint-section" aria-label={copy.title} aria-busy={busy}>
  <SectionHeading title={copy.title}/>
  {admin&&<p className="model-search-hint" role={unsupported?'status':undefined}>{unsupported?copy.unsupported:copy.hint}</p>}
  {error&&<div className="inline-error" role="alert">{error}<button onClick={()=>void load()} disabled={busy}>{text.common.refresh}</button></div>}
  {success&&<p role="status">{success}</p>}
  {snapshot?.policy_state==='restore_review_required'&&<p role="status">{copy.restored}</p>}
  {admin&&snapshot&&!snapshot.items.length&&<p>{copy.empty}</p>}
  <div className="installed-models">
   {admin&&!unsupported?items.map(item=><article className="installed-model endpoint-model" key={item.model_id}>
    <h3>{item.model_id}</h3><p>{item.provider} · {item.model}</p><p className="model-runtime-meta"><bdi>{item.url}</bdi></p>
    <p>{item.review_required?copy.review:item.enabled?copy.enabled:copy.disabled}</p>
    {!item.current?<p>{copy.removed}</p>:!item.installed?<p>{copy.restart}</p>:null}
    {!item.credential_ready&&<p>{copy.credential}</p>}{item.managed&&<p>{copy.managed}</p>}
    <details><summary>{copy.actors}</summary><ul>{item.actors.map(actor=><li key={actor}><bdi>{actor}</bdi></li>)}</ul></details>
    <div className="model-actions">{item.enabled&&select(item.model_id)}
     <button className={item.enabled?'danger-button':'secondary-button'} disabled={busy||(!item.enabled&&(!item.installed||!item.current))} onClick={()=>setReview({item,action:item.enabled?'disable':'enable',policy:snapshot!.policy_revision})}>{item.enabled?copy.disable:copy.enable}</button>
     <button className="secondary-button" disabled={busy||!item.probe_allowed} onClick={()=>setReview({item,action:'probe',policy:snapshot!.policy_revision})}>{copy.probe}</button>
    </div>
   </article>):publicModels.map(model=><article className="installed-model endpoint-model" key={model.id}><h3>{model.id}</h3><p>{model.engine}</p><div className="model-actions">{select(model.id)}</div></article>)}
  </div>
  {review&&<ConfirmDialog title={review.item.model_id} tone={review.action==='disable'?'danger':'neutral'} body={`${review.item.url}\n\n${review.action==='enable'?copy.allowHint:review.action==='disable'?copy.stopHint:copy.probeHint}`} confirmLabel={copy[review.action]} close={()=>setReview(null)} confirm={perform}/>}
 </section>;
}
