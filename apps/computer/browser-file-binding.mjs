import {uploadBinding,uploadTaskBinding,uploadAction} from './browser-upload-ledger-values.mjs';

const reject=()=>{throw Error('computer_browser_file_scope_invalid');};
const same=(actual,expected)=>Object.keys(expected).every(key=>actual[key]===expected[key]);
export const browserFileCapabilities=Object.freeze(['browser-discovery-v1','browser-workspace-v1','browser-structured-v1','browser-file-transfers-v1']);

// The expected tuple comes only from the trusted host's authenticated task and
// principal checks. A service reply or action ID cannot manufacture that grant.
export function browserFileTask(value,workspace){
  try{const task=uploadTaskBinding(value);if(task.workspaceId!==workspace)reject();return task;}
  catch{reject();}
}
export function browserFileBinding(wire,expected,sessionId){
  try{
    if(!wire||Object.getPrototypeOf(wire)!==Object.prototype||Object.keys(wire).sort().join(',')!=='actor_id,input_id,run_id,session_id,version,workspace_id'||wire.version!==1)reject();
    const binding=uploadBinding({actorId:wire.actor_id,workspaceId:wire.workspace_id,runId:wire.run_id,inputId:wire.input_id,sessionId:wire.session_id});
    if(!same(binding,expected)||binding.sessionId!==sessionId)reject();
    return binding;
  }catch{reject();}
}
export function browserFileAction(action,binding){
  const actual=browserFileBinding(action?.file_binding,binding,binding.sessionId);
  try{return Object.freeze({...uploadAction({actionId:action.id},actual.runId),binding:actual});}
  catch{reject();}
}
