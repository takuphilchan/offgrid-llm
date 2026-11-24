import test from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {DatabaseSync} from 'node:sqlite';
import {mkdtemp,rm,access,writeFile,readFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {CompanionSession} from '../session.mjs';
import {BrowserWorkspace} from '../browser-workspace.mjs';

const run='run-'+'a'.repeat(32),id='b'.repeat(32),origin='https://approved.example';
const fileTask={actorId:'local-admin',workspaceId:'workspace',runId:run,inputId:'input-initial'};
const wire={version:1,actor_id:'local-admin',workspace_id:'workspace',run_id:run,input_id:'input-initial',session_id:id};
const capabilities=['browser-discovery-v1','browser-workspace-v1','browser-structured-v1','browser-file-transfers-v1'];
// A transport/authority fixture, deliberately not browser-effect qualification.
async function fixture(t,{pair={},action={},caps=capabilities,storageCheck={prepare:async()=>true,stop:async()=>{}}}={}){
 const directory=await mkdtemp(join(tmpdir(),'offgrid-file-binding-'));
 const counts={opens:0,prepares:0,effects:0,polls:0,closed:0},requests=[],bindings=[],actions=[];
 const driver=Object.assign(Object.create(BrowserWorkspace.prototype),{
  browser:Object.assign(new EventEmitter(),{isConnected:()=>false}),allowedOrigins:[origin],control:'automated',
  prepare:async()=>{counts.prepares++;return {approval_class:'read_only'};},
  bindFileTask:async(...args)=>{bindings.push(args);},
  disableFileTransfers:()=>{counts.disabled=true;},
  execute:async(...args)=>{counts.effects++;actions.push(args[2]);return {observed:true};},
  close:async()=>{counts.closed++;},
 });
 const session=new CompanionSession({service:'http://127.0.0.1:11611',directory,storageCheck,openBrowser:async()=>{counts.opens++;return driver;}});
 session.request=async(path,body)=>{
  requests.push({path,body});
  if(path.endsWith('/system'))return {product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:caps};
  if(path.endsWith('/pair'))return {token:'fixture-only',session:{id,origin},file_binding:{...wire},...pair};
  if(path.endsWith('/poll')){
   if(++counts.polls>1){session.stopping=true;return {action:null};}
   return {action:{id:run+':observe',kind:'browser_observe',arguments:{},authorization:'auto',file_binding:{...wire},...action}};
  }
  if(path.endsWith('/reply'))return {};
  throw Error('unexpected fixture request');
 };
 t.after(async()=>{await session.stop();await rm(directory,{recursive:true});});
 return {session,counts,requests,bindings,actions,directory,start:async(options={})=>{
  const target=await session.start({origin,code:'c'.repeat(64),workspace:'workspace',expectedBrowserWorkspace:true,fileTask:{...fileTask},...options});
  await session.running;return target;
 }};
}
test('foreign private action binding is rejected before dispatch intent or driver preparation',async t=>{
 const f=await fixture(t,{action:{file_binding:{...wire,actor_id:'other'}}});await f.start();
 assert.equal(f.counts.effects,0);assert.equal(f.counts.prepares,0);
 const db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
 try{assert.equal(db.prepare('SELECT COUNT(*) AS n FROM dispatch').get().n,0);}finally{db.close();}
 assert.equal(f.session.state,'error');
});
test('private pairing matches the expected host tuple and session before polling',async t=>{
 for(const change of [{actor_id:'other'},{workspace_id:'other'},{run_id:'run-'+'d'.repeat(32)},{input_id:'input-other'},{session_id:'e'.repeat(32)},{version:2},{extra:'untrusted'}]){
  const f=await fixture(t,{pair:{file_binding:{...wire,...change}}});
  await assert.rejects(f.start(),/computer_browser_file_scope_invalid/);assert.equal(f.counts.polls,0);assert.equal(f.session.token,'');
  assert.deepEqual(f.bindings,[]);
 }
});
test('upgraded pairing cannot silently fall back or infer a missing host task',async t=>{
 for(const cap of capabilities){
  const f=await fixture(t,{caps:capabilities.filter(value=>value!==cap)});
  await assert.rejects(f.start(),/computer_upgrade_required/);assert.equal(f.counts.opens,0);
 }
 const f=await fixture(t);await assert.rejects(f.start({fileTask:{...fileTask,actorId:''}}),/computer_browser_file_scope_invalid/);assert.equal(f.counts.opens,0);
 const absent=await fixture(t,{pair:{file_binding:undefined}});await assert.rejects(absent.start(),/computer_browser_file_scope_invalid/);assert.equal(absent.counts.polls,0);
});
test('matching private authority negotiates transfers and executes once with task-bound journal identity',async t=>{
 const f=await fixture(t);await f.start();assert.equal(f.counts.effects,1);
 const paired=f.requests.find(request=>request.path.endsWith('/pair')).body;
 assert.deepEqual(paired.browser_features,{discovery:1,workspace:1,structured:1,transfers:1});
 assert.equal(Object.hasOwn(paired,'file_binding'),false);
 assert.equal(f.session.binding,JSON.stringify(['http://127.0.0.1:11611','workspace',id,'browser-files-v1','local-admin',run,'input-initial']));
 assert.deepEqual(f.bindings.map(([directory,binding])=>[directory,binding]),[[f.directory,{...fileTask,sessionId:id}]]);
 assert.equal(typeof f.bindings[0][2],'function');
 assert.deepEqual(f.actions,[{actionId:run+':observe',documentId:'',binding:{...fileTask,sessionId:id}}]);
});
test('legacy pairing does not accept an unsolicited stronger transfer binding',async t=>{
 const f=await fixture(t);await assert.rejects(f.start({fileTask:undefined}),/computer_browser_file_scope_invalid/);assert.equal(f.counts.polls,0);
});

test('missing and old storage checks preserve ordinary task-bound browsing without a file owner',async t=>{
 for(const storageCheck of [null,{prepare:async()=>false,stop:async()=>{}}]){
  const f=await fixture(t,{storageCheck});await f.start();
  assert.equal(f.counts.effects,1);assert.equal(f.counts.disabled,true);assert.deepEqual(f.bindings,[]);
  assert.equal(f.session.fileStorageReady,false);
 }
});

test('an available native checker rejection prevents journal creation and all browser activation',async t=>{
 const f=await fixture(t,{storageCheck:{prepare:async()=>{throw Error('computer_storage_unavailable');},stop:async()=>{}}});
 await assert.rejects(f.start(),/computer_storage_unavailable/);assert.equal(f.counts.opens,0);assert.equal(f.counts.polls,0);
 await assert.rejects(access(join(f.directory,'dispatch.sqlite')),{code:'ENOENT'});assert.deepEqual(f.bindings,[]);
 const existing=await fixture(t,{storageCheck:{prepare:async()=>{throw Error('computer_storage_unavailable');},stop:async()=>{}}});
 await writeFile(join(existing.directory,'dispatch.sqlite'),'existing journal must remain untouched');
 await assert.rejects(existing.start(),/computer_storage_unavailable/);assert.equal(existing.counts.opens,0);
 assert.equal(await readFile(join(existing.directory,'dispatch.sqlite'),'utf8'),'existing journal must remain untouched');
});

test('unconfirmed helper exit prevents browser activation and preserves Stop failure',async t=>{
 const f=await fixture(t,{storageCheck:{prepare:async()=>{throw Error('computer_stop_unconfirmed');},stop:async()=>{throw Error('computer_stop_unconfirmed');}}});
 await assert.rejects(f.start(),/computer_stop_unconfirmed/);assert.equal(f.counts.opens,0);assert.equal(f.counts.polls,0);
 f.session.storageCheck={stop:async()=>{}};
});
