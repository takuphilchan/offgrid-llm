const test=require('node:test');
const assert=require('node:assert/strict');
const {EventEmitter}=require('node:events');
const {ComputerRuntime,isComputerLink,parseComputerLink,validateTarget}=require('../computer-runtime');
const copy=require('../../web/app/src/i18n/computer-experience.json');
const fs=require('node:fs');
const path=require('node:path');
const code='a'.repeat(64);
const request={origin:'demo',workspace:'owned-workspace'};
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function fixture(overrides={}) {
 const child=new EventEmitter(); const messages=[]; const forks=[]; let confirmations=0;
 child.postMessage=m=>{messages.push(m);if(m.type==='stop') queueMicrotask(()=>child.emit('exit',0));};
 child.kill=()=>child.emit('exit',1);
 const runtime=new ComputerRuntime({root:'test-pack',directory:'test-profile',service:()=> 'http://127.0.0.1:11611',
   identity:async()=>({product:'offgrid',api_version:2,workspace_id:'owned-workspace',capabilities:['native-computer-sessions-v2']}),
   pairing:async()=>({code}),
   confirm:async()=>{confirmations++;return true;},
   fork:(...args)=>{forks.push(args);queueMicrotask(()=>child.emit('message',{state:'booted'}));return child;},...overrides});
 return {runtime,child,messages,forks,confirmations:()=>confirmations};
}
test('computer deep links carry no service, credentials, target or command',()=>{
 assert.equal(isComputerLink('offgrid://computer'),true);
 for(const value of ['offgrid://computer?code=secret','offgrid://computer/run','offgrid://computer#command','https://computer']) assert.equal(isComputerLink(value),false);
 for(const value of ['http://example.com','https://user:password@example.com','https://127.0.0.1','https://[::1]','https://router.local']) assert.throws(()=>validateTarget(value));
 assert.equal(validateTarget('https://example.com'),'https://example.com');
 assert.equal(validateTarget('https://example.com/path?q=1#section'),'https://example.com/path?q=1#section');
 assert.match(fs.readFileSync(path.join(__dirname,'../main.js'),'utf8'),/!installerTest && !customHome\) app\.setAsDefaultProtocolClient/);
 const main=fs.readFileSync(path.join(__dirname,'../main.js'),'utf8');
 assert.match(main,/require\(app\.isPackaged \? '\.\/computer-pack\/pack\.cjs'/);
 assert.doesNotMatch(main,/require\(path\.join\(root,'pack\.cjs'\)\)/);
});

