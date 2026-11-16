import {useEffect,useRef,useState} from 'react';

import {api,type Document,type Model} from '../../api/client';
import type {Synthesis,SynthesisHistory,SynthesisRequest} from '../../api/document-synthesis';
import {useI18n} from '../../i18n';
import {documentSynthesis} from '../../i18n/document-synthesis';
import {documentComparison} from '../../i18n/document-comparison';
import {documentReader} from '../../i18n/document-reader';
import {restoreFocus} from '../../lib/restore-focus';
import {useWorkspaceState} from '../../lib/workspace-context';
import {Icon} from '../../components/Icon';
import {SynthesisSections} from './SynthesisSections';
import {useReviewDrafts} from './review-drafts';
import {ComparisonExport} from './ComparisonExport';
import {CompositionPanel} from './CompositionPanel';
import {compositionCopy} from '../../i18n/document-composition';
import './document-comparison.css';
import './document-synthesis.css';

export function DocumentSynthesis({initial=[],models,close}:{initial?:string[];models:Model[];close:()=>void}){
 const {locale,language,messages}=useI18n(),copy=documentSynthesis(locale),common=documentComparison(locale),reader=documentReader(locale);
 const dialog=useRef<HTMLElement>(null),opener=useRef(document.activeElement as HTMLElement|null),operation=useRef<AbortController|null>(null),lock=useRef(false);
 const [id,setID]=useWorkspaceState<string>('knowledge.synthesis.id','');
 const reviewDrafts=useReviewDrafts();
 const [documents,setDocuments]=useState<Document[]>([]),[history,setHistory]=useState<SynthesisHistory>(),[job,setJob]=useState<Synthesis>();
 const [selected,setSelected]=useState(initial.slice(0,16)),[model,setModel]=useState(''),[workflow,setWorkflow]=useState<'summary'|'requirements'>(location.hash.endsWith('/requirements')?'requirements':'summary');
 const [view,setView]=useState<'new'|'result'|'history'>(/\/(summary|requirements)$/.test(location.hash)?'new':id?'result':'new'),[offset,setOffset]=useState(0),[reload,setReload]=useState(0);
 const [busy,setBusy]=useState(false),[loading,setLoading]=useState(true),[error,setError]=useState(false),[uncertain,setUncertain]=useState(false);
 const [compositionEnabled,setCompositionEnabled]=useState(false),[resultView,setResultView]=useState<'drafts'|'report'>('drafts');
 const composition=compositionCopy(locale);
 useEffect(()=>{let active=true;void api.systemIdentity().then(v=>{if(active)setCompositionEnabled(v.capabilities?.includes('document-composition-v1')===true);}).catch(()=>{});return()=>{active=false;};},[]);
 useEffect(()=>setResultView('drafts'),[id]);
 const number=(n:number)=>n.toLocaleString(language),name=(id:string)=>documents.find(d=>d.id===id)?.name??reader.document;
 const clear=()=>{reviewDrafts.clear();setJob(undefined);setHistory(undefined);setDocuments([]);setError(true);};
 const refresh=()=>{setJob(undefined);setHistory(undefined);setError(false);if(uncertain)setView('history');setUncertain(false);setReload(v=>v+1);};
 useEffect(()=>{const node=dialog.current!,previous=opener.current;node.focus();return()=>{operation.current?.abort();requestAnimationFrame(()=>restoreFocus(previous));};},[]);
 useEffect(()=>{const abort=new AbortController();setLoading(true);void Promise.all([api.synthesis.history(offset,abort.signal),api.documents()]).then(([h,d])=>{if(!abort.signal.aborted){setHistory(h);setDocuments(d.documents);}}).catch(()=>{if(!abort.signal.aborted)clear();}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});return()=>abort.abort();},[offset,reload]);
 useEffect(()=>{if(!id)return;const abort=new AbortController();let timer:ReturnType<typeof setTimeout>|undefined;
  const poll=async()=>{try{const value=await api.synthesis.get(id,abort.signal);if(abort.signal.aborted)return;setJob(value);timer=setTimeout(()=>void poll(),3000);}catch{if(!abort.signal.aborted)clear();}};
  void poll();return()=>{abort.abort();clearTimeout(timer);};
 },[id,reload]);
 const start=async()=>{
  if(lock.current||!model||!selected.length||uncertain)return;const sources=selected.map(id=>documents.find(d=>d.id===id)).filter((d):d is Document=>!!d&&!!d.extraction_revision);if(sources.length!==selected.length){clear();return;}
  const abort=new AbortController();operation.current=abort;lock.current=true;setBusy(true);setError(false);
  let submitted=false;
  try{const refs=await Promise.all(sources.map(async d=>{const structure=await api.documentStructure(d.id,d.extraction_revision!,'',0,abort.signal);return {document_id:d.id,revision:structure.revision,...(structure.original_sha256?{original_sha256:structure.original_sha256}:{})};}));if(abort.signal.aborted)return;
   const body:SynthesisRequest={request_key:crypto.randomUUID(),workflow,model,language:locale,sources:refs};submitted=true;const value=await api.synthesis.create(body,abort.signal);if(!abort.signal.aborted){setJob(value);setID(value.id);setView('result');}}
  catch{if(!abort.signal.aborted){setUncertain(submitted);setError(!submitted);if(submitted){setView('history');setReload(v=>v+1);}}}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const control=async(action:'cancel'|'retry')=>{if(lock.current||!job)return;const abort=new AbortController();operation.current=abort;lock.current=true;setBusy(true);try{await api.synthesis.control(job.id,action,abort.signal);if(!abort.signal.aborted)refresh();}catch{if(!abort.signal.aborted){setUncertain(true);setJob(undefined);}}finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}};
 return <section tabIndex={-1} role="region" className="document-comparison document-synthesis" ref={dialog} aria-label={copy.title} onKeyDown={event=>{if(event.key==='Escape'&&!event.defaultPrevented&&!(event.target instanceof HTMLSelectElement)){event.preventDefault();close();}}}>
  <header className="comparison-header"><div className="comparison-heading"><Icon name="file" size={20}/><h2>{copy.title}</h2></div><button autoFocus className="icon-button" aria-label={reader.close} onClick={close}><Icon name="close" size={20}/></button></header>
  <nav className="comparison-tabs" aria-label={copy.title}><div><button className="text-button" aria-pressed={view==='new'} disabled={busy} onClick={()=>setView('new')}>{copy.create}</button>{id&&<button className="text-button" aria-pressed={view==='result'} onClick={()=>setView('result')}>{job?copy[job.plan.workflow]:copy.review}</button>}<button className="text-button" aria-pressed={view==='history'} onClick={()=>{setView('history');setReload(v=>v+1);}}>{copy.history}</button></div><button className="icon-button" aria-label={messages.common.refresh} disabled={busy} onClick={refresh}><Icon name="refresh" size={17}/></button></nav>
  <div className="comparison-body">
   {error&&<p role="alert" className="comparison-notice">{messages.common.error}</p>}{uncertain&&<p role="alert" className="comparison-notice">{common.unknown}</p>}
   {view==='new'&&<form className="synthesis-start" onSubmit={event=>{event.preventDefault();void start();}}>
    <div className="synthesis-intro"><h3>{copy.title}</h3><p>{copy.help}</p></div>
    <fieldset className="synthesis-modes"><legend className="sr-only">{copy.title}</legend>{(['summary','requirements'] as const).map(value=><label key={value} data-selected={workflow===value}><input type="radio" name="document-workflow" checked={workflow===value} onChange={()=>setWorkflow(value)}/><span className="action-card-icon"><Icon name={value==='summary'?'file':'knowledge'} size={22}/></span><span>{copy[value]}</span><Icon name="check" size={16}/></label>)}</fieldset>
    <label className="synthesis-model">{messages.nav.models}<select value={model} onChange={e=>setModel(e.target.value)} disabled={busy}><option value="">{messages.nav.models}</option>{models.filter(m=>!m.type||m.type==='llm'||m.type==='chat'||m.type==='vlm').map(m=><option key={m.id} value={m.id}>{m.id}</option>)}</select></label>
    <fieldset className="synthesis-documents"><legend>{reader.document}<span>{number(selected.length)} / {number(16)}</span></legend>{documents.filter(d=>d.extraction_revision).map(d=><label key={d.id}><input type="checkbox" checked={selected.includes(d.id)} disabled={busy||(!selected.includes(d.id)&&selected.length>=16)} onChange={e=>setSelected(ids=>e.target.checked?[...ids,d.id]:ids.filter(id=>id!==d.id))}/><Icon name="file" size={18}/><span><bdi>{d.name}</bdi></span></label>)}</fieldset>
    <div className="synthesis-start-footer"><button className="primary-button" disabled={busy||loading||error||uncertain||!model||!selected.length}>{busy?messages.common.loading:copy.create}<Icon name="chevron-right" size={16}/></button></div>
   </form>}
   {view==='result'&&job&&<section className="synthesis-result">
    {resultView==='drafts'&&<><div className="synthesis-result-heading"><h3>{copy[job.plan.workflow]}</h3><p>{job.plan.sources.map(s=>name(s.document_id)).join(' · ')}</p></div>
    <div className="comparison-statusbar"><p role="status"><span className="comparison-state">{job.state==='awaiting_review'?copy.review:common.states[job.state]}</span><span>{common.units}: {number(job.completed_units)} / {number(job.plan.units.length)}</span></p><div className="comparison-actions">{['running','queued','interrupted'].includes(job.state)&&<button className="text-button" disabled={busy} onClick={()=>void control('cancel')}>{messages.models.cancel}</button>}{['queued','cancelled','interrupted','failed'].includes(job.state)&&<button className="secondary-button" disabled={busy} onClick={()=>void control('retry')}>{messages.common.retry}</button>}</div></div>
    <p className="synthesis-review-note">{copy.help}</p>{job.plan.gaps.length>0&&<p className="comparison-notice">{reader.partial}</p>}</>}
    {compositionEnabled&&job.plan.workflow==='summary'&&job.state==='awaiting_review'&&<nav className="composition-view-tabs" aria-label={composition.title}><button className="text-button" aria-pressed={resultView==='drafts'} onClick={()=>setResultView('drafts')}>{composition.drafts}</button><button className="text-button" aria-pressed={resultView==='report'} onClick={()=>setResultView('report')}>{composition.title}</button></nav>}
    {resultView==='drafts'&&job.completed_batches>0&&<SynthesisSections key={job.id} job={job} names={job.plan.sources.map(s=>name(s.document_id))} invalidate={clear} drafts={reviewDrafts}/>}
    {resultView==='drafts'&&job.state==='awaiting_review'&&<ComparisonExport key={'export:'+job.id} id={job.id} synthesis/>}
    {resultView==='report'&&compositionEnabled&&job.plan.workflow==='summary'&&job.state==='awaiting_review'&&<CompositionPanel key={job.id} parent={job} names={job.plan.sources.map(s=>name(s.document_id))} invalidate={clear}/>}
   </section>}
   {view==='history'&&history&&<section className="comparison-history" aria-label={copy.history}>{!history.analyses.length&&<p className="comparison-waiting">{copy.empty}</p>}<ul>{history.analyses.map(item=><li key={item.id}><button disabled={busy} onClick={()=>{setJob(undefined);setID(item.id);setUncertain(false);setView('result');setReload(v=>v+1);}}><Icon name="file" size={20}/><span><span className="comparison-history-names">{item.documents.map(name).join(' · ')}</span><small>{item.state==='awaiting_review'?copy.review:common.states[item.state]} · {new Date(item.created_at).toLocaleString(language)}</small></span><Icon name="chevron-right" size={18}/></button></li>)}</ul><nav><button className="text-button" disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-20))}>{reader.previous}</button><button className="text-button" disabled={history.next_offset===null} onClick={()=>setOffset(history.next_offset!)}>{reader.next}</button></nav></section>}
   {(loading||(view==='result'&&!job&&!error))&&<p className="comparison-loading" role="status">{messages.common.loading}</p>}
  </div>
 </section>;
}
