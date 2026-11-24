import {useCallback,useEffect,useRef,useState} from 'react';
import {useI18n} from '../../i18n';
import {workspaceRecovery} from '../../i18n/workspace-recovery';
import {useWorkspace} from '../../lib/workspace-context';
import type {DesktopBridge,WorkspaceRecoveryState,WorkspaceRecoveryOperation,WorkspaceRecoveryEncryption} from '../../platform';

const phases=['idle','selecting','confirming','stopping','running','succeeded','cancelled','failed','uncertain','resuming'];

export function WorkspaceRecoverySettings(){
 const {scope,workspace,admin}=useWorkspace();
 // An actor/workspace change must not retain another scope's recovery receipts.
 return <RecoverySettings key={JSON.stringify([scope,workspace,admin])} admin={admin} bridge={window.electron}/>;
}

function RecoverySettings({admin,bridge}:{admin:boolean;bridge:DesktopBridge|undefined}){
 const {locale,messages}=useI18n(),copy=workspaceRecovery(locale);
 const supported=bridge?.workspaceRecoveryProtocol===1&&!!bridge.getWorkspaceRecovery&&!!bridge.startWorkspaceRecovery&&!!bridge.cancelWorkspaceRecovery&&!!bridge.resumeWorkspaceAfterRecovery&&!!bridge.onWorkspaceRecovery;
 const [state,setState]=useState<WorkspaceRecoveryState>(),[operation,setOperation]=useState<WorkspaceRecoveryOperation>('backup'),[encryption,setEncryption]=useState<WorkspaceRecoveryEncryption>('passphrase');
 const [pending,setPending]=useState(false),[cancelling,setCancelling]=useState(false),[readPending,setReadPending]=useState(false),[readError,setReadError]=useState(false),[operationError,setOperationError]=useState(false),[external,setExternal]=useState(false);
 const busy=useRef(false),reading=useRef(false),status=useRef<HTMLParagraphElement>(null),mounted=useRef(false),current=useRef<WorkspaceRecoveryState|undefined>(undefined),readGeneration=useRef(0);
 const accept=useCallback((next:WorkspaceRecoveryState)=>{
  if(!next||next.version!==1||!Number.isSafeInteger(next.revision)||next.revision<0||!phases.includes(next.phase)||['available','active','serviceStopped','canResume'].some(key=>typeof next[key as keyof WorkspaceRecoveryState]!=='boolean'))return false;
  if(mounted.current&&(!current.current||current.current.revision<=next.revision)){
   current.current=next;setState(next);setReadError(false);
  }
  return true;
 },[]);
 const readStatus=useCallback(async()=>{
  if(!supported||!admin||reading.current)return;
  reading.current=true;const generation=++readGeneration.current,before=current.current;
  setReadPending(true);
  try{
   const next=await bridge!.getWorkspaceRecovery!();
   if(mounted.current&&generation===readGeneration.current){if(!accept(next))throw Error('recovery_status_invalid');setOperationError(false);}
  }catch{
   // A fresh event received during this read is better evidence than its failure.
   if(mounted.current&&generation===readGeneration.current&&current.current===before)setReadError(true);
  }finally{
   if(mounted.current&&generation===readGeneration.current){reading.current=false;setReadPending(false);}
  }
 },[supported,admin,bridge,accept]);
 useEffect(()=>{
  mounted.current=true;
  if(!supported||!admin)return()=>{mounted.current=false;};
  const unsubscribe=bridge!.onWorkspaceRecovery!(next=>{accept(next);});
  void readStatus();
  void bridge!.getBackendInfo?.().then(next=>{if(mounted.current)setExternal(next.managedByDesktop===false);}).catch(()=>{});
  return()=>{mounted.current=false;readGeneration.current++;reading.current=false;unsubscribe();};
 },[supported,admin,bridge,accept,readStatus]);
 useEffect(()=>{if(state&&!state.active)setCancelling(false);},[state]);
 const invoke=async(kind:'start'|'cancel'|'resume')=>{
  if(busy.current||!supported||!admin)return;
  const known=current.current;
  if(kind==='start'&&(!known?.available||known.active||known.serviceStopped||readError||operationError||readPending))return;
  if(kind!=='start'&&!known?.id)return;
  if(kind==='resume'&&(!known?.canResume||known.active||readError||operationError||readPending))return;
  busy.current=true;setPending(true);setOperationError(false);
  if(kind==='cancel')setCancelling(true);
  try{
   const next=await (kind==='start'?bridge!.startWorkspaceRecovery!({operation,encryption}):kind==='cancel'?bridge!.cancelWorkspaceRecovery!({id:known!.id!}):bridge!.resumeWorkspaceAfterRecovery!({id:known!.id!}));
   if(!accept(next))throw Error('recovery_status_invalid');
   if(mounted.current)status.current?.focus();
  }catch{if(mounted.current){setOperationError(true);setReadError(true);setCancelling(false);}}
  finally{busy.current=false;if(mounted.current)setPending(false);}
 };
 const active=pending||state?.active||state?.phase==='resuming';
 const eligible=!!(supported&&admin&&state?.available&&!state.active&&!state.serviceStopped&&state.phase!=='uncertain'&&!readError&&!operationError&&!readPending&&!pending);
 const lifecycle=!!(supported&&admin&&state&&(state.phase!=='idle'||state.active||state.serviceStopped));
 const changeOperation=(next:WorkspaceRecoveryOperation)=>{setOperation(next);if(next==='backup'&&encryption==='legacy')setEncryption('passphrase');};
 const labels={idle:copy.idle,selecting:copy.selecting,confirming:copy.confirming,stopping:copy.stopping,running:copy.running,succeeded:copy.succeeded,cancelled:copy.cancelled,failed:copy.failed,uncertain:copy.uncertain,resuming:copy.resuming};
 const unavailable=!bridge?copy.browserUnavailable:!supported?copy.unsupported:!admin?copy.permissionRequired:state&&!state.available&&!lifecycle?(external?copy.externalUnavailable:copy.unavailableStatus):undefined;
 const summary=unavailable??(operationError?copy.requestUnconfirmed:readError?copy.statusFailed:cancelling?copy.cancelling:readPending&&!lifecycle||!state||pending&&!lifecycle?copy.checking:state.serviceStopped&&state.phase==='idle'?copy.stopped:labels[state.phase]);
 const statusFailed=!unavailable&&(readError||operationError||state?.phase==='failed');
 return <div className="settings-card workspace-recovery" aria-labelledby="workspace-recovery-title">
  <header className="recovery-heading"><h3 id="workspace-recovery-title">{copy.title}</h3><p>{copy.purpose}</p></header>
  <div className="recovery-status">
   <p ref={status} role={statusFailed?'alert':'status'} aria-live={statusFailed?'assertive':'polite'} aria-atomic="true" tabIndex={-1}>{summary}</p>
   {bridge&&!supported&&<p>{copy.unsupportedDetail}</p>}
   {unavailable&&<a href="#/settings/service">{messages.common.serviceDetails}</a>}
   {supported&&admin&&(readError||operationError)&&<button className="secondary-button" disabled={readPending} onClick={()=>void readStatus()}>{readPending?copy.checking:messages.common.retry}</button>}
  </div>
  {eligible&&<>
   <div className="recovery-choices">
    <label htmlFor="recovery-operation">{copy.operation}<select id="recovery-operation" value={operation} onChange={event=>changeOperation(event.target.value as WorkspaceRecoveryOperation)}>
     <option value="backup">{copy.backup}</option><option value="verify">{copy.verify}</option><option value="restore">{copy.restore}</option>
    </select></label>
    <details className="recovery-protection"><summary>{copy.encryption}: {encryption==='passphrase'?copy.passphraseMode:encryption==='recipient'?copy.recipient:copy.legacy}</summary>
     <label htmlFor="recovery-encryption">{copy.encryption}<select id="recovery-encryption" value={encryption} onChange={event=>setEncryption(event.target.value as WorkspaceRecoveryEncryption)}>
      <option value="passphrase">{copy.passphraseMode}</option><option value="recipient">{copy.recipient}</option>{operation!=='backup'&&<option value="legacy">{copy.legacy}</option>}
     </select></label>
    </details>
   </div>
   <p id="recovery-boundary" className="recovery-boundary">{copy.boundary}</p>
   <div className="settings-actions"><button className="primary-button" aria-describedby="recovery-boundary" onClick={()=>void invoke('start')}>{copy[operation]}</button></div>
  </>}
  {lifecycle&&<div className="recovery-lifecycle">
   {state?.operation&&<p>{copy.operation}: {copy[state.operation]}</p>}
   <div className="settings-actions">
    {state?.active&&state.id&&<button className="secondary-button" disabled={pending||cancelling} onClick={()=>void invoke('cancel')}>{cancelling?copy.cancelling:copy.cancel}</button>}
    {state?.canResume&&state.id&&<button className="secondary-button" disabled={!!active||readError||operationError||readPending} onClick={()=>void invoke('resume')}>{copy.resume}</button>}
   </div>
   {state?.serviceStopped&&state.canResume&&<p>{copy.stopped}</p>}
  </div>}
  {admin&&state?.result&&<div className="recovery-result"><p>{state.result.authenticated?copy.authenticated:copy.legacy}</p>
   <dl><div><dt>{copy.files}</dt><dd>{new Intl.NumberFormat(locale).format(state.result.files)}</dd></div><div><dt>{copy.bytes}</dt><dd>{new Intl.NumberFormat(locale).format(state.result.bytes)}</dd></div><div><dt>{messages.settings.version}</dt><dd><bdi>{state.result.application_version}</bdi></dd></div></dl>
   {state.result.published&&<p>{copy.published}</p>}{state.result.database_checked&&<p>{copy.databaseChecked}</p>}
  </div>}
  {admin&&!!state?.retainedPaths?.length&&<div className="recovery-retained"><h4>{copy.retained}</h4><p>{copy.retainedDetail}</p><ul>{state.retainedPaths.map((item,index)=><li key={index}><code dir="auto">{item.path}</code>{item.contains_plaintext&&<p>{copy.plaintext}</p>}</li>)}</ul></div>}
  <details className="recovery-scope"><summary>{copy.scopeTitle}</summary><p>{copy.scope}</p></details>
 </div>;
}
