import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {createServer} from 'node:http';
import {mkdtemp,writeFile,readFile,rm,readdir} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {DatabaseSync} from 'node:sqlite';
import {BrowserWorkspace} from '../browser-workspace.mjs';
import {BrowserOwnedUploadStore} from '../browser-owned-upload-store.mjs';
import {BrowserUploadLedger} from '../browser-upload-ledger.mjs';
import {CompanionSession} from '../session.mjs';
import {DispatchJournal} from '../journal.mjs';

const binding={actorId:'local-admin',workspaceId:'workspace',runId:'run-'+'a'.repeat(32),inputId:'input-initial',sessionId:'b'.repeat(32)};
const fileAction={actionId:binding.runId+':upload',binding};
const wire={version:1,actor_id:binding.actorId,workspace_id:binding.workspaceId,run_id:binding.runId,input_id:binding.inputId,session_id:binding.sessionId};
const digest=value=>createHash('sha256').update(value).digest('hex');
const deferred=()=>{let resolve;const promise=new Promise(done=>{resolve=done;});return {promise,resolve};};
async function fixture(t,{bind=true}={}){
 const directory=await mkdtemp(join(tmpdir(),'offgrid-upload-integration-')),path=join(directory,'source.txt');
 const content=Buffer.from('approved bytes owned across browser lifetime');await writeFile(path,content);
 const grant={id:'c'.repeat(32),path,name:'source.txt',size:content.length,sha256:digest(content)},received=[];
 const server=createServer(async(req,res)=>{
  if(req.url==='/received'){
   let size=0;const hash=createHash('sha256');for await(const chunk of req){size+=chunk.length;hash.update(chunk);}
   received.push({size,sha256:hash.digest('hex')});res.end('received');return;
  }
  res.setHeader('content-type','text/html');res.end('<input type="file" aria-label="Attach report"><script>window.assignments=0;document.querySelector("input").onchange=()=>{window.assignments++};window.sendSelected=()=>fetch("/received",{method:"POST",body:document.querySelector("input").files[0]});</script>');
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const origin=`http://127.0.0.1:${server.address().port}`;
 const workspace=await BrowserWorkspace.open(origin,{headless:true,testLoopback:true,upload:grant});
 const browserClose=workspace.browser.close.bind(workspace.browser);
 t.after(async()=>{
  workspace.browser.close=browserClose;await workspace.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));
  await rm(directory,{recursive:true});
 });
 if(bind)await workspace.bindFileTask(directory,binding);
 const input=async()=>{
  const driver=workspace.current(),view=await driver.observe(),element=view.elements.find(row=>row.label==='Attach report');assert.ok(element);
  return {driver,handle:driver.elements.get(element.id),args:{observation_id:view.observation_id,element:element.id}};
 };
 return {workspace,directory,grant,content,origin,received,input,browserClose};
}

test('task-bound owner is shared across tabs and durable before exact native assignment',async t=>{
 const f=await fixture(t),store=f.workspace.root.uploadStore;
 await f.workspace.execute('browser_tab_open',{url:f.origin});
 const {driver,handle,args}=await f.input();assert.equal(driver.uploadStore,store);
 for(const key of ['actorId','workspaceId','runId','inputId','sessionId']){
  const foreign={...binding,[key]:key==='runId'?'run-'+'d'.repeat(32):key==='sessionId'?'e'.repeat(32):'other'};
  await assert.rejects(f.workspace.execute('browser_upload',args,{...fileAction,binding:foreign}),error=>error.code==='computer_browser_precondition_failed');
  assert.deepEqual(store.retained().records,[]);
 }
 await assert.rejects(f.workspace.execute('browser_upload',args),error=>error.code==='computer_browser_precondition_failed');
 const assign=handle.setInputFiles.bind(handle);let assignments=0;
 handle.setInputFiles=async(...values)=>{
  assignments++;const row=store.retained().records[0];
  assert.equal(row.state,'in_use');assert.deepEqual(row.binding,binding);assert.equal(row.actionId,fileAction.actionId);
  assert.equal(row.documentId,driver.frames.provenance().frame_document_id);
  await writeFile(f.grant.path,Buffer.alloc(f.content.length,0x58));return assign(...values);
 };
 const result=await f.workspace.execute('browser_upload',args,fileAction);
 assert.equal(result.verification_scope,'file_input_selection');assert.equal(assignments,1);
 await driver.page.evaluate(()=>window.sendSelected());
 assert.deepEqual(f.received,[{size:f.content.length,sha256:digest(f.content)}]);
 assert.equal(store.retained().records[0].state,'in_use');
});

