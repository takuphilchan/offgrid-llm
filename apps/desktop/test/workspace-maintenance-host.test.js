'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {createWorkspaceRecovery,boundedJSON}=require('../workspace-maintenance-host');
function fixture(account,identity={}){
 const hash='a'.repeat(64),channels=new Map(),calls=[];
 const runtime={state:{state:'ready'},child:{__ownedStdio:true,__binaryPin:{path:'/owned/binary'}},workspace:{data:'/owned/data'},options:{version:'v1',uiBuildID:hash},url:'http://127.0.0.1:11611',snapshot(){return {...this.state,workspaceID:'original'};}};
 const win={isDestroyed:()=>false,webContents:{mainFrame:{routingId:1},send(){},session:{fetch:async url=>new Response(JSON.stringify(url.endsWith('/me')?account:{product:'offgrid',workspace_id:'original',version:'v1',ui_build_id:hash,...identity}))}}};
 const recovery=createWorkspaceRecovery({runtime,window:()=>win,locale:()=> 'en',handle:(name,handler)=>channels.set(name,handler),dialog:{showSaveDialog:async()=>{calls.push('picker');return {canceled:true};}},showMessage:async()=>{calls.push('secret');return {response:0};},computer:{stop:async()=>{}},execution:{stop:async()=>{}},terminal:()=>null});
 recovery.options.files={...recovery.options.files,existing:async path=>({path})};
 return {recovery,channels,calls};
}
test('current administrator and exact owned workspace build are required before native path or secret selection',async()=>{
 for(const [account,identity] of [[{auth_required:true,authenticated:false},{}],[{auth_required:true,authenticated:true,user:{id:'reader',role:'user'}},{}],[{auth_required:false},{workspace_id:'other'}],[{auth_required:false},{version:'other'}],[{auth_required:false},{ui_build_id:'b'.repeat(64)}]]){
  const f=fixture(account,identity);f.channels.get('workspace-recovery-start')({operation:'backup',encryption:'passphrase'});await f.recovery.current.done;
  assert.equal(f.recovery.snapshot().phase,'failed');assert.deepEqual(f.calls,[]);
 }
});
test('local administrator may choose a path but cancel never stops the service',async()=>{
 const f=fixture({auth_required:false});f.channels.get('workspace-recovery-start')({operation:'backup',encryption:'passphrase'});await f.recovery.current.done;
 assert.equal(f.recovery.snapshot().phase,'cancelled');assert.deepEqual(f.calls,['picker']);assert.equal(f.recovery.runtime.state.state,'ready');
});
test('oversized or failed principal responses never reach parser or native consent',async()=>{
 await assert.rejects(boundedJSON(new Response('x'.repeat(65537))),/authority_changed/);
 await assert.rejects(boundedJSON(new Response('{}',{status:401})),/authority_changed/);
});
