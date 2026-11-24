const test=require('node:test');
const assert=require('node:assert/strict');
const {EventEmitter}=require('node:events');
const {ComputerRuntime}=require('../computer-runtime');
const {workspaceChannelAllowed}=require('../backend');
const fs=require('node:fs'),path=require('node:path');
const {computerPrincipal}=require('../computer-browser-control');
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const runId='run-'+'a'.repeat(32),requestId='input-renew';
const resumeRequest={workspace:'workspace',runId,requestId,session:'session-old'};
const identity=()=>({product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:['browser-workspace-v1','browser-task-pairing-v1']});
const saved=()=>({run_id:runId,status:'waiting_for_input',pending_input:{kind:'computer',mode:'browser',id:requestId,previous_session:'session-old'},requested_computer_approval_mode:'scoped_changes',computer_approval_mode:'scoped_changes'});

async function fixture(overrides={}){
 const child=new EventEmitter(),messages=[],pairs=[],consents=[];
 child.kill=()=>child.emit('exit',0);
 child.postMessage=message=>{messages.push(message);if(message.type==='stop')queueMicrotask(()=>child.emit('exit',0));};
 const runtime=new ComputerRuntime({root:'test-pack',directory:'private-directory',service:()=> 'http://127.0.0.1:11611',identity:async()=>identity(),principal:async()=> 'actor',task:async()=>saved(),verify:async()=>{},
  confirm:async(...args)=>{consents.push(args);return true;},pairing:async(_service,body)=>{pairs.push(body);return {code:'a'.repeat(64)};},
  fork:()=>{queueMicrotask(()=>child.emit('message',{state:'booted',browserManualProtocol:1}));return child;},browserMonitorMs:60000,...overrides});
 const starting=runtime.start({origin:'https://one.example/path',additionalOrigins:['https://two.example'],workspace:'workspace',requestId:'input-initial'});
 await tick();child.emit('message',{state:'ready',target:{id:'session-old',origin:'https://one.example'}});await starting;
 const manual=async()=>{
  const result=runtime.browserControl.takeover({session:'session-old'});const op=messages.at(-1);
  child.emit('message',{state:'manual',requestId:op.requestId,target:{id:'session-old',origin:'https://one.example'}});
  assert.equal((await result).state,'manual');
 };
 return {runtime,child,messages,pairs,consents,manual};
}

test('manual host handoff and renewed consent bind exact task, scope and correlated replies',async()=>{
 const f=await fixture();
 try{
  assert.equal(f.runtime.browserControl.status().browserManualProtocol,1);
  await f.manual();
  const pending=f.runtime.browserControl.resume(resumeRequest);await tick();
  const message=f.messages.at(-1);assert.equal(message.type,'resume');
  assert.deepEqual(f.pairs[1],{protocol_version:1,run_id:runId,input_id:requestId,previous_session:'session-old',approval_mode:'scoped_changes'});
  assert.deepEqual(f.consents[0],f.consents[1],'renewed consent must retain exact original destinations and tab bound');
  assert.deepEqual(message.additionalOrigins,['https://two.example']);assert.equal(message.directory,'private-directory');assert.equal(message.upload,undefined);
  assert.equal(message.previousSession,'session-old');assert.equal(message.expectedBrowserWorkspace,true);
  f.child.emit('message',{state:'ready',requestId:'stale',target:{id:'session-forged',origin:'https://one.example'}});
  f.child.emit('message',{state:'ready',target:{id:'session-uncorrelated',origin:'https://one.example'}});
  assert.equal(f.runtime.state.state,'resuming');assert.equal(f.runtime.state.target.id,'session-old');
  f.child.emit('message',{state:'ready',requestId:message.requestId,target:{id:'session-new',origin:'https://one.example'}});
  const result=await pending;assert.equal(result.target.id,'session-new');assert.equal(f.runtime.pending,false);
  assert.deepEqual(await f.runtime.browserControl.resume({session:resumeRequest.session,requestId,runId,workspace:'workspace'}),result,'lost renderer reply observes same operation regardless of object property order');
  assert.equal(f.pairs.length,2);assert.equal(f.messages.filter(m=>m.type==='resume').length,1);
 }finally{await f.runtime.stop();}
});

