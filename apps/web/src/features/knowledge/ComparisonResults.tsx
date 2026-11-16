import {useEffect,useRef,useState} from 'react';
import {api} from '../../api/client';
import type {Analysis,AnalysisResults,AnalysisSource} from '../../api/document-analyses';
import {useI18n} from '../../i18n';
import {documentComparison} from '../../i18n/document-comparison';
import {documentReader} from '../../i18n/document-reader';
import {workflow} from '../../i18n/workflow';
import {Icon} from '../../components/Icon';
import {restoreFocus} from '../../lib/restore-focus';
import {useWorkspaceState} from '../../lib/workspace-context';

export function ComparisonResults({job,names,invalidate}:{job:Analysis;names:string[];invalidate:()=>void}){
 const {locale,language,messages}=useI18n(),copy=documentComparison(locale),reader=documentReader(locale);
 const [step,setStep]=useWorkspaceState(`comparison.${job.id}.step`,0),[offset,setOffset]=useWorkspaceState(`comparison.${job.id}.offset`,0),[page,setPage]=useState<AnalysisResults>();
 const [source,setSource]=useState<AnalysisSource>(),[reading,setReading]=useState(false),[loading,setLoading]=useState(false);
 const [selected,setSelected]=useState('');
 const sourceButton=useRef<HTMLButtonElement|null>(null);
 const current=useRef(job),failed=useRef(invalidate),transfer=useRef<AbortController|null>(null),focus=useRef<HTMLDivElement>(null);current.current=job;failed.current=invalidate;
 const number=(n:number)=>n.toLocaleString(language);
 useEffect(()=>{if(source)focus.current?.focus();},[source]);
 useEffect(()=>{
  const abort=new AbortController();transfer.current?.abort();setPage(undefined);setSource(undefined);setSelected('');setReading(false);setLoading(true);
  void api.analyses.results(current.current,step,offset,abort.signal).then(value=>{if(!abort.signal.aborted)setPage(value);}).catch(()=>{if(!abort.signal.aborted)failed.current();}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>{abort.abort();transfer.current?.abort();};
 },[job.id,step,offset]);
 const open=async(index:number,side:'before'|'after')=>{
  const expected=page?.result.changes[index][side];if(!expected)return;
  transfer.current?.abort();const abort=new AbortController();transfer.current=abort;setSource(undefined);setSelected(`${offset+index}:${side}`);setReading(true);
  try{const value=await api.analyses.source(current.current,step,offset+index,side,expected,abort.signal);if(!abort.signal.aborted)setSource(value);}
  catch{if(!abort.signal.aborted)failed.current();}finally{if(!abort.signal.aborted)setReading(false);}
 };
 return <div className="comparison-results">
  {job.completed_steps>1&&<nav className="comparison-pagination" aria-label={copy.units}><button className="text-button" disabled={loading||step===0} onClick={()=>{setStep(step-1);setOffset(0);}}><Icon name="chevron-left" size={16}/>{reader.previous}</button><bdi>{number(step+1)} / {number(job.completed_steps)}</bdi><button className="text-button" disabled={loading||step+1>=job.completed_steps} onClick={()=>{setStep(step+1);setOffset(0);}}>{reader.next}<Icon name="chevron-right" size={16}/></button></nav>}
  {loading&&<p className="comparison-loading" role="status">{messages.common.loading}</p>}
  {page&&<>
   {page.result.ambiguous_cells>0&&<p className="comparison-notice">{copy.gaps} · {number(page.result.ambiguous_cells)}</p>}
   <div className="comparison-diff-grid">{(['before','after'] as const).map((side,sideIndex)=><section className="comparison-side" key={side} aria-label={copy[side]}>
    <header className="comparison-source-heading"><Icon name="file" size={18}/><div><small>{copy[side]}</small><h3><bdi>{names[sideIndex]}</bdi></h3></div></header>
    <ol className="comparison-changes">{page.result.changes.map((change,index)=>change[side]&&<li key={offset+index} data-active={selected===`${offset+index}:${side}`}>
     <div className="comparison-change-kind"><span aria-hidden="true">{change.kind==='cell_modified'?'~':side==='before'?'−':'+'}</span><span>{change.kind==='cell_modified'?copy.modified:side==='before'?copy.removed:copy.added}</span></div>
     {page.previews?.[index][side]&&<div className="comparison-excerpts"><pre dir="auto">{page.previews[index][side]!.text}</pre>{page.previews[index][side]!.truncated&&<small>{workflow[locale].sourceTruncated}</small>}</div>}
     <button className="text-button comparison-source-link" aria-label={workflow[locale].viewSource+' · '+copy[side]} aria-pressed={selected===`${offset+index}:${side}`} onClick={event=>{sourceButton.current=event.currentTarget;void open(index,side);}}>{workflow[locale].viewSource}<Icon name="chevron-right" size={14}/></button>
    </li>)}</ol>
    {!page.result.changes.some(c=>!!c[side])&&<p className="comparison-side-empty">{page.total_changes?copy.pageEmpty:copy.empty}</p>}
   </section>)}</div>
   {(offset>0||page.next_offset!==null)&&<nav className="comparison-pagination" aria-label={copy.changes}><button className="text-button" disabled={loading||offset===0} onClick={()=>setOffset(Math.max(0,offset-20))}>{reader.previous}</button><bdi>{number(offset+(page.result.changes.length?1:0))}–{number(offset+page.result.changes.length)} / {number(page.total_changes)}</bdi><button className="text-button" disabled={loading||page.next_offset===null} onClick={()=>setOffset(page.next_offset!)}>{reader.next}</button></nav>}
  </>}
  {reading&&<p className="comparison-loading" role="status">{messages.common.loading}</p>}
  {source&&<div className="comparison-source" ref={focus} tabIndex={-1} role="region" aria-label={workflow[locale].viewSource}>
   <div className="comparison-evidence-heading"><div><small>{source.citation.source===0?copy.before:copy.after} · {workflow[locale].viewSource}</small><h4><bdi>{names[source.citation.source]}</bdi></h4></div><button className="icon-button" aria-label={reader.close} onClick={()=>{setSource(undefined);setSelected('');restoreFocus(sourceButton.current);}}><Icon name="close" size={18}/></button></div>
   <pre dir="auto">{source.text}</pre>{source.truncated&&<p>{workflow[locale].sourceTruncated}</p>}
   {source.cell&&<dl>{[['value',copy.value],['formula',copy.formula],['cached_value',copy.cached]].map(([key,label])=>typeof source.cell![key]==='string'&&<div key={key}><dt>{label}</dt><dd><bdi>{source.cell![key] as string}</bdi></dd></div>)}</dl>}
   <details className="comparison-evidence-details"><summary>{reader.document}</summary><p><bdi>{source.citation.unit_id}</bdi> · <bdi>{source.citation.revision}</bdi></p>{source.cell&&<><p>{copy.metadata}</p><pre dir="ltr">{JSON.stringify(source.cell,null,2)}</pre></>}</details>
  </div>}
 </div>;
}
