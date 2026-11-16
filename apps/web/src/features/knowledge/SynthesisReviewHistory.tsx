import {useEffect,useState} from 'react';
import {api} from '../../api/client';
import type {SynthesisReview,SynthesisResults} from '../../api/document-synthesis';
import {useI18n} from '../../i18n';
import {synthesisReviewCopy} from '../../i18n/document-synthesis-review';

export function SynthesisReviewHistory({latest,receipt,invalidate}:{latest:SynthesisReview;receipt:SynthesisResults['receipts'][number];invalidate:()=>void}){
 const {locale,language,messages}=useI18n(),copy=synthesisReviewCopy(locale);
 const [open,setOpen]=useState(false),[version,setVersion]=useState(1),[review,setReview]=useState<SynthesisReview>();
 useEffect(()=>{if(!open)return;const abort=new AbortController();setReview(undefined);
  void api.synthesis.review(latest.analysis_id,latest.batch,receipt,abort.signal,version).then(v=>{if(!abort.signal.aborted)setReview(v);}).catch(()=>{if(!abort.signal.aborted)invalidate();});return()=>abort.abort();
 },[open,version,latest.analysis_id,latest.batch]);
 if(latest.revision<2)return null;
 return <details className="synthesis-review-history" onToggle={e=>setOpen(e.currentTarget.open)}><summary>{copy.history}</summary><select aria-label={copy.history} value={version} onChange={e=>setVersion(Number(e.target.value))}>{Array.from({length:latest.revision-1},(_,i)=><option key={i+1} value={i+1}>#{(i+1).toLocaleString(language)}</option>)}</select>
 {!review?<p role="status">{messages.common.loading}</p>:<div><p><bdi>{review.author}</bdi> · {new Date(review.updated_at).toLocaleString(language)}</p><label className="synthesis-review-check"><input type="checkbox" disabled checked={review.section_checked}/><span>{copy.checked}</span></label>{review.decisions.map((d,i)=><article key={i}><p dir="auto">{receipt.result.candidates[i].text}</p><strong>{d.decision==='unreviewed'?copy.pending:copy[d.decision]}</strong>{d.note&&<p dir="auto">{d.note}</p>}</article>)}</div>}
 </details>;
}
