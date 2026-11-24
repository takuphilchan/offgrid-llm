import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve,dirname,basename} from 'node:path';
import {fileURLToPath} from 'node:url';
import {chromium} from 'playwright';
import {startAttachmentNetworkSite} from './fixtures/attachment-network-site.mjs';

// An opt-in qualification probe, intentionally expected to fail while the
// candidate DNR + Fetch boundary cannot meet the approved attached scope.
const enabled=process.env.OFFGRID_ATTACHMENT_NETWORK_PROBE==='1';
const options={skip:!enabled,timeout:45000};
async function fixture(t) {
  const directory=await mkdtemp(join(tmpdir(),'offgrid-attachment-network-'));
  const site=await startAttachmentNetworkSite();
  let context;
  t.after(async()=>{
    try { await context?.close(); }
    finally {
      await site.close();
      assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
      assert.ok(basename(directory).startsWith('offgrid-attachment-network-'));
      await rm(directory,{recursive:true,force:true});
    }
  });
  const extension=fileURLToPath(new URL('./fixtures/attachment-network-extension',import.meta.url));
  context=await chromium.launchPersistentContext(directory,{
    channel:'chromium',headless:true,ignoreHTTPSErrors:true,
    args:[`--disable-extensions-except=${extension}`,`--load-extension=${extension}`,
      '--host-resolver-rules=MAP *.offgrid-fixture.example 127.0.0.1','--no-proxy-server'],
  });
  const worker=context.serviceWorkers()[0]??await context.waitForEvent('serviceworker',{timeout:10000});
  assert.match(worker.url(),/^chrome-extension:\/\//);
  const origin=`https://selected.offgrid-fixture.example:${site.port}`;
  const privateOrigin=`https://private.offgrid-fixture.example:${site.port}`;
  const sink=`https://sink.offgrid-fixture.example:${site.port}`;
  const page=await context.newPage(),sentinel=await context.newPage();
  await page.goto(`${origin}/selected`);
  await page.evaluate(()=>window.socketReady.then(()=>true));
  assert.equal(site.counts.upgrades.length,1,'socket must predate attachment');
  await sentinel.goto(`${origin}/sentinel`);
  const installed=await worker.evaluate(config=>globalThis.installNetworkProbe(config),{origin,privateOrigin});
  assert.equal(installed.rules.length,3);
  assert.ok(installed.rules.every(rule=>rule.condition.tabIds.length===1&&rule.condition.tabIds[0]===installed.tabId));
  assert.equal(await page.evaluate(url=>fetch(url).then(response=>response.text()),`${origin}/allowed-control`),'fixture-response');
  const firstStatus=await worker.evaluate(()=>globalThis.networkProbeStatus());
  assert.ok(firstStatus.intercepted.some(request=>request.url===`${origin}/allowed-control`&&request.allowed),'Fetch must actually intercept the allowed control request');
  // A new forbidden request is denied; the same URL succeeds in the untouched tab.
  assert.equal(await page.evaluate(url=>fetch(url).then(()=>false,()=>true),`${sink}/blocked-control`),true);
  assert.equal(site.counts.requests.filter(request=>request.path==='/blocked-control').length,0);
  assert.equal(await sentinel.evaluate(url=>fetch(url).then(response=>response.text()),`${sink}/sentinel-control`),'fixture-response');
  assert.equal(site.counts.requests.filter(request=>request.path==='/sentinel-control').length,1);
  t.diagnostic(JSON.stringify({browser:context.browser().version(),extension:worker.url().split('/')[2],installedRuleCount:installed.rules.length,sentinelUnaffected:true}));
  return {context,site,page,sentinel,worker,origin,privateOrigin,sink};
}

test('attached candidate prevents effects through a WebSocket established before DNR and Fetch',options,async t=>{
  const {site,page,worker,sink,sentinel}=await fixture(t);
  const newSocketBlocked=await page.evaluate(url=>new Promise(resolve=>{
    const socket=new WebSocket(url.replace(/^https:/,'wss:')+'/new-socket');
    const timer=setTimeout(()=>{socket.close();resolve(false);},3000);
    socket.onopen=()=>{clearTimeout(timer);socket.close();resolve(false);};
    socket.onerror=()=>{clearTimeout(timer);resolve(true);};
  }),sink);
  assert.equal(newSocketBlocked,true,'new forbidden socket handshake must be denied');
  assert.equal(site.counts.upgrades.length,1);
  await page.locator('#send').click();
  await page.waitForFunction(()=>window.lastMessage==='effect-after-install',{},{timeout:3000}).catch(()=>{});
  assert.equal(await sentinel.title(),'Unrelated sentinel');
  const status=await worker.evaluate(()=>globalThis.networkProbeStatus());
  t.diagnostic(JSON.stringify({case:'preexisting-websocket',serverMessages:site.counts.messages,newHandshakeBlocked:newSocketBlocked,intercepted:status.intercepted,errors:status.errors}));
  assert.deepEqual(status.errors,[]);
  assert.equal(site.counts.messages.length,0,'required containment failed: existing WebSocket carried an effect after both controls were installed');
});

test('attached candidate prevents an approved HTTPS hostname from reaching a private resolved address',options,async t=>{
  const {site,page,worker,sentinel}=await fixture(t);
  await page.locator('#private').click();
  const result=await page.evaluate(()=>window.privateDone);
  const status=await worker.evaluate(()=>globalThis.networkProbeStatus());
  const effects=site.counts.requests.filter(request=>request.path==='/private-effect');
  assert.equal(await sentinel.title(),'Unrelated sentinel');
  t.diagnostic(JSON.stringify({case:'private-resolved-address',result,effects,responses:status.responses,intercepted:status.intercepted,errors:status.errors}));
  assert.deepEqual(status.errors,[]);
  assert.equal(effects.length,0,'required containment failed: URL-approved hostname reached the private fixture listener');
});
