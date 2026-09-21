import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:https';
import {createServer as httpServer} from 'node:http';
import {readFileSync} from 'node:fs';
import {chromium} from 'playwright';
import {pinnedDestination, publicPage, startEgress} from '../network.mjs';

test('real page addresses retain paths, queries and fragments but never embedded credentials or local addresses', () => {
  assert.equal(publicPage('https://example.com/article?q=hello#part').href,'https://example.com/article?q=hello#part');
  for (const address of ['https://127.1','https://[::1]','https://router.local','https://localhost','file:///tmp/file','http://example.com','https://user:password@example.com']) assert.throws(() => publicPage(address));
});

test('direct DNS pinning and explicit trusted fake-DNS mode do not grant arbitrary private destinations', async () => {
  const resolve = values => async () => values.map(address=>({address}));
  assert.equal((await pinnedDestination('https://example.com','direct',resolve(['93.184.215.14']))).address,'93.184.215.14');
  for (const address of ['198.18.0.99','198.19.255.254']) {
    await assert.rejects(pinnedDestination('https://example.com','direct',resolve([address])),/network_blocked/);
    assert.equal((await pinnedDestination('https://example.com','trusted-vpn',resolve([address]))).address,address);
  }
  for (const mode of ['direct','trusted-vpn']) {
    for (const address of ['127.0.0.1','10.0.0.1','192.168.0.1','169.254.169.254','::1','198.51.100.1']) await assert.rejects(pinnedDestination('https://example.com',mode,resolve([address])),/network_blocked/);
    await assert.rejects(pinnedDestination('https://example.com',mode,resolve(['93.184.215.14','127.0.0.1'])),/network_blocked/);
    await assert.rejects(pinnedDestination('https://example.com',mode,resolve([])),/network_blocked/);
  }
  await assert.rejects(pinnedDestination('https://example.com','anything',resolve(['93.184.215.14'])),/network_mode_invalid/);
});

test('real Chromium relay blocks redirect and subresource escapes, including loopback proxy bypass', async () => {
  let privateHits=0;
  const sink=httpServer((req,res)=>{privateHits++;res.end('private content must never be reached');});
  await new Promise(resolve=>sink.listen(0,'127.0.0.1',resolve));
  const privateURL=`http://127.0.0.1:${sink.address().port}`;
  const server=createServer({key:readFileSync(new URL('./fixtures/localhost-test-key.pem',import.meta.url)),cert:readFileSync(new URL('./fixtures/localhost-test-cert.pem',import.meta.url))}, (req,res)=>{
    if(req.url==='/redirect-private'){res.writeHead(302,{location:privateURL});res.end();return;}
    if(req.url==='/redirect-other'){res.writeHead(302,{location:'https://unapproved.example/'});res.end();return;}
    if(req.url==='/redirect-approved'){res.writeHead(302,{location:'/article'});res.end();return;}
    res.setHeader('content-type','text/html');res.end(`<h1>Allowed article</h1><img src="${privateURL}/leak">`);
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  let browser,relay;
  try {
    relay=await startEgress({hostname:'approved.example',port:server.address().port,address:'127.0.0.1'});
    browser=await chromium.launch({headless:true,proxy:{server:relay.server,bypass:'<-loopback>'}});
    // Self-signed certificates are accepted ONLY by this owned test fixture.
    // Production BrowserDriver never disables TLS verification.
    const context=await browser.newContext({ignoreHTTPSErrors:true});
    const page=await context.newPage();
    const base=`https://approved.example:${server.address().port}`;
    await page.goto(base+'/redirect-approved');assert.equal(await page.locator('h1').textContent(),'Allowed article');
    await page.goto(base+'/redirect-private').catch(()=>{});
    await page.goto(base+'/redirect-other').catch(()=>{});
    await page.goto(privateURL).catch(()=>{});
    assert.equal(privateHits,0);
    await relay.close();relay=null;
    await assert.rejects(page.goto(base+'/article',{timeout:2000}));
  } finally {
    await browser?.close();await relay?.close();
    server.closeAllConnections();sink.closeAllConnections();
    await Promise.all([new Promise(resolve=>server.close(resolve)),new Promise(resolve=>sink.close(resolve))]);
  }
});
