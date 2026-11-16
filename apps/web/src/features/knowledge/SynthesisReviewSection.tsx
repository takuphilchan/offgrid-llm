import {useEffect,useId,useRef,useState,type ReactNode} from 'react';
import {api} from '../../api/client';
import type {SynthesisResults,SynthesisReview} from '../../api/document-synthesis';
import {useI18n} from '../../i18n';
import {synthesisReviewCopy} from '../../i18n/document-synthesis-review';
import {SynthesisReviewHistory} from './SynthesisReviewHistory';

export type ReviewDrafts=Map<string,SynthesisReview>;
type Props={id:string;batch:number;receipt:SynthesisResults['receipts'][number];enabled:boolean;drafts:ReviewDrafts;invalidate:()=>void;children:(controls:(index:number)=>ReactNode)=>ReactNode};
export function SynthesisReviewSection({id,batch,receipt,enabled,drafts,invalidate,children}:Props){
 const {locale,language,messages}=useI18n(),copy=synthesisReviewCopy(locale),key=id+':'+receipt.batch_id;
 const [value,setValue]=useState<SynthesisReview>(),[busy,setBusy]=useState(false),[uncertain,setUncertain]=useState(false),[saved,setSaved]=useState(false),[reload,setReload]=useState(0);
 const operation=useRef<AbortController|null>(null),lock=useRef(false);
 const fieldID=useId();
 useEffect(()=>{if(!enabled)return;const abort=new AbortController();setValue(undefined);
  void api.synthesis.review(id,batch,receipt,abort.signal).then(r=>{if(abort.signal.aborted)return;const draft=drafts.get(key);setValue(draft&&draft.response_sha256===r.response_sha256&&draft.decisions.length===r.decisions.length?draft:r);setUncertain(false);}).catch(()=>{if(!abort.signal.aborted)invalidate();});
  return()=>{abort.abort();operation.current?.abort();};
 },[id,receipt.batch_id,enabled,reload]);
 const edit=(next:SynthesisReview)=>{setValue(next);drafts.set(key,next);setSaved(false);};
 const save=async()=>{
  if(!value||lock.current||uncertain||saved)return;const abort=new AbortController();operation.current=abort;lock.current=true;setBusy(true);setSaved(false);
  try{const r=await api.synthesis.saveReview({id,batch,batch_id:receipt.batch_id,response_sha256:receipt.response_sha256,expected_revision:value.revision,section_checked:value.section_checked,decisions:value.decisions},receipt,abort.signal);if(!abort.signal.aborted){setValue(r);drafts.delete(key);setSaved(true);}}
  catch{if(!abort.signal.aborted)setUncertain(true);}finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const controls=(index:number)=>enabled&&value&&<fieldset className="synthesis-review-choice" disabled={busy||uncertain}><legend>{copy.title}</legend>
  <select aria-label={copy.title} value={value.decisions[index].decision} onChange={event=>{const decisions=value.decisions.map((d,i)=>i===index?{...d,decision:event.target.value as 'keep'|'exclude'|'unreviewed'}:d);edit({...value,decisions,section_checked:false});}}>
   <option value="unreviewed">{copy.pending}</option><option value="keep">{copy.keep}</option><option value="exclude">{copy.exclude}</option>
  </select>
  <div className="synthesis-review-note-field"><label htmlFor={fieldID+'-'+index}>{copy.note}</label><textarea id={fieldID+'-'+index} dir="auto" rows={2} value={value.decisions[index].note} onChange={event=>{const note=event.target.value;if(new TextEncoder().encode(note).length<=2048)edit({...value,decisions:value.decisions.map((d,i)=>i===index?{...d,note}:d)});}}/></div>
 </fieldset>;
 return <section className="synthesis-section">{children(controls)}{enabled&&<div className="synthesis-review-footer">
  {!value?<p role="status">{messages.common.loading}</p>:<>
   <label className="synthesis-review-check"><input type="checkbox" checked={value.section_checked} disabled={busy||uncertain||value.decisions.some(d=>d.decision==='unreviewed')} onChange={e=>edit({...value,section_checked:e.target.checked})}/><span>{copy.checked}</span></label>
   <div className="synthesis-review-actions"><button className="secondary-button" aria-disabled={busy||uncertain||saved} onClick={()=>void save()}>{busy?messages.common.loading:copy.save}</button>{saved&&<span role="status">{copy.saved}</span>}{value.revision>0&&<small><bdi>{value.author}</bdi> · {new Date(value.updated_at).toLocaleString(language)}</small>}</div>
   {uncertain&&<div role="alert"><p>{copy.recover}</p><button className="text-button" onClick={()=>{drafts.delete(key);setSaved(false);setReload(v=>v+1);}}>{copy.reload}</button></div>}
   <SynthesisReviewHistory latest={value} receipt={receipt} invalidate={invalidate}/>
  </>}
 </div>}</section>;
}
