import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash,randomUUID} from 'node:crypto';
import {mkdtemp,writeFile,readFile,readdir,rm,rename,mkdir} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {dirname,join,resolve} from 'node:path';
import {BrowserFileSnapshot,browserFileLimit} from '../browser-file-snapshot.mjs';

const hash=value=>createHash('sha256').update(value).digest('hex');
async function fixture(run){
 const root=await mkdtemp(join(tmpdir(),'offgrid-file-snapshot-'));assert.equal(dirname(resolve(root)),resolve(tmpdir()));
 const content=Buffer.from('Approved exact bytes\n'),path=join(root,'source.txt'),staging=join(root,'private');
 await writeFile(path,content);await mkdir(staging);
 const grant={id:randomUUID().replaceAll('-',''),path,name:'source.txt',size:content.length,sha256:hash(content)};
 const snapshot=new BrowserFileSnapshot(staging);
 try{await run({root,staging,path,content,grant,snapshot});}
 finally{await snapshot.close();await rm(root,{recursive:true,force:true});}
}

test('snapshot preserves exact approved bytes after original source replacement and cleans only owned files',async()=>{
 await fixture(async({snapshot,grant,path,content,staging})=>{
  await snapshot.prepare(grant);assert.equal(snapshot.state,'ready');assert.notEqual(snapshot.path,path);
  await writeFile(path,'Different later source');assert.deepEqual(await readFile(snapshot.path),content);
  assert.deepEqual(snapshot.metadata,{id:grant.id,name:grant.name,size:content.length,sha256:hash(content)});
  await snapshot.close();assert.deepEqual(await readdir(staging),[]);assert.equal(await readFile(path,'utf8'),'Different later source');
 });
});

test('changed content, special source and oversized admission produce no ready snapshot',async()=>{
 for(const kind of ['digest','directory','size'])await fixture(async({snapshot,grant,root})=>{
  if(kind==='digest')grant.sha256='0'.repeat(64);
  if(kind==='directory')grant.path=root;
  if(kind==='size')grant.size=browserFileLimit+1;
  await assert.rejects(snapshot.prepare(grant));assert.equal(snapshot.state,'failed');assert.equal(snapshot.metadata,undefined);
 });
});

test('cancelled preparation has no file assignment or retained staging',async()=>{
 await fixture(async({snapshot,grant,staging})=>{
  const abort=new AbortController();abort.abort();
  await assert.rejects(snapshot.prepare(grant,{signal:abort.signal}),error=>error.name==='AbortError');
  await snapshot.close();assert.deepEqual(await readdir(staging),[]);
 });
});

test('Stop races preparation without publishing a ready snapshot or leaking its owner',async()=>{
 await fixture(async({snapshot,grant,staging})=>{
  const preparing=assert.rejects(snapshot.prepare(grant),error=>error.name==='AbortError');
  await snapshot.close();await preparing;assert.equal(snapshot.state,'closed');
  assert.equal(snapshot.metadata,undefined);assert.deepEqual(await readdir(staging),[]);
 });
});

test('grant names cannot redirect a snapshot into another path or alternate stream',async()=>{
 for(const name of ['../other.txt','source.txt:secret','..','NUL','COM1.txt'])await fixture(async({snapshot,grant,staging})=>{
  await assert.rejects(snapshot.prepare({...grant,name}));await snapshot.close();
  assert.deepEqual(await readdir(staging),[]);
 });
});

test('Windows stream and device names reject before opening the proposed source',{skip:process.platform!=='win32'},async()=>{
 for(const name of ['source.txt:secret','NUL','COM1.txt','CONIN$','LPT¹.log'])await fixture(async({snapshot,grant,root,staging})=>{
  await assert.rejects(snapshot.prepare({...grant,path:join(root,name),name}),/name is unsupported/);
  await snapshot.close();assert.deepEqual(await readdir(staging),[]);
 });
});

test('cleanup refuses a replaced object and retains ownership for an explicit retry',async()=>{
 await fixture(async({snapshot,grant,root})=>{
  await snapshot.prepare(grant);const owned=snapshot.path,saved=join(root,'retained-snapshot.txt');
  await rename(owned,saved);await writeFile(owned,'Unrelated replacement');
  await assert.rejects(snapshot.close(),/ownership changed/);assert.equal(snapshot.state,'cleanup_failed');
  assert.equal(await readFile(owned,'utf8'),'Unrelated replacement');
  await rm(owned);await rename(saved,owned);await snapshot.close();assert.equal(snapshot.state,'closed');
 });
});
