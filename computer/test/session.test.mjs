import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,writeFile,mkdir} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {CompanionSession,localService} from '../session.mjs';
import {createManifest,verifyPack} from '../pack.cjs';

test('companion transport rejects remote services and credentials',()=>{
 for(const url of ['https://example.com','http://localhost:11611','http://a:b@127.0.0.1:11611','http://127.0.0.1:11611/path']) assert.throws(()=>localService(url));
 assert.equal(localService('http://127.0.0.1:11611').hostname,'127.0.0.1');
});
test('stop while browser startup is pending closes the late resource and never pairs',async()=>{
 const directory=await mkdtemp(join(tmpdir(),'offgrid-session-test-'));let release,closed=0,paired=0;
 const wait=new Promise(resolve=>release=resolve);
 const session=new CompanionSession({service:'http://127.0.0.1:11611',directory,openBrowser:async()=>{await wait;return {close:async()=>{closed++;}};}});
 session.request=async(path)=>{if(path.includes('pair'))paired++;return {product:'offgrid',api_version:2};};
 const start=session.start({origin:'https://example.com',code:'a'.repeat(64)});
 await new Promise(resolve=>setImmediate(resolve));await session.stop();release();
 await assert.rejects(start,/stopped/);assert.equal(closed,1);assert.equal(paired,0);
 await rm(directory,{recursive:true});
});
test('journal rejects redelivery after a lost acknowledgement instead of repeating an effect',async()=>{
 const directory=await mkdtemp(join(tmpdir(),'offgrid-journal-test-'));let effects=0;
 const action={id:'immutable-action',kind:'browser_click',arguments:{element:'observed'},authorization:'exact'};
 try {
  for(let attempt=0;attempt<2;attempt++) {
   const session=new CompanionSession({service:'http://127.0.0.1:11611',directory,openBrowser:async()=>({close:async()=>{},prepare:async()=>({prepared:true,approval_class:'consequential'}),execute:async()=>{effects++;return {verified:false};}})});
   session.request=async(path)=>{
    if(path.endsWith('/system'))return {product:'offgrid',api_version:2};
    if(path.endsWith('/pair'))return {token:'test-only',session:{id:'session',origin:'https://example.com'}};
    if(path.endsWith('/poll'))return {action};
    throw Error('lost_reply');
   };
   await session.start({origin:'https://example.com',code:'a'.repeat(64)});await session.running;
   assert.equal(session.state,'error');assert.equal(session.token,'');
  }
  assert.equal(effects,1);
 }finally{await rm(directory,{recursive:true});}
});
test('runtime manifests reject tampering and wrong architecture',async()=>{
 const directory=await mkdtemp(join(tmpdir(),'offgrid-pack-test-'));
 try{
  await mkdir(join(directory,'browsers'));await writeFile(join(directory,'browsers','fixture'),'browser-fixture');await writeFile(join(directory,'managed.cjs'),'entry-fixture');
  await createManifest(directory,process.platform,process.arch,'1.62.1');await verifyPack(directory);
  await assert.rejects(verifyPack(directory,process.platform,'wrong'),/pack_invalid/);
  await writeFile(join(directory,'managed.cjs'),'tampered');await assert.rejects(verifyPack(directory),/pack_invalid/);
 }finally{await rm(directory,{recursive:true});}
});

test('same-session redelivery returns the persisted reply and never repeats input',async()=>{
 const directory=await mkdtemp(join(tmpdir(),'offgrid-replay-test-'));let effects=0,replies=0;
 const action={id:'run:call',kind:'browser_click',arguments:{element:'observed'},authorization:'exact'};
 let firstReply;
 const session=new CompanionSession({service:'http://127.0.0.1:11611',directory,openBrowser:async()=>({close:async()=>{},prepare:async()=>({prepared:true,approval_class:'consequential'}),execute:async()=>{effects++;return {changed:true};}})});
 session.request=async(path,body)=>{
  if(path.endsWith('/system'))return {product:'offgrid',api_version:2,workspace_id:'workspace'};
  if(path.endsWith('/pair'))return {token:'test-only',session:{id:'session',origin:'https://example.com'}};
  if(path.endsWith('/poll'))return {action};
  if(path.endsWith('/reply')) {
   replies++;
   if(replies===1)firstReply=body;
   else {assert.deepEqual(body,firstReply);await session.stop();}
   return {};
  }
 };
 try {
  await session.start({origin:'https://example.com',code:'b'.repeat(64)});await session.running;
  assert.equal(effects,1);assert.equal(replies,2);assert.equal(session.state,'stopped');
 }finally{await session.stop();await rm(directory,{recursive:true});}
});

test('browser companion enforces the immutable local approval mode before input',async()=>{
 for(const [mode,approvalClass,authorization,wantError,wantEffects] of [
  ['scoped_changes','consequential','auto','computer_approval_invalid',0],
  ['full_task','consequential','auto','',1],
  ['full_task','forbidden','exact','computer_prohibited_action',0],
  ['ask_every_time','invalid','exact','computer_uncertain_outcome',0],
 ]) {
  const directory=await mkdtemp(join(tmpdir(),'offgrid-policy-test-'));let effects=0,reply;
  const action={id:`run:${mode}:${approvalClass}`,kind:'browser_click',arguments:{observation_id:'observed',element:'control'},authorization};
  const session=new CompanionSession({service:'http://127.0.0.1:11611',directory,openBrowser:async()=>({
   close:async()=>{},prepare:async()=>({prepared:true,approval_class:approvalClass}),execute:async()=>{effects++;return {changed:true};}
  })});
  session.request=async(path,body)=>{
   if(path.endsWith('/system'))return {product:'offgrid',api_version:2,workspace_id:'workspace'};
   if(path.endsWith('/pair')){assert.equal(body.approval_mode,mode);return {token:'test-only',session:{id:'session',origin:'https://example.com'}};}
   if(path.endsWith('/poll'))return {action};
   if(path.endsWith('/reply')){reply=body;session.stopping=true;return {};}
   throw Error('unexpected request');
  };
  try {
   await session.start({origin:'https://example.com',code:'c'.repeat(64),workspace:'workspace',approvalMode:mode});
   await session.running;
   assert.equal(effects,wantEffects);
   assert.equal(reply?.error??'',wantError);
  } finally {await session.stop();await rm(directory,{recursive:true});}
 }
});
