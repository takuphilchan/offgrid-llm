import {useEffect,useRef,useState} from 'react';
import {api,type AgentRun} from '../../api/client';
import {useI18n} from '../../i18n';
import {taskDesktopText} from '../../i18n/task-desktop';
import {taskWorkspaceText} from '../../i18n/task-workspace';
import {nativeAppError} from '../../i18n/native-app';
import {stopOwnedComputer} from './TaskControls';
import {ensureDesktopSessionAccess} from './desktopSessionAccess';
import {taskSessionAccessText} from '../../i18n/task-session-access';

export async function grantTaskDesktop(run:AgentRun,workspace:string,signal?:AbortSignal){
  const check=()=>{if(signal?.aborted)throw new DOMException('Computer access cancelled','AbortError');};
  check();
  if(window.electron?.desktopComputerProtocol!==3||!window.electron.startComputerDesktop||!run.pending_input)throw Error('computer_upgrade_required');
  const sessions=await api.computerSessions();
  check();
  const ready=sessions.sessions.find(s=>s.run_id===run.run_id&&s.input_id===run.pending_input!.id&&s.desktop&&s.state==='in_use');
  if(ready){await api.resolveJobInput(run.run_id,run.pending_input.id,ready.id);return;}
  if(run.requested_computer_approval_mode==='full_task')await ensureDesktopSessionAccess(workspace);
  check();
  const state=await window.electron.startComputerDesktop({workspace,runId:run.run_id,requestId:run.pending_input.id});
  if(signal?.aborted){await stopOwnedComputer(state.target?.id,run.pending_input.id);check();}
  if(!state.target||state.state!=='ready')throw Error(state.code??'computer_session_unavailable');
  await api.resolveJobInput(run.run_id,run.pending_input.id,state.target.id);
}

// Recovery is explicit. Mounting, navigation or a lost reply never triggers a
// consent prompt or repeats Start. Only a live explicit composer submission can
// continue its one-shot access intent when the model later requests host work.
export function TaskDesktopAccess({run,workspace,onResolved,cancelSignal,initialConsentPending=false}:{run:AgentRun;workspace:string;onResolved:()=>void;cancelSignal?:AbortSignal;initialConsentPending?:boolean}){
  const {locale}=useI18n(),copy=taskDesktopText(locale),task=taskWorkspaceText(locale);
  const [busy,setBusy]=useState(false),[error,setError]=useState('');const lock=useRef(false),mounted=useRef(true);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  const allow=async()=>{
    if(lock.current||cancelSignal?.aborted)return;lock.current=true;setBusy(true);setError('');
    try{await grantTaskDesktop(run,workspace,cancelSignal);if(!cancelSignal?.aborted)onResolved();}
    catch(e){if(mounted.current&&!cancelSignal?.aborted)setError(nativeAppError(locale,e instanceof Error?e.message:'',e instanceof Error?e.message:''));}
    finally{lock.current=false;if(mounted.current)setBusy(false);}
  };
  return <section className="task-access" aria-label={copy.desktop}>
    <h3>{copy.desktop}</h3><p>{window.electron?.enableDesktopAccess&&run.requested_computer_approval_mode==='full_task'?taskSessionAccessText(locale).hint:copy.hint}</p>
    {window.electron?.desktopComputerProtocol===3?<div className="button-row">
      <button className="primary-button" disabled={busy||initialConsentPending||cancelSignal?.aborted} onClick={()=>void allow()}>{busy||initialConsentPending?copy.working:copy.allow}</button>
    </div>:<><p>{task.handoff}</p><a className="primary-button" href={`offgrid://computer?task=${encodeURIComponent(run.run_id)}`}>{task.desktop}</a></>}
    {error&&<p role="alert">{error}</p>}
  </section>;
}
