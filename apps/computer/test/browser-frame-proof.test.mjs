import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {chromium} from 'playwright';
import {BrowserDriver} from '../browser.mjs';
import {registerFrameProof,acquireBrowserFrameProof} from '../browser-frame-proof.mjs';

await registerFrameProof();
async function fixture(run){
 const server=createServer((req,res)=>{
  if(req.url==='/received'){res.end('received');return;}
  res.setHeader('Content-Type','text/html');
  res.end(req.url==='/child'?'<button onclick="fetch(\'/received\',{method:\'POST\'})">Send</button>':'<iframe src="/child"></iframe><iframe src="/child"></iframe><button>Root</button>');
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
 try{
  driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
  await driver.page.locator('iframe').nth(1).contentFrame().getByRole('button').waitFor();
  await run(driver);
 }finally{await driver?.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
}

test('same-URL inline siblings receive distinct native frame/document proofs matching actual requests',async()=>{
 await fixture(async driver=>{
  const proofs=[];
  try{
   for(const child of driver.page.frames().filter(frame=>frame!==driver.page.mainFrame())){
    const handle=await child.getByRole('button',{name:'Send'}).elementHandle();
    const proof=await acquireBrowserFrameProof(handle);proofs.push(proof);
    await proof.session.send('Network.enable');
    const sent=new Promise(resolve=>proof.session.on('Network.requestWillBeSent',event=>{if(event.request.url.endsWith('/received'))resolve(event);}));
    await handle.click();const request=await sent;
    assert.equal(proof.frameId,request.frameId);assert.ok(proof.loaderId);await proof.validate();
   }
   assert.equal(proofs.length,2);assert.notEqual(proofs[0].frameId,proofs[1].frameId);
  }finally{for(const proof of proofs)await proof.close();}
 });
});

test('proof does not mutate DOM or leave globals/listeners and cannot rebind after frame navigation',async()=>{
 await fixture(async driver=>{
  const child=driver.page.frames().find(frame=>frame!==driver.page.mainFrame());
  await child.evaluate(()=>{window.changed=0;new MutationObserver(items=>{window.changed+=items.length;}).observe(document,{subtree:true,childList:true,attributes:true});});
  const handle=await child.getByRole('button').elementHandle(),proof=await acquireBrowserFrameProof(handle);
  try{
   assert.equal(await child.evaluate(()=>window.changed),0);
   assert.deepEqual(await child.evaluate(()=>Object.getOwnPropertyNames(globalThis).filter(name=>name.startsWith('offgrid_frame_'))),[]);
   const isolated=await proof.session.send('Runtime.evaluate',{uniqueContextId:proof.uniqueContextId,expression:'Object.getOwnPropertyNames(globalThis).filter(name=>name.startsWith("offgrid_frame_"))',returnByValue:true});
   assert.deepEqual(isolated.result.value,[]);
   assert.equal(proof.session.listenerCount('Runtime.bindingCalled'),0);
   await child.goto(child.url());await assert.rejects(proof.validate(),/changed/);
  }finally{await proof.close();}
  assert.equal(proof.session.listenerCount('Runtime.executionContextCreated'),0);
  assert.equal(proof.session.listenerCount('Runtime.executionContextDestroyed'),0);
 });
});

test('detached exact targets and pre-aborted correlation never select a replacement',async()=>{
 await fixture(async driver=>{
  const handle=await driver.page.getByRole('button',{name:'Root'}).elementHandle();
  const abort=new AbortController();abort.abort();
  await assert.rejects(acquireBrowserFrameProof(handle,{signal:abort.signal}),error=>error.name==='AbortError');
  await handle.evaluate(element=>element.replaceWith(element.cloneNode(true)));
  await assert.rejects(acquireBrowserFrameProof(handle),/target changed/);
 });
});

test('separate Chromium frame targets retain exact identity and fail after process navigation',async()=>{
 const browser=await chromium.launch({headless:true,args:['--site-per-process']});
 try{
  const context=await browser.newContext();
  await context.route('**/*',route=>route.fulfill({contentType:'text/html',body:route.request().url().startsWith('https://root.example/')?'<iframe src="https://child.example.net/" name="child"></iframe>':'<button>Separate</button>'}));
  const page=await context.newPage();await page.goto('https://root.example/');
  const child=page.frame({name:'child'}),handle=await child.getByRole('button').elementHandle();
  const proof=await acquireBrowserFrameProof(handle);
  try{
   const native=(await proof.session.send('Page.getFrameTree')).frameTree.frame;
   assert.equal(native.url,'https://child.example.net/');assert.equal(proof.frameId,native.id);
   const navigated=page.waitForEvent('framenavigated',{predicate:frame=>frame===child&&frame.url()==='https://root.example/child'});
   await page.locator('iframe').first().evaluate(element=>{element.src='https://root.example/child';});await navigated;
   assert.equal(browser.isConnected(),true);assert.equal(page.isClosed(),false);
   await assert.rejects(proof.validate(),/changed/);
  }finally{await proof.close();}
 }finally{await browser.close();}
});

test('a default-world binding call cannot forge the retained child frame even with the test nonce',async()=>{
 await fixture(async driver=>{
  const context=driver.page.context(),create=context.newCDPSession.bind(context);let session,installed=false;
  const calls=[],contexts=new Map();
  context.newCDPSession=async target=>{
   const value=await create(target);session=value;const send=value.send.bind(value);
   value.on('Runtime.executionContextCreated',event=>contexts.set(event.context.id,event.context));
   value.on('Runtime.bindingCalled',event=>calls.push(event));
   value.send=async(method,params)=>{
    const result=await send(method,params);
    // Deliberately broaden only this test's native binding to the page world.
    // Production restricts it to native-observed isolated context names.
    if(method==='Runtime.addBinding'&&!installed){installed=true;await send(method,{name:params.name});}
    return result;
   };
   return value;
  };
  const child=driver.page.frames().find(frame=>frame!==driver.page.mainFrame());
  const handle=await child.getByRole('button').elementHandle(),query=handle.$.bind(handle);let bindingName;
  handle.$=async selector=>{
   const command=JSON.parse(selector.slice(selector.indexOf('=')+1));
   if(command.op==='identify'){
    bindingName=command.name;
    await driver.page.evaluate(({name,nonce})=>globalThis[name](nonce),command);
   }
   return query(selector);
  };
  let proof;
  try{
   proof=await acquireBrowserFrameProof(handle);
   assert.equal(calls.length,2);assert.equal(contexts.get(calls[0].executionContextId).auxData.isDefault,true);
   assert.equal(contexts.get(calls[1].executionContextId).auxData.isDefault,false);
   assert.equal(proof.frameId,contexts.get(calls[1].executionContextId).auxData.frameId);
   assert.notEqual(proof.frameId,contexts.get(calls[0].executionContextId).auxData.frameId);
  }finally{
   if(bindingName)for(const frame of driver.page.frames())await frame.evaluate(name=>{delete globalThis[name];},bindingName);
   await proof?.close();context.newCDPSession=create;
   session?.removeAllListeners('Runtime.bindingCalled');session?.removeAllListeners('Runtime.executionContextCreated');
  }
 });
});

test('document replacement while binding is installed cannot certify a replacement target',async()=>{
 await fixture(async driver=>{
  const child=driver.page.frames().find(frame=>frame!==driver.page.mainFrame());
  const handle=await child.getByRole('button').elementHandle(),query=handle.$.bind(handle);
  let replaced=false;
  handle.$=async selector=>{
   if(JSON.parse(selector.slice(selector.indexOf('=')+1)).op==='identify'){
    replaced=true;await child.goto(child.url());
   }
   return query(selector);
  };
  await assert.rejects(acquireBrowserFrameProof(handle));assert.equal(replaced,true);
  assert.equal(await child.getByRole('button').count(),1);
 });
});
