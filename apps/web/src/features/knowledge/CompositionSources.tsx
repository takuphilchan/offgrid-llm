import {useEffect,useRef,useState} from 'react';
import {api} from '../../api/client';
import type {Synthesis,SynthesisQuote} from '../../api/document-synthesis';
import type {CompositionReport,CompositionEvidence} from '../../api/document-composition';
import {useI18n} from '../../i18n';
import {documentSynthesis} from '../../i18n/document-synthesis';
import {documentReader} from '../../i18n/document-reader';
import {synthesisReviewCopy} from '../../i18n/document-synthesis-review';
import {DocumentReview} from '../chat/DocumentReview';
import {Icon} from '../../components/Icon';
import {restoreFocus} from '../../lib/restore-focus';

export function CompositionSources({parent,report,section,names,close,invalidate}:{parent:Synthesis;report:CompositionReport;section:number;names:string[];close:()=>void;invalidate:()=>void}){
 const {locale,language,messages}=useI18n(),copy=documentSynthesis(locale),reader=documentReader(locale),review=synthesisReviewCopy(locale);
 const [offset,setOffset]=useState(0),[page,setPage]=useState<CompositionEvidence>(),[source,setSource]=useState<SynthesisQuote>();
 const panel=useRef<HTMLElement>(null),opener=useRef(document.activeElement as HTMLElement|null);
 useEffect(()=>{panel.current?.focus({preventScroll:true});return()=>restoreFocus(opener.current);},[]);
 useEffect(()=>{const abort=new AbortController();setPage(undefined);setSource(undefined);
  void api.composition.evidence(parent,report,section,offset,abort.signal).then(value=>{if(!abort.signal.aborted)setPage(value);}).catch(()=>{if(!abort.signal.aborted)invalidate();});return()=>abort.abort();
 },[report.composition.id,section,offset]);
 const quotes=(items:SynthesisQuote[],label:string)=>items.length>0&&<div className="synthesis-quote-group"><h5>{label}</h5>{items.map((q,i)=><button className="synthesis-quote" key={i} onClick={()=>setSource(q)}><span dir="auto">{q.text}</span><small><bdi>{names[q.citation.source]}</bdi><Icon name="chevron-right" size={14}/></small></button>)}</div>;
 return <aside ref={panel} tabIndex={-1} className="composition-evidence" aria-label={copy.evidence}><header><h4>{copy.evidence}</h4><button className="icon-button" aria-label={reader.close} onClick={close}><Icon name="close" size={17}/></button></header>
  {page?<><nav className="comparison-pagination" aria-label={copy.evidence}><button className="icon-button" aria-label={reader.previous} disabled={!offset} onClick={()=>setOffset(n=>Math.max(0,n-8))}><Icon name="chevron-left" size={15}/></button><span>{(offset+1).toLocaleString(language)}–{(offset+page.items.length).toLocaleString(language)} / {page.total.toLocaleString(language)}</span><button className="icon-button" aria-label={reader.next} disabled={page.next_offset===null} onClick={()=>setOffset(page.next_offset!)}><Icon name="chevron-right" size={15}/></button></nav>
   {page.items.map(item=><article key={item.input.id}><p className="synthesis-draft-label">{item.input.decision==='keep'?review.keep:review.pending} · {review.history} {item.input.review_revision.toLocaleString(language)}</p>{quotes(item.candidate.evidence,copy.evidence)}{quotes(item.candidate.conditions,copy.conditions)}{quotes(item.candidate.exceptions,copy.exceptions)}</article>)}
  </>:<p role="status">{messages.common.loading}</p>}
  {source&&<section className="composition-source-reader"><h4><bdi>{names[source.citation.source]}</bdi></h4><DocumentReview key={source.citation.revision+source.citation.span.start_byte} id={parent.plan.sources[source.citation.source].document_id} revision={source.citation.revision} highlight={source.text} readOnly/></section>}
 </aside>;
}
