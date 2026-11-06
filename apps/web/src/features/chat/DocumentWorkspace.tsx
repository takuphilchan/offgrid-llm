import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import type { DocumentCitation } from '../../api/client';
import { useI18n } from '../../i18n';
import { documentReader } from '../../i18n/document-reader';
import { DocumentReader } from './DocumentReader';
import './document-reader.css';

export type ReaderSource = { name: string; path: string; original: () => Promise<Blob>; sha256?: string; citation?: DocumentCitation; verifiedText?: string; passage?: string };
const ReaderContext = createContext<((source: ReaderSource) => void) | null>(null);
export const useDocumentReader = () => useContext(ReaderContext);

export function DocumentWorkspace({ children, scope }: { children: ReactNode; scope: string }) {
 const { locale, messages } = useI18n(), copy = documentReader(locale);
 const [selection, setSelection] = useState<{ scope: string; source: ReaderSource }>();
 const source = selection?.scope === scope ? selection.source : undefined;
 // Reset only the reader. Remounting Chat would detach its scroll observers and
 // input ownership; an old scope's preview must still disappear synchronously.
 useEffect(() => { setSelection(undefined); }, [scope]);
 const [view, setView] = useState('document');
 const trigger = useRef<HTMLElement | null>(null);
 const open = useCallback((next: ReaderSource) => {
  trigger.current = document.activeElement as HTMLElement;
  // Reopening the same attachment is navigation, not a new preview request.
  // Keep its page, zoom and search; a citation deliberately selects a passage.
  setSelection(current => current?.scope === scope && current.source.path === next.path && current.source.sha256 === next.sha256
   && !next.citation && !next.passage && !next.verifiedText ? current : {scope,source:next});
  setView('document');
 }, [scope]);
 const close = useCallback(() => { setSelection(undefined); requestAnimationFrame(() => { if (trigger.current?.isConnected) trigger.current.focus(); }); }, []);
 return <ReaderContext.Provider value={open}><div className={`document-workspace ${source ? 'has-document' : ''}`} data-view={view}>
  {source && <div className="document-view-tabs" role="tablist" aria-label={copy.document} onKeyDown={event=>{if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const next=event.key==='Home'?'chat':event.key==='End'?'document':view==='chat'?'document':'chat';setView(next);event.currentTarget.querySelectorAll<HTMLButtonElement>('button')[next==='chat'?0:1]?.focus();}}}>
   <button className="text-button" role="tab" tabIndex={view==='chat'?0:-1} aria-selected={view === 'chat'} onClick={() => setView('chat')}>{messages.chat.title}</button>
   <button className="text-button" role="tab" tabIndex={view==='document'?0:-1} aria-selected={view === 'document'} onClick={() => setView('document')}>{copy.document}</button>
  </div>}
  <div className="document-chat">{children}</div>
  {source && <DocumentReader key={source.path + JSON.stringify(source.citation) + (source.passage ?? '')} source={source} close={close} />}
 </div></ReaderContext.Provider>;
}
