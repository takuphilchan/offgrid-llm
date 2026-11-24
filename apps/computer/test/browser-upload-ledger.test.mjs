import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,readFile,lstat,rm} from 'node:fs/promises';
import {join,dirname,resolve} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {fork} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {DatabaseSync} from 'node:sqlite';
import {BrowserUploadLedger} from '../browser-upload-ledger.mjs';
import {uploadIdentityFromStat,uploadNativeIdentity} from '../browser-upload-ledger-values.mjs';
import {DispatchJournal} from '../journal.mjs';

const binding={actorId:'actor',workspaceId:'workspace',runId:'run-'+'1'.repeat(32),inputId:'input-owned',sessionId:'2'.repeat(32)};
const content=Buffer.from('QQQQ'),grant={id:'3'.repeat(32),name:'selected.bin',size:4,sha256:createHash('sha256').update(content).digest('hex')};
const action={actionId:binding.runId+':selected',documentId:'11111111-1111-4111-8111-111111111111'};
const fixturePath=fileURLToPath(new URL('./fixtures/browser-upload-ledger-child.mjs',import.meta.url));
async function fixture(run,options={}){
  const directory=await mkdtemp(join(tmpdir(),'offgrid-upload-ledger-'));assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
  const handles=[];const ledger=async()=>{const value=await BrowserUploadLedger.open({directory,maxRetainedBytes:8,maxActiveRecords:4,...options});handles.push(value);return value;};
  try{await run({directory,ledger});}finally{for(const handle of handles)handle.close();await rm(directory,{recursive:true,force:true});}
}
async function ready(ledger,directory){
  const record=ledger.reserve({binding,grant}),path=join(directory,record.relativeDirectory);await mkdir(path,{recursive:true});
  ledger.recordDirectory(record.resourceId,uploadIdentityFromStat(await lstat(path,{bigint:true})));
  const file=join(path,grant.name);await writeFile(file,content);
  ledger.recordFile(record.resourceId,uploadIdentityFromStat(await lstat(file,{bigint:true})));
  return ledger.ready(record.resourceId,{size:grant.size,sha256:grant.sha256});
}
function child(directory,phase,options={}){
  const process=fork(fixturePath,[],{windowsHide:true,stdio:['ignore','ignore','ignore','ipc']});
  const result=new Promise((resolve,reject)=>{
    const timer=setTimeout(()=>{process.kill();reject(Error('fixture child timed out'));},10000);
    process.once('message',value=>{clearTimeout(timer);resolve(value);});process.once('error',reject);
  });
  const exited=new Promise(resolve=>process.once('exit',(code,signal)=>resolve({code,signal})));
  process.send({directory,phase,...options});
  return {process,result,exited};
}

test('ledger preserves dispatch tombstones and survives manual session journal closure',async()=>{
  await fixture(async({directory,ledger})=>{
    const journal=new DispatchJournal(join(directory,'dispatch.sqlite'));
    const command={id:action.actionId,kind:'browser_upload',arguments:{element:'1'}};
    journal.prepare(command,'bound-session');
    const owner=await ledger(),record=await ready(owner,directory);owner.inUse(record.resourceId,action);
    journal.forgetResults('bound-session');journal.close();
    const reopened=await ledger();assert.equal(reopened.listRetained().records[0].recoveryState,'recovery_uncertain');
    assert.throws(()=>reopened.beginCleanup(record.resourceId),/owner_mismatch/);
    assert.throws(()=>owner.abortPreparation(record.resourceId),/transition/);
    assert.throws(()=>owner.beginCleanup(record.resourceId),/transition/);
    owner.browserClosed();owner.beginCleanup(record.resourceId);owner.removed(record.resourceId);
    assert.deepEqual(owner.listRetained().records,[]);
    const retry=new DispatchJournal(join(directory,'dispatch.sqlite'));
    try{assert.throws(()=>retry.prepare(command,'bound-session'),/action_uncertain/);}finally{retry.close();}
    const db=new DatabaseSync(join(directory,'dispatch.sqlite'));try{assert.equal(db.prepare('PRAGMA user_version').get().user_version,2);}finally{db.close();}
  });
});