test('saved-task handoff accepts only an opaque task ID and never task instructions',()=>{
 const task='run-'+'a'.repeat(32);
 assert.deepEqual(parseComputerLink(`offgrid://computer?task=${task}`),{task});
 for(const link of [`offgrid://computer?task=${task}&task=${task}`,`offgrid://computer?task=${task}&approval=full_task`,'offgrid://computer?task=open-notepad','offgrid://evil/computer','offgrid://user:secret@computer',`offgrid://computer?task=${task}#run`])assert.equal(parseComputerLink(link),null);
 const main=fs.readFileSync(path.join(__dirname,'../main.js'),'utf8');
 assert.match(main,/#\/agents\/task\/\$\{computerRequested.task\}/);
});

test('packaged desktop includes the trusted application launcher',()=>{
 const packageJSON=JSON.parse(fs.readFileSync(path.join(__dirname,'../package.json'),'utf8'));
 assert.ok(packageJSON.build.files.includes('application-launcher.js'));
});

test('normal desktop launches always reconnect to the authoritative workspace',()=>{
 const main=fs.readFileSync(path.join(__dirname,'../main.js'),'utf8');
 assert.match(main,/runtime\.connect\(false\)/);
 assert.doesNotMatch(main,/desktop-connection\.json|nextWorkspace|connection-isolated/);
 // Recovery remains explicit and session-scoped; it is never remembered.
 assert.match(main,/startup-local/);
 assert.doesNotMatch(main,/rememberConnection/);
});

test('native picker uses one owned controller and pairs only the chosen returned target',async()=>{
 const f=fixture();const pending=f.runtime.discoverNative({workspace:'owned-workspace'});await tick();
 assert.equal(f.forks.length,1);assert.match(f.forks[0][0],/native-managed.cjs$/);
 assert.equal(f.messages[0].type,'discover');assert.equal(f.messages[0].code,undefined);
 await assert.rejects(f.runtime.start(request),/session_active/);
 f.child.emit('message',{state:'selecting',targets:[{id:'target',title:'Actual application',driver:'windows-uia'}]});
 assert.equal((await pending).targets[0].id,'target');
 await assert.rejects(f.runtime.startNative({workspace:'owned-workspace',target:'invented'}),/stale_target/);
 const started=f.runtime.startNative({workspace:'owned-workspace',target:'target'});await tick();
 assert.deepEqual(f.messages.at(-1),{type:'start-native',target:'target',code,approvalMode:'scoped_changes'});
 f.child.emit('message',{state:'ready',target:{id:'session',origin:'Actual application',driver:'windows-uia'}});
 assert.equal((await started).target.id,'session');await f.runtime.stop();assert.equal(f.runtime.child,null);
});

test('native discovery requires administrator access and cancels without launching a late worker',async()=>{
 const denied=fixture({pairing:async()=>{throw Error('pairing_failed');}});
 await assert.rejects(denied.runtime.discoverNative({workspace:'owned-workspace'}),/pairing_failed/);assert.equal(denied.forks.length,0);
 let release;const pendingVerify=new Promise(resolve=>release=resolve);
 const f=fixture({verify:()=>pendingVerify});const started=f.runtime.discoverNative({workspace:'owned-workspace'});await tick();
 await f.runtime.stop();release();await started;assert.equal(f.forks.length,0);
});

test('scoped stop cannot revoke a different task and cancels only its own pending startup',async()=>{
 for(const method of ['start','discoverNative']) {
   let release;const verification=new Promise(resolve=>release=resolve);
   const f=fixture({verify:()=>verification});
   const args=method==='start'?request:{workspace:'owned-workspace'};
   const pending=f.runtime[method]({...args,requestId:'input-owned'});await tick();
   const generation=f.runtime.generation;
   assert.equal((await f.runtime.stopAccess({requestId:'input-other'})).state,'not_owned');
   assert.equal(f.runtime.generation,generation);
   assert.equal((await f.runtime.stopAccess({requestId:'input-owned'})).state,'stopped');
   release();await pending;assert.equal(f.forks.length,0);
 }
});

test('scoped native control binds discovery and start to the same access request',async()=>{
 const f=fixture();const args={workspace:'owned-workspace',requestId:'input-owned'};
 const discovery=f.runtime.discoverNative(args);await tick();
 f.child.emit('message',{state:'selecting',targets:[{id:'target',title:'Editor'}]});await discovery;
 await assert.rejects(f.runtime.startNative({...args,requestId:'input-other',target:'target'}),/stale_target/);
 assert.equal((await f.runtime.stopAccess({requestId:'input-other'})).state,'not_owned');
 const started=f.runtime.startNative({...args,target:'target'});await tick();
 f.child.emit('message',{state:'ready',target:{id:'session-owned',origin:'Editor'}});await started;
 assert.equal((await f.runtime.stopAccess({session:'session-other'})).state,'not_owned');
 assert.equal(f.runtime.child,f.child);
 assert.equal((await f.runtime.stopAccess({session:'session-owned'})).state,'stopped');
 assert.equal(f.runtime.child,null);
 assert.throws(()=>f.runtime.stopAccess({}),/invalid_session/);
 assert.throws(()=>f.runtime.stopAccess({requestId:'input-owned',session:'session-owned'}),/invalid_session/);
});

test('scoped browser stop cannot terminate a replacement session',async()=>{
 const f=fixture();const pending=f.runtime.start({...request,requestId:'input-new'});await tick();
 f.child.emit('message',{state:'ready',target:{id:'session-new',origin:'offgrid-demo://research'}});await pending;
 assert.equal((await f.runtime.stopAccess({requestId:'input-old'})).state,'not_owned');
 assert.equal((await f.runtime.stopAccess({session:'session-old'})).state,'not_owned');
 assert.equal(f.messages.some(m=>m.type==='stop'),false);
 await f.runtime.stopAccess({requestId:'input-new'});
 assert.equal(f.runtime.child,null);
});

test('native discovery refuses an older backend without pretending native support exists',async()=>{
 const f=fixture({identity:async()=>({product:'offgrid',api_version:2,workspace_id:'owned-workspace'})});
 await assert.rejects(f.runtime.discoverNative({workspace:'owned-workspace'}),/computer_upgrade_required/);
 assert.equal(f.forks.length,0);assert.equal(f.messages.length,0);
});
test('native consent refusal and workspace mismatch cannot spawn a browser',async()=>{
 const denied=fixture({confirm:async()=>false});
 assert.equal((await denied.runtime.start(request)).code,'consent_declined');
 assert.equal(denied.forks.length,0);
 const mismatch=fixture();
 await assert.rejects(mismatch.runtime.start({...request,workspace:'other'}),/workspace_changed/);
 assert.equal(mismatch.confirmations(),0);assert.equal(mismatch.forks.length,0);
 await assert.rejects(mismatch.runtime.start({...request,command:'shell'}),/invalid_session/);
});

test('trusted routing is explicit, locally confirmed, and cannot be selected by arbitrary worker arguments',async()=>{
 let received;
 const f=fixture({confirm:async(...args)=>{received=args;return false;}});
 const requested={origin:'https://example.com/article?q=1',workspace:'owned-workspace',networkMode:'trusted-vpn'};
 assert.equal((await f.runtime.start(requested)).code,'consent_declined');
 assert.deepEqual(received,[requested.origin,'http://127.0.0.1:11611','trusted-vpn','scoped_changes']);
 assert.equal(f.forks.length,0);
 for(const invalid of ['automatic','proxy','',true]) await assert.rejects(f.runtime.start({...requested,networkMode:invalid}),/network_mode_invalid/);
 await assert.rejects(f.runtime.start({...requested,proxy:'http://127.0.0.1:9999'}),/invalid_session/);
 await assert.rejects(f.runtime.start({...request,networkMode:'trusted-vpn'}),/network_mode_invalid/);
 const accepted=fixture();const pending=accepted.runtime.start(requested);await tick();
 assert.equal(accepted.messages[0].networkMode,'trusted-vpn');assert.equal(accepted.messages[0].origin,requested.origin);
 accepted.child.emit('message',{state:'ready',target:{id:'session',origin:'https://example.com'}});await pending;await accepted.runtime.stop();
});

test('unresponsive companion shutdown is bounded without falsely acknowledging stop or losing ownership',async()=>{
 const f=fixture({stopGraceMs:5,stopAckMs:5});
 const started=f.runtime.start(request);await tick();
 f.child.emit('message',{state:'ready',target:{id:'session',origin:'offgrid-demo://research'}});await started;
 f.child.postMessage=()=>{};let kills=0;f.child.kill=()=>{kills++;return true;};
 const result=await f.runtime.stop();
 assert.equal(result.code,'stop_unconfirmed');assert.equal(f.runtime.child,f.child);assert.equal(kills,1);
 await assert.rejects(f.runtime.start(request),/session_active/);
 f.child.emit('exit',1);assert.equal(f.runtime.child,null);
});
test('owned companion receives secrets only over IPC and stops independently of the service',async()=>{
 const f=fixture();const started=f.runtime.start(request);await tick();
 assert.deepEqual(f.forks[0][1],[]);assert.equal(f.forks[0][2].stdio,'ignore');
 assert.equal(JSON.stringify(f.forks).includes(code),false);
 assert.equal(f.messages[0].code,code);
 await assert.rejects(f.runtime.start(request),/session_active/);
 f.child.emit('message',{state:'ready',target:{id:'session',origin:'offgrid-demo://research'}});
 assert.equal((await started).state,'ready');
 await f.runtime.stop();assert.equal(f.runtime.child,null);assert.equal(f.runtime.state.state,'stopped');
 assert.equal(f.messages.at(-1).type,'stop');
});
test('upload grants stay in trusted IPC and reject changed or renderer-shaped values',async()=>{
 const upload={id:'b'.repeat(32),name:'research.pdf',path:path.resolve('private-selection.pdf'),size:42,sha256:'c'.repeat(64)};
 const f=fixture();const started=f.runtime.start({...request,upload});await tick();
 assert.deepEqual(f.messages[0].upload,upload);
 assert.equal(JSON.stringify(f.forks).includes(upload.path),false);
 f.child.emit('message',{state:'ready',target:{id:'session',origin:'offgrid-demo://research'}});await started;await f.runtime.stop();
 for(const invalid of [{...upload,path:'relative.pdf'},{...upload,id:'public-path'},{...upload,size:513*1024*1024},{...upload,extra:'authority'}]) {
   await assert.rejects(fixture().runtime.start({...request,upload:invalid}),/computer_upload_invalid/);
 }
});
test('stop during identity or pack verification cannot launch later',async()=>{
 for(const stage of ['identity','verify']) {
   let release; const waiting=new Promise(resolve=>release=resolve);
   const f=fixture({[stage]:async()=>{await waiting;return {product:'offgrid',api_version:2,workspace_id:'owned-workspace'};}});
   const pending=f.runtime.start(request);await tick();await f.runtime.stop();release();await pending;
   assert.equal(f.forks.length,0);assert.equal(f.runtime.state.state,'stopped');
 }
});
test('corrupt runtime fails before spawning and all nine shared dictionaries agree',async()=>{
 const f=fixture({verify:async()=>{throw Error('pack_invalid');}});
 await assert.rejects(f.runtime.start(request),/pack_invalid/);assert.equal(f.forks.length,0);
 assert.equal(Object.keys(copy).length,9);
 for(const locale of Object.values(copy)){assert.deepEqual(Object.keys(locale).sort(),Object.keys(copy.en).sort());assert.equal(locale.actions.length,7);assert.equal(locale.checkboxStates.length,2);}
});

test('enrollment happens after local consent and pack verification; rejected authorization spawns nothing',async()=>{
 const sequence=[];
 const f=fixture({confirm:async()=>{sequence.push('consent');return true;},verify:async()=>{sequence.push('verified');},pairing:async()=>{sequence.push('enrollment');throw Error('pairing_failed');}});
 await assert.rejects(f.runtime.start(request),/pairing_failed/);
 assert.deepEqual(sequence,['consent','verified','enrollment']);assert.equal(f.forks.length,0);
 const denied=fixture({confirm:async()=>false,pairing:async()=>{throw Error('must not enroll');}});
 assert.equal((await denied.runtime.start(request)).code,'consent_declined');
});
