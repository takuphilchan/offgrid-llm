import test from 'node:test';
import assert from 'node:assert/strict';
import {fork} from 'node:child_process';
import {once} from 'node:events';
import {mkdtemp,readFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {dirname,join,resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import {DispatchJournal} from '../journal.mjs';

const action={id:'run:crash',kind:'browser_click',arguments:{element:'fixture',observation_id:'fixture'}};
for(const phase of ['intent','effect','complete'])test(`actual child termination after ${phase} cannot make the recorded action executable`,async()=>{
 const directory=await mkdtemp(join(tmpdir(),'offgrid-journal-crash-'));
 assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
 let child,closed,timer;
 try{
  child=fork(fileURLToPath(new URL('./fixtures/dispatch-crash-child.mjs',import.meta.url)),[],{stdio:['ignore','ignore','pipe','ipc'],windowsHide:true});
  closed=once(child,'close');let errors='';child.stderr.on('data',data=>{errors=(errors+data.toString()).slice(-2048);});
  const ready=once(child,'message');
  const timeout=new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(`Crash fixture timed out: ${errors}`)),10000);});
  child.send({directory,phase});const [message]=await Promise.race([ready,timeout]);
  assert.deepEqual(message,{phase});clearTimeout(timer);
  assert.equal(child.kill('SIGKILL'),true);const [,signal]=await closed;assert.equal(signal,'SIGKILL');
  const journal=new DispatchJournal(join(directory,'dispatch.sqlite'));
  try{
   if(phase==='complete')assert.deepEqual(journal.prepare(action,'fixture-workspace:session'),{execute:false,reply:{id:action.id,result:'{"fixture_effect":true}'}});
   else assert.throws(()=>journal.prepare(action,'fixture-workspace:session'),/action_uncertain/);
  }finally{journal.close();}
  if(phase==='intent')await assert.rejects(readFile(join(directory,'independent-effect')),error=>error.code==='ENOENT');
  else assert.equal(await readFile(join(directory,'independent-effect'),'utf8'),'one effect');
 }finally{
  clearTimeout(timer);
  if(child&&child.exitCode===null&&child.signalCode===null){child.kill('SIGKILL');await closed;}
  await rm(directory,{recursive:true,force:true});
 }
});
