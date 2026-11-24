const test=require('node:test'),assert=require('node:assert/strict');
const {EventEmitter}=require('node:events');
const {ComputerRuntime}=require('../computer-runtime');
const {savedBrowserTask}=require('../computer-browser-task');
const {resolve}=require('node:path');
const runId='run-'+'a'.repeat(32),requestId='input-initial';
const origin='https://approved.example',session='b'.repeat(32);
const request={origin,workspace:'workspace',runId,requestId,approvalMode:'scoped_changes'};
const caps=['browser-discovery-v1','browser-workspace-v1','browser-structured-v1','browser-file-transfers-v1','browser-task-pairing-v1'];
const saved=()=>({run_id:runId,status:'waiting_for_input',pending_input:{kind:'computer',mode:'browser',id:requestId,url:origin+'/saved'},requested_computer_approval_mode:'scoped_changes'});
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(t,overrides={}){
 const messages=[],pairs=[],events=[];const child=new EventEmitter();
 child.postMessage=message=>{messages.push(message);if(message.type==='stop')queueMicrotask(()=>child.emit('exit',0));};child.kill=()=>child.emit('exit',0);
 const runtime=new ComputerRuntime({root:'fixture-pack',directory:'fixture-private',service:()=> 'http://127.0.0.1:11611',identity:async()=>({product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:caps}),principal:async()=> 'local-admin',task:async()=>saved(),verify:async()=>{events.push('verified');},confirm:async()=>{events.push('consent');return true;},pairing:async(_service,body)=>{pairs.push(body);return {code:'c'.repeat(64)};},fork:()=>{events.push('fork');queueMicrotask(()=>child.emit('message',{state:'booted',browserManualProtocol:1,browserFileBindingProtocol:1}));return child;},browserMonitorMs:60000,...overrides});
 t.after(()=>runtime.stop());return {runtime,messages,pairs,events,child};
}
test('host binds the authenticated actor and saved initial task over private IPC after consent',async t=>{
 const f=fixture(t);const starting=f.runtime.start(request);await tick();
 assert.deepEqual(f.events,['consent','verified','fork']);
 assert.deepEqual(f.pairs,[{protocol_version:1,run_id:runId,input_id:requestId,previous_session:'',approval_mode:'scoped_changes'}]);
 assert.deepEqual(f.messages[0].fileTask,{actorId:'local-admin',workspaceId:'workspace',runId,inputId:requestId});
 f.child.emit('message',{state:'ready',target:{id:session,origin}});assert.equal((await starting).state,'ready');
});
test('host rejects stale or foreign saved task before consent and enrollment',async t=>{
 const variants=[{run_id:'run-'+'d'.repeat(32)},{status:'running'},{pending_input:{...saved().pending_input,url:'https://foreign.example'}},{requested_computer_approval_mode:'full_task'},
  ...[{id:'input-other'},{kind:'other'},{mode:'desktop'},{previous_session:'old-session'}].map(change=>({pending_input:{...saved().pending_input,...change}}))];
 for(const change of variants){const f=fixture(t,{task:async()=>({...saved(),...change})});await assert.rejects(f.runtime.start(request),/computer_scope_violation/);assert.equal(f.events.length,0);assert.equal(f.pairs.length,0);}
});
test('saved input and principal changes during consent cannot enroll or fork',async t=>{
 for(const changing of ['task','principal']){
  let changed=false;const f=fixture(t,{confirm:async()=>{changed=true;return true;},task:async()=>changed&&changing==='task'?{...saved(),status:'cancelled'}:saved(),principal:async()=>changed&&changing==='principal'?'other':'local-admin'});
  await assert.rejects(f.runtime.start(request),/computer_scope_violation|computer_browser_scope_changed/);assert.equal(f.pairs.length,0);assert.equal(f.events.includes('fork'),false);
 }
});
test('older service retains legacy pairing without a stronger private transfer claim',async t=>{
 const f=fixture(t,{identity:async()=>({product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:caps.filter(cap=>cap!=='browser-file-transfers-v1')})});
 const starting=f.runtime.start(request);await tick();assert.deepEqual(f.pairs,[undefined]);assert.equal(f.messages[0].fileTask,undefined);
 f.child.emit('message',{state:'ready',target:{id:session,origin}});await starting;
});
test('task or account changes during pack verification and enrollment cannot launch',async t=>{
 for(const point of ['verify','pairing'])for(const change of ['task','actor']){
  let changed=false;
  const f=fixture(t,{task:async()=>changed&&change==='task'?{...saved(),status:'cancelled'}:saved(),principal:async()=>changed&&change==='actor'?'other':'local-admin',
   verify:async()=>{if(point==='verify')changed=true;},pairing:async()=>{if(point==='pairing')changed=true;return {code:'c'.repeat(64)};}});
  await assert.rejects(f.runtime.start(request),/computer_scope_violation|computer_browser_scope_changed/);assert.equal(f.events.includes('fork'),false);
 }
});
test('renewal binds a new input without restoring the original file grant',async t=>{
 let current=saved(),roots=[resolve('first-workspace')];const f=fixture(t,{task:async()=>current,browserWorkspaceRoots:()=>roots});
 const starting=f.runtime.start(request);await tick();f.child.emit('message',{state:'ready',target:{id:session,origin}});await starting;
 assert.deepEqual(f.messages[0].browserWorkspaceRoots,[resolve('first-workspace')]);roots=[resolve('renewed-workspace')];
 const handoff=f.runtime.browserControl.takeover({session}),handMessage=f.messages.at(-1);
 f.child.emit('message',{state:'manual',requestId:handMessage.requestId,target:{id:session,origin}});await handoff;
 current={...current,pending_input:{kind:'computer',mode:'browser',id:'input-renewed',previous_session:session}};
 const resumed=f.runtime.browserControl.resume({workspace:'workspace',runId,requestId:'input-renewed',session});await tick();
 const message=f.messages.at(-1);assert.equal(message.type,'resume');assert.equal(message.fileTask.inputId,'input-renewed');assert.equal(message.fileTask.actorId,'local-admin');assert.equal(message.upload,undefined);
 assert.deepEqual(message.browserWorkspaceRoots,[resolve('renewed-workspace')]);
 f.child.emit('message',{state:'ready',requestId:message.requestId,target:{id:'d'.repeat(32),origin}});assert.equal((await resumed).state,'ready');
});

