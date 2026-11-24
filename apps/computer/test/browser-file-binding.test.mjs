import test from 'node:test';
import assert from 'node:assert/strict';
import {browserFileTask,browserFileBinding,browserFileAction} from '../browser-file-binding.mjs';

const task=()=>({actorId:'local-admin',workspaceId:'workspace',runId:'run-'+'a'.repeat(32),inputId:'input-initial'});
const session='b'.repeat(32);
const wire=()=>({version:1,actor_id:'local-admin',workspace_id:'workspace',run_id:'run-'+'a'.repeat(32),input_id:'input-initial',session_id:session});

test('private transfer authority is a frozen exact task and session tuple',()=>{
 const source=task(),expected=browserFileTask(source,'workspace');source.actorId='other';
 const received=wire(),binding=browserFileBinding(received,expected,session);received.actor_id='other';
 assert.deepEqual(binding,{...task(),sessionId:session});assert.ok(Object.isFrozen(binding));assert.ok(Object.isFrozen(expected));
 const action=browserFileAction({id:task().runId+':call-1',file_binding:wire()},binding);
 assert.equal(action.actionId,task().runId+':call-1');assert.equal(action.documentId,'');assert.ok(Object.isFrozen(action.binding));
});

test('pairing never infers actor or task authority from a partial or foreign tuple',()=>{
 for(const key of Object.keys(task())){
  const missing=task();delete missing[key];assert.throws(()=>browserFileTask(missing,'workspace'),/file_scope_invalid/);
 }
 assert.throws(()=>browserFileTask({...task(),sessionId:session},'workspace'),/file_scope_invalid/);
 assert.throws(()=>browserFileTask(task(),'other'),/file_scope_invalid/);
 for(const key of Object.keys(wire())){
  const missing=wire();delete missing[key];assert.throws(()=>browserFileBinding(missing,task(),session),/file_scope_invalid/);
  const changed=wire();changed[key]=key==='version'?2:key==='run_id'?'run-'+'c'.repeat(32):key==='session_id'?'c'.repeat(32):key==='input_id'?'input-other':'other';
  assert.throws(()=>browserFileBinding(changed,task(),session),/file_scope_invalid/);
 }
 assert.throws(()=>browserFileBinding({...wire(),path:'private'},task(),session),/file_scope_invalid/);
 assert.throws(()=>browserFileBinding(wire(),task(),'c'.repeat(32)),/file_scope_invalid/);
});

test('dispatch revalidates the complete tuple and exact run prefix before journaling',()=>{
 const binding=browserFileBinding(wire(),task(),session);
 for(const id of ['call','run-'+ 'c'.repeat(32)+':call',task().runId+':',task().runId+':bad/path'])assert.throws(()=>browserFileAction({id,file_binding:wire()},binding),/file_scope_invalid/);
 for(const field of ['actor_id','workspace_id','run_id','input_id','session_id'])assert.throws(()=>browserFileAction({id:task().runId+':call',file_binding:{...wire(),[field]:'foreign'}},binding),/file_scope_invalid/);
 assert.throws(()=>browserFileAction({id:task().runId+':call'},binding),/file_scope_invalid/);
});