test('ledger-first opening preserves legacy dispatch migration and permanent tombstones',async()=>{
  await fixture(async({directory,ledger})=>{
    const path=join(directory,'dispatch.sqlite'),old=new DatabaseSync(path);
    old.exec("CREATE TABLE dispatch(id TEXT PRIMARY KEY,hash TEXT NOT NULL,status TEXT NOT NULL); INSERT INTO dispatch VALUES('old:call','legacy-hash','completed')");old.close();
    const owner=await ledger(),record=owner.reserve({binding,grant});
    const journal=new DispatchJournal(path);
    try{
      assert.throws(()=>journal.prepare({id:'old:call',kind:'browser_upload',arguments:{}},'new-scope'),/action_uncertain/);
      assert.equal(owner.listRetained().records[0].resourceId,record.resourceId);
      owner.abortPreparation(record.resourceId);owner.beginCleanup(record.resourceId);owner.removed(record.resourceId);
    }finally{journal.close();}
    const inspect=new DatabaseSync(path);
    try{assert.equal(inspect.prepare("SELECT hash FROM dispatch WHERE id='old:call'").get().hash,'legacy-hash');assert.equal(inspect.prepare('PRAGMA user_version').get().user_version,2);}finally{inspect.close();}
  });
});

test('preparation abort permits cleanup but can never clear selected-possible state',async()=>{
  await fixture(async({directory,ledger})=>{
    const owner=await ledger(),record=await ready(owner,directory);
    owner.abortPreparation(record.resourceId);owner.beginCleanup(record.resourceId);owner.removed(record.resourceId);
    const another=await ready(owner,directory);owner.inUse(another.resourceId,action);
    assert.throws(()=>owner.abortPreparation(another.resourceId),/transition/);
    assert.throws(()=>owner.inUse(another.resourceId,{actionId:'run-'+'9'.repeat(32)+':foreign'}),/invalid/);
    owner.browserClosed();assert.throws(()=>owner.reserve({binding,grant}),/owner_closed/);
  });
});

test('concurrent ledger handles share capacity and cannot raise persisted policy',async()=>{
  await fixture(async({ledger,directory})=>{
    const first=await ledger(),second=await ledger();
    first.reserve({binding,grant});second.reserve({binding,grant});
    assert.throws(()=>first.reserve({binding,grant}),/capacity/);
    await assert.rejects(BrowserUploadLedger.open({directory,maxRetainedBytes:16,maxActiveRecords:4}),/policy_changed/);
  });
});

test('authority writes use verified DELETE plus EXTRA on the actual ledger connection',async()=>{
  await fixture(async({directory,ledger})=>{
    const owner=await ledger(),db=new DatabaseSync(join(directory,'dispatch.sqlite'));
    try{
      db.exec('CREATE TABLE fixture_durability(sync INTEGER,mode TEXT) STRICT; CREATE TRIGGER record_durability AFTER INSERT ON browser_upload_resources BEGIN INSERT INTO fixture_durability SELECT synchronous,journal_mode FROM pragma_synchronous,pragma_journal_mode; END');
      owner.reserve({binding,grant});
      assert.deepEqual({...db.prepare('SELECT * FROM fixture_durability').get()},{sync:3,mode:'delete'});
      db.exec('PRAGMA journal_mode=WAL');
      assert.throws(()=>owner.reserve({binding,grant}),/durability_unavailable/);
      assert.equal(owner.listRetained().records.length,1);
    }finally{db.close();}
  });
});

test('bounded records, read-only paging and detached metadata expose no cleanup authority',async()=>{
  await fixture(async({ledger})=>{
    const owner=await ledger(),request={binding:{...binding},grant:{...grant,size:0}};
    const first=owner.reserve(request);request.binding.runId='changed';request.grant.name='changed';
    owner.reserve({binding,grant:{...grant,size:0}});
    assert.throws(()=>owner.reserve({binding,grant:{...grant,size:0}}),/capacity/);
    const page=owner.listRetained({limit:1});assert.equal(page.records.length,1);assert.ok(page.next);
    const last=owner.listRetained({after:page.next,limit:1});assert.equal(last.records.length,1);assert.equal(last.next,null);
    assert.equal(first.binding.runId,binding.runId);assert.equal(first.grant.name,grant.name);
    assert.throws(()=>{first.binding.runId='mutated';},TypeError);assert.throws(()=>page.records.push(first),TypeError);
    assert.equal(owner.db,undefined);assert.equal('owner' in first,false);assert.equal('path' in first,false);
    assert.throws(()=>owner.listRetained({limit:65}),/invalid/);
  },{maxActiveRecords:2});
});

