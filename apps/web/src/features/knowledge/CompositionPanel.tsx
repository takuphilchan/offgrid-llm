import {useEffect,useRef,useState} from 'react';
import {api} from '../../api/client';
import type {Synthesis} from '../../api/document-synthesis';
import type {Composition,CompositionHistory,CompositionReport} from '../../api/document-composition';
import {useI18n} from '../../i18n';
import {compositionCopy} from '../../i18n/document-composition';
import {documentSynthesis} from '../../i18n/document-synthesis';
import {documentComparison} from '../../i18n/document-comparison';
import {documentReader} from '../../i18n/document-reader';
import {Icon} from '../../components/Icon';
import {CompositionSources} from './CompositionSources';
import {ComparisonExport} from './ComparisonExport';
import './document-composition.css';

export function CompositionPanel({parent,names,invalidate}:{parent:Synthesis;names:string[];invalidate:()=>void}){
 const {locale,language,messages}=useI18n(),copy=compositionCopy(locale),draft=documentSynthesis(locale),common=documentComparison(locale),reader=documentReader(locale);
 const [history,setHistory]=useState<CompositionHistory>(),[offset,setOffset]=useState(0),[id,setID]=useState(''),[job,setJob]=useState<Composition>(),[report,setReport]=useState<CompositionReport>();
 const [section,setSection]=useState<number>(),[busy,setBusy]=useState(false),[uncertain,setUncertain]=useState(false),[reload,setReload]=useState(0);
 const operation=useRef<AbortController|null>(null),lock=useRef(false),selection=useRef('');
 const number=(n:number)=>n.toLocaleString(language);
 const historyState=(item:Composition)=>{const state=job?.id===item.id?job.state:item.state;return state==='awaiting_review'?draft.review:common.states[state];};
 useEffect(()=>()=>operation.current?.abort(),[]);
 useEffect(()=>{const abort=new AbortController();setHistory(undefined);
  void api.composition.history(parent,offset,abort.signal).then(value=>{if(!abort.signal.aborted){setHistory(value);if(!selection.current&&value.items.length){selection.current=value.items[0].id;setID(selection.current);}}}).catch(()=>{if(!abort.signal.aborted)invalidate();});return()=>abort.abort();
 },[parent.id,offset,reload]);
 useEffect(()=>{setJob(undefined);setReport(undefined);setSection(undefined);if(!id)return;const abort=new AbortController();let timer:ReturnType<typeof setTimeout>|undefined;
  const poll=async()=>{try{const value=await api.composition.get(parent,id,abort.signal);if(!abort.signal.aborted){setJob(value);timer=setTimeout(()=>void poll(),3000);}}catch{if(!abort.signal.aborted){setReport(undefined);setJob(undefined);invalidate();}}};
  void poll();return()=>{abort.abort();clearTimeout(timer);};
 },[parent.id,id,reload]);
 useEffect(()=>{if(job?.state!=='awaiting_review')return;const abort=new AbortController();
  void api.composition.report(parent,job.id,abort.signal).then(value=>{if(!abort.signal.aborted)setReport(value);}).catch(()=>{if(!abort.signal.aborted)invalidate();});return()=>abort.abort();
 },[job?.id,job?.state,reload]);
 const act=async(action:'create'|'cancel'|'retry')=>{
  if(lock.current||uncertain)return;const abort=new AbortController();operation.current=abort;lock.current=true;setBusy(true);
  try{if(action==='create'){const value=await api.composition.create(parent,crypto.randomUUID(),abort.signal);if(!abort.signal.aborted){selection.current=value.id;setID(value.id);setOffset(0);setReload(n=>n+1);}}
   else if(job){await api.composition.control(job.id,action,abort.signal);if(!abort.signal.aborted)setReload(n=>n+1);}}
  catch{if(!abort.signal.aborted){setUncertain(true);setJob(undefined);setReport(undefined);setSection(undefined);}}
  finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 const refresh=()=>{setUncertain(false);setReport(undefined);setSection(undefined);setReload(n=>n+1);};
 const pending=job&&['queued','running'].includes(job.state);
 return <section className="composition-panel" aria-label={copy.title}>
  <header className="composition-toolbar"><div><h3>{copy.title}</h3><p><bdi>{parent.model.model_id}</bdi></p></div><div className="comparison-actions"><button className="icon-button" aria-label={messages.common.refresh} disabled={busy} onClick={refresh}><Icon name="refresh" size={17}/></button><button className="primary-button" disabled={busy||uncertain||!!pending} onClick={()=>void act('create')}>{copy.create}<Icon name="chevron-right" size={16}/></button></div></header>
  <p className="synthesis-review-note">{draft.help}</p>
  {uncertain&&<p role="alert" className="comparison-notice">{common.unknown}</p>}
  {history&&history.items.length>0&&<div className="composition-history"><label>{draft.history}<select value={id} disabled={busy} onChange={e=>{selection.current=e.target.value;setID(e.target.value);}}>{history.items.map(item=><option value={item.id} key={item.id}>{new Date(item.created_at).toLocaleString(language)} · {historyState(item)}</option>)}</select></label><button className="icon-button" aria-label={reader.previous} disabled={!offset} onClick={()=>setOffset(n=>Math.max(0,n-20))}><Icon name="chevron-left" size={16}/></button><button className="icon-button" aria-label={reader.next} disabled={history.next_offset===null} onClick={()=>setOffset(history.next_offset!)}><Icon name="chevron-right" size={16}/></button></div>}
  {job&&<><div className="composition-progress" role="status"><span>{job.state==='awaiting_review'?draft.review:common.states[job.state]}</span><span>{copy.progress}: {number(job.completed_steps)} / {number(job.total_steps)}</span>{pending&&<progress value={job.completed_steps} max={Math.max(1,job.total_steps)} aria-label={copy.progress}/>}</div>
   {!job.reviews_current&&<p className="comparison-notice" role="status">{copy.stale}</p>}
   <div className="comparison-actions">{pending&&<button className="text-button" disabled={busy||uncertain} onClick={()=>void act('cancel')}>{messages.models.cancel}</button>}{['interrupted','failed','cancelled'].includes(job.state)&&job.reviews_current&&<button className="secondary-button" disabled={busy||uncertain} onClick={()=>void act('retry')}>{messages.common.retry}</button>}</div>
  </>}
  {report&&job&&<>
   {(report.gaps.length+report.extraction_gaps.length+report.candidate_gaps.length>0||report.unrepresented_batches>0)&&<div className="composition-coverage"><Icon name="warning" size={18}/><div><p>{reader.partial}</p>{report.unrepresented_batches>0&&<p>{copy.omitted}: {number(report.unrepresented_batches)}</p>}</div></div>}
   <div className="composition-reading" data-evidence={section!==undefined}><div className="composition-prose">{report.sections.map((item,i)=><article key={i}><p dir="auto">{item.text}</p><button className="text-button composition-source-link" aria-expanded={section===i} onClick={()=>setSection(section===i?undefined:i)}><Icon name="file" size={15}/>{draft.evidence}<span>{number(item.contributors)}</span><Icon name="chevron-right" size={14}/></button></article>)}{!report.sections.length&&<p className="comparison-waiting">{draft.empty}</p>}</div>
    {section!==undefined&&<CompositionSources key={report.composition.id+':'+section} parent={parent} report={report} section={section} names={names} close={()=>setSection(undefined)} invalidate={invalidate}/>}</div>
   <ComparisonExport key={report.composition.id} id={report.composition.id} composition/>
  </>}
  {(!history||(id&&!job&&!uncertain)||(job?.state==='awaiting_review'&&!report))&&<p role="status" className="comparison-loading">{messages.common.loading}</p>}
 </section>;
}
