'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),{EventEmitter}=require('node:events');
const {DesktopRuntime}=require('../runtime');
function fixture(){
 const children=[],calls=[];let occupied=false;
 const runtime=new DesktopRuntime({url:'http://127.0.0.1:11611',version:'test',uiBuildID:'a'.repeat(64),binary:'/test/offgrid',uiDir:'/test/ui',workspace:{models:'/test/models',data:'/test/data'},
  ownedStdio:true,pinBinary:async()=>({digest:'pinned'}),maintenanceGraceMs:10,retryMs:1,timeoutMs:50,
  fs:{access:async()=>{},mkdir:async()=>{}},inspect:async()=>({state:runtime.child||occupied?'ready':'offline',workspaceID:'original',uiBuildID:'a'.repeat(64),bridgeProtocol:1}),
  spawn:(binary,args,options)=>{calls.push({binary,args,options});const child=new EventEmitter();child.stdout=child.stderr={resume(){}};child.stdin=new EventEmitter();child.stdin.end=()=>{child.ended=true;queueMicrotask(()=>child.emit('exit',0,null));};child.kill=()=>{throw Error('unexpected signal');};children.push(child);return child;}});
 return {runtime,children,calls,occupy:()=>{occupied=true;}};
}
test('maintenance drains the existing private owner by EOF and resumes only explicitly',async()=>{
 const f=fixture();await f.runtime.connect();const child=f.runtime.child;
 assert.deepEqual(f.calls[0].args,['serve','--desktop-owned-stdio']);assert.equal(f.calls[0].options.stdio[0],'pipe');assert.equal(child.__binaryPin.digest,'pinned');
 await f.runtime.stopForMaintenance(child);assert.equal(child.ended,true);assert.equal(f.runtime.child,null);assert.equal(f.calls.length,1);
 assert.equal((await f.runtime.connect()).state,'maintenance');assert.equal(f.calls.length,1);
 assert.equal((await f.runtime.resumeAfterMaintenance()).state,'ready');assert.equal(f.calls.length,2);assert.notEqual(f.runtime.child,child);
 await f.runtime.stop();
});
test('maintenance never treats a failing or unconfirmed drain as a backup handoff',async()=>{
 for(const failure of ['exit','signal','timeout']){
  const f=fixture();await f.runtime.connect();const child=f.runtime.child;
  child.stdin.end=()=>{if(failure==='exit')queueMicrotask(()=>child.emit('exit',1,null));if(failure==='signal')queueMicrotask(()=>child.emit('exit',null,'SIGTERM'));};
  await assert.rejects(f.runtime.stopForMaintenance(child),/service_(drain_failed|stop_unconfirmed)/);
  assert.equal(f.runtime.maintenanceDrained,false);await assert.rejects(f.runtime.resumeAfterMaintenance(),/unavailable/);
  if(failure==='timeout'){assert.equal(f.runtime.child,child);child.emit('exit',0,null);}
 }
});
test('an external service occupying the old address is never adopted during resume',async()=>{
 const f=fixture();await f.runtime.connect();await f.runtime.stopForMaintenance(f.runtime.child);f.occupy();
 assert.equal((await f.runtime.resumeAfterMaintenance()).state,'maintenance');assert.equal(f.runtime.child,null);assert.equal(f.calls.length,1);assert.equal(f.runtime.externalAttached,undefined);
});
test('stale child and legacy owner cannot authorize maintenance',async()=>{
 const f=fixture();await f.runtime.connect();await assert.rejects(f.runtime.stopForMaintenance({__ownedStdio:true}),/owned_service/);
 f.runtime.child.__ownedStdio=false;await assert.rejects(f.runtime.stopForMaintenance(f.runtime.child),/owned_service/);
 f.runtime.child.__ownedStdio=true;await f.runtime.stop();
});
