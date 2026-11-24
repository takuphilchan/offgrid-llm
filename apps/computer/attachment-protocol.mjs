import {browserOrigins} from './browser-scope.mjs';
import {validApprovalMode} from './approval-policy.mjs';
import {encodeNativeMessage,NativeMessageError} from './native-message-framing.mjs';

export const attachmentProtocolVersion=1;
const id=value=>typeof value==='string'&&/^[a-zA-Z0-9_-]{1,128}$/.test(value);
const nonce=value=>typeof value==='string'&&/^[a-f0-9]{64}$/.test(value);
const exact=(value,keys)=>value&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).sort().join(',')===[...keys].sort().join(',');
const fail=()=>{throw new NativeMessageError('attachment_message_invalid');};
const scopeKeys=['actor_id','workspace_id','run_id','input_id'];
function scope(value){
  if(!exact(value,scopeKeys)||typeof value.actor_id!=='string'||!/^[a-zA-Z0-9:_-]{1,256}$/.test(value.actor_id)||!id(value.workspace_id)||
    typeof value.run_id!=='string'||!/^run-[a-f0-9]{32}$/.test(value.run_id)||typeof value.input_id!=='string'||!/^input-[a-zA-Z0-9_-]{1,122}$/.test(value.input_id))fail();
}
function origins(value){
  if(!Array.isArray(value)||!value.length||value.length>16||value.some(item=>typeof item!=='string'))fail();
  try {if(JSON.stringify(browserOrigins(value[0],value.slice(1)))!==JSON.stringify(value))fail();}catch{fail();}
}
function freeze(value){if(value&&typeof value==='object'){for(const item of Object.values(value))freeze(item);Object.freeze(value);}return value;}

// Metadata only. A valid message grants no authority. There is deliberately no
// action/evaluate/CDP dispatch message while attachment containment is unresolved.
export function attachmentMessage(input,sender){
  if(!['host','extension'].includes(sender))fail();
  let value;
  const frame=encodeNativeMessage(input);
  try {value=JSON.parse(frame.subarray(4).toString('utf8'));}finally{frame.fill(0);}
  if(value.protocol_version!==attachmentProtocolVersion)fail();
  const common=['protocol_version','type'];
  const routed=[...common,'connection_id','request_id'];
  const match=keys=>{if(!exact(value,keys))fail();};
  switch(value.type){
    case 'hello':
      if(sender!=='extension')fail();
      match([...common,'extension_id','browser_version','client_nonce']);
      if(!/^[a-p]{32}$/.test(value.extension_id??'')||typeof value.browser_version!=='string'||!/^\d{1,4}(?:\.\d{1,6}){1,3}$/.test(value.browser_version)||!nonce(value.client_nonce))fail();
      break;
    case 'hello_ack':
      if(sender!=='host')fail();
      match([...common,'connection_id','client_nonce','host_nonce']);
      if(!id(value.connection_id)||!nonce(value.client_nonce)||!nonce(value.host_nonce))fail();
      break;
    case 'select':
      if(sender!=='host')fail();
      match([...routed,'scope','origins','approval_mode']);scope(value.scope);origins(value.origins);
      if(!validApprovalMode(value.approval_mode))fail();
      break;
    case 'selection':
      if(sender!=='extension')fail();
      match([...routed,'selection_id','tab_ref','document_id','origin','title']);
      if(!['selection_id','tab_ref','document_id'].every(key=>id(value[key]))||typeof value.title!=='string'||Buffer.byteLength(value.title)>512||/[\x00-\x1f\x7f]/.test(value.title))fail();
      origins([value.origin]);break;
    case 'bind':
      if(sender!=='host')fail();
      match([...routed,'scope','selection_id','session_id','previous_session','attachment_id','grant_epoch','origins','approval_mode']);
      scope(value.scope);origins(value.origins);
      if(!['selection_id','session_id','attachment_id'].every(key=>id(value[key]))||value.previous_session!==''&&!id(value.previous_session)||!Number.isSafeInteger(value.grant_epoch)||value.grant_epoch<1||!validApprovalMode(value.approval_mode))fail();
      break;
    case 'release':
      if(sender!=='host')fail();
      match([...routed,'session_id','attachment_id','grant_epoch','reason']);
      if(!['stop','manual','revoked'].includes(value.reason))fail();
      if(!id(value.session_id)||!id(value.attachment_id)||!Number.isSafeInteger(value.grant_epoch)||value.grant_epoch<1)fail();
      break;
    case 'released':
      if(sender!=='extension')fail();
      match([...routed,'session_id','attachment_id','grant_epoch','confirmed']);
      if(!id(value.session_id)||!id(value.attachment_id)||!Number.isSafeInteger(value.grant_epoch)||value.grant_epoch<1||typeof value.confirmed!=='boolean')fail();
      break;
    default:fail();
  }
  if(!['hello','hello_ack'].includes(value.type)&&(!id(value.connection_id)||!id(value.request_id)))fail();
  return freeze(value);
}
