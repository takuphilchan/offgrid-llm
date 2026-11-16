import { useEffect, useRef, useState } from 'react';
import { useI18n } from '../i18n';
import { workspaceHelp } from '../i18n/workspace-help';
import { helpCenter } from '../i18n/help-center';
import { helpArticles, findHelpArticles } from './help-articles';
import { restoreFocus } from '../lib/restore-focus';
import { Icon } from './Icon';
import { SearchField } from './SearchField';
import { useWorkspaceState } from '../lib/workspace-context';
import type { ModelCategory } from '../api/client';
import './workspace-help.css';

/** Local guidance only. The underlying workspace remains mounted with its draft. */
export function WorkspaceHelp({ close }: { close:()=>void }) {
  const {locale,messages:text}=useI18n(), copy=workspaceHelp[locale], labels=helpCenter[locale];
  const [query,setQuery]=useState(''), [selected,setSelected]=useState('chat');
  const [,setCategory]=useWorkspaceState<ModelCategory>('models.category','language');
  const [,setView]=useWorkspaceState<'installed'|'discover'>('models.view','installed');
  const root=useRef<HTMLElement>(null), article=useRef<HTMLElement>(null);
  const topics=helpArticles(locale,text,!!window.electron);
  const matches=findHelpArticles(topics,query), active=matches.find(item=>item.id===selected)??matches[0];
  useEffect(()=>{
    const previous=document.activeElement as HTMLElement|null;
    root.current?.querySelector<HTMLInputElement>('input[type="search"]')?.focus();
    return()=>restoreFocus(previous===document.body||!previous?.isConnected?document.querySelector<HTMLElement>('.help-trigger'):previous);
  },[]);
  return <section ref={root} className="workspace-help" aria-labelledby="workspace-help-title" onKeyDown={event=>{
    if(event.key==='Escape'&&!event.nativeEvent.isComposing){event.preventDefault();event.stopPropagation();close();}
  }}>
    <header className="help-heading"><div><h2 id="workspace-help-title">{copy.help}</h2><p>{labels.browse}</p></div><button type="button" className="icon-button utility-button" onClick={close} aria-label={copy.close} title={copy.close}><Icon name="close" size={18}/></button></header>
    <div className="help-search"><SearchField label={labels.search} value={query} onChange={setQuery}/><span className="help-result-count" role="status" aria-live="polite">{matches.length} · {labels.topics}</span></div>
    <div className="help-layout">
      <nav className="help-topics" aria-label={labels.topics}>
        {matches.map(topic=><button type="button" key={topic.id} aria-current={active?.id===topic.id?'page':undefined} onClick={()=>{setSelected(topic.id);requestAnimationFrame(()=>article.current?.focus());}}><Icon name={topic.icon} size={19}/><span>{topic.title}</span><Icon name="chevron-right" size={14}/></button>)}
        <button type="button" className="help-setup" onClick={()=>{setQuery('');setSelected('models');requestAnimationFrame(()=>article.current?.focus());}}><Icon name="home" size={19}/><span>{text.settings.showGuide}</span></button>
      </nav>
      {active ? <article ref={article} tabIndex={-1} className="help-article" aria-labelledby="help-article-title" key={active.id}>
        <span className="help-article-icon"><Icon name={active.icon} size={26}/></span>
        <h3 id="help-article-title">{active.title}</h3>
        {active.sections.map((section,index)=><section key={index}>{section.title&&<h4>{section.title}</h4>}{section.paragraphs.map((body,i)=><p key={i}>{body}</p>)}</section>)}
        <div className="help-destinations">{active.links.map(link=><a className="help-destination" key={link.href} href={link.href} onClick={()=>{if(link.category){setCategory(link.category);setView('installed');}close();}}><span>{link.label}</span><Icon name="chevron-right" size={16}/></a>)}</div>
      </article> : <div className="help-empty"><Icon name="search" size={28}/><p>{labels.empty}</p><button className="text-button" type="button" onClick={()=>setQuery('')}>{labels.browse}</button></div>}
    </div>
  </section>;
}