test('unknown handoff and resume replies retain correlation without repeating enrollment',async()=>{
 const f=await fixture({browserControlTimeoutMs:8});
 try{
  const taking=f.runtime.browserControl.takeover({session:'session-old'}),operation=f.messages.at(-1);
  await assert.rejects(taking,/handoff_unconfirmed/);assert.equal(f.runtime.state.state,'taking_over');
  const again=f.runtime.browserControl.takeover({session:'session-old'});
  assert.equal(f.messages.filter(m=>m.type==='takeover').length,1);
  f.child.emit('message',{state:'manual',requestId:operation.requestId,target:{id:'session-old',origin:'https://one.example'}});await again;
  await assert.rejects(f.runtime.browserControl.resume(resumeRequest),/resume_unconfirmed/);
  const message=f.messages.at(-1),retry=f.runtime.browserControl.resume(resumeRequest);
  assert.equal(f.pairs.length,2);assert.equal(f.messages.filter(m=>m.type==='resume').length,1);
  f.child.emit('message',{state:'ready',requestId:message.requestId,target:{id:'session-new',origin:'https://one.example'}});await retry;
 }finally{await f.runtime.stop();}
});

test('untrusted resume fields, changed task and policy never create renewed authority',async()=>{
 for(const changed of ['id','previous','mode','policy','status']){
  const f=await fixture();await f.manual();
  const next=saved();if(changed==='id')next.pending_input.id='input-other';if(changed==='previous')next.pending_input.previous_session='other';if(changed==='mode')next.pending_input.mode='desktop';if(changed==='policy')next.requested_computer_approval_mode='full_task';if(changed==='status')next.status='completed';
  f.runtime.task=async()=>next;
  try{await assert.rejects(f.runtime.browserControl.resume(resumeRequest),/scope_violation/);assert.equal(f.pairs.length,1);assert.equal(f.runtime.state.state,'manual');}
  finally{await f.runtime.stop();}
 }
 const f=await fixture();await f.manual();
 try{
  for(const field of ['origin','code','approvalMode','directory','additionalOrigins','upload'])assert.throws(()=>f.runtime.browserControl.resume({...resumeRequest,[field]:'injected'}),/invalid_session/);
  assert.throws(()=>f.runtime.browserControl.takeover({session:'session-old',approved:true}),/invalid_session/);
  assert.equal(f.pairs.length,1);
 }finally{await f.runtime.stop();}
});

test('changed actor, workspace or service closes retained resources before renewed pairing',async()=>{
 for(const changed of ['actor','workspace','service']){
  const f=await fixture();await f.manual();
  if(changed==='actor')f.runtime.principal=async()=> 'other';
  if(changed==='workspace')f.runtime.identity=async()=>({...identity(),workspace_id:'other'});
  if(changed==='service')f.runtime.service=()=> 'http://127.0.0.1:12000';
  await assert.rejects(f.runtime.browserControl.resume(resumeRequest),/scope_changed|stopped/);
  assert.equal(f.pairs.length,1);assert.equal(f.runtime.child,null);assert.equal(f.runtime.pending,false);
 }
});

test('Stop during consent or delayed reply invalidates ownership and ignores stale child events',async()=>{
 for(const stage of ['consent','reply']){
  const f=await fixture();await f.manual();let release;
  if(stage==='consent')f.runtime.confirm=()=>new Promise(resolve=>{release=resolve;});
  const pending=f.runtime.browserControl.resume(resumeRequest);const rejection=assert.rejects(pending,/computer_stopped/);await tick();
  const message=f.messages.at(-1);
  await f.runtime.stopAccess({session:'session-old'});release?.(true);await rejection;await tick();
  f.child.emit('message',{state:'ready',requestId:message.requestId,target:{id:'session-new',origin:'https://one.example'}});
  assert.equal(f.runtime.child,null);assert.equal(f.runtime.pending,false);assert.equal(f.runtime.state.state,'stopped');
  assert.equal(f.pairs.length,stage==='consent'?1:2);
 }
});

test('consent rechecks scope and maintenance hold; outages retain manual ownership and local Stop',async()=>{
 for(const changed of ['actor','task','hold','declined']){
  const f=await fixture();await f.manual();
  f.runtime.confirm=async()=>{
   if(changed==='actor')f.runtime.principal=async()=> 'other';
   if(changed==='task')f.runtime.task=async()=>({...saved(),status:'completed'});
   if(changed==='hold')f.runtime.service=()=>{throw Error('service_unavailable');};
   return changed!=='declined';
  };
  await assert.rejects(f.runtime.browserControl.resume(resumeRequest),/scope_changed|scope_violation|stopped|service_unavailable|consent_declined/);
  assert.equal(f.pairs.length,1);await f.runtime.stop();
 }
 const f=await fixture();await f.manual();f.runtime.service=()=>{throw Error('service_unavailable');};
 await assert.rejects(f.runtime.browserControl.resume(resumeRequest),/service_unavailable/);
 assert.equal(f.runtime.child,f.child);assert.equal(f.runtime.state.state,'manual');await f.runtime.stop();
 for(const loading of [false,true]){
  assert.equal(workspaceChannelAllowed('computer-takeover-access',false,loading),true);
  assert.equal(workspaceChannelAllowed('computer-browser-resume',false,loading),false);
 }
});

