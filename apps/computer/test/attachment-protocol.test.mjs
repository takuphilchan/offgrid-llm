import test from 'node:test';
import assert from 'node:assert/strict';
import {attachmentMessage} from '../attachment-protocol.mjs';

const routed={protocol_version:1,connection_id:'owned-connection',request_id:'host-correlation'};
const scope={actor_id:'local-admin',workspace_id:'saved-workspace',run_id:'run-'+'a'.repeat(32),input_id:'input-pending-access'};
const select={...routed,type:'select',scope,origins:['https://one.example','https://two.example'],approval_mode:'scoped_changes'};
const bind={...select,type:'bind',selection_id:'chosen-tab-generation',session_id:'new-service-session',previous_session:'',attachment_id:'host-owned-attachment',grant_epoch:1};

test('attachment metadata preserves exact canonical IDs in a detached immutable snapshot',()=>{
  const input=structuredClone(bind),message=attachmentMessage(input,'host');
  input.scope.input_id='changed';input.origins.push('https://three.example');
  assert.equal(message.scope.input_id,'input-pending-access');assert.equal(message.origins.length,2);
  assert.ok(Object.isFrozen(message));assert.ok(Object.isFrozen(message.scope));assert.ok(Object.isFrozen(message.origins));
  assert.equal(attachmentMessage({...bind,previous_session:'old-service-session',grant_epoch:2},'host').previous_session,'old-service-session');
});

test('attachment metadata rejects direction confusion, unknown dispatch, changed shape and unsafe origins',()=>{
  assert.throws(()=>attachmentMessage(select,'extension'));
  for(const input of [
    {...select,protocol_version:2},{...select,tabId:99},{...select,type:'action',action:{kind:'browser_click'}},
    {...select,type:'evaluate',script:'anything'}, {...select,origins:['https://one.example/path']},
    {...select,origins:['https://127.0.0.1']},{...select,origins:['https://one.example','https://one.example']},
    {...select,origins:['https://ONE.example']},{...select,scope:{...scope,task_id:'alias'}},
    {...select,scope:{...scope,run_id:'saved-run'}},{...select,scope:{...scope,input_id:'pending-input'}},
    {...bind,grant_epoch:0},{...bind,previous_session:null},{...bind,approval_mode:'always'},
  ])assert.throws(()=>attachmentMessage(input,'host'));
});

test('selection and release descriptions are bounded data, never static extension-ID authority',()=>{
  const offer={...routed,type:'selection',selection_id:'selection',tab_ref:'opaque-tab',document_id:'opaque-document',origin:'https://one.example',title:'Selected synthetic tab'};
  assert.equal(attachmentMessage(offer,'extension').tab_ref,'opaque-tab');
  for(const changed of [{...offer,title:'bad\nline'},{...offer,title:'界'.repeat(171)},{...offer,tabId:3}])assert.throws(()=>attachmentMessage(changed,'extension'));
  const hello={protocol_version:1,type:'hello',extension_id:'a'.repeat(32),browser_version:'151.0.7922.34',client_nonce:'c'.repeat(64)};
  assert.equal(attachmentMessage(hello,'extension').type,'hello');
  assert.throws(()=>attachmentMessage({...hello,authorized:true},'extension'));
  assert.throws(()=>attachmentMessage(hello,'host'));
  const released={...routed,type:'released',session_id:'session',attachment_id:'attachment',grant_epoch:1,confirmed:false};
  assert.equal(attachmentMessage(released,'extension').confirmed,false);
});
