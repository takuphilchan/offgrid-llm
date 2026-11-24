import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,writeFile,readFile,readdir,rm,rename} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {DatabaseSync} from 'node:sqlite';
import {BrowserOwnedUploadStore} from '../browser-owned-upload-store.mjs';
import {BrowserUploadLedger} from '../browser-upload-ledger.mjs';

const binding={actorId:'local-admin',workspaceId:'workspace',runId:'run-'+'a'.repeat(32),inputId:'input-initial',sessionId:'b'.repeat(32)};
const action={actionId:binding.runId+':upload',binding};
async function fixture(t){
 const root=await mkdtemp(join(tmpdir(),'offgrid-owned-upload-')),directory=join(root,'private'),path=join(root,'source.txt'),content=Buffer.from('exact locally approved fixture bytes');
 await writeFile(path,content);
 const grant={id:'c'.repeat(32),path,name:'source.txt',size:content.length,sha256:createHash('sha256').update(content).digest('hex')};
 const store=await BrowserOwnedUploadStore.open({directory,binding,maxRetainedBytes:content.length*2});
 t.after(async()=>{if(!store.closed){await store.browserClosed();await store.close();}await rm(root,{recursive:true});});
 return {root,directory,grant,content,store};
}
test('actual snapshot identities and content are durably recorded before possible selection',async t=>{
 const f=await fixture(t),snapshot=await f.store.get(f.grant,action);
 assert.deepEqual(await readFile(snapshot.path),f.content);
 const record=f.store.retained().records[0];assert.equal(record.state,'ready');assert.deepEqual(record.binding,binding);
 assert.equal(typeof record.fileIdentity.inode,'string');assert.equal(record.grant.sha256,f.grant.sha256);
 const db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
 try{const row=db.prepare('SELECT * FROM browser_upload_resources').get();assert.equal(row.state,'ready');assert.equal(JSON.stringify(row).includes(f.grant.path),false);}finally{db.close();}
 await f.store.inUse(snapshot,action,'d'.repeat(8)+'-dddd-dddd-dddd-'+'d'.repeat(12));
 assert.equal(f.store.retained().records[0].state,'in_use');
 await assert.rejects(f.store.close(),/browser_close_unconfirmed/);assert.deepEqual(await readFile(snapshot.path),f.content);
 await f.store.browserClosed();await f.store.close();assert.deepEqual(await readdir(join(f.directory,'uploads')),[]);
 const reopened=await BrowserUploadLedger.open({directory:f.directory,maxRetainedBytes:f.content.length*2});
 try{assert.deepEqual(reopened.listRetained().records,[]);}finally{reopened.close();}
});
test('durable capacity and old native File ownership survive rebinding and journal closure',async t=>{
 const f=await fixture(t),snapshot=await f.store.get(f.grant,action);await f.store.inUse(snapshot,action);
 const before=f.store.retained().records[0];
 const next={...binding,inputId:'input-renewed',sessionId:'e'.repeat(32)};f.store.rebind(next);
 await assert.rejects(f.store.get(f.grant,{...action,binding:next}),/scope_invalid/);
 assert.deepEqual(await readFile(snapshot.path),f.content);assert.deepEqual(f.store.retained().records[0],before);
 const observer=await BrowserUploadLedger.open({directory:f.directory,maxRetainedBytes:f.content.length*2});
 try{const row=observer.listRetained().records[0];assert.equal(row.recoveryState,'recovery_uncertain');assert.throws(()=>observer.beginCleanup(row.resourceId),/owner_mismatch/);}finally{observer.close();}
 const second=await f.store.get({...f.grant,id:'f'.repeat(32)},{...action,binding:next});assert.notEqual(second.path,snapshot.path);
 await assert.rejects(f.store.get({...f.grant,id:'1'.repeat(32)},{...action,binding:next}),/capacity/);
});
test('ignored identity and ready writes prevent preparation and confirm cleanup before refund',async t=>{
 for(const state of ['directory','file','ready']){
  const f=await fixture(t),db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
  db.exec(`CREATE TRIGGER deny_${state} BEFORE UPDATE OF state ON browser_upload_resources WHEN NEW.state='${state}' BEGIN SELECT RAISE(IGNORE); END`);
  try{await assert.rejects(f.store.get(f.grant,action),/write_unconfirmed/);assert.deepEqual(f.store.retained().records,[]);assert.deepEqual(await readdir(join(f.directory,'uploads')),[]);}finally{db.close();}
 }
});
test('lost final removal receipt retains its reservation until confirmed retry',async t=>{
 const f=await fixture(t),snapshot=await f.store.get(f.grant,action);await f.store.inUse(snapshot,action);await f.store.browserClosed();
 const db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
 db.exec("CREATE TRIGGER deny_removed BEFORE UPDATE OF state ON browser_upload_resources WHEN NEW.state='removed' BEGIN SELECT RAISE(IGNORE); END");
 try{
  await assert.rejects(f.store.close(),/write_unconfirmed/);assert.deepEqual(await readdir(join(f.directory,'uploads')),[]);
  assert.equal(f.store.retained().records[0].state,'deleting');db.exec('DROP TRIGGER deny_removed');await f.store.close();
 }finally{db.close();}
});
test('failed cleanup checkpoint preserves exact bytes and retry does not replay selection',async t=>{
 const f=await fixture(t),snapshot=await f.store.get(f.grant,action);await f.store.inUse(snapshot,action);await f.store.browserClosed();
 const db=new DatabaseSync(join(f.directory,'dispatch.sqlite'));
 db.exec("CREATE TRIGGER deny_cleanup BEFORE UPDATE OF state ON browser_upload_resources WHEN NEW.state='deleting' BEGIN SELECT RAISE(IGNORE); END");
 try{await assert.rejects(f.store.close(),/write_unconfirmed/);assert.deepEqual(await readFile(snapshot.path),f.content);assert.equal(f.store.retained().records[0].state,'in_use');db.exec('DROP TRIGGER deny_cleanup');await f.store.close();}finally{db.close();}
});
test('replacement file is preserved and charged until exact owned bytes return for cleanup',async t=>{
 const f=await fixture(t),snapshot=await f.store.get(f.grant,action),path=snapshot.path,saved=join(f.root,'saved.txt');
 await f.store.inUse(snapshot,action);await f.store.browserClosed();await rename(path,saved);await writeFile(path,'unrelated sentinel');
 try{
  await assert.rejects(f.store.close(),/ownership changed/);assert.equal(await readFile(path,'utf8'),'unrelated sentinel');assert.equal(f.store.retained().records[0].state,'deleting');
 }finally{await rm(path);await rename(saved,path);}
 await f.store.close();
});
test('Stop settles pending preparation before browser-closed cleanup eligibility',async t=>{
 const f=await fixture(t);const preparing=assert.rejects(f.store.get(f.grant,action),/unavailable|aborted/);
 await f.store.browserClosed();await preparing;await f.store.close();assert.deepEqual(await readdir(join(f.directory,'uploads')),[]);
});
