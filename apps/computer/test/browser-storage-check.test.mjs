import test from 'node:test';
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {resolve,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {EventEmitter,once} from 'node:events';
import {PassThrough} from 'node:stream';
import {BrowserStorageCheck} from '../browser-storage-check.mjs';

const root=resolve('fixture-pack'),directory=resolve('fixture-private');
function fixture(mode='success',options={}){
 const calls=[];
 const checker=new BrowserStorageCheck({root,directory,workspaceRoots:[],verify:async()=>({browser_storage:1}),timeoutMs:1000,stopMs:1000,
  spawnWorker:(executable,args,config)=>{
   calls.push({executable,args,config});
   return spawn(process.execPath,[fileURLToPath(new URL('./fixtures/browser-storage-check-child.mjs',import.meta.url)),mode],{...config,env:{...config.env,OFFGRID_STORAGE_CHECK_FIXTURE:'1'}});
  },...options});
 return {checker,calls};
}

test('verified storage-only subprocess sends bounded private roots and requires clean process exit',async()=>{
 const roots=[resolve('fixture-workspace')],f=fixture('success',{workspaceRoots:roots});
 roots.push(resolve('later-mutation'));
 assert.equal(await f.checker.prepare(),true);assert.equal(f.checker.child,null);
 const call=f.calls[0];assert.equal(call.executable,join(root,'native',process.platform==='win32'?'offgrid-computer.exe':'offgrid-computer'));
 assert.deepEqual(call.args,['--browser-storage','--state',directory]);assert.equal(call.config.shell,false);assert.equal(call.config.windowsHide,true);
 for(const key of ['NODE_OPTIONS','NODE_PATH','ELECTRON_RUN_AS_NODE'])assert.equal(Object.hasOwn(call.config.env,key),false);
 assert.deepEqual(f.checker.workspaceRoots,[resolve('fixture-workspace')]);
 assert.equal(await f.checker.prepare(),true);assert.equal(f.calls.length,2,'resume/recheck launches a fresh verified check');await f.checker.stop();
});

test('old or absent native storage contract does not launch a helper or qualify transfers',async()=>{
 for(const verify of [async()=>({contracts:{native_documents:1}}),async()=>{throw Error('computer_driver_unavailable');}]){
  const f=fixture('success',{verify});assert.equal(await f.checker.prepare(),false);assert.equal(f.calls.length,0);await f.checker.stop();
 }
 const f=fixture('success',{verify:async()=>{throw Error('pack_invalid');}});await assert.rejects(f.checker.prepare(),/pack_invalid/);assert.equal(f.calls.length,0);
});

test('unknown, relative, oversized or excessive workspace roots reject before launching',async()=>{
 for(const workspaceRoots of [undefined,null]){
  const f=fixture('success',{workspaceRoots});assert.equal(await f.checker.prepare(),false);assert.equal(f.calls.length,0);
 }
 for(const workspaceRoots of [['relative'],new Array(17).fill(directory),[directory+'x'.repeat(4096)]]){
  const f=fixture('success',{workspaceRoots});await assert.rejects(f.checker.prepare(),/computer_storage_unavailable/);assert.equal(f.calls.length,0);
 }
});

test('ready reply alone cannot activate storage while helper remains alive',async()=>{
 const f=fixture('hold');let settled=false;
 const pending=f.checker.prepare().finally(()=>{settled=true;});const rejected=assert.rejects(pending,/computer_stopped/);
 while(!f.checker.child)await new Promise(resolve=>setImmediate(resolve));
 await once(f.checker.child.stdout,'data');assert.equal(settled,false);
 await f.checker.stop();await rejected;assert.equal(f.checker.child,null);
});

test('malformed, duplicate, excessive, wrong-version and failed-exit replies never qualify storage',async()=>{
 for(const mode of ['duplicate','extra','version','utf8','oversize','double','exit-error','hang']){
  const f=fixture(mode,{timeoutMs:mode==='hang'?100:1000});
  await assert.rejects(f.checker.prepare(),error=>error.message==='computer_storage_unavailable');
  assert.equal(f.checker.child,null);await f.checker.stop();
 }
});

test('Stop fences verification before process creation and verification timeout is bounded',async()=>{
 let release;
 const f=fixture('success',{verify:()=>new Promise(resolve=>{release=resolve;})});
 const pending=assert.rejects(f.checker.prepare(),/computer_stopped/);await new Promise(resolve=>setImmediate(resolve));
 await f.checker.stop();await pending;release({browser_storage:1});await new Promise(resolve=>setImmediate(resolve));assert.equal(f.calls.length,0);
 const timed=fixture('success',{verify:()=>new Promise(()=>{}),timeoutMs:20});await assert.rejects(timed.checker.prepare(),/computer_storage_unavailable/);assert.equal(timed.calls.length,0);
});

test('unconfirmed helper termination retains the exact process for explicit Stop retry',async()=>{
 const child=Object.assign(new EventEmitter(),{stdin:new PassThrough(),stdout:new PassThrough(),stderr:new PassThrough(),kill:()=>false});
 const f=fixture('success',{spawnWorker:()=>child,timeoutMs:20,stopMs:20});
 await assert.rejects(f.checker.prepare(),/computer_stop_unconfirmed/);assert.equal(f.checker.child,child);
 await assert.rejects(f.checker.stop(),/computer_stop_unconfirmed/);assert.equal(f.checker.child,child);
 child.emit('close',null);await f.checker.stop();assert.equal(f.checker.child,null);
});
