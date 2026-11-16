import {useEffect, useRef, useState} from 'react';
import type {CommandWindowState} from '../../platform';
import {useI18n} from '../../i18n';
import {desktopCommand} from '../../i18n/desktop-command';
import {Icon} from '../Icon';
import {cancelPresentationTransition, transitionPresentation} from '../../lib/presentation-transition';
import './command-window.css';

export function useCommandWindow() {
 const [state, setState] = useState<CommandWindowState>({mode:'full',shortcut:'',registered:false});
 const latest = useRef(state); latest.current = state;
 useEffect(() => {
  let disposed = false, received = false;
  const accept = (value:CommandWindowState) => {
   if (disposed) return;
   received = true; setState(value); latest.current = value;
   if (value.mode === 'compact') {
    if (!['', '#/home', '#/chat', '#/chat/new'].includes(location.hash) && !location.hash.startsWith('#/chat/')) location.hash = '#/chat';
    requestAnimationFrame(() => document.querySelector<HTMLTextAreaElement>('.composer textarea')?.focus());
   }
  };
  const unsubscribe = window.electron?.onCommandWindow?.(accept);
  const transition = window.electron?.onCommandTransition?.(({id}) => {
   void transitionPresentation(reduceMotion => window.electron?.acknowledgeCommandTransition?.({id,reduceMotion})).catch(() => {});
  });
  const dismiss = () => cancelPresentationTransition();
  window.addEventListener('offgrid-desktop-dismiss', dismiss);
  void window.electron?.getCommandWindow?.().then(value => {if (!received) accept(value);}).catch(() => {});
  const navigate = () => {if (latest.current.mode === 'compact' && !/^#\/(?:home|chat)(?:\/|$)/.test(location.hash)) void window.electron?.expandCommandWindow?.();};
  const escape = (event:KeyboardEvent) => {
   if (event.key !== 'Escape' || event.defaultPrevented || event.isComposing || latest.current.mode !== 'compact' || document.querySelector('dialog[open], :popover-open, [role="dialog"][aria-modal="true"]')) return;
   event.preventDefault(); void window.electron?.dismissCommandWindow?.();
  };
  window.addEventListener('hashchange', navigate); window.addEventListener('keydown', escape);
  return () => { disposed = true; unsubscribe?.(); transition?.(); cancelPresentationTransition(); window.removeEventListener('offgrid-desktop-dismiss', dismiss); window.removeEventListener('hashchange', navigate); window.removeEventListener('keydown', escape); };
 }, []);
 return state;
}

export function CommandWindowHeader() {
 const {locale,messages} = useI18n(), copy = desktopCommand(locale);
 return <header className="command-window-header"><strong>{messages.product}</strong><div>
  <button className="icon-button" aria-label={messages.chat.newChat} title={messages.chat.newChat} onClick={()=>{location.hash='#/chat/new';}}><Icon name="plus" size={18}/></button>
  <button className="icon-button" aria-label={copy.expand} title={copy.expand} onClick={()=>void window.electron?.expandCommandWindow?.()}><Icon name="sidebar" size={18}/></button>
  <button className="icon-button" aria-label={copy.hide} title={copy.hide} onClick={()=>void window.electron?.dismissCommandWindow?.()}><Icon name="close" size={18}/></button>
 </div></header>;
}