test('unsupported native or older browser hosts return an explicit fallback and never resume',async()=>{
 const f=await fixture();
 try{
  f.runtime.browserControl.context.protocol=0;
  assert.equal((await f.runtime.browserControl.takeover({session:'session-old'})).state,'unsupported');
  f.runtime.browserControl.invalidate();
  assert.equal((await f.runtime.browserControl.takeover({session:'session-old'})).state,'unsupported');
  assert.equal((await f.runtime.browserControl.takeover({session:'foreign'})).state,'not_owned');
  assert.equal(f.messages.some(m=>m.type==='takeover'||m.type==='resume'),false);
 }finally{await f.runtime.stop();}
 const manifest=JSON.parse(fs.readFileSync(path.join(__dirname,'../package.json'),'utf8'));
 assert.ok(manifest.build.files.includes('computer-browser-control.js'));
 const preload=fs.readFileSync(path.join(__dirname,'../preload.js'),'utf8');
 assert.match(preload,/browserManualProtocol: 1/);assert.match(preload,/computer-takeover-access/);assert.match(preload,/computer-browser-resume/);
});

test('unconfirmed child or host Stop retains the exact target for scoped cleanup retry',async()=>{
 const f=await fixture({stopGraceMs:4,stopAckMs:4});
 f.child.emit('message',{state:'error',code:'computer_stop_unconfirmed'});
 assert.equal(f.runtime.state.target?.id,'session-old');
 f.child.postMessage=message=>f.messages.push(message);f.child.kill=()=>false;
 const failed=await f.runtime.stopAccess({session:'session-old'});
 assert.equal(failed.code,'stop_unconfirmed');assert.equal(failed.target?.id,'session-old');
 assert.equal(f.runtime.child,f.child);
 f.child.postMessage=message=>{f.messages.push(message);if(message.type==='stop')queueMicrotask(()=>f.child.emit('exit',0));};
 assert.equal((await f.runtime.stopAccess({session:'foreign'})).state,'not_owned');
 assert.equal((await f.runtime.stopAccess({session:'session-old'})).state,'stopped');
 assert.equal(f.runtime.child,null);
});

test('background authority checks close revoked sessions but retain temporary outages',async()=>{
 for(const code of ['computer_scope_violation','service_unavailable','fetch failed']){
  const f=await fixture({browserMonitorMs:2});await f.manual();
  let checked,timer;
  // Production's monitor intentionally does not keep Electron alive. This
  // EventEmitter child has no process handle, so the fixture must own a bounded
  // referenced wait until a real monitor tick runs (not invoke checks itself).
  const observed=new Promise((resolve,reject)=>{checked=resolve;timer=setTimeout(()=>reject(Error('background authority monitor did not run')),5000);});
  try{
   f.runtime.principal=async()=>{checked();throw Error(code);};
   await observed;await tick();await tick();
   if(code==='computer_scope_violation')assert.equal(f.runtime.child,null);
   else {assert.equal(f.runtime.child,f.child);assert.equal(f.runtime.state.state,'manual');}
  }finally{clearTimeout(timer);await f.runtime.stop();}
 }
});

test('authenticated principal responses distinguish definitive denial from service failure',async()=>{
 const response=(body,status=200)=>new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
 for(const status of [401,403])await assert.rejects(computerPrincipal(response({},status)),/computer_scope_violation/);
 for(const status of [429,500,503])await assert.rejects(computerPrincipal(response({},status)),/service_unavailable/);
 for(const body of [{auth_required:true,authenticated:false},{auth_required:true,authenticated:true,user:{id:'actor',role:'user'}},{auth_required:true,authenticated:true,user:{role:'admin'}}])await assert.rejects(computerPrincipal(response(body)),/computer_scope_violation/);
 assert.equal(await computerPrincipal(response({auth_required:false})),'local-admin');
 assert.equal(await computerPrincipal(response({auth_required:true,authenticated:true,user:{id:'actor',role:'admin'}})),'actor');
});