test('target replacement after durable intent rejects before native file assignment',async t=>{
 const f=await fixture(t),{driver,args}=await f.input(),store=driver.uploadStore;
 const inUse=store.inUse.bind(store);
 store.inUse=async(...values)=>{await inUse(...values);await driver.page.evaluate(()=>document.querySelector('input').outerHTML='<input type="file" aria-label="Replacement">');};
 await assert.rejects(f.workspace.execute('browser_upload',args,fileAction),error=>error.code==='computer_browser_precondition_failed');
 assert.equal(await driver.page.evaluate(()=>window.assignments),0);assert.deepEqual(f.received,[]);
 assert.equal(store.retained().records[0].state,'in_use','conservative intent remains owned even without dispatch');
});

test('revocation during preparation and an unconfirmed in-use write both prevent assignment',async t=>{
 for(const failure of ['revoked','write']){
  const f=await fixture(t),{driver,args}=await f.input(),store=driver.uploadStore;
  let db;
  if(failure==='revoked'){
   const get=store.get.bind(store);store.get=async(...values)=>{const snapshot=await get(...values);driver.upload=undefined;return snapshot;};
  }else{
   db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
   db.exec("CREATE TRIGGER deny_in_use BEFORE UPDATE OF state ON browser_upload_resources WHEN NEW.state='in_use' BEGIN SELECT RAISE(IGNORE); END");
  }
  try{
   await assert.rejects(f.workspace.execute('browser_upload',args,fileAction),error=>error.code==='computer_browser_precondition_failed');
   assert.equal(await driver.page.evaluate(()=>window.assignments),0);assert.deepEqual(f.received,[]);
  }finally{if(db){db.exec('DROP TRIGGER deny_in_use');db.close();}}
 }
});

test('upgraded pairing cannot replace a legacy owner with a live selected File',async t=>{
 const f=await fixture(t,{bind:false}),{driver,args}=await f.input(),legacy=driver.uploadStore;
 await f.workspace.execute('browser_upload',args);
 await assert.rejects(f.workspace.bindFileTask(f.directory,binding),/scope_invalid/);
 assert.equal(driver.uploadStore,legacy);assert.deepEqual(await readFile(legacy.snapshot.path),f.content);
 await driver.page.evaluate(()=>window.sendSelected());assert.equal(f.received.length,1);
});

test('manual renewal keeps selected bytes alive but cannot revive the old task grant',async t=>{
 const f=await fixture(t),{driver,args}=await f.input(),store=driver.uploadStore;
 await f.workspace.execute('browser_upload',args,fileAction);
 const row=store.retained().records[0],snapshot=store.records.get(f.grant.id).snapshot;
 await f.workspace.takeover();assert.equal(driver.upload,undefined);
 const next={...binding,inputId:'input-renewed',sessionId:'d'.repeat(32)};
 await f.workspace.bindFileTask(f.directory,next);await f.workspace.resumeManual();
 assert.equal(f.workspace.root.uploadStore,store);assert.deepEqual(store.retained().records[0],row);
 const renewed=await f.input();
 await assert.rejects(f.workspace.execute('browser_upload',renewed.args,{...fileAction,binding:next}),error=>error.code==='computer_browser_precondition_failed');
 await assert.rejects(store.get(f.grant,{...fileAction,binding:next}),/scope_invalid/);
 assert.deepEqual(await readFile(snapshot.path),f.content);
 await driver.page.evaluate(()=>window.sendSelected());assert.equal(f.received.length,1);
 assert.equal(await driver.page.evaluate(()=>window.assignments),1);
});

test('Stop between capture and assignment closes the browser before removing owned bytes',async t=>{
 const f=await fixture(t),{driver,args}=await f.input(),store=driver.uploadStore,ready=deferred(),release=deferred();
 const get=store.get.bind(store);let snapshot,assignments=0;
 store.get=async(...values)=>{snapshot=await get(...values);ready.resolve();await release.promise;return snapshot;};
 const handle=driver.elements.get(args.element);handle.setInputFiles=async()=>{assignments++;throw Error('unexpected assignment');};
 const execution=assert.rejects(f.workspace.execute('browser_upload',args,fileAction));
 await ready.promise;await f.workspace.close();release.resolve();await execution;
 assert.equal(assignments,0);assert.equal(store.closed,true);assert.equal(f.workspace.browser.isConnected(),false);
 assert.deepEqual(await readdir(join(f.directory,'uploads')),[]);assert.deepEqual(f.received,[]);
});

