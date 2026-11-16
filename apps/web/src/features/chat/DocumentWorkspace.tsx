import { createContext, useCallback, useContext, useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import type { DocumentCitation } from '../../api/client';
import { useI18n } from '../../i18n';
import { documentReader } from '../../i18n/document-reader';
import { DocumentReader } from './DocumentReader';
import { Icon } from '../../components/Icon';
import './document-reader.css';
import {useDraftScope} from '../../lib/draft-scope';
import {useDraft} from '../../lib/drafts';
import {DocumentTabs,readerSourceKey} from './DocumentTabs';

export type ReaderSource = { name: string; path: string; original: () => Promise<Blob>; sha256?: string; citation?: DocumentCitation; verifiedText?: string; passage?: string; documentId?: string; extractionRevision?: string; attachment?: {session: string; id: string; revision: string} };
const ReaderContext = createContext<((source: ReaderSource) => void) | null>(null);
export const useDocumentReader = () => useContext(ReaderContext);

export function DocumentWorkspace({ children, scope, selectPage, addFile }: { children: ReactNode; scope: string; selectPage?: (source: ReaderSource, page: number) => void; addFile?:()=>void }) {
 const { locale, messages } = useI18n(), copy = documentReader(locale);
 const layoutDraft=useDraft(useDraftScope(),'document-pane-width');
 const storedWidth=Number(layoutDraft.value),savedWidth=storedWidth>=25&&storedWidth<=65?storedWidth:40;
 const [dragWidth,setDragWidth]=useState<number>();
 const width=dragWidth??savedWidth,frame=useRef<HTMLDivElement>(null);
 const clamp=(n:number)=>Math.round(Math.min(65,Math.max(25,n)));
 const [selection, setSelection] = useState<{ scope: string; source: ReaderSource; sources:ReaderSource[] }>();
 const source = selection?.scope === scope ? selection.source : undefined;
 // Reset only the reader. Remounting Chat would detach its scroll observers and
 // input ownership; an old scope's preview must still disappear synchronously.
 useEffect(() => { setSelection(undefined); }, [scope]);
 const [view, setView] = useState('document');
 const [chatVisible, setChatVisible] = useState(true);
 const trigger = useRef<HTMLElement | null>(null);
 const open = useCallback((next: ReaderSource) => {
  void window.electron?.getCommandWindow?.().then(state=>{if(state.mode==='compact')void window.electron?.expandCommandWindow?.();}).catch(()=>{});
  trigger.current = document.activeElement as HTMLElement;
  // Reopening the same attachment is navigation, not a new preview request.
  // Keep its page, zoom and search; a citation deliberately selects a passage.
  setSelection(current => {
   if(current?.scope===scope&&readerSourceKey(current.source)===readerSourceKey(next)&&!next.citation&&!next.passage&&!next.verifiedText)return current;
   const existing=current?.scope===scope?current.sources:[],key=readerSourceKey(next);
   const sources=existing.some(item=>readerSourceKey(item)===key)?existing.map(item=>readerSourceKey(item)===key?next:item):[...existing,next].slice(-8);
   return {scope,source:next,sources};
  });
  setView('document');
 }, [scope]);
 const closeTab = useCallback((closing:ReaderSource) => {
  setSelection(current=>{
   if(!current||current.scope!==scope)return current;
   const sources=current.sources.filter(item=>readerSourceKey(item)!==readerSourceKey(closing));
   if(!sources.length)return undefined;
   const nextIndex=Math.max(0,Math.min(current.sources.findIndex(item=>readerSourceKey(item)===readerSourceKey(closing)),sources.length-1));
   return {...current,sources,source:readerSourceKey(current.source)===readerSourceKey(closing)?sources[nextIndex]:current.source};
  });
  requestAnimationFrame(()=>{const next=frame.current?.querySelector<HTMLElement>('.document-tab[aria-selected=true]');if(next)next.focus();else if(trigger.current?.isConnected)trigger.current.focus();});
 }, [scope]);
 const close=()=>{if(source)closeTab(source);};
 return <ReaderContext.Provider value={open}><div ref={frame} style={{'--document-chat-size':`${width}%`} as CSSProperties} className={`document-workspace ${source ? 'has-document' : ''}${!chatVisible?' document-focused':''}`} data-view={view}>
  {source && <div className="document-view-tabs" role="tablist" aria-label={copy.document} onKeyDown={event=>{if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const next=event.key==='Home'?'chat':event.key==='End'?'document':view==='chat'?'document':'chat';setView(next);event.currentTarget.querySelectorAll<HTMLButtonElement>('button')[next==='chat'?0:1]?.focus();}}}>
   <button className="text-button" role="tab" tabIndex={view==='chat'?0:-1} aria-selected={view === 'chat'} onClick={() => setView('chat')}>{messages.chat.title}</button>
   <button className="text-button" role="tab" tabIndex={view==='document'?0:-1} aria-selected={view === 'document'} onClick={() => setView('document')}>{copy.document}</button>
  </div>}
  <div className="document-chat">{children}</div>
  {source&&chatVisible&&<div className="document-pane-resize" role="separator" aria-orientation="vertical" aria-label={copy.document} aria-valuemin={25} aria-valuemax={65} aria-valuenow={width} tabIndex={0}
   onPointerDown={event=>{event.currentTarget.setPointerCapture(event.pointerId);setDragWidth(width);event.preventDefault();}}
   onPointerMove={event=>{if(!event.currentTarget.hasPointerCapture(event.pointerId)||!frame.current)return;const rect=frame.current.getBoundingClientRect(),fraction=(event.clientX-rect.left)/rect.width;setDragWidth(clamp((getComputedStyle(frame.current).direction==='rtl'?1-fraction:fraction)*100));}}
   onPointerUp={event=>{layoutDraft.setValue(String(width));setDragWidth(undefined);event.currentTarget.releasePointerCapture(event.pointerId);}}
   onPointerCancel={()=>setDragWidth(undefined)}
   onKeyDown={event=>{if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key))return;event.preventDefault();const direction=document.documentElement.dir==='rtl'?-1:1;layoutDraft.setValue(String(event.key==='Home'?25:event.key==='End'?65:clamp(width+(event.key==='ArrowRight'?2:-2)*direction)));}}/>}
  {source && <DocumentReader key={JSON.stringify([source.path,source.sha256,source.extractionRevision,source.citation,source.passage])} source={source} close={close} tabs={<DocumentTabs sources={selection!.sources} active={source} select={next=>setSelection(current=>current&&{...current,source:next})} close={closeTab} add={addFile}/>} selectPage={selectPage ? page=>{selectPage(source,page);setChatVisible(true);setView('chat');}:undefined} actions={<button type="button" className="icon-button document-chat-toggle" aria-label={messages.chat.title} title={messages.chat.title} aria-pressed={chatVisible} onClick={()=>setChatVisible(value=>!value)}><Icon name="sidebar" size={18}/></button>}/>}
 </div></ReaderContext.Provider>;
}
