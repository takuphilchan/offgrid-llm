'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),{EventEmitter}=require('node:events');
const {WorkspaceMaintenance}=require('../workspace-maintenance');
function fixture(extra={}){
 const calls=[],binary={path:'/isolated/offgrid',digest:'owned'},child={__ownedStdio:true,__binaryPin:binary};
 const runtime={state:{state:'ready'},workspace:{data:'/isolated/data'},child,options:{},
  async stopForMaintenance(expected){assert.equal(expected,child);calls.push('drain');this.child=null;this.state.state='maintenance';this.maintenanceHeld=true;this.maintenanceDrained=true;},
  async resumeAfterMaintenance(){calls.push('resume');this.child=child;this.state.state='ready';this.maintenanceHeld=this.maintenanceDrained=false;return this.state;}};
 const pins={};let actor='admin',process;
 const options={runtime,environment:()=>({}),authority:async()=>({actor}),
  files:{existing:async path=>({path}),destination:async path=>({path}),outside:()=>true,check:async pin=>{if(pins[pin.path])throw Error('changed');},checkDestination:async pin=>{if(pins[pin.path])throw Error('changed');},checkBinary:async()=>{},readKeys:async path=>({pin:{path},text:'private test key'})},
  select:async()=>({source:'/isolated/archive',destination:'/isolated/target',key:'/isolated/key'}),secret:async()=> 'temporary secret',confirm:async()=>true,quiesce:async()=>{calls.push('quiesce');},
  createProcess:()=>{process=new EventEmitter();process.run=async(_binary,request)=>{calls.push('helper');process.request=structuredClone(request);return {phase:'succeeded',result:{summary:{files:1,bytes:5,published:true}}};};return process;},...extra};
 const recovery=new WorkspaceMaintenance(options);
 return {recovery,runtime,calls,pins,options,actor:value=>{actor=value;},process:()=>process};
}
const request={operation:'backup',encryption:'passphrase'};
test('reviewed operation pins authority, drains once, reports compact results and resumes explicitly',async()=>{
 const f=fixture(),state=f.recovery.start(request);assert.equal(state.phase,'selecting');assert.equal(state.active,true);
 assert.throws(()=>f.recovery.start(request),/owned_service/);await f.recovery.current.done;
 assert.deepEqual(f.calls,['quiesce','drain','helper']);assert.equal(f.recovery.snapshot().phase,'succeeded');assert.equal(f.recovery.snapshot().canResume,true);
 assert.equal(f.process().request.passphrase,'temporary secret');assert.equal(f.recovery.current.secret,undefined);
 assert.doesNotMatch(JSON.stringify(f.recovery.snapshot()),/temporary secret|isolated|private/);
 await f.recovery.resume({id:state.id});assert.equal(f.calls.at(-1),'resume');assert.equal(f.recovery.snapshot().serviceStopped,false);
});
test('renderer paths, legacy backup and stale operation tokens never authorize work',async()=>{
 const f=fixture();for(const value of [{...request,destination:'/unsafe'},{...request,encryption:'legacy'},{operation:'shell',encryption:'recipient'}])assert.throws(()=>f.recovery.start(value),/request_invalid/);
 const state=f.recovery.start(request);assert.throws(()=>f.recovery.cancel({id:'b'.repeat(64)}),/request_invalid/);f.recovery.cancel({id:state.id});await f.recovery.current.done;
 assert.equal(f.recovery.snapshot().phase,'cancelled');assert.deepEqual(f.calls,[]);
});
test('account, binary, archive, key, destination and source replacement reject before handoff',async()=>{
 for(const changed of ['actor','binary','/isolated/archive','/isolated/key','/isolated/target','/isolated/data']){
  const f=fixture();f.options.confirm=async()=>{if(changed==='actor')f.actor('other');else if(changed==='binary')f.options.files.checkBinary=async()=>{throw Error('changed');};else f.pins[changed]=true;return true;};
  // Constructor snapshots top-level callbacks; inject the test's reviewed dialog.
  f.recovery.options.confirm=f.options.confirm;
  f.recovery.start({operation:'restore',encryption:'recipient'});await f.recovery.current.done;
  assert.equal(f.recovery.snapshot().phase,'failed',changed);assert.deepEqual(f.calls,[],changed);
 }
});
test('failed quiescence or service drain cannot dispatch helper or resume',async()=>{
 for(const phase of ['quiesce','drain']){
  const f=fixture();if(phase==='quiesce')f.recovery.options.quiesce=async()=>{throw Error('uncertain');};else f.runtime.stopForMaintenance=async()=>{throw Error('uncertain');};
  f.recovery.start(request);await f.recovery.current.done;assert.equal(f.calls.includes('helper'),false);assert.equal(f.recovery.snapshot().canResume,false);
  assert.equal(f.recovery.snapshot().phase,phase==='drain'?'uncertain':'failed');
 }
});
test('revocation during a queued dialog cancels and never grants offline work',async()=>{
 let release;const f=fixture({select:()=>new Promise(resolve=>{release=resolve;})});f.recovery.start(request);
 while(!release)await new Promise(resolve=>setImmediate(resolve));f.recovery.invalidate();release({destination:'/isolated/target'});await f.recovery.current.done;
 assert.equal(f.recovery.snapshot().phase,'cancelled');assert.equal(f.recovery.snapshot().canResume,false);assert.deepEqual(f.calls,[]);
 f.runtime.child=null;assert.throws(()=>f.recovery.start(request),/owned_service/);
});
test('cancelled handoff retains stopped state without dispatch; original workspace may resume',async()=>{
 const f=fixture(),stop=f.runtime.stopForMaintenance.bind(f.runtime);f.runtime.stopForMaintenance=async child=>{await stop(child);f.recovery.cancel({id:f.recovery.state.id});};
 f.recovery.start(request);await f.recovery.current.done;assert.equal(f.calls.includes('helper'),false);assert.equal(f.recovery.snapshot().phase,'cancelled');assert.equal(f.recovery.snapshot().canResume,true);
});
test('host admission closes before quiesce and remains closed throughout asynchronous post-stop checks',async()=>{
 const {TerminalSessions}=require('../terminal-sessions');
 const f=fixture();let release,rechecking=false,spawns=0;
 const terminal=new TerminalSessions({admit:()=>!f.runtime.maintenanceAdmissionHold,choose:async()=>'/isolated',spawnProcess:()=>{spawns++;throw Error('must not start');}});
 f.recovery.options.quiesce=async()=>{assert.equal(f.runtime.maintenanceAdmissionHold,true);await terminal.stop();rechecking=true;};
 f.recovery.options.files.checkBinary=async()=>{if(rechecking)await new Promise(resolve=>{release=resolve;});};
 f.recovery.start(request);while(!release)await new Promise(resolve=>setImmediate(resolve));
 await assert.rejects(terminal.create({kind:'shell',cols:80,rows:24}),/terminal_stopping/);assert.equal(spawns,0);assert.equal(f.calls.includes('helper'),false);
 rechecking=false;release();await f.recovery.current.done;assert.equal(f.runtime.maintenanceAdmissionHold,true);assert.equal(f.calls.includes('helper'),true);
});
test('native dialog that cannot close prevents an unconfirmed quit rather than hanging forever',async()=>{
 let release;const f=fixture({stopTimeoutMs:10,select:()=>new Promise(resolve=>{release=resolve;})});f.recovery.start(request);while(!release)await new Promise(resolve=>setImmediate(resolve));
 await assert.rejects(f.recovery.stop(),/unconfirmed/);release(null);await f.recovery.current.done;assert.deepEqual(f.calls,[]);
});
test('revoked operation authority redacts prior results and retained private paths',async()=>{
 const f=fixture();f.recovery.start(request);await f.recovery.current.done;
 f.recovery.publish({retainedPaths:[{path:'/private/retained',directory:true,contains_plaintext:true}]});assert.equal(f.recovery.snapshot().retainedPaths.length,1);
 const revision=f.recovery.snapshot().revision;f.recovery.invalidate();const state=f.recovery.snapshot();
 assert.equal(state.canResume,false);assert.equal(state.result,undefined);assert.equal(state.retainedPaths,undefined);assert.ok(state.revision>revision);
});
