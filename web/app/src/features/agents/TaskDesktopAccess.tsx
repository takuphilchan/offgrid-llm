import {useEffect,useRef,useState} from 'react';
import {api,type AgentRun} from '../../api/client';
import {useI18n} from '../../i18n';
import {taskDesktopText} from '../../i18n/task-desktop';
import {taskWorkspaceText} from '../../i18n/task-workspace';
import {nativeAppError,nativeAppFeedback} from '../../i18n/native-app';
import {stopOwnedComputer} from './TaskControls';
import {ensureDesktopSessionAccess} from './desktopSessionAccess';
import {taskSessionAccessText} from '../../i18n/task-session-access';

export async function grantTaskDesktop(run:AgentRun,workspace:string){
  if(window.electron?.desktopComputerProtocol!==3||!window.electron.startComputerDesktop||!run.pending_input)throw Error('computer_upgrade_required');
  const sessions=await api.computerSessions();
  const ready=sessions.sessions.find(s=>s.run_id===run.run_id&&s.input_id===run.pending_input!.id&&s.desktop&&s.state==='in_use');
  if(ready){await api.resolveJobInput(run.run_id,run.pending_input.id,ready.id);return;}
  if(run.requested_computer_approval_mode==='full_task')await ensureDesktopSessionAccess(workspace);
  const state=await window.electron.startComputerDesktop({workspace,runId:run.run_id,requestId:run.pending_input.id});
  if(!state.target||state.state!=='ready')throw Error(state.code??'computer_session_unavailable');
  await api.resolveJobInput(run.run_id,run.pending_input.id,state.target.id);
}

// Recovery is explicit. Mounting, navigation or a lost reply never triggers a
// consent prompt or repeats Start. The initial composer calls grantTaskDesktop
// directly from its submit handler, once it has a durable task identity.
export function TaskDesktopAccess({run,workspace,onResolved,cancelSignal,initialConsentPending=false}:{run:AgentRun;workspace:string;onResolved:()=>void;cancelSignal?:AbortSignal;initialConsentPending?:boolean}){
  const {locale}=useI18n(),copy=taskDesktopText(locale),task=taskWorkspaceText(locale);
  const [busy,setBusy]=useState(false),[error,setError]=useState('');const lock=useRef(false),mounted=useRef(true);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  const allow=async()=>{
    if(lock.current||cancelSignal?.aborted)return;lock.current=true;setBusy(true);setError('');
    try{await grantTaskDesktop(run,workspace);if(!cancelSignal?.aborted)onResolved();}
    catch(e){if(mounted.current&&!cancelSignal?.aborted)setError(nativeAppError(locale,e instanceof Error?e.message:'',e instanceof Error?e.message:''));}
    finally{lock.current=false;if(mounted.current)setBusy(false);}
  };
  const stop=async()=>{await Promise.all([stopOwnedComputer(undefined,run.pending_input?.id),api.jobAction(run.run_id,'cancel')]);onResolved();};
  return <section className="task-access" aria-label={copy.desktop}>
    <h3>{copy.desktop}</h3><p>{window.electron?.enableDesktopAccess&&run.requested_computer_approval_mode==='full_task'?taskSessionAccessText(locale).hint:copy.hint}</p>
    {window.electron?.desktopComputerProtocol===3?<div className="button-row">
      <button className="primary-button" disabled={busy||initialConsentPending||cancelSignal?.aborted} onClick={()=>void allow()}>{busy||initialConsentPending?copy.working:copy.allow}</button>
      {(busy||initialConsentPending)&&<button className="secondary-button" onClick={()=>void stop().catch(e=>setError(String(e)))}>{nativeAppFeedback(locale).stop}</button>}
    </div>:<><p>{task.handoff}</p><a className="primary-button" href={`offgrid://computer?task=${encodeURIComponent(run.run_id)}`}>{task.desktop}</a></>}
    {error&&<p role="alert">{error}</p>}
  </section>;
}
