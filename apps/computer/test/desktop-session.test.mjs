import test from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {DesktopSession} from '../desktop-session.mjs';
import {DispatchJournal} from '../journal.mjs';

function fixture(){
 const desktop={id:'desktop',os_session:'os',process_generation:'desktop',surface:'desktop',driver:'windows-uia'};
 const binding={actor:'alice',task:'run',companion:'session',session:'session',target:desktop};
 const windowTarget={...desktop,id:'chrome-window',process_generation:'chrome-process',surface:'chrome-surface'};
 const worker=new EventEmitter();worker.protocol=3;worker.calls=[];worker.stop=async()=>{worker.stopped=true;};
 worker.request=async(kind,args)=>{
  worker.calls.push({kind,args});
  if(kind==='desktop_identity')return desktop;
  if(kind==='grant_desktop')return {granted:true,binding};
  if(kind==='applications')return [{id:'chrome',name:'Google Chrome'}];
  if(kind==='prepare_lifecycle')return {action:{id:'prepared',operation:args.operation,approval_class:'reversible'},binding:args.operation.kind==='open_application'?binding:{...binding,target:windowTarget}};
  if(kind==='approve')return {id:'grant'};
  if(kind==='execute')return [{action_id:'prepared',outcome:'dispatched'}];
  if(kind==='verify_application')return {verified:true,check:'application_open',application_id:args.application,target:windowTarget,view:{observation:{id:'view',target:windowTarget},elements:[]}};
  if(kind==='surfaces')return [{identity:windowTarget,application_id:'chrome'}];
  throw Error('unexpected '+kind);
 };
 const session=new DesktopSession({service:'http://127.0.0.1:11611',directory:'unused',worker});session.binding=binding;session.approvalMode='full_task';session.session='session';
 const action=(kind,args={},id='call')=>({id,kind,arguments:args,binding,authorization:'auto'});
 return {desktop,binding,worker,session,action,windowTarget};
}

test('missing target read keeps the desktop transport alive but scope loss stops it',async()=>{
 for(const code of ['computer_target_not_selected','computer_application_not_ready','computer_document_changed','computer_scope_violation']){
  const {session,worker,action}=fixture(),directory=await mkdtemp(join(tmpdir(),'offgrid-read-recovery-'));
  const replies=[];let polls=0;const original=worker.request;
  worker.request=async(kind,args)=>{if(kind==='observe')throw Error(code);return original(kind,args);};
  session.scope='fixture';session.journal=new DispatchJournal(join(directory,'journal.sqlite'));
  session.companion=async(kind,body)=>{
   if(kind==='reply'){replies.push(body);return {};}
   if(kind==='poll'){polls++;if(polls===1)return {action:action('computer_observe',{},'read')};if(polls===2)return {action:action('computer_applications',{},'catalog')};session.stopping=true;return {};}
   return {};
  };
  try{
   await session.loop();
   assert.equal(replies[0].error,code);
   assert.equal(replies.length,code==='computer_scope_violation'?1:2);
   assert.equal(worker.calls.some(c=>['prepare','approve','execute'].includes(c.kind)),false);
  }finally{await session.stop();await rm(directory,{recursive:true,force:true});}
 }
});
test('desktop discovery, durable launch, independent readback and target switch use one grant',async()=>{
 const f=fixture();const {session,worker,action}=f;
 assert.equal((await session.execute(action('computer_applications'))).applications[0].id,'chrome');
 await session.execute(action('computer_prepare',{tool:'computer_open_application',arguments:{application_id:'chrome'}},'launch:prepare'));
 const opened=await session.execute(action('computer_open_application',{application_id:'chrome'},'launch'));
 assert.equal(opened.actions[0].outcome,'dispatched');assert.equal(session.view,null);
 const verified=await session.execute(action('computer_verify_application',{application_id:'chrome'}));assert.equal(verified.verified,true);
 await session.execute(action('computer_windows'));
 await session.execute(action('computer_focus_window',{target_id:'chrome-window'},'focus'));
 const approvals=worker.calls.filter(c=>c.kind==='approve');assert.equal(approvals.length,2);
 assert.equal(approvals[0].args.step.binding.target.id,'desktop');assert.equal(approvals[1].args.step.binding.target.id,'chrome-window');
 assert.equal(worker.calls.some(c=>c.kind==='select'||c.kind==='grant_desktop'),false);
 await session.stop();
});