test('scope, metadata and native identity validation reject paths, extra fields and lossy IDs',async()=>{
  await fixture(async({ledger})=>{
    const owner=await ledger();
    for(const scope of [{...binding,runId:'https://secret.example/'},{...binding,inputId:'input-../../file'},{...binding,sessionId:'wrong'},{...binding,token:'secret'}])assert.throws(()=>owner.reserve({binding:scope,grant}),/invalid/);
    assert.throws(()=>owner.reserve({binding,grant:{...grant,path:'C:\\private'}}),/invalid/);
    assert.throws(()=>uploadIdentityFromStat({dev:1,ino:2,birthtimeNs:3}),/invalid/);
    assert.throws(()=>uploadNativeIdentity({platform:process.platform,device:'1',inode:'0',birthtimeNs:'3'}),/invalid/);
    assert.equal(uploadIdentityFromStat({dev:1n,ino:9007199254740993n,birthtimeNs:3n}).inode,'9007199254740993');
    assert.deepEqual(owner.listRetained().records,[]);
  });
});

test('future schema, missing version metadata and corrupted phase fail closed',async()=>{
  for(const kind of ['future','missing','corrupt'])await fixture(async({directory,ledger})=>{
    const owner=await ledger();owner.reserve({binding,grant});owner.close();
    const db=new DatabaseSync(join(directory,'dispatch.sqlite'));
    if(kind==='future')db.exec('UPDATE browser_upload_schema SET version=2');
    if(kind==='missing')db.exec('DELETE FROM browser_upload_schema');
    if(kind==='corrupt')db.exec("PRAGMA ignore_check_constraints=ON; UPDATE browser_upload_resources SET state='unknown'");
    db.close();await assert.rejects(ledger(),/incompatible|corrupt/);
  });
});

test('ignored SQLite transitions never acknowledge durable identity, selection or cleanup',async()=>{
  for(const operation of ['directory','file','ready','inUse','abort','cleanup','removed'])await fixture(async({directory,ledger})=>{
    const owner=await ledger(),record=owner.reserve({binding,grant}),id=record.resourceId;
    const identity=uploadIdentityFromStat(await lstat(directory,{bigint:true}));
    if(operation!=='directory')owner.recordDirectory(id,identity);
    if(!['directory','file'].includes(operation))owner.recordFile(id,identity);
    if(!['directory','file','ready'].includes(operation))owner.ready(id,{size:grant.size,sha256:grant.sha256});
    if(['cleanup','removed'].includes(operation))owner.abortPreparation(id);
    if(operation==='removed')owner.beginCleanup(id);
    const before=owner.listRetained().records[0],db=new DatabaseSync(join(directory,'dispatch.sqlite'));
    try{
      db.exec('CREATE TRIGGER ignore_upload_write BEFORE UPDATE ON browser_upload_resources BEGIN SELECT RAISE(IGNORE); END');
      let assignments=0;
      const invoke=()=>{
        if(operation==='directory')owner.recordDirectory(id,identity);
        if(operation==='file')owner.recordFile(id,identity);
        if(operation==='ready')owner.ready(id,{size:grant.size,sha256:grant.sha256});
        if(operation==='inUse'){owner.inUse(id,action);assignments++;}
        if(operation==='abort')owner.abortPreparation(id);
        if(operation==='cleanup')owner.beginCleanup(id);
        if(operation==='removed')owner.removed(id);
      };
      assert.throws(invoke,/write_unconfirmed/);assert.equal(assignments,0);
      assert.deepEqual(owner.listRetained().records[0],before);
    }finally{db.close();}
  });
});

test('ignored or altered reservation inserts cannot acknowledge uncharged bytes',async()=>{
  for(const altered of [false,true])await fixture(async({directory,ledger})=>{
    const owner=await ledger(),db=new DatabaseSync(join(directory,'dispatch.sqlite'));
    try{
      db.exec(altered?'CREATE TRIGGER alter_reservation AFTER INSERT ON browser_upload_resources BEGIN UPDATE browser_upload_resources SET size=0 WHERE id=NEW.id; END':'CREATE TRIGGER ignore_reservation BEFORE INSERT ON browser_upload_resources BEGIN SELECT RAISE(IGNORE); END');
      assert.throws(()=>owner.reserve({binding,grant}),/write_unconfirmed/);
      assert.deepEqual(owner.listRetained().records,[]);
    }finally{db.close();}
  });
});

