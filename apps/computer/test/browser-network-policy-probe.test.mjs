import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {chromium} from 'playwright';
import {BrowserDriver} from '../browser.mjs';

// Investigation only. No product imports or network-policy changes. Assertions
// are against independent server effects, never just page-reported failure.
test('page-scoped native URLPattern denial and target-coverage diagnostic',{
 skip:process.env.OFFGRID_BROWSER_NETWORK_PROBE!=='1',timeout:30000,
},async t=>{
 const effects=[];let browser,server;
 try{
  server=createServer((req,res)=>{
   effects.push({host:req.headers.host,path:req.url,method:req.method});
   if(req.url.startsWith('/redirect/')){res.writeHead(307,{location:decodeURIComponent(req.url.slice(10))});res.end();return;}
   if(req.url.startsWith('/worker.js')){
    res.setHeader('content-type','application/javascript');
    res.end('onmessage=async e=>{let ok=false;try{ok=(await fetch(e.data,{method:"POST",body:"worker-effect",mode:"no-cors"})).ok}catch{};postMessage(ok)}');return;
   }
   res.setHeader('access-control-allow-origin','*');
   res.setHeader('content-type','text/html');res.end('<title>native policy probe</title>');
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const port=server.address().port,origin=`http://127.0.0.1:${port}`,foreign=`http://localhost:${port}`;
  browser=await chromium.launch({headless:true});
  const context=await browser.newContext({serviceWorkers:'block'}),page=await context.newPage();
  // Prove the supposedly forbidden endpoint is reachable in this disposable
  // fixture before applying policy; DNS failure is not a denial oracle.
  await page.goto(foreign+'/reachable-control');
  assert.ok(effects.some(row=>row.host===`localhost:${port}`&&row.path==='/reachable-control'));
  const session=await context.newCDPSession(page);
  await session.send('Network.enable');
  await session.send('Network.setBlockedURLs',{urlPatterns:[{urlPattern:`http://:@127.0.0.1:${port}/*`,block:false}],urls:['*']});
  await page.goto(origin);
  effects.length=0;
  await t.test('ordinary allowed request reaches receiver',async()=>{
   assert.equal(await page.evaluate(async()=> (await fetch('/allowed',{method:'POST',body:'allowed'})).ok),true);
   assert.deepEqual(effects.splice(0),[{host:`127.0.0.1:${port}`,path:'/allowed',method:'POST'}]);
  });
  await t.test('foreign host produces zero effects',async()=>{
   await page.evaluate(async foreign=>{try{await fetch(foreign+'/foreign',{method:'POST',body:'effect',mode:'no-cors'})}catch{}},foreign);
   assert.deepEqual(effects.splice(0),[]);
  });
  await t.test('credential-bearing native navigation produces zero effects',async()=>{
   await page.goto(`http://user:secret@127.0.0.1:${port}/credential`,{timeout:3000}).catch(()=>{});
   const observed=effects.splice(0);
   await page.goto(origin);effects.length=0;
   assert.deepEqual(observed,[]);
  });
  await t.test('307 redirect cannot send a denied destination body',async()=>{
   const target=foreign+'/redirect-effect';
   await page.evaluate(async target=>{try{await fetch('/redirect/'+encodeURIComponent(target),{method:'POST',body:'effect',mode:'no-cors'})}catch{}},target);
   const observed=effects.splice(0);
   assert.equal(observed.filter(row=>row.path==='/redirect-effect').length,0,JSON.stringify(observed));
   assert.equal(observed.length,1);
  });
  await t.test('page policy covers a dedicated worker before its first effect',async()=>{
   await page.evaluate(async foreign=>new Promise((resolve,reject)=>{
    const worker=new Worker('/worker.js');
    const timer=setTimeout(()=>{worker.terminate();reject(Error('worker_probe_timeout'));},3000);
    worker.onmessage=()=>{clearTimeout(timer);worker.terminate();resolve()};worker.onerror=()=>{clearTimeout(timer);worker.terminate();resolve()};
    worker.postMessage(foreign+'/worker-effect');
   }),foreign);
   const observed=effects.splice(0);
   assert.equal(observed.filter(row=>row.path==='/worker-effect').length,0,JSON.stringify(observed));
  });
  await t.test('new popup cannot send a first request outside the page policy',async()=>{
   const popup=page.waitForEvent('popup');
   await page.evaluate(url=>window.open(url),foreign+'/popup-effect');
   const opened=await popup;await opened.waitForLoadState('load',{timeout:3000}).catch(()=>{});
   const observed=effects.splice(0);await opened.close();
   assert.equal(observed.filter(row=>row.path==='/popup-effect').length,0,JSON.stringify(observed));
  });
  await t.test('selective native Fetch credential pattern blocks before HTTP effects',async()=>{
   let paused=0;
   const handler=event=>{paused++;void session.send('Fetch.failRequest',{requestId:event.requestId,errorReason:'BlockedByClient'}).catch(()=>{});};
   session.on('Fetch.requestPaused',handler);
   await session.send('Fetch.enable',{patterns:[{urlPattern:'*://*@*',requestStage:'Request'}]});
   try{
    await page.goto(`http://user:secret@127.0.0.1:${port}/selective-credential`,{timeout:3000}).catch(()=>{});
    const observed=effects.splice(0);t.diagnostic(`Selective credential pattern paused ${paused} request(s)`);
    assert.deepEqual(observed,[]);
    assert.equal(paused,1);
   }finally{await session.send('Fetch.disable');session.off('Fetch.requestPaused',handler);}
  });
  await t.test('current BrowserDriver route rejects the same credential-bearing navigation',async()=>{
   await browser.close();browser=null;
   const driver=await BrowserDriver.open(origin,{headless:true,testLoopback:true});
   try{
    effects.length=0;
    await driver.page.goto(`http://user:secret@127.0.0.1:${port}/existing-driver-credential`,{timeout:3000}).catch(()=>{});
    assert.deepEqual(effects.splice(0),[]);
   }finally{await driver.close();}
  });
 }finally{
  try{await browser?.close();}
  finally{if(server){server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}}
 }
});
