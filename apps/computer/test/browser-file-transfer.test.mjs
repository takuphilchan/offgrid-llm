import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash,randomUUID} from 'node:crypto';
import {createServer} from 'node:http';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve,dirname} from 'node:path';
import {BrowserDriver} from '../browser.mjs';

const digest=value=>createHash('sha256').update(value).digest('hex');
async function fixture(run,{size}={}){
 const directory=await mkdtemp(join(tmpdir(),'offgrid-browser-transfer-'));
 assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
 const approved=size?Buffer.alloc(size,0x51):Buffer.from('USER APPROVED CONTENT'),replacement=Buffer.alloc(approved.length,0x58);
 assert.equal(approved.length,replacement.length);
 const path=join(directory,'selected.txt');await writeFile(path,approved);
 const receipts=[];
 const server=createServer(async(req,res)=>{
  if(req.url==='/received'){
   const hash=createHash('sha256');let size=0;for await(const chunk of req){size+=chunk.length;hash.update(chunk);}
   receipts.push({size,sha256:hash.digest('hex')});res.end('received');return;
  }
  res.setHeader('Content-Type','text/html');
  res.end('<input type="file" aria-label="Attach report"><script>const input=document.querySelector("input");input.addEventListener("change",()=>{const file=input.files[0];window.readSelected=async()=>fetch("/received",{method:"POST",body:await file.arrayBuffer()});if(!window.deferRead)void window.readSelected();});</script>');
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
 const origin=`http://127.0.0.1:${server.address().port}`;
 const grant={id:randomUUID().replaceAll('-',''),path,name:'selected.txt',size:approved.length,sha256:digest(approved)};
 try{
  driver=await BrowserDriver.open(origin,{headless:true,testLoopback:true,upload:grant,downloadDir:join(directory,'downloads')});
  await run({driver,path,approved,replacement,receipts,origin});
 }finally{
  await driver?.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));
  await rm(directory,{recursive:true,force:true});
 }
}

test('upload consumes approved snapshot bytes when the selected source changes immediately before assignment',async()=>{
 await fixture(async({driver,path,approved,replacement,receipts,origin})=>{
  const view=await driver.observe(),element=view.elements.find(item=>item.label==='Attach report');
  assert.ok(element);const handle=driver.elements.get(element.id),assign=handle.setInputFiles.bind(handle);
  let assignments=0;
  handle.setInputFiles=async(...args)=>{assignments++;await writeFile(path,replacement);return assign(...args);};
  const received=driver.page.waitForResponse(response=>response.url()===origin+'/received');
  const result=await driver.execute('browser_upload',{observation_id:view.observation_id,element:element.id});
  await received;assert.equal(assignments,1);
  assert.deepEqual(receipts,[{size:approved.length,sha256:digest(approved)}],'the independent recipient must receive exactly the consented bytes');
  assert.equal(result.verified,true);assert.equal(result.file.sha256,digest(approved));
  assert.equal(result.check,'file_input_matches_grant');assert.equal(result.verification_scope,'file_input_selection');
  assert.match(result.message,/Remote receipt has not been verified/);
 });
});

test('changed upload source is rejected before assigning a file or triggering remote effects',async()=>{
 await fixture(async({driver,path,replacement,receipts})=>{
  const view=await driver.observe(),element=view.elements.find(item=>item.label==='Attach report');
  assert.ok(element);await writeFile(path,replacement);
  const handle=driver.elements.get(element.id);let assignments=0;
  handle.setInputFiles=async()=>{assignments++;throw Error('unexpected assignment');};
  await assert.rejects(driver.execute('browser_upload',{observation_id:view.observation_id,element:element.id}),error=>error.code==='computer_browser_precondition_failed');
  assert.equal(assignments,0);assert.deepEqual(receipts,[]);
 });
});

test('upload cannot verify a same-name same-size file substituted by the page after assignment',async()=>{
 await fixture(async({driver,approved,receipts})=>{
  await driver.page.evaluate(size=>{
   window.deferRead=true;
   document.querySelector('input').addEventListener('change',event=>{
    const replacement=new DataTransfer();
    replacement.items.add(new File([new Uint8Array(size).fill(0x58)],'selected.txt'));
    event.target.files=replacement.files;
   });
  },approved.length);
  const view=await driver.observe(),element=view.elements.find(item=>item.label==='Attach report');assert.ok(element);
  const handle=driver.elements.get(element.id),assign=handle.setInputFiles.bind(handle);let assignments=0;
  handle.setInputFiles=async(...args)=>{assignments++;return assign(...args);};
  await assert.rejects(driver.execute('browser_upload',{observation_id:view.observation_id,element:element.id}),error=>{
   assert.equal(error.code,undefined,'a failure after assignment must remain uncertain, not safe to replay');
   return /content differs/.test(error.message);
  });
  assert.equal(assignments,1);assert.deepEqual(receipts,[]);
 });
});

test('upload retains a file above the Playwright buffer limit for a later independent browser read',async()=>{
 await fixture(async({driver,approved,receipts,origin,path,replacement})=>{
  await driver.page.evaluate(()=>{window.deferRead=true;});
  const view=await driver.observe(),element=view.elements.find(item=>item.label==='Attach report');assert.ok(element);
  const result=await driver.execute('browser_upload',{observation_id:view.observation_id,element:element.id});
  assert.equal(result.file.size,approved.length);assert.deepEqual(receipts,[]);
  await writeFile(path,replacement);
  const received=driver.page.waitForResponse(response=>response.url()===origin+'/received');
  await driver.page.evaluate(()=>window.readSelected());await received;
  assert.deepEqual(receipts,[{size:approved.length,sha256:digest(approved)}]);
 },{size:51*1024*1024});
});
