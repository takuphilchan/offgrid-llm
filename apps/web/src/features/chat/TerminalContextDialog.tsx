import { useEffect, useRef, useState } from 'react';
import type { ChatSession } from '../../api/client';
import { useI18n } from '../../i18n';
import { terminalContextText } from '../../i18n/terminal-context';
import { terminalContextDraft, useTerminalContext } from '../../lib/terminal-context';
import { restoreFocus } from '../../lib/restore-focus';
import { visibleSessionTitle } from './chat-presentation';

export function TerminalContextDialog({sessions,current,disabled,accept}:{sessions:ChatSession[];current:string;disabled:boolean;accept:(name:string,draft:string)=>void}) {
  const {pending,clear}=useTerminalContext();const {locale,messages}=useI18n();const copy=terminalContextText[locale];
  const ref=useRef<HTMLDialogElement>(null);const [target,setTarget]=useState('');const [draft,setDraft]=useState('');
  useEffect(()=>{if(!pending)return;setTarget(current);setDraft(terminalContextDraft(pending));const previous=document.activeElement as HTMLElement|null;ref.current?.showModal();return()=>{ref.current?.close();restoreFocus(previous);};},[pending,current]);
  if(!pending)return null;
  return <dialog ref={ref} className="history-delete-dialog terminal-context-dialog" aria-label={copy.title} onCancel={event=>{event.preventDefault();clear();}}>
    <h2>{copy.title}</h2><p>{copy.notice}</p>
    <label>{copy.conversation}<select aria-label={copy.conversation} value={target} onChange={event=>setTarget(event.target.value)}><option value="">{messages.chat.newChat}</option>{sessions.map(session=><option key={session.name} value={session.name}>{visibleSessionTitle(session)}</option>)}</select></label>
    <label>{copy.preview}<textarea value={draft} maxLength={45000} rows={10} onChange={event=>setDraft(event.target.value)} /></label>
    <p>{copy.review}</p><div className="history-dialog-actions"><button className="secondary-button" onClick={clear}>{messages.models.cancel}</button><button className="primary-button" disabled={disabled||!draft.trim()||!!target&&!sessions.some(session=>session.name===target)} onClick={()=>{accept(target,draft);clear();}}>{copy.add}</button></div>
  </dialog>;
}