test('unknown execution ownership disables protected transfers without converting it to empty scope',async t=>{
 for(const browserWorkspaceRoots of [undefined,()=>{throw Error('uncertain owner');},()=>['relative'],()=>new Array(17).fill(resolve('workspace'))]){
  const f=fixture(t,{browserWorkspaceRoots});const starting=f.runtime.start(request);await tick();
  assert.equal(f.messages[0].browserWorkspaceRoots,null);
  f.child.emit('message',{state:'ready',target:{id:session,origin}});assert.equal((await starting).state,'ready');
 }
 const f=fixture(t);await assert.rejects(f.runtime.start({...request,browserWorkspaceRoots:[]}),/invalid_session/);
});
test('older companion cannot silently ignore required task binding',async t=>{
 let f;f=fixture(t,{fork:()=>{queueMicrotask(()=>f.child.emit('message',{state:'booted',browserManualProtocol:1}));return f.child;}});
 await assert.rejects(f.runtime.start(request),/computer_upgrade_required/);assert.equal(f.messages.some(message=>message.type==='start'),false);
});
test('saved destination cannot inject credentials, protocol or prior-session authority',()=>{
 for(const value of ['https://user:pass@approved.example','http://approved.example','offgrid-demo://other','not a url'])assert.throws(()=>savedBrowserTask({...saved(),pending_input:{...saved().pending_input,url:value}},{runId,inputId:requestId,approvalMode:'scoped_changes',origin}),/computer_scope_violation/);
});
test('canonical pending_input URL constrains initial consent before enrollment',async t=>{
 const f=fixture(t,{task:async()=>({...saved(),pending_input:{...saved().pending_input,url:'https://foreign.example/path'}}),fork:()=>{throw Error('must reject before launching');}});
 await assert.rejects(f.runtime.start(request),/computer_scope_violation/);assert.equal(f.events.length,0);assert.equal(f.pairs.length,0);
});
