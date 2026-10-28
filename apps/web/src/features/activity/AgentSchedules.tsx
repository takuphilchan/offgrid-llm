import { useEffect, useId, useRef, useState } from 'react';
import { api, type AgentProfile, type AgentSchedule, type AgentScheduleOccurrence, type ExecutionWorkspace as Workspace } from '../../api/client';
import { useI18n } from '../../i18n';
import { agentSchedulesText } from '../../i18n/agent-schedules';
import { conversationWorkText } from '../../i18n/conversation-work';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { useWorkspace } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { ActionGroup, ScopedNotice, SectionHeading } from '../../components/WorkspacePresentation';
import { ExecutionWorkspace } from '../chat/ExecutionWorkspace';
import { useTaskStatus } from '../agents/useTaskStatus';
import './agent-schedules.css';

export function AgentSchedules() {
  const {scope,workspace}=useWorkspace();
  return <SchedulePanel key={JSON.stringify([scope,workspace])}/>;
}

function SchedulePanel(){
  const {locale,messages}=useI18n(),copy=agentSchedulesText[locale],status=useTaskStatus();
  const [items,setItems]=useState<AgentSchedule[]>([]),[supported,setSupported]=useState(false),[loading,setLoading]=useState(true),[error,setError]=useState('');
  const [draft,setDraft]=useState<AgentSchedule|null|undefined>(undefined),[history,setHistory]=useState<{id:string;items:AgentScheduleOccurrence[]}|null>(null),[busy,setBusy]=useState(false);
  const alive=useRef(true),generation=useRef(0),historyGeneration=useRef(0);
  const editorOrigin=useRef<HTMLElement|null>(null);
  const edit=(item:AgentSchedule|null)=>{editorOrigin.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDraft(item);};
  const closeEditor=()=>{setDraft(undefined);requestAnimationFrame(()=>{if(editorOrigin.current?.isConnected)editorOrigin.current.focus();});};
  const refresh=async()=>{
    const version=++generation.current;setLoading(true);setError('');
    try{const identity=await api.systemIdentity();if(!alive.current||version!==generation.current)return;
      const available=identity.capabilities?.includes('agent-schedules-v1')===true;setSupported(available);
      if(available){const data=await api.agentSchedules();if(alive.current&&version===generation.current)setItems(data.schedules);}
    }catch(reason){if(alive.current&&version===generation.current)setError(reason instanceof Error?reason.message:messages.common.error);}
    finally{if(alive.current&&version===generation.current)setLoading(false);}
  };
  useEffect(()=>{alive.current=true;void refresh();return()=>{alive.current=false;generation.current++;historyGeneration.current++;};},[]);
  useWorkspaceRefresh(refresh);
  const inspect=async(id:string)=>{
    const version=++historyGeneration.current;setHistory(null);setError('');
    try{const result=await api.agentScheduleOccurrences(id);if(alive.current&&version===historyGeneration.current)setHistory({id,items:result.occurrences});}
    catch(reason){if(alive.current&&version===historyGeneration.current)setError(reason instanceof Error?reason.message:messages.common.error);}
  };
  const disable=async(item:AgentSchedule)=>{
    if(busy)return;setBusy(true);setError('');
    try{await api.disableAgentSchedule(item.id,item.revision);if(alive.current)await refresh();}
    catch(reason){if(alive.current)setError(reason instanceof Error?reason.message:messages.common.error);}
    finally{if(alive.current)setBusy(false);}
  };
  const timestamp=(value:string,zone?:string)=>{try{return new Intl.DateTimeFormat(locale,{dateStyle:'medium',timeStyle:'short',timeZone:zone}).format(new Date(value));}catch{return value;}};
  return <section className="agent-schedules" aria-label={copy.title}>
    <SectionHeading title={copy.title} description={copy.hint} actions={<><button type="button" className="secondary-button" disabled={loading||busy} onClick={()=>void refresh()}>{copy.refresh}</button>{supported&&<button type="button" className="secondary-button" disabled={busy||draft!==undefined} onClick={()=>edit(null)}>{copy.create}</button>}</>}/>
    {error&&<ScopedNotice kind="error">{error}</ScopedNotice>}
    {loading&&<p role="status">{messages.common.loading}</p>}
    {!loading&&!supported&&<p>{copy.unsupported}</p>}
    {supported&&draft!==undefined&&<ScheduleEditor key={draft?.id??'new'} original={draft} onCancel={closeEditor} onSaved={()=>{closeEditor();setHistory(null);historyGeneration.current++;void refresh();}}/>}
    {supported&&!loading&&items.length===0&&<p>{copy.empty}</p>}
    {items.length>0&&<ul className="schedule-list">{items.map(item=><li key={item.id}>
      <div><h3 dir="auto">{item.settings.name}</h3><p className="workspace-secondary">{item.profile.name} · {item.profile.revision} · <bdi>{item.model}</bdi></p><p>{item.settings.enabled?copy.enabled:status('paused')}</p>{item.settings.enabled&&<p>{copy.next}: <time dateTime={item.next_at}>{timestamp(item.next_at,item.settings.calendar.zone)}</time> · <bdi>{item.settings.calendar.zone}</bdi></p>}</div>
      <ActionGroup><button type="button" className="secondary-button" disabled={busy||draft!==undefined} onClick={()=>edit(item)}>{copy.edit}</button><button type="button" className="secondary-button" disabled={busy} onClick={()=>void inspect(item.id)}>{copy.history}</button>{item.settings.enabled&&<button type="button" className="secondary-button" disabled={busy||draft!==undefined} onClick={()=>void disable(item)}>{copy.disable}</button>}</ActionGroup>
      {history?.id===item.id&&<div className="schedule-history" role="region" aria-label={`${copy.history}: ${item.settings.name}`}><h4>{copy.history}</h4>{history.items.length===0&&<p>{copy.empty}</p>}<ul>{history.items.map(occurrence=><li key={occurrence.id}>
        <time dateTime={occurrence.due_at}>{timestamp(occurrence.due_at,item.settings.calendar.zone)}</time><span>{occurrence.state==='missed'?copy.missed:occurrence.state==='blocked'?copy.blocked:occurrence.state==='claimed'?copy.pending:occurrence.state==='admitted'?status('running'):status(occurrence.state)}</span>
        {occurrence.job_id&&<a className="secondary-button" href={`#/activity/task/${encodeURIComponent(occurrence.job_id)}`}>{copy.inspect}</a>}
        {occurrence.reason&&<details><summary>{conversationWorkText[locale].details}</summary><code dir="ltr">{occurrence.reason}</code></details>}
      </li>)}</ul></div>}
    </li>)}</ul>}
  </section>;
}