test('readiness failures are structured and never expose an empty usable target',async()=>{
 for(const state of ['starting','surface_unavailable','unsupported_identity','observation_unavailable']){
  const {session,worker,action}=fixture();
  worker.request=async()=>({verified:false,check:'application_open',application_id:'chrome',readiness:state});
  const result=await session.execute(action('computer_verify_application',{application_id:'chrome'}));
  assert.equal(result.readiness,state);assert.equal(session.view,null);
  worker.request=async()=>({...result,target:{}});
  await assert.rejects(session.execute(action('computer_verify_application',{application_id:'chrome'})),/protocol_invalid/);
  worker.request=async()=>({...result,readiness:'invented'});
  await assert.rejects(session.execute(action('computer_verify_application',{application_id:'chrome'})),/protocol_invalid/);
  await session.stop();
 }
});
test('desktop rejects changed scope, arbitrary paths and reused prepared parameters',async()=>{
 const {session,worker,action,binding}=fixture();
 await assert.rejects(session.execute({...action('computer_applications'),binding:{...binding,task:'other'}}),/scope_violation/);
 await assert.rejects(session.execute(action('computer_open_application',{application_id:'C:\\cmd.exe'})),/invalid_action/);
 await session.execute(action('computer_prepare',{tool:'computer_open_application',arguments:{application_id:'chrome'}},'launch:prepare'));
 await assert.rejects(session.execute(action('computer_open_application',{application_id:'editor'},'launch')),/approval_invalid/);
 assert.equal(worker.calls.some(c=>c.kind==='execute'),false);await session.stop();
});
test('desktop enrollment activates only after host consent; stop during consent cannot activate later',async()=>{
 for(const stop of [false,true]){
  const f=fixture();const directory=await mkdtemp(join(tmpdir(),'offgrid-desktop-session-'));
  const requests=[];let consent;const original=f.worker.request;
  f.worker.request=(kind,args)=>kind==='grant_desktop'?new Promise(resolve=>{consent=()=>resolve({granted:true,binding:f.binding});}):original(kind,args);
  const fetchRequest=async(url,options)=>{
   requests.push(url.pathname);
   const data=url.pathname==='/api/v2/system'?{product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:['task-desktop-sessions-v3']}:url.pathname.endsWith('/pair')?{session:{id:'session',run_id:'run',input_id:'input',approval_mode:'full_task',state:'awaiting_consent',target:f.desktop,desktop:{workspace:'workspace',binding:f.binding}},token:'b'.repeat(64)}:{active:true};
   return {ok:true,json:async()=>data};
  };
  const session=new DesktopSession({service:'http://127.0.0.1:11611',directory,worker:f.worker,fetchRequest});session.loop=async()=>{};
  try{
   const start=session.start({code:'a'.repeat(64),workspace:'workspace',run:'run',input:'input',prompt:'Open Chrome',approvalMode:'full_task'});
   const rejected=stop?assert.rejects(start,/computer_stopped/):null;
   while(!consent)await new Promise(resolve=>setImmediate(resolve));
   assert.equal(requests.some(p=>p.endsWith('/activate')),false);
   if(stop)await session.stop();consent();
   if(stop)await rejected;else assert.equal((await start).scope,'desktop');
   assert.equal(requests.some(p=>p.endsWith('/activate')),!stop);
  }finally{await session.stop();await rm(directory,{recursive:true,force:true});}
 }
});

test('two task sessions release their authority and listeners without ending the consented host',async()=>{
 const f=fixture(),directory=await mkdtemp(join(tmpdir(),'offgrid-desktop-reuse-'));let releases=0;
 try {
  for(const run of ['first','second']) {
   const events=[];const binding={...f.binding,task:run,session:run,companion:run};
   const original=f.worker.request;
   f.worker.request=async(kind,args)=>kind==='grant_desktop'?(assert.equal(args.service,'http://127.0.0.1:11611'),{granted:true,binding}):original(kind,args);
   const fetchRequest=async(url)=>{
    if(url.pathname.endsWith('/poll'))return {ok:false,status:403}; // task ended on service
    const data=url.pathname==='/api/v2/system'?{product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:['task-desktop-sessions-v3']}:url.pathname.endsWith('/pair')?{session:{id:run,run_id:run,input_id:'input',approval_mode:'full_task',state:'awaiting_consent',target:f.desktop,desktop:{workspace:'workspace',binding}},token:'b'.repeat(64)}:{active:true};
    return {ok:true,json:async()=>data};
   };
   const session=new DesktopSession({service:'http://127.0.0.1:11611',directory,worker:f.worker,fetchRequest,emit:e=>events.push(e),releaseWorker:async()=>{releases++;}});
   await session.start({code:'a'.repeat(64),workspace:'workspace',run,input:'input',prompt:'Owned fixture',approvalMode:'full_task'});await session.running;
   assert.equal(session.failed,undefined);assert.equal(session.token,'');assert.equal(session.journal,null);
   assert.equal(f.worker.listenerCount('stopped'),0);assert.equal(f.worker.stopped,undefined);
   assert.equal(events.some(e=>e.state==='consent'||e.state==='error'),false);
  }
  assert.equal(releases,2);
 }finally{await f.worker.stop();await rm(directory,{recursive:true,force:true});}
});
