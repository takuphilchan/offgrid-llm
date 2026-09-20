const test=require('node:test');
const assert=require('node:assert/strict');
const {EventEmitter}=require('node:events');
const {ComputerRuntime,isComputerLink,validateTarget}=require('../computer-runtime');
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
   identity:async()=>({product:'offgrid',api_version:2,workspace_id:'owned-workspace'}),
   pairing:async()=>({code}),
   confirm:async()=>{confirmations++;return true;},
   fork:(...args)=>{forks.push(args);queueMicrotask(()=>child.emit('message',{state:'booted'}));return child;},...overrides});
 return {runtime,child,messages,forks,confirmations:()=>confirmations};
}
test('computer deep links carry no service, credentials, target or command',()=>{
 assert.equal(isComputerLink('offgrid://computer'),true);
 for(const value of ['offgrid://computer?code=secret','offgrid://computer/run','offgrid://computer#command','https://computer']) assert.equal(isComputerLink(value),false);
 for(const value of ['http://example.com','https://user:password@example.com','https://example.com/path','https://example.com/?x=1']) assert.throws(()=>validateTarget(value));
 assert.equal(validateTarget('https://example.com'),'https://example.com');
 assert.match(fs.readFileSync(path.join(__dirname,'../main.js'),'utf8'),/!installerTest && !customHome\) app\.setAsDefaultProtocolClient/);
 const main=fs.readFileSync(path.join(__dirname,'../main.js'),'utf8');
 assert.match(main,/require\(app\.isPackaged \? '\.\/computer-pack\/pack\.cjs'/);
 assert.doesNotMatch(main,/require\(path\.join\(root,'pack\.cjs'\)\)/);
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
