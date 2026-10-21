import { useEffect, useId, useState } from 'react';
import type { ComputerSession } from '../../api/client';
import { UtilityPopover } from '../../components/UtilityPopover';
import { useI18n } from '../../i18n';
import { approvalPolicyText } from '../../i18n/native-app';
import { taskAccessText } from '../../i18n/task-access';
import { taskDesktopText } from '../../i18n/task-desktop';
import { taskSessionAccessText } from '../../i18n/task-session-access';
import {ensureDesktopSessionAccess,revokeDesktopSessionAccess} from './desktopSessionAccess';

export type TaskComputerPolicy = ComputerSession['approval_mode'];
export function taskComputerPolicy(value: string): TaskComputerPolicy {
  return value === 'ask_every_time' || value === 'scoped_changes' || value === 'full_task' ? value : 'full_task';
}

export function TaskAccessPolicy({ value, onChange, disabled, scope, onScopeChange,workspace }: { value: TaskComputerPolicy; onChange: (value: TaskComputerPolicy) => void; disabled: boolean; scope?:'desktop'|'selected_window'; onScopeChange?:(scope:'desktop'|'selected_window')=>void;workspace?:string }) {
  const { locale } = useI18n(), copy = approvalPolicyText(locale), access = taskAccessText(locale);
  const id = useId();
  const session=taskSessionAccessText(locale),supportsSession=scope==='desktop'&&!!workspace&&!!window.electron?.enableDesktopAccess;
  const [enabled,setEnabled]=useState(false),[changing,setChanging]=useState(false),[error,setError]=useState('');
  useEffect(()=>{
    if(!supportsSession||changing)return;
    let disposed=false;let timer:ReturnType<typeof setTimeout>;
    const refresh=async()=>{try{const state=await window.electron!.getDesktopAccess!();if(!disposed)setEnabled(state.enabled&&state.workspace===workspace);}catch{if(!disposed)setEnabled(false);}finally{if(!disposed)timer=setTimeout(()=>void refresh(),2000);}};
    void refresh();return()=>{disposed=true;clearTimeout(timer);};
  },[supportsSession,workspace,changing]);
  const change=async(action:()=>void, revoke=false)=>{
    if(changing)return;setChanging(true);setError('');
    try{if(revoke){await revokeDesktopSessionAccess();setEnabled(false);}action();}
    catch(e){setError(e instanceof Error?e.message:String(e));}finally{setChanging(false);}
  };
  const selectFullAccess=async()=>{
    if(changing||!workspace)return;setChanging(true);setError('');
    try{await ensureDesktopSessionAccess(workspace);setEnabled(true);onChange('full_task');}
    catch(e){setError(e instanceof Error?e.message:String(e));}finally{setChanging(false);}
  };
  const choices = [
    { value: 'ask_every_time', label: copy.ask, help: copy.askHelp },
    { value: 'scoped_changes', label: copy.scoped, help: copy.scopedHelp },
    { value: 'full_task', label: copy.full, help: copy.fullHelp },
  ] as const;
  return <UtilityPopover label={access.title} trigger={supportsSession&&value==='full_task'&&!enabled?access.title:choices.find(choice => choice.value === value)?.label} triggerClassName="secondary-button" className="task-access-policy">
    {scope&&onScopeChange&&<label className="field"><span>{taskDesktopText(locale).scope}</span><select disabled={disabled||changing} value={scope} onChange={event=>{const next=event.target.value as 'desktop'|'selected_window';void change(()=>onScopeChange(next),next!=='desktop');}}><option value="desktop">{taskDesktopText(locale).desktop}</option><option value="selected_window">{taskDesktopText(locale).window}</option></select></label>}
    <fieldset disabled={disabled||changing} aria-describedby={`${id}-consent`}>
      <legend>{access.title}</legend>
      {choices.map(choice => <label className="task-policy-choice" key={choice.value}>
        <input type="radio" name={id} value={choice.value} checked={choice.value === value&&(choice.value!=='full_task'||!supportsSession||enabled)} onChange={() => {if(choice.value==='full_task'&&supportsSession)void selectFullAccess();else void change(()=>onChange(choice.value),choice.value!=='full_task');}} />
        <span><strong>{choice.label}</strong><small>{choice.help}</small></span>
      </label>)}
    </fieldset>
    {supportsSession&&value==='full_task'&&<p className="task-session-access" role="status">{enabled?session.ready:session.disabled}</p>}
    <p id={`${id}-consent`} className="workspace-secondary">{supportsSession&&value==='full_task'?session.hint:scope==='desktop'?taskDesktopText(locale).hint:access.consent}</p>
    {error&&<p role="alert">{error}</p>}
  </UtilityPopover>;
}