test('unconfirmed browser closure retains snapshot and reservation until a cleanup-only retry',async t=>{
 const f=await fixture(t),{driver,args}=await f.input(),store=driver.uploadStore;
 await f.workspace.execute('browser_upload',args,fileAction);const snapshot=store.records.get(f.grant.id).snapshot;
 f.workspace.browser.close=async()=>{throw Error('isolated close failure');};
 await assert.rejects(f.workspace.close(),/isolated close failure/);
 assert.deepEqual(await readFile(snapshot.path),f.content);assert.equal(store.retained().records[0].browserClosed,false);
 assert.equal(await driver.page.evaluate(()=>window.assignments),1);
 f.workspace.browser.close=f.browserClose;await f.workspace.close();assert.equal(store.closed,true);
 assert.deepEqual(await readdir(join(f.directory,'uploads')),[]);
});

test('owner initialization racing Stop remains attached until confirmed closure and cleanup',async t=>{
 const f=await fixture(t,{bind:false}),open=BrowserOwnedUploadStore.open,ready=deferred(),release=deferred();let store;
 BrowserOwnedUploadStore.open=async options=>{store=await open(options);ready.resolve();await release.promise;return store;};
 try{
  const bindingPending=f.workspace.bindFileTask(f.directory,binding);await ready.promise;
  const stopping=f.workspace.close();release.resolve();await bindingPending;await stopping;
  assert.equal(f.workspace.root.uploadStore,store);assert.equal(store.closed,true);assert.equal(f.workspace.browser.isConnected(),false);
 }finally{release.resolve();BrowserOwnedUploadStore.open=open;}
});

test('lost upload acknowledgement keeps a task-bound tombstone and uncertain retained bytes without replay',async t=>{
 const f=await fixture(t,{bind:false}),{args}=await f.input();
 const action={id:fileAction.actionId,kind:'browser_upload',arguments:args,authorization:'exact',file_binding:wire};
 const session=new CompanionSession({service:'http://127.0.0.1:11611',directory:f.directory,storageCheck:{prepare:async()=>true,stop:async()=>{}},prepared:{driver:f.workspace,demo:{origin:f.origin,close:async()=>{}}}});
 let replies=0;
 session.request=async path=>{
  if(path.endsWith('/system'))return {product:'offgrid',api_version:2,workspace_id:binding.workspaceId,capabilities:['browser-discovery-v1','browser-workspace-v1','browser-structured-v1','browser-file-transfers-v1']};
  if(path.endsWith('/pair'))return {token:'fixture-only',session:{id:binding.sessionId,origin:'offgrid-demo://research'},file_binding:wire};
  if(path.endsWith('/poll'))return {action};
  if(path.endsWith('/reply')){replies++;f.workspace.browser.close=async()=>{throw Error('isolated uncertain browser closure');};throw Error('connection_lost');}
  throw Error('unexpected fixture request');
 };
 try{
  const {sessionId,...fileTask}=binding;
  await session.start({origin:'demo',workspace:binding.workspaceId,fileTask,code:'e'.repeat(64)});await session.running;
  assert.equal(replies,1);assert.equal(session.state,'error');assert.equal(f.workspace.browser.isConnected(),true);
  assert.equal(await f.workspace.page.evaluate(()=>window.assignments),1);
  const store=f.workspace.root.uploadStore,row=store.retained().records[0];
  assert.equal(row.browserClosed,false);assert.equal(row.actionId,action.id);
  assert.deepEqual(await readFile(store.records.get(f.grant.id).snapshot.path),f.content);
  const journal=new DispatchJournal(join(f.directory,'dispatch.sqlite'));
  try{assert.throws(()=>journal.prepare(action,session.binding),/action_uncertain/);}finally{journal.close();}
  const observer=await BrowserUploadLedger.open({directory:f.directory});
  try{const recovered=observer.listRetained().records[0];assert.equal(recovered.recoveryState,'recovery_uncertain');assert.throws(()=>observer.beginCleanup(recovered.resourceId),/owner_mismatch/);}finally{observer.close();}
  const db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
  try{assert.equal(db.prepare('SELECT COUNT(*) AS n FROM dispatch').get().n,1);}finally{db.close();}
 }finally{f.workspace.browser.close=f.browserClose;await session.stop();}
});
