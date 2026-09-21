import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {DatabaseSync} from 'node:sqlite';
import {DispatchJournal} from '../journal.mjs';

const action = {id:'run:call',kind:'browser_fill',arguments:{observation:'opaque',element:'field',text:'private draft'}};
const reply = {id:action.id,result:'{"changed":true}'};
async function fixture(t) {
  const dir = await mkdtemp(join(tmpdir(),'offgrid-dispatch-'));
  t.after(()=>rm(dir,{recursive:true,force:true}));
  return join(dir,'dispatch.sqlite');
}

test('persisted result survives lost acknowledgement without repeating input',async t=>{
  const path = await fixture(t);
  let journal = new DispatchJournal(path);
  assert.equal(journal.prepare(action,'workspace:session').execute,true);
  journal.complete(action.id,reply);
  journal.close();
  journal = new DispatchJournal(path);
  try {
    assert.deepEqual(journal.prepare(action,'workspace:session'),{execute:false,reply});
    const reordered = {...action,arguments:{text:'private draft',element:'field',observation:'opaque'}};
    assert.deepEqual(journal.prepare(reordered,'workspace:session'),{execute:false,reply});
    assert.throws(()=>journal.prepare({...action,arguments:{...action.arguments,text:'changed'}},'workspace:session'),/action_conflict/);
    assert.throws(()=>journal.prepare(action,'other-workspace:session'),/action_conflict/);
    assert.throws(()=>journal.complete(action.id,reply),/action_conflict/);
    journal.forgetResults('workspace:session');
    assert.throws(()=>journal.prepare(action,'workspace:session'),/action_uncertain/);
  } finally { journal.close(); }
});

test('crash after intent and failed result persistence remain uncertain',async t=>{
  const path = await fixture(t);
  let journal = new DispatchJournal(path);
  assert.equal(journal.prepare(action,'scope').execute,true);
  journal.close();
  journal = new DispatchJournal(path);
  try {
    assert.throws(()=>journal.prepare(action,'scope'),/action_uncertain/);
    assert.throws(()=>journal.complete(action.id,{...reply,result:'x'.repeat(48*1024+1)}),/invalid_result/);
    assert.throws(()=>journal.prepare(action,'scope'),/action_uncertain/);
  } finally { journal.close(); }
});

test('concurrent journal handles cannot both claim an action',async t=>{
  const path = await fixture(t), first = new DispatchJournal(path), second = new DispatchJournal(path);
  try {
    assert.equal(first.prepare(action,'scope').execute,true);
    assert.throws(()=>second.prepare(action,'scope'),/action_uncertain/);
    first.complete(action.id,reply);
    assert.deepEqual(second.prepare(action,'scope'),{execute:false,reply});
  } finally { first.close(); second.close(); }
});

test('legacy tombstones are preserved, never treated as executable',async t=>{
  const path = await fixture(t), old = new DatabaseSync(path);
  old.exec('CREATE TABLE dispatch(id TEXT PRIMARY KEY,hash TEXT NOT NULL,status TEXT NOT NULL)');
  old.prepare('INSERT INTO dispatch VALUES(?,?,?)').run(action.id,'legacy-hash','completed');
  old.close();
  const journal = new DispatchJournal(path);
  try { assert.throws(()=>journal.prepare(action,'scope'),/action_uncertain/); }
  finally { journal.close(); }
  const db = new DatabaseSync(path);
  try { assert.equal(db.prepare('SELECT hash FROM dispatch WHERE id=?').get(action.id).hash,'legacy-hash'); }
  finally { db.close(); }
});

test('newer journal schema is rejected without downgrading',async t=>{
  const path = await fixture(t), db = new DatabaseSync(path);
  db.exec('PRAGMA user_version=99');db.close();
  assert.throws(()=>new DispatchJournal(path),/journal_incompatible/);
});
