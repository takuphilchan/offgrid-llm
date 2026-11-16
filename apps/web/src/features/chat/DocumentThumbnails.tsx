import {useEffect,useLayoutEffect,useRef,useState} from 'react';
import {api,type DocumentPreview} from '../../api/client';
import {Icon} from '../../components/Icon';
import {useI18n} from '../../i18n';
import {documentNavigation} from '../../i18n/document-navigation';
import {knowledgeUnitLabels} from '../../i18n/knowledge-locators';
import {documentThumbnails} from '../../i18n/document-thumbnails';
import './document-thumbnails.css';
import {UtilityPopover} from '../../components/UtilityPopover';
import {DocumentFileIcon} from './DocumentFileIcon';
import {sourceClose} from '../../i18n/knowledge-original';

export function DocumentThumbnails({name,preview,path,page,select}:{name:string;preview:DocumentPreview;path:string;page:number;select:(page:number)=>void}){
 const {locale,language,messages}=useI18n(),number=new Intl.NumberFormat(language);
 const [open,setOpen]=useState(false),[images,setImages]=useState<Record<number,string>>({}),[failed,setFailed]=useState<number[]>([]),[retry,setRetry]=useState(0);
 const [start,setStart]=useState(0),list=useRef<HTMLElement>(null),keyboardNavigation=useRef(false);
 const rowHeight=preview.thumbnails_available?240:112,units=preview.units.slice(start,start+4);
 useLayoutEffect(()=>{
  if(!open||!list.current)return;
  const panel=list.current.closest<HTMLElement>('[popover]');if(!panel)return;
  const update=()=>setStart(Math.max(0,Math.min(Math.floor(panel.scrollTop/rowHeight),preview.units.length-4)));
  panel.scrollTop=Math.max(0,(page-1)*rowHeight);
  update();panel.addEventListener('scroll',update,{passive:true});
  return()=>panel.removeEventListener('scroll',update);
 },[open,rowHeight,preview.units.length]);
 useLayoutEffect(()=>{
  if(!open||!list.current)return;
  const panel=list.current.closest<HTMLElement>('[popover]');if(!panel)return;
  const top=(page-1)*rowHeight;
  if(top<panel.scrollTop||top+rowHeight>panel.scrollTop+panel.clientHeight-64)panel.scrollTop=top;
 },[page,open,rowHeight]);
 useLayoutEffect(()=>{
  if(!keyboardNavigation.current)return;
  const selected=list.current?.querySelector<HTMLButtonElement>('[aria-current=page]');
  if(selected){selected.focus({preventScroll:true});keyboardNavigation.current=false;}
 },[page,start,open]);
 useEffect(()=>{
  setImages({});setFailed([]);if(!open||!preview.thumbnails_available)return;
  const controller=new AbortController(),urls:string[]=[];
  // One bounded raster request at a time; only four thumbnails remain decoded.
  void(async()=>{for(const unit of preview.units.slice(start,start+4)){
   if(controller.signal.aborted)break;
   try{const blob=await api.documentThumbnail(`${path}${path.includes('?')?'&':'?'}sha256=${preview.sha256}&page=${unit.index}&thumbnail=true`,preview.sha256,controller.signal);
    if(controller.signal.aborted)break;
    const url=URL.createObjectURL(blob);urls.push(url);setImages(value=>({...value,[unit.index]:url}));
   }catch{if(!controller.signal.aborted)setFailed(value=>[...value,unit.index]);}
  }})();
  return()=>{controller.abort();urls.forEach(url=>URL.revokeObjectURL(url));};
 },[open,start,path,preview.sha256,preview.units,preview.thumbnails_available,retry]);
 return <UtilityPopover className="document-thumbnails" panelClassName="document-thumbnail-panel" compact preferredHeight={720} label={documentThumbnails(locale).pages} triggerClassName="document-pages-trigger" trigger={<><DocumentFileIcon name={name}/><span dir="auto">{name}</span><Icon name="chevron-down" size={12}/></>} onOpenChange={setOpen}>
  <div className="document-thumbnail-heading"><span>{documentThumbnails(locale).pages} · {number.format(preview.units.length)}</span><button type="button" className="icon-button" data-popover-dismiss title={sourceClose[locale]} aria-label={sourceClose[locale]}><Icon name="sidebar" size={16}/></button></div>
  {open&&<div className="document-thumbnails-strip">
   <nav ref={list} aria-label={knowledgeUnitLabels[locale].page} onKeyDown={event=>{
    if(!['ArrowUp','ArrowDown','Home','End'].includes(event.key))return;
    event.preventDefault();
    const focused=Number((event.target as HTMLElement).closest<HTMLElement>('[data-reader-page]')?.dataset.readerPage)||page;
    const next=event.key==='Home'?1:event.key==='End'?preview.units.length:Math.max(1,Math.min(preview.units.length,focused+(event.key==='ArrowDown'?1:-1)));
    keyboardNavigation.current=true;
    // Home/End must also work when the selected page has not changed, but
    // scrolling has removed its thumbnail from the virtual window.
    const panel=list.current?.closest<HTMLElement>('[popover]');
    if(panel){panel.scrollTop=(next-1)*rowHeight;setStart(Math.max(0,Math.min(Math.floor(panel.scrollTop/rowHeight),preview.units.length-4)));}
    select(next);
   }}><div aria-hidden="true" style={{height:start*rowHeight}}/>{units.map((unit,index)=>{
    const label=`${knowledgeUnitLabels[locale].page} ${number.format(unit.index)}`,review=!['ready','empty'].includes(unit.state);
    return <button key={unit.index} type="button" className="document-thumbnail" data-reader-page={start+index+1} style={{height:rowHeight}} aria-label={label} aria-current={page===start+index+1?'page':undefined} onClick={()=>select(start+index+1)}>
     <span className="document-thumbnail-number">{number.format(unit.index)}{review&&<span className="document-thumbnail-review" title={documentNavigation(locale).needsReview}><span className="sr-only"> · {documentNavigation(locale).needsReview}</span></span>}</span>
     {preview.thumbnails_available ? <span className="document-thumbnail-image" aria-busy={!images[unit.index]&&!failed.includes(unit.index)}>{images[unit.index]?<img src={images[unit.index]} alt="" onError={()=>{setFailed(value=>[...value,unit.index]);setImages(value=>{const next={...value};delete next[unit.index];return next;});}}/>:<Icon name="file" size={22}/>}</span> : <span className="document-thumbnail-excerpt" dir="auto">{unit.name || unit.text.slice(0,120) || label}</span>}
    </button>;
   })}<div aria-hidden="true" style={{height:Math.max(0,preview.units.length-start-4)*rowHeight}}/></nav>
  </div>}
  {open&&failed.length>0&&<button type="button" className="text-button" onClick={()=>setRetry(value=>value+1)}>{messages.common.retry}</button>}
 </UtilityPopover>;
}
