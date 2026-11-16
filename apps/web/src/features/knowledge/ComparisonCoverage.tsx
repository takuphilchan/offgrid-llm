import {useEffect,useRef,useState} from 'react';
import type {Analysis} from '../../api/document-analyses';
import {useI18n} from '../../i18n';
import {documentComparison} from '../../i18n/document-comparison';
import {documentReview} from '../../i18n/document-review';
import {documentReader} from '../../i18n/document-reader';
import {DocumentReview} from '../chat/DocumentReview';
import {documentNavigation} from '../../i18n/document-navigation';
import {UtilityPopover} from '../../components/UtilityPopover';
import {Icon} from '../../components/Icon';

export function ComparisonCoverage({job,names}:{job:Analysis;names:string[]}){
 const {locale,language}=useI18n(),copy=documentComparison(locale),review=documentReview(locale),reader=documentReader(locale),navigation=documentNavigation(locale);
 const [offset,setOffset]=useState(0),[opened,setOpened]=useState<number>();
 const panel=useRef<HTMLDivElement>(null),opener=useRef<HTMLButtonElement|null>(null);
 useEffect(()=>{if(opened!==undefined)panel.current?.focus();},[opened]);
 return <><UtilityPopover className="comparison-coverage comparison-gaps" label={navigation.coverage} trigger={<><Icon name="info" size={16}/>{navigation.coverage}</>}><div className="comparison-coverage-content">
  <p>{copy.help}</p>{(job.plan.gaps.length>0||job.review_required||job.state==='incomplete')&&<><p>{copy.gaps}</p><ul>{job.plan.gaps.slice(offset,offset+20).map((gap,i)=><li key={offset+i}><bdi>{names[gap.source]}</bdi> · <bdi>{gap.unit_id||review.all}</bdi></li>)}</ul>{job.plan.gaps.length>20&&<nav aria-label={copy.gaps}><button className="text-button" disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-20))}>{reader.previous}</button><bdi>{(offset+1).toLocaleString(language)} / {job.plan.gaps.length.toLocaleString(language)}</bdi><button className="text-button" disabled={offset+20>=job.plan.gaps.length} onClick={()=>setOffset(offset+20)}>{reader.next}</button></nav>}</>}
  <div className="comparison-actions">{job.plan.sources.map((source,i)=><button className="text-button" key={i} aria-pressed={opened===i} onClick={event=>{opener.current=event.currentTarget.closest('.comparison-coverage')?.querySelector('button')??null;event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover();setOpened(i);}}>{review.title} · {i===0?copy.before:copy.after}</button>)}</div>
 </div></UtilityPopover>
 {opened!==undefined&&<div className="comparison-expanded-review" ref={panel} tabIndex={-1} role="region" aria-label={review.title}>
  <header><h3><bdi>{names[opened]}</bdi></h3><button className="icon-button" aria-label={reader.close} onClick={()=>{setOpened(undefined);opener.current?.focus();}}><Icon name="close" size={18}/></button></header>
  <DocumentReview key={opened+job.plan.sources[opened].revision} id={job.plan.sources[opened].document_id} revision={job.plan.sources[opened].revision} readOnly/>
 </div>}</>;
}
