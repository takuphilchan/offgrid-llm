const taskID=value=>typeof value==='string'&&/^run-[a-f0-9]{32}$/.test(value);
const inputID=value=>typeof value==='string'&&/^input-[a-zA-Z0-9_-]{1,122}$/.test(value);
const opaque=value=>typeof value==='string'&&/^[a-zA-Z0-9_-]{1,128}$/.test(value);
const fileCapabilities=Object.freeze(['browser-discovery-v1','browser-workspace-v1','browser-structured-v1','browser-file-transfers-v1']);
const supportsBrowserFileBinding=identity=>fileCapabilities.every(capability=>identity.capabilities?.includes(capability));
function savedBrowserTask(saved,{runId,inputId,previousSession='',approvalMode,origin}){
 if(!taskID(runId)||!inputID(inputId)||saved?.run_id!==runId||saved.status!=='waiting_for_input'||saved.pending_input?.kind!=='computer'||saved.pending_input.mode!=='browser'||saved.pending_input.id!==inputId||(saved.pending_input.previous_session??'')!==previousSession||
  saved.requested_computer_approval_mode&&saved.requested_computer_approval_mode!==approvalMode||saved.computer_approval_mode&&saved.computer_approval_mode!==approvalMode)throw Error('computer_scope_violation');
 if(!previousSession&&saved.pending_input.url){
  let requested;
  try{const url=new URL(saved.pending_input.url);if(url.username||url.password)throw Error();requested=url.protocol==='offgrid-demo:'&&url.href==='offgrid-demo://research'?url.href:url.protocol==='https:'?url.origin:'';}catch{throw Error('computer_scope_violation');}
  if(requested!==(origin==='demo'?'offgrid-demo://research':new URL(origin).origin))throw Error('computer_scope_violation');
 }
}
function browserFileTask({actor,workspace,runId,inputId}){
 if(!opaque(actor)||!opaque(workspace)||!taskID(runId)||!inputID(inputId))throw Error('computer_scope_violation');
 return Object.freeze({actorId:actor,workspaceId:workspace,runId,inputId});
}
module.exports={savedBrowserTask,browserFileTask,supportsBrowserFileBinding,taskID,inputID};
