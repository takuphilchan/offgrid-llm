import {useEffect,useRef,useState} from 'react';
import {api} from '../../api/client';
import type {Synthesis,SynthesisResults,SynthesisQuote} from '../../api/document-synthesis';
import {useI18n} from '../../i18n';
import {documentSynthesis} from '../../i18n/document-synthesis';
import {documentReader} from '../../i18n/document-reader';
import {restoreFocus} from '../../lib/restore-focus';
import {DocumentReview} from '../chat/DocumentReview';
import {Icon} from '../../components/Icon';
import {SynthesisReviewSection,type ReviewDrafts} from './SynthesisReviewSection';

export function SynthesisSections({job,names,invalidate,drafts}:{job:Synthesis;names:string[];invalidate:()=>void;drafts:ReviewDrafts}){
 const {locale,messages,language}=useI18n(),copy=documentSynthesis(locale),reader=documentReader(locale);
 const [offset,setOffset]=useState(0),[page,setPage]=useState<SynthesisResults>(),[source,setSource]=useState<SynthesisQuote>();
 const sourcePanel=useRef<HTMLElement>(null),sourceButton=useRef<HTMLButtonElement|null>(null);
 const [reviewEnabled,setReviewEnabled]=useState(false);
 useEffect(()=>{let active=true;void api.systemIdentity().then(v=>{if(active)setReviewEnabled(v.capabilities?.includes('document-synthesis-review-v1')===true);}).catch(()=>{});return()=>{active=false;};},[]);
 useEffect(()=>{if(source){sourcePanel.current?.focus();sourcePanel.current?.scrollIntoView({block:'nearest'});}},[source]);
 useEffect(()=>{const abort=new AbortController();setPage(undefined);setSource(undefined);void api.synthesis.results(job,offset,abort.signal).then(p=>{if(!abort.signal.aborted)setPage(p);}).catch(()=>{if(!abort.signal.aborted)invalidate();});return()=>abort.abort();},[job.id,job.completed_batches,offset]);
 const quotes=(items:SynthesisQuote[],label:string)=>items.length>0&&<div className="synthesis-quote-group"><h5>{label}</h5>{items.map((q,i)=><button className="synthesis-quote" key={i} onClick={event=>{sourceButton.current=event.currentTarget;setSource(q);}}><span dir="auto">{q.text}</span><small><bdi>{names[q.citation.source]}</bdi><Icon name="chevron-right" size={14}/></small></button>)}</div>;
 if(!page)return <p className="comparison-loading">{messages.common.loading}</p>;
 return <div className="synthesis-sections">
  <nav className="comparison-pagination" aria-label={reader.document}><button className="icon-button" aria-label={reader.previous} disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-2))}><Icon name="chevron-left"/></button><span>{(offset+1).toLocaleString(language)}–{(offset+page.receipts.length).toLocaleString(language)} / {page.total_batches.toLocaleString(language)}</span><button className="icon-button" aria-label={reader.next} disabled={page.next_offset===null} onClick={()=>setOffset(page.next_offset!)}><Icon name="chevron-right"/></button></nav>
  {page.receipts.map((receipt,batch)=><SynthesisReviewSection key={receipt.batch_id} id={job.id} batch={offset+batch} receipt={receipt} enabled={reviewEnabled&&job.state==='awaiting_review'} drafts={drafts} invalidate={invalidate}>{controls=><>
   {receipt.result.gaps.length>0&&<p className="synthesis-review-note">{reader.partial}</p>}
   {!receipt.result.candidates.length&&<p className="comparison-waiting">{copy.empty}</p>}
   {receipt.result.candidates.map((candidate,i)=><article key={i} className="synthesis-candidate"><div className="synthesis-candidate-text"><span className="synthesis-draft-label">{copy.review}</span><p dir="auto">{candidate.text}</p>{controls(i)}</div><aside>{quotes(candidate.evidence,copy.evidence)}{quotes(candidate.conditions,copy.conditions)}{quotes(candidate.exceptions,copy.exceptions)}</aside></article>)}
  </>}</SynthesisReviewSection>)}
  {source&&<section className="synthesis-source" ref={sourcePanel} tabIndex={-1} aria-label={reader.document}><header><h4><bdi>{names[source.citation.source]}</bdi></h4><button className="icon-button" aria-label={reader.close} onClick={()=>{setSource(undefined);restoreFocus(sourceButton.current);}}><Icon name="close" size={18}/></button></header><blockquote dir="auto">{source.text}</blockquote><details><summary>{reader.document}</summary><DocumentReview key={source.citation.revision+source.citation.span.start_byte} id={job.plan.sources[source.citation.source].document_id} revision={source.citation.revision} highlight={source.text} readOnly/></details><p>{copy.help}</p></section>}
 </div>;
}
