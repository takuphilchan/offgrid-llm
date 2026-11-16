import {Icon} from '../../components/Icon';
import {useI18n} from '../../i18n';
import {documentReader} from '../../i18n/document-reader';
import {chatFiles} from '../../i18n/chat-files';
import {DocumentFileIcon} from './DocumentFileIcon';
import type {ReaderSource} from './DocumentWorkspace';

export const readerSourceKey=(source:ReaderSource)=>JSON.stringify([source.path,source.sha256,source.extractionRevision,source.attachment?.revision]);

export function DocumentTabs({sources,active,select,close,add}:{sources:ReaderSource[];active:ReaderSource;select:(source:ReaderSource)=>void;close:(source:ReaderSource)=>void;add?:()=>void}) {
 const {locale}=useI18n(),copy=documentReader(locale);
 return <div className="document-tabs-bar"><div className="document-tabs" role="tablist" aria-label={copy.document} onKeyDown={event=>{
  if(event.target instanceof HTMLElement&&event.target.getAttribute('role')!=='tab')return;
  if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;
  event.preventDefault();const current=sources.findIndex(item=>readerSourceKey(item)===readerSourceKey(active));
  const delta=(event.key==='ArrowRight'?1:-1)*(getComputedStyle(event.currentTarget).direction==='rtl'?-1:1);
  const index=event.key==='Home'?0:event.key==='End'?sources.length-1:(current+delta+sources.length)%sources.length;
  select(sources[index]);event.currentTarget.querySelectorAll<HTMLButtonElement>('[role=tab]')[index]?.focus();
 }}>{sources.map(source=>{
  const selected=readerSourceKey(source)===readerSourceKey(active);
  return <div key={readerSourceKey(source)} className="document-file-tab" data-active={selected}>
   <button type="button" role="tab" className="document-tab" aria-selected={selected} tabIndex={selected?0:-1} title={source.name} onClick={()=>select(source)}><DocumentFileIcon name={source.name}/><span dir="auto">{source.name}</span></button>
   <button type="button" className="icon-button" aria-label={selected?copy.close:`${copy.close}: ${source.name}`} title={copy.close} onClick={()=>close(source)}><Icon name="close" size={16}/></button>
  </div>;
 })}</div>{add&&<button type="button" className="icon-button document-add-file" aria-label={chatFiles(locale).add} title={chatFiles(locale).add} onClick={add}><Icon name="plus" size={19}/></button>}</div>;
}
