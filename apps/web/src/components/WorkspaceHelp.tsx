import { useEffect, useRef } from 'react';
import { useI18n } from '../i18n';
import { workspaceHelp } from '../i18n/workspace-help';
import { restoreFocus } from '../lib/restore-focus';
import { Icon } from './Icon';
import './workspace-help.css';

/** Local guidance: navigation only, never grants, submissions or downloads. */
export function WorkspaceHelp({ close, guide }: { close:()=>void; guide:()=>void }) {
  const {locale,messages:text}=useI18n(), copy=workspaceHelp[locale];
  const dialog=useRef<HTMLDialogElement>(null);
  useEffect(()=>{
    const node=dialog.current!, previous=document.activeElement as HTMLElement|null;
    const overflow=document.documentElement.style.overflow;
    document.documentElement.style.overflow='hidden';
    node.showModal();
    return()=>{
      node.close();
      document.documentElement.style.overflow=overflow;
      restoreFocus(previous===document.body||!previous?.isConnected?document.querySelector<HTMLElement>('.help-trigger'):previous);
    };
  },[]);
  const sections=[
    {title:text.nav.chat,body:copy.chat,link:'#/chat',action:text.nav.chat},
    {title:text.nav.agents,body:copy.agents,link:'#/agents/profiles',action:text.nav.agents},
    {title:copy.settings,body:copy.access,link:null,action:null},
    {title:text.nav.activity,body:copy.returning,link:'#/activity',action:text.nav.activity},
  ];
  return <dialog ref={dialog} className="workspace-help" aria-labelledby="workspace-help-title" onCancel={event=>{event.preventDefault();close();}}>
    <header><h2 id="workspace-help-title">{copy.help}</h2><button autoFocus type="button" className="icon-button utility-button" onClick={close} aria-label={copy.close}><Icon name="close" size={18}/></button></header>
    <div className="workspace-help-body">
      <p className="workspace-help-example">{copy.example}</p>
      {sections.map(section=><section key={section.title}><h3>{section.title}</h3><p>{section.body}</p>{section.link&&<a className="text-button" href={section.link} onClick={close}>{section.action}</a>}</section>)}
    </div>
    <footer><button type="button" className="text-button" onClick={guide}>{text.settings.showGuide}</button><a className="text-button" href="#/settings" onClick={close}>{text.nav.settings}</a></footer>
  </dialog>;
}
