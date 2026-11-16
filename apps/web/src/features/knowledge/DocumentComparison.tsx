import {useEffect,useId,useRef,useState} from 'react';

import {api,type Document} from '../../api/client';
import type {Analysis,AnalysisHistory,AnalysisRef} from '../../api/document-analyses';
import {useI18n} from '../../i18n';
import {libraryFlow} from '../../i18n/library-flow';
import {documentComparison} from '../../i18n/document-comparison';
import {documentReader} from '../../i18n/document-reader';
import {sourceClose} from '../../i18n/knowledge-original';
import {restoreFocus} from '../../lib/restore-focus';
import {ComparisonResults} from './ComparisonResults';
import {ComparisonCoverage} from './ComparisonCoverage';
import {ComparisonExport} from './ComparisonExport';
import {Icon} from '../../components/Icon';
import './document-comparison.css';

export function DocumentComparison({initial=[],close}:{initial?:string[];close:()=>void}){
 const {locale,language,messages}=useI18n(),copy=documentComparison(locale),reader=documentReader(locale);
 const dialog=useRef<HTMLElement>(null),opener=useRef(document.activeElement as HTMLElement|null),operation=useRef<AbortController|null>(null),lock=useRef(false);
 const request=useRef<{key:string;sources:AnalysisRef[]}|undefined>(undefined);
 const [documents,setDocuments]=useState<Document[]>([]),[history,setHistory]=useState<AnalysisHistory>(),[offset,setOffset]=useState(0),[reload,setReload]=useState(0);
 const [before,setBefore]=useState(initial[0]??''),[after,setAfter]=useState(initial[1]??'');
 const [id,setID]=useState(''),[job,setJob]=useState<Analysis>(),[jobReload,setJobReload]=useState(0);
 const [loading,setLoading]=useState(true),[busy,setBusy]=useState(false),[error,setError]=useState(false),[uncertain,setUncertain]=useState(false);
 const [view,setView]=useState<'new'|'result'|'history'>('new');
 const formID=useId();
 const number=(v:number)=>v.toLocaleString(language),name=(id:string)=>documents.find(d=>d.id===id)?.name??id;
 const clearJob=()=>{setJob(undefined);setID('');};
 const newComparison=()=>{request.current=undefined;setUncertain(false);clearJob();setBefore('');setAfter('');setView('new');};
 const refresh=()=>{if(uncertain)setView('history');setError(false);setUncertain(false);setJob(undefined);setHistory(undefined);setDocuments([]);setLoading(true);setReload(v=>v+1);setJobReload(v=>v+1);};
 useEffect(()=>{const node=dialog.current!,previous=opener.current;node.focus();return()=>{operation.current?.abort();requestAnimationFrame(()=>restoreFocus(previous));};},[]);
 useEffect(()=>{
  const abort=new AbortController();setLoading(true);setHistory(undefined);setError(false);
  void Promise.all([api.analyses.history(offset,abort.signal),api.documents()]).then(([page,list])=>{
   if(abort.signal.aborted)return;setHistory(page);setDocuments(list.documents);setUncertain(false);
  }).catch(()=>{if(!abort.signal.aborted){setDocuments([]);setHistory(undefined);clearJob();setError(true);}}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>abort.abort();
 },[offset,reload]);
 useEffect(()=>{
  if(!id)return;const abort=new AbortController();let timer:ReturnType<typeof setTimeout>|undefined;
  const poll=async()=>{try{const value=await api.analyses.get(id,abort.signal);if(abort.signal.aborted)return;setJob(value);setHistory(page=>page?{...page,analyses:page.analyses.map(item=>item.id===value.id?{...item,state:value.state,updated_at:value.updated_at}:item)}:page);timer=setTimeout(()=>void poll(),3000);}catch{if(!abort.signal.aborted){setJob(undefined);setDocuments([]);setHistory(undefined);setError(true);}}};
  void poll();return()=>{abort.abort();clearTimeout(timer);};
 },[id,jobReload]);
 const choose=(which:'before'|'after',value:string)=>{request.current=undefined;setUncertain(false);which==='before'?setBefore(value):setAfter(value);};
 const start=async()=>{
  if(lock.current||loading||error||uncertain||!before||!after||before===after)return;
  lock.current=true;setBusy(true);setError(false);clearJob();const abort=new AbortController();operation.current=abort;
  let submitted=false;
  try{
   if(!request.current){
    // A correction/import may publish a new revision while this workspace is open.
    // A new comparison uses current sources; existing history remains immutable.
    const current=await api.documents();if(abort.signal.aborted)return;setDocuments(current.documents);
    const refs=await Promise.all([before,after].map(async id=>{const d=current.documents.find(d=>d.id===id);if(!d?.extraction_revision)throw Error();const structure=await api.documentStructure(id,d.extraction_revision,'',0,abort.signal);return {document_id:id,revision:structure.revision,...(structure.original_sha256?{original_sha256:structure.original_sha256}:{})};}));request.current={key:crypto.randomUUID(),sources:refs};
   }
   if(abort.signal.aborted)return;submitted=true;
   const value=await api.analyses.compare(request.current.key,request.current.sources,abort.signal);
   if(!abort.signal.aborted){setID(value.id);setJob(value);setUncertain(false);request.current=undefined;setView('result');setReload(v=>v+1);}
  }catch{if(!abort.signal.aborted){setUncertain(submitted);setError(!submitted);}}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const control=async(action:'cancel'|'retry')=>{
  if(lock.current||!id)return;lock.current=true;setBusy(true);setError(false);setJob(undefined);
  const abort=new AbortController();operation.current=abort;
  try{await api.analyses.control(id,action,abort.signal);if(!abort.signal.aborted)setJobReload(v=>v+1);}
  catch{if(!abort.signal.aborted){setUncertain(true);setJobReload(v=>v+1);}}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const selectionChanged=!!job&&(job.plan.sources[0]?.document_id!==before||job.plan.sources[1]?.document_id!==after);
 const hasPair=[before,after].every(id=>documents.some(d=>d.id===id&&!!d.extraction_revision))&&before!==after;
 return <section tabIndex={-1} role="region" ref={dialog} className="document-comparison" aria-label={copy.title} onKeyDown={event=>{if(event.key==='Escape'&&!event.currentTarget.querySelector('[popover]:popover-open')&&!event.defaultPrevented&&!(event.target instanceof HTMLSelectElement)&&!(event.target instanceof Element&&event.target.closest('[popover]'))){event.preventDefault();close();}}}>
  <header className="comparison-header"><div className="comparison-heading"><Icon name="compare" size={22}/><h2>{copy.title}</h2></div><nav className="comparison-tabs" aria-label={copy.title}>
   {view!=='history'&&<button form={formID} type="submit" className="primary-button comparison-submit" disabled={busy||loading||error||uncertain||!hasPair||!!job&&['queued','running'].includes(job.state)}>{busy?messages.common.loading:copy.title}<Icon name="chevron-right" size={16}/></button>}
   <button className="icon-button" title={view==='history'?copy.changes:copy.history} aria-label={view==='history'?copy.changes:copy.history} aria-pressed={view==='history'} onClick={()=>setView(view==='history'?(job?'result':'new'):'history')}><Icon name="activity" size={18}/></button>
   {view!=='new'&&<button className="icon-button" title={copy.newComparison} aria-label={copy.newComparison} disabled={busy} onClick={newComparison}><Icon name="plus" size={19}/></button>}
   <button className="icon-button" aria-label={messages.common.refresh} title={messages.common.refresh} disabled={busy||loading} onClick={refresh}><Icon name="refresh" size={17}/></button>
   <button autoFocus className="icon-button" aria-label={sourceClose[locale]} onClick={close}><Icon name="close" size={20}/></button>
  </nav></header>
  <div className="comparison-body">
  {error&&<p className="comparison-notice" role="alert">{copy.unavailable}</p>}{uncertain&&<p className="comparison-notice" role="alert">{copy.unknown}</p>}
  {view!=='history'&&<form id={formID} className={`comparison-start${job?' comparison-start-compact':''}`} onSubmit={event=>{event.preventDefault();void start();}}>
   {!job&&<div className="comparison-start-heading"><p>{copy.choose}</p></div>}
   <div className="comparison-choice">{(['before','after'] as const).map(side=><label key={side}><span>{copy[side]}</span><div className="comparison-file-choice"><Icon name="file" size={19}/><select aria-label={copy[side]} value={side==='before'?before:after} disabled={busy||loading||error} onChange={event=>choose(side,event.target.value)}><option value="">{copy.choose}</option>{documents.filter(d=>!!d.extraction_revision).map(d=><option key={d.id} value={d.id} disabled={d.id===(side==='before'?after:before)}>{d.name}</option>)}</select></div></label>)}
    <button type="button" className="icon-button comparison-swap" title={copy.swap} aria-label={copy.swap} disabled={busy||loading} onClick={()=>{request.current=undefined;setBefore(after);setAfter(before);setUncertain(false);}}><Icon name="compare" size={18}/></button>
   </div>
   {!job&&<p className="comparison-scope-note">{copy.help}</p>}
  </form>}
  {selectionChanged&&view==='result'&&<p className="comparison-selection-notice" role="status">{libraryFlow(locale).changed}</p>}
  {view==='result'&&job&&!selectionChanged&&<section className="comparison-job" aria-label={copy.changes}>
   <div className="comparison-result-tools">
   <div className="comparison-statusbar"><p role="status"><span className="comparison-state" data-state={job.state}>{['complete'].includes(job.state)?<Icon name="check" size={14}/>:['incomplete','failed','interrupted'].includes(job.state)?<Icon name="warning" size={14}/>:null}{copy.states[job.state]}</span><span>{copy.units}: <bdi>{number(job.completed_units)} / {number(job.plan.units.length)}</bdi></span></p><div className="comparison-actions">{['queued','running','interrupted'].includes(job.state)&&<button className="text-button" disabled={busy} onClick={()=>void control('cancel')}>{messages.models.cancel}</button>}{['queued','interrupted','cancelled','failed'].includes(job.state)&&<button className="secondary-button" disabled={busy} onClick={()=>void control('retry')}>{messages.common.retry}</button>}</div></div>
   <ComparisonCoverage key={'coverage:'+job.id} job={job} names={job.plan.sources.map(s=>name(s.document_id))}/>
   {['complete','incomplete'].includes(job.state)&&<ComparisonExport key={'export:'+job.id} id={job.id}/>}
   </div>
   {['queued','running'].includes(job.state)&&<progress value={job.completed_units} max={Math.max(1,job.plan.units.length)} aria-label={copy.units}/>}
   {job.completed_steps>0?<ComparisonResults key={'results:'+job.id} job={job} names={job.plan.sources.map(s=>name(s.document_id))} invalidate={()=>{clearJob();setError(true);setHistory(undefined);setDocuments([]);}}/>:<p className="comparison-waiting">{copy.states[job.state]}</p>}
  </section>}
  {(loading||(!error&&view==='result'&&!job))&&<p className="comparison-loading" role="status">{messages.common.loading}</p>}
  {view==='history'&&history&&<section className="comparison-history" aria-label={copy.history}>{!history.analyses.length&&<p className="comparison-waiting">{copy.none}</p>}<ul>{history.analyses.map(item=><li key={item.id}><button disabled={busy} aria-pressed={id===item.id} onClick={()=>{setError(false);setJob(undefined);setBefore(item.documents[0]??'');setAfter(item.documents[1]??'');request.current=undefined;setView('result');setID(item.id);setJobReload(v=>v+1);}}><Icon name="file" size={20}/><span><span className="comparison-history-names"><bdi>{name(item.documents[0])}</bdi><Icon name="chevron-right" size={14}/><bdi>{name(item.documents[1])}</bdi></span><small>{copy.states[item.state]} · <time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString(language)}</time></small></span><Icon name="chevron-right" size={18}/></button></li>)}</ul>{(offset>0||history.next_offset!==null)&&<nav aria-label={copy.history}><button className="text-button" disabled={busy||offset===0} onClick={()=>{clearJob();setOffset(Math.max(0,offset-20));}}>{reader.previous}</button><button className="text-button" disabled={busy||history.next_offset===null} onClick={()=>{clearJob();setOffset(history.next_offset!);}}>{reader.next}</button></nav>}</section>}
  </div>
 </section>;
}
