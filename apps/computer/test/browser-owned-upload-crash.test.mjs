import test from 'node:test';
import assert from 'node:assert/strict';
import {fork} from 'node:child_process';
import {once} from 'node:events';
import {mkdtemp,writeFile,readFile,rm,lstat} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {BrowserUploadLedger} from '../browser-upload-ledger.mjs';

test('killed snapshot owners retain real bytes, identities and reservations without adoption',async()=>{
 for(const phase of ['directory','file','ready','in_use','browser_closed','removed']){
  const root=await mkdtemp(join(tmpdir(),'offgrid-upload-owner-crash-')),directory=join(root,'private'),path=join(root,'source.txt');
  const content=Buffer.from('approved durable snapshot crash fixture');await writeFile(path,content);
  const binding={actorId:'local-admin',workspaceId:'workspace',runId:'run-'+'a'.repeat(32),inputId:'input-initial',sessionId:'b'.repeat(32)};
  const grant={id:'c'.repeat(32),path,name:'source.txt',size:content.length,sha256:createHash('sha256').update(content).digest('hex')};
  let child,ledger;
  try{
   child=fork(new URL('./fixtures/browser-owned-upload-child.mjs',import.meta.url),[],{env:{...process.env,OFFGRID_UPLOAD_CHILD:'1',OFFGRID_UPLOAD_FIXTURE:JSON.stringify({directory,binding,grant,phase})},stdio:['ignore','ignore','pipe','ipc']});
   let stderr='';child.stderr.on('data',data=>{stderr+=data;});
   const terminated=once(child,'exit');
   const boundary=await Promise.race([once(child,'message',{signal:AbortSignal.timeout(10000)}),terminated.then(()=>{throw Error('child ended before checkpoint: '+stderr);})]);
   assert.equal(boundary[0].phase,phase);assert.equal(child.kill('SIGKILL'),true);await terminated;
   ledger=await BrowserUploadLedger.open({directory,maxRetainedBytes:content.length});
   const records=ledger.listRetained().records;
   if(phase==='removed'){assert.deepEqual(records,[]);continue;}
   assert.equal(records.length,1);const row=records[0];assert.equal(row.recoveryState,'recovery_uncertain');assert.equal(row.currentOwner,false);
   assert.throws(()=>ledger.beginCleanup(row.resourceId),/owner_mismatch/);
   assert.throws(()=>ledger.reserve({binding,grant:{id:'d'.repeat(32),name:grant.name,size:grant.size,sha256:grant.sha256}}),/capacity/);
   const file=join(directory,row.relativeDirectory,grant.name);
   if(phase==='directory')await assert.rejects(lstat(file),{code:'ENOENT'});
   else if(phase==='file')assert.equal((await lstat(file)).size,0);
   else assert.deepEqual(await readFile(file),content);
   assert.equal(row.browserClosed,phase==='browser_closed');
  }finally{
   ledger?.close();if(child&&child.exitCode===null&&child.signalCode===null){const ended=once(child,'exit');child.kill('SIGKILL');await ended;}
   await rm(root,{recursive:true});
  }
 }
});