test('partial browser-closed update and trigger-altered metadata roll back atomically',async()=>{
  await fixture(async({directory,ledger})=>{
    const owner=await ledger(),first=await ready(owner,directory),second=await ready(owner,directory);
    owner.inUse(first.resourceId,action);owner.inUse(second.resourceId,action);
    const db=new DatabaseSync(join(directory,'dispatch.sqlite'));
    try{
      db.exec(`CREATE TRIGGER ignore_one_close BEFORE UPDATE OF browser_closed ON browser_upload_resources WHEN OLD.id='${first.resourceId}' BEGIN SELECT RAISE(IGNORE); END`);
      assert.throws(()=>owner.browserClosed(),/write_unconfirmed/);
      assert.ok(owner.listRetained().records.every(row=>row.browserClosed===false));
      assert.throws(()=>owner.beginCleanup(first.resourceId),/transition/);
      db.exec('DROP TRIGGER ignore_one_close');
      db.exec(`CREATE TRIGGER alter_upload_digest AFTER UPDATE OF action_id ON browser_upload_resources BEGIN UPDATE browser_upload_resources SET sha256='${'0'.repeat(64)}' WHERE id=NEW.id; END`);
      assert.throws(()=>owner.inUse(first.resourceId,{actionId:binding.runId+':next'}),/write_unconfirmed/);
      assert.ok(owner.listRetained().records.every(row=>row.grant.sha256===grant.sha256));
      db.exec('DROP TRIGGER alter_upload_digest');owner.browserClosed();
      assert.ok(owner.listRetained().records.every(row=>row.browserClosed===true));
    }finally{db.close();}
  });
});

test('actual child crashes preserve each phase, lost receipts and aggregate reservations',async()=>{
  for(const phase of ['reserved','directory_before_identity','directory','file_before_identity','partial_copy','ready','in_use','browser_closed','deleting','deleted_before_receipt','removed'])await fixture(async({directory,ledger})=>{
    const sentinel=join(directory,'unrelated.txt');await writeFile(sentinel,'preserve unrelated bytes');
    const spawned=child(directory,phase,{maxBytes:4,maxActive:2});
    try{
      const result=await spawned.result;assert.equal(result.ready,true,JSON.stringify(result));
      spawned.process.kill('SIGKILL');const exit=await spawned.exited;assert.equal(exit.signal,'SIGKILL');
      const reopened=await ledger(),records=reopened.listRetained().records;
      if(phase==='removed'){assert.equal(records.length,0);reopened.reserve({binding,grant});}
      else{
        assert.equal(records.length,1);assert.equal(records[0].resourceId,result.resourceId);assert.equal(records[0].recoveryState,'recovery_uncertain');
        assert.throws(()=>reopened.beginCleanup(result.resourceId),/owner_mismatch/);
        assert.throws(()=>reopened.reserve({binding,grant}),/capacity/);
      }
      assert.equal(await readFile(sentinel,'utf8'),'preserve unrelated bytes');
    }finally{if(spawned.process.exitCode===null&&spawned.process.signalCode===null)spawned.process.kill('SIGKILL');await spawned.exited;}
  },{maxRetainedBytes:4,maxActiveRecords:2});
});

test('two actual child processes cannot both exceed one aggregate reservation',async()=>{
  await fixture(async({directory,ledger})=>{
    // Initialize the additive schema before the concurrent ownership attempt.
    const initial=await ledger();initial.close();
    const children=[child(directory,'reserved',{maxBytes:4,maxActive:2}),child(directory,'reserved',{maxBytes:4,maxActive:2})];
    try{
      const results=await Promise.all(children.map(value=>value.result));
      assert.equal(results.filter(value=>value.ready).length,1);assert.equal(results.filter(value=>value.error==='upload_ledger_capacity').length,1);
    }finally{for(const value of children)if(value.process.exitCode===null&&value.process.signalCode===null)value.process.kill('SIGKILL');await Promise.all(children.map(value=>value.exited));}
  },{maxRetainedBytes:4,maxActiveRecords:2});
});