function ScheduleEditor({original,onCancel,onSaved}:{original:AgentSchedule|null;onCancel:()=>void;onSaved:()=>void}){
  const {locale,messages}=useI18n(),copy=agentSchedulesText[locale],id=useId();
  const [profiles,setProfiles]=useState<AgentProfile[]>([]),[selected,setSelected]=useState(original?`${original.profile.id}:${original.profile.revision}`:'');
  const [name,setName]=useState(original?.settings.name??''),[prompt,setPrompt]=useState(original?.settings.prompt??'');
  const [zone,setZone]=useState(original?.settings.calendar.zone??'UTC'),[time,setTime]=useState(original?`${String(original.settings.calendar.hour).padStart(2,'0')}:${String(original.settings.calendar.minute).padStart(2,'0')}`:'09:00');
  const [days,setDays]=useState(original?.settings.calendar.weekdays??[0,1,2,3,4,5,6]),[enabled,setEnabled]=useState(original?.settings.enabled??false),[budget,setBudget]=useState(original?.max_iterations??10);
  const [host,setHost]=useState<Workspace|null>(original?.execution_workspace??null),[hostReady,setHostReady]=useState(!original?.execution),[reviewed,setReviewed]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const alive=useRef(true);
  const firstInput=useRef<HTMLInputElement>(null);
  useEffect(()=>{firstInput.current?.focus();},[]);
  useEffect(()=>{alive.current=true;const abort=new AbortController();void(async()=>{try{
    const data=await api.agentProfiles(abort.signal);let values=data.filter(p=>!p.archived);
    if(original&&!values.some(p=>p.id===original.profile.id&&p.revision===original.profile.revision)){const pinned=await api.agentProfile(original.profile.id,original.profile.revision,abort.signal);values=[pinned,...values];}
    if(!abort.signal.aborted)setProfiles(values);
  }catch(reason){if(!abort.signal.aborted)setError(reason instanceof Error?reason.message:messages.common.error);}})();return()=>{alive.current=false;abort.abort();};},[]);
  const profile=profiles.find(p=>`${p.id}:${p.revision}`===selected);
  const save=async()=>{
    if(busy||!reviewed||!profile||profile.archived||!hostReady)return;setBusy(true);setError('');
    const [hour,minute]=time.split(':').map(Number);
    try{await api.saveAgentSchedule({expected_revision:original?.revision??0,settings:{name,prompt,calendar:{zone,hour,minute,weekdays:days},enabled},profile_id:profile.id,profile_revision:profile.revision,max_iterations:budget,...(host?{execution:host.reference}:{})},original?.id);if(alive.current)onSaved();}
    catch(reason){if(alive.current)setError(reason instanceof Error?reason.message:messages.common.error);}
    finally{if(alive.current)setBusy(false);}
  };
  const field=(key:string)=>`${id}-${key}`;
  return <form className="schedule-editor" aria-label={copy.edit} onSubmit={event=>{event.preventDefault();void save();}}>
    <fieldset disabled={busy} className="schedule-fields" onChange={()=>setReviewed(false)}><legend>{original?copy.edit:copy.create}</legend>
      <label htmlFor={field('name')}>{copy.name}</label><input ref={firstInput} id={field('name')} required maxLength={128} value={name} onChange={e=>setName(e.target.value)}/>
      <label htmlFor={field('prompt')}>{copy.prompt}</label><textarea id={field('prompt')} required maxLength={32000} rows={4} value={prompt} onChange={e=>setPrompt(e.target.value)}/>
      <label htmlFor={field('profile')}>{copy.profile}</label><select id={field('profile')} required value={selected} onChange={e=>{setSelected(e.target.value);const p=profiles.find(p=>`${p.id}:${p.revision}`===e.target.value);if(p)setBudget(p.settings.max_iterations);}}><option value="">—</option>{profiles.map(p=><option key={`${p.id}:${p.revision}`} value={`${p.id}:${p.revision}`}>{p.settings.name} · {p.revision} · {p.settings.model}</option>)}</select>
      <div className="schedule-calendar"><label>{copy.zone}<input required value={zone} onChange={e=>setZone(e.target.value)} autoComplete="off" dir="ltr"/></label><label>{copy.time}<input required type="time" value={time} onChange={e=>setTime(e.target.value)}/></label></div>
      <fieldset className="schedule-days"><legend>{copy.days}</legend><ActionGroup><button type="button" className="secondary-button" onClick={()=>{setDays([0,1,2,3,4,5,6]);setReviewed(false);}}>{copy.daily}</button><button type="button" className="secondary-button" onClick={()=>{setDays([1,2,3,4,5]);setReviewed(false);}}>{copy.weekdays}</button></ActionGroup><div>{[0,1,2,3,4,5,6].map(day=><label key={day}><input type="checkbox" checked={days.includes(day)} onChange={e=>setDays(e.target.checked?[...days,day].sort():days.filter(d=>d!==day))}/>{new Intl.DateTimeFormat(locale,{weekday:'short',timeZone:'UTC'}).format(new Date(Date.UTC(2026,9,25+day)))}</label>)}</div></fieldset>
      <label htmlFor={field('budget')}>{copy.budget}</label><input id={field('budget')} type="number" min={1} max={profile?.settings.max_iterations??50} required value={budget} onChange={e=>setBudget(Number(e.target.value))}/>
      <ExecutionWorkspace value={host} disabled={busy} onReady={setHostReady} ceiling={profile?.settings.execution} onChange={next=>{setHost(next);setReviewed(false);}}/>
      <label className="schedule-check"><input type="checkbox" checked={enabled} onChange={e=>setEnabled(e.target.checked)}/>{copy.enabled}</label>
    </fieldset>
    <div className="schedule-review"><h4>{copy.review}</h4><p dir="auto">{profile?`${profile.settings.name} · ${profile.revision} · ${profile.settings.model}`:'—'}</p><p dir="auto">{host?`${host.reference.host} · ${host.root} · ${host.backend}`:copy.noHost}</p><p>{copy.budget}: {budget} · <bdi>{zone} {time}</bdi></p>
      {profile&&<details><summary>{agentProfilesText[locale].resources}</summary><pre dir="auto">{JSON.stringify(profile.settings,null,2)}</pre></details>}
      {host&&<details><summary>{conversationWorkText[locale].details}</summary><pre dir="auto">{JSON.stringify({reference:host.reference,permissions:host.permissions,ceiling:profile?.settings.execution},null,2)}</pre></details>}
      <p>{copy.policy}</p><label className="schedule-check"><input type="checkbox" checked={reviewed} disabled={busy||!profile||profile.archived||!hostReady} onChange={e=>setReviewed(e.target.checked)}/>{copy.review}</label></div>
    {error&&<ScopedNotice kind="error">{error}</ScopedNotice>}
    <ActionGroup><button type="submit" className="primary-button" disabled={busy||!reviewed||!profile||profile.archived||!hostReady||days.length===0}>{copy.save}</button><button type="button" className="secondary-button" disabled={busy} onClick={onCancel}>{copy.cancel}</button></ActionGroup>
  </form>;
}
