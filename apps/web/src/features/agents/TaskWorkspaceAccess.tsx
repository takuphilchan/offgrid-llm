import { useEffect, useRef, useState } from 'react';
import { api, type AgentRun, type ExecutionWorkspace as Workspace } from '../../api/client';
import { useI18n } from '../../i18n';
import { executionWorkspaceText, executionNativeText, attachWorkspaceText } from '../../i18n/execution-workspace';
import { ExecutionWorkspace } from '../chat/ExecutionWorkspace';

// A deliberate attachment to the waiting canonical job. Rendering, refreshing
// and discovering hosts never grant access or resume execution.
export function TaskWorkspaceAccess({run,onResolved,cancelSignal}: {run:AgentRun;onResolved:()=>void;cancelSignal?:AbortSignal}) {
  const {locale,messages}=useI18n(), copy=executionWorkspaceText[locale];
  const [selected,setSelected]=useState<Workspace|null>(null),[ready,setReady]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const active=useRef(true),locked=useRef(false);
  useEffect(()=>{active.current=true;return()=>{active.current=false;};},[]);
  const input=run.pending_input!;
  const valid=['read','edit','trusted'].includes(input.mode);
  const requested={read:true,write:input.mode!=='read',processes:input.mode==='trusted'};
  const sufficient=selected?.permissions.read && (!requested.write||selected.permissions.write) && (!requested.processes||selected.permissions.processes);
  const attach=async()=>{
    if(locked.current||!valid||!selected||!ready||!sufficient||cancelSignal?.aborted)return;
    locked.current=true;setBusy(true);setError('');
    try{
      const result=await api.resolveWorkspaceInput(run.run_id,input.id,selected.reference);
      if(result.run_id!==run.run_id)throw Error(copy.unavailable);
      if(active.current&&!cancelSignal?.aborted)onResolved();
    }catch(reason){if(active.current)setError(reason instanceof Error?reason.message:copy.unavailable);}
    finally{locked.current=false;if(active.current)setBusy(false);}
  };
  return <section className="task-access" aria-label={executionNativeText[locale].access}>
    <h3>{executionNativeText[locale].access}</h3>
    <p>{[copy.read,requested.write&&copy.edit,requested.processes&&copy.commands].filter(Boolean).join(' · ')}</p>
    <ExecutionWorkspace value={selected} onChange={setSelected} onReady={setReady} disabled={busy||!valid} initialAccess={valid?input.mode as 'read'|'edit'|'trusted':'read'}/>
    <button type="button" className="primary-button" disabled={busy||!valid||!ready||!sufficient||cancelSignal?.aborted} onClick={()=>void attach()}>{attachWorkspaceText[locale]}</button>
    {busy&&<p role="status">{messages.common.loading}</p>}
    {error&&<p role="alert">{error}</p>}
  </section>;
}
