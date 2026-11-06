import {useEffect, useRef, useState} from 'react';
import {api, type WorkItem, type WorkPage} from '../../api/client';
import {useI18n} from '../../i18n';
import {activityText} from '../../i18n/activity';
import {attentionWorkText} from '../../i18n/known-work';
import {workspaceAudit} from '../../i18n/workspace-audit';
import {useTaskStatus} from './useTaskStatus';
import {HistoryToolbar} from '../../components/HistoryToolbar';
import {HistoryItemActions} from '../../components/HistoryItemActions';
import {HistoryDeleteDialog, type HistoryItem} from '../../components/HistoryDeleteDialog';
import {EmptyState, ScopedNotice} from '../../components/WorkspacePresentation';
import {useWorkspaceState} from '../../lib/workspace-context';
import {useWorkspaceRefresh} from '../../lib/workspace-refresh';
import {workChangedEvent, type WorkChange} from '../../lib/work-changes';
import type {WorkBucket} from '../../lib/work-state';
import {workBucket} from '../../lib/work-state';
import {useKnownWork, useWorkActions} from '../../lib/active-work';
import {WorkLinks} from '../../components/WorkLinks';
import {knownWorkText} from '../../i18n/known-work';

type Bucket = WorkBucket | 'all';
const locationBucket = () => window.location.hash.match(/^#\/activity\/(attention|running|history|all)$/)?.[1] as Bucket | undefined;

/** Only metadata is paged. The inspector separately reads the selected job. */
export function WorkHistory() {
  const {locale,messages:text}=useI18n(), copy=activityText(locale), status=useTaskStatus();
  const work=useWorkActions(), known=useKnownWork();
  const [choice,setChoice]=useWorkspaceState<Bucket>('activity.summary.filter',locationBucket() ?? 'attention');
  const [query,setQuery]=useWorkspaceState('activity.summary.query','');
  const [page,setPage]=useState<WorkPage>();
  const [rows,setRows]=useState<WorkItem[]>([]);
  const [busy,setBusy]=useState(false), [collecting,setCollecting]=useState(false), [error,setError]=useState('');
  const [deleting,setDeleting]=useState<HistoryItem[] | null>(null);
  const read=useRef<AbortController | null>(null), review=useRef<AbortController | null>(null), epoch=useRef(0);
  const expanded=useRef(false);
  const mutationTimer=useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const filter=choice;
  const latest=useRef({filter,query}); latest.current={filter,query};
  const refresh = async (cursor?: string) => {
    clearTimeout(mutationTimer.current);
    read.current?.abort();
    const controller=new AbortController(); read.current=controller;
    const revision=++epoch.current;
    const token=work.beginSummary();
    setBusy(true);
    try {
      const next=await api.workSummary({query:latest.current.query,bucket:latest.current.filter,cursor,limit:20},controller.signal);
      if(controller.signal.aborted || revision!==epoch.current) return;
      expanded.current=!!cursor;
      work.acceptSummary(token,next.counts);
      setPage(next);setRows(previous=>cursor ? [...previous,...next.items.filter(item=>!previous.some(old=>old.id===item.id))] : next.items);setError('');
    } catch(reason) {if(!controller.signal.aborted && revision===epoch.current) setError(reason instanceof Error ? reason.message : text.common.error);}
    finally {if(read.current===controller){read.current=null;setBusy(false);}}
  };
  useEffect(()=>{
    setRows([]);setPage(undefined);
    const timer=setTimeout(()=>void refresh(),150);
    return()=>{clearTimeout(timer);epoch.current++;read.current?.abort();review.current?.abort();};
  },[filter,query]);
  useEffect(()=>{
    const changed=()=>{const next=locationBucket();if(next)setChoice(next);};
    changed();
    const mutation=(event:Event)=>{
      const change=(event as CustomEvent<WorkChange>).detail;
      epoch.current++;read.current?.abort();
      setRows(current=>current.flatMap(row=>row.id!==change.id?[row]:change.deleted?[]:[{...row,status:change.status??row.status,deletable:false}]));
      clearTimeout(mutationTimer.current);mutationTimer.current=setTimeout(()=>void refresh(),150);
    };
    window.addEventListener('hashchange',changed);window.addEventListener(workChangedEvent,mutation);
    // Poll only the current bounded first page; don't collapse an expanded list.
    const timer=setInterval(()=>{if(!read.current && !review.current && !expanded.current && !document.hidden) void refresh();},10000);
    return()=>{clearInterval(timer);clearTimeout(mutationTimer.current);epoch.current++;read.current?.abort();review.current?.abort();window.removeEventListener('hashchange',changed);window.removeEventListener(workChangedEvent,mutation);};
  },[]);
  useWorkspaceRefresh(async()=>{await refresh();});
  const collect = async () => {
    if(review.current)return;
    const controller=new AbortController();review.current=controller;setCollecting(true);setError('');
    const selection={...latest.current};
    try {
      const items=new Map<string,HistoryItem>();let cursor:string|undefined;let scanned=0;
      do {
        const next=await api.workSummary({bucket:selection.filter,query:selection.query,cursor,limit:100},controller.signal);
        if(controller.signal.aborted)return;
        scanned+=next.items.length;
        if(scanned>5000)throw Error(copy.tooLarge);
        for(const item of next.items)if(item.deletable)items.set(item.id,{id:item.id,label:item.title});
        if(next.has_more && (!next.next_cursor || cursor===next.next_cursor))throw Error(copy.changed);
        cursor=next.has_more?next.next_cursor:undefined;
      } while(cursor);
      if(items.size)setDeleting([...items.values()]);else await refresh();
    } catch(reason){if(!controller.signal.aborted)setError(reason instanceof Error?reason.message:text.common.error);}
    finally{if(review.current===controller){review.current=null;setCollecting(false);}}
  };
  const chats=known.entries.filter(entry=>entry.kind==='chat' && workBucket(entry.status,entry.stale)!=='history');
  const counts=known.counts ? {...known.counts} : undefined;
  if(counts)for(const chat of chats){counts.all++;counts[workBucket(chat.status,chat.stale)]++;}
  const visibleChats=chats.filter(chat=>(filter==='all'||workBucket(chat.status,chat.stale)===filter)&&chat.title.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  const names={attention:attentionWorkText[locale],running:text.recovery.running,history:workspaceAudit(locale).history,all:workspaceAudit(locale).allWork};
  return <section className="task-list" aria-label={text.agentRuntime.history}>
    <h2>{text.agentRuntime.history}</h2>
    <div className="work-state-filters" role="group" aria-label={text.nav.activity}>{(['attention','running','history','all'] as const).map(value=><button key={value} className="secondary-button" aria-pressed={filter===value} onClick={()=>{setChoice(value);window.history.replaceState(null,'',`#/activity/${value}`);}}>{names[value]} <span>{counts?.[value] ?? '—'}</span></button>)}</div>
    <HistoryToolbar visibleClear query={query} onQuery={setQuery} searchLabel={text.history.searchTasks} title={text.agentRuntime.history} count={counts?.all ?? 0} busy={busy || collecting} refresh={()=>void refresh()} clear={()=>void collect()} clearLabel={copy.review} clearDisabled={!page?.removable_count || !!error} protection={text.history.protectedTasks}/>
    {collecting && <p role="status">{copy.preparing}</p>}
    {error && <ScopedNotice kind="error" action={<button className="text-button" onClick={()=>void refresh()}>{text.common.retry}</button>}>{error}</ScopedNotice>}
    {!rows.length && !visibleChats.length && !error && (busy ? <p role="status">{text.common.loading}</p> : <EmptyState title={counts?.all ? text.history.noMatches : text.agentRuntime.noTasks} action={<a className="text-button" href="#/chat">{text.nav.chat}</a>}/>)}
    {!!visibleChats.length && <section className="activity-known-chats"><p className="workspace-secondary">{knownWorkText[locale].scope}</p><WorkLinks entries={visibleChats}/></section>}
    <ul>{rows.map(item=><li className="task-history-row" key={item.id}>
      <a className="task-list-item" href={`#/activity/task/${item.id}`} title={item.title}><span>{item.title}</span><small>{status(item.status,item.outcome)}</small><time dateTime={item.created_at}>{new Date(item.created_at).toLocaleDateString(locale)}</time></a>
      <HistoryItemActions title={item.title} label={text.history.deleteTask} disabled={!item.deletable || busy} protection={copy.protection(item.protection)} onDelete={()=>setDeleting([{id:item.id,label:item.title}])}/>
    </li>)}</ul>
    {page?.has_more && <button className="text-button task-show-more" disabled={busy} onClick={()=>void refresh(page.next_cursor)}>{text.history.showMore}</button>}
    {deleting && <HistoryDeleteDialog items={deleting} kind="tasks" remove={api.deleteJob} onClose={()=>setDeleting(null)} onDeleted={ids=>{setRows(current=>current.filter(item=>!ids.includes(item.id)));void refresh();}}/>}
  </section>;
}
