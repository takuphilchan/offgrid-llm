import test from 'node:test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { BrowserDriver, publicIPv4, validateBrowserAction } from '../browser.mjs';
import { startDemo, DEMO_ORIGIN } from '../demo.mjs';

test('typed action contracts reject extra fields, coercion and oversized Unicode before execution', () => {
 for (const [kind,args] of [
  ['browser_observe',{script:'not allowed'}], ['browser_observe',null], ['browser_observe',[]],
  ['browser_select',{observation_id:'seen',element:'1',option:2}],
  ['browser_set_checked',{observation_id:'seen',element:'1',checked:'false'}],
  ['browser_fill',{observation_id:'seen',element:'1',text:'語'.repeat(1334)}],
  ['browser_verify',{text:'語'.repeat(334)}],
  ['browser_click',{observation_id:'',element:'1'}],
 ]) assert.throws(()=>validateBrowserAction(kind,args));
 validateBrowserAction('browser_fill',{observation_id:'seen',element:'1',text:''});
 validateBrowserAction('browser_set_checked',{observation_id:'seen',element:'1',checked:false});
});

test('native dropdowns and checkboxes use observed IDs, explicit state and postcondition checks', async () => {
 const server=createServer((req,res)=>{res.setHeader('Content-Type','text/html; charset=utf-8');res.end(`<html><body>
  <label for="region">Region</label><select id="region"><option>Africa</option><option>Zimbabwe — 2026</option><option disabled>Unavailable</option><optgroup disabled label="Restricted"><option>Blocked group</option></optgroup></select>
  <label><input id="include" type="checkbox">Include sources</label>
  <input aria-label="Disabled field" disabled><select aria-label="Multiple" multiple><option>A</option></select>
  <p id="status">Not saved</p><button onclick="document.querySelector('#status').textContent='Saved: '+document.querySelector('#region').selectedOptions[0].text+'; sources: '+document.querySelector('#include').checked">Save preferences</button>
 </body></html>`)});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve)); let driver;
 try {
  driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
  let view=await driver.observe();
  const region=view.elements.find(e=>e.label==='Region');
  assert.equal(region.options[1].label,'Zimbabwe — 2026');
  assert.equal(region.options[2].disabled,true);assert.equal(region.options[3].disabled,true);
  const old=view.observation_id;
  view=await driver.execute('browser_select',{observation_id:old,element:region.id,option:region.options[1].id});
  assert.equal(view.check,'selected_option_matches');assert.equal(view.verified,true);
  await assert.rejects(driver.execute('browser_select',{observation_id:old,element:region.id,option:'1'}),/Stale/);
  for(const checked of [true,true,false,true]) {
   view=await driver.execute('browser_set_checked',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Include sources').id,checked});
   assert.equal(view.check,'checked_state_matches');assert.equal(view.elements.find(e=>e.label==='Include sources').checked,checked);
  }
  view=await driver.execute('browser_click',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Save preferences').id});
  assert.equal((await driver.execute('browser_verify',{text:'Saved: Zimbabwe — 2026; sources: true'})).verified,true);
  for(const [label,option] of [['Region','3'],['Region','4'],['Region','999'],['Multiple','1']]) {
   view=await driver.observe();
   await assert.rejects(driver.execute('browser_select',{observation_id:view.observation_id,element:view.elements.find(e=>e.label===label).id,option}),/enabled option/);
  }
  view=await driver.observe();
  await assert.rejects(driver.execute('browser_set_checked',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Region').id,checked:true}),/native checkbox/);
  view=await driver.observe();
  await assert.rejects(driver.execute('browser_fill',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Disabled field').id,text:'No'}),/Disabled/);
  // Silent changes to selectedIndex do not fire input/change events.
  view=await driver.observe();await driver.page.locator('#region').evaluate(el=>{el.selectedIndex=0});
  await assert.rejects(driver.execute('browser_select',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Region').id,option:'2'}),/Stale/);
 } finally {await driver?.close();await new Promise(resolve=>server.close(resolve))}
});

test('private/reserved network addresses are rejected', () => {
 for (const ip of ['127.0.0.1','10.1.2.3','172.16.0.1','192.168.1.1','169.254.169.254','100.64.0.1','198.18.0.1','::1']) assert.equal(publicIPv4(ip), false);
 assert.equal(publicIPv4('93.184.215.14'), true);
});

test('input events and silent property changes invalidate an observed action', async () => {
 const demo=await startDemo(); let driver;
 try {
  driver=await BrowserDriver.open(demo.origin,{headless:true,testLoopback:true});
  for (const edit of ['manual','script']) {
   const view=await driver.observe();
   if(edit==='manual') await driver.page.locator('#report').fill('Manual edit');
   else await driver.page.locator('#report').evaluate(el=>{el.value='Silent edit'});
   await assert.rejects(driver.execute('browser_click',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Save draft').id}),/Stale/);
   assert.equal(await driver.page.locator('#status').innerText(),'No draft saved');
   const next=await driver.observe();
   assert.equal(JSON.stringify(next).includes(edit==='manual'?'Manual edit':'Silent edit'),false,'field values must not leave the companion');
  }
 } finally {await driver?.close();await demo.close()}
});

test('credential controls are excluded using labels, accessible names and placeholders', async () => {
 const server=createServer((req,res)=>{res.setHeader('Content-Type','text/html');res.end('<html><body><label for="x">Password</label><input id="x"><span id="hint">Verification code</span><input aria-labelledby="hint"><input placeholder="Credit card number"><input aria-label="Report title"></body></html>')});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve)); let driver;
 try {
  driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
  const view=await driver.observe();
  assert.deepEqual(view.elements.map(e=>e.label),['Report title']);
  // Defense in depth: even a supplied handle cannot bypass execution checks.
  driver.elements.set('forged',await driver.page.locator('#x').elementHandle());
  await assert.rejects(driver.execute('browser_fill',{observation_id:view.observation_id,element:'forged',text:'SYNTHETIC_ONLY'}),/Credential/);
  assert.equal(await driver.page.locator('#x').inputValue(),'');
 } finally {await driver?.close();await new Promise(resolve=>server.close(resolve))}
});

test('owned demo performs useful work without permitting other local services', async () => {
 const demo = await startDemo();
 let driver;
 try {
  assert.equal(DEMO_ORIGIN, 'offgrid-demo://research');
  assert.equal((await fetch(demo.origin + '/etc/passwd')).status, 404);
  assert.equal((await fetch(demo.origin, {method:'POST', body:'ignored'})).status, 404);
  driver = await BrowserDriver.open(demo.origin, {headless:true, testLoopback:true});
  let view = await driver.execute('browser_observe', {});
  assert.match(view.text, /Research notes/);
  const field = view.elements.find(el => el.label === 'Report title');
  assert.equal(field?.tag, 'input', 'Associated HTML labels identify fields without exposing their values');
  const oldObservation = view.observation_id;
  view = await driver.execute('browser_fill', {observation_id:view.observation_id, element:field.id, text:'OffGrid test'});
  assert.notEqual(view.observation_id, oldObservation);
  assert.equal(view.verified, true);
  await assert.rejects(driver.execute('browser_click', {observation_id:oldObservation, element:'2'}), /Stale/);
  const filledObservation = view.observation_id;
  view = await driver.execute('browser_click', {observation_id:view.observation_id, element:view.elements.find(el => el.label === 'Save draft').id});
  assert.notEqual(view.observation_id, filledObservation);
  assert.equal(view.verified, false, 'A fresh post-click observation is not task success');
  assert.match(view.text, /Draft saved: OffGrid test/);
  assert.equal((await driver.execute('browser_verify', {text:'Draft saved: OffGrid test'})).verified, true);
  await assert.rejects(driver.execute('browser_navigate', {url:'http://127.0.0.1:11611'}), /outside/);
 } finally { await driver?.close(); await demo.close(); }
 await assert.rejects(fetch(demo.origin));
});

test('real browser observes, edits, verifies, and rejects stale or out-of-scope actions', async () => {
 const server = createServer((req, res) => { res.setHeader('Content-Type','text/html'); res.end('<html><head><title>Research fixture</title></head><body><h1>Notes</h1><input aria-label="Report title"><input type="password" aria-label="Password"><button onclick="document.querySelector(\'h1\').textContent=\'Saved report\'">Save</button></body></html>'); });
 await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
 let driver;
 try {
  const origin = `http://127.0.0.1:${server.address().port}`;
  await assert.rejects(BrowserDriver.open(origin, { headless: true }), /HTTPS/);
  driver = await BrowserDriver.open(origin, { headless: true, testLoopback: true });
  let view = await driver.execute('browser_observe', {});
  assert.equal(view.elements.some(el => el.type === 'password'), false);
  const field = view.elements.find(el => el.label === 'Report title');
  assert.equal((await driver.execute('browser_fill', { observation_id:view.observation_id, element:field.id, text:'Local research' })).verified, true);
  await assert.rejects(driver.execute('browser_fill', { observation_id:view.observation_id, element:field.id, text:'duplicate' }), /Stale/);
  view = await driver.execute('browser_observe', {});
  const button = view.elements.find(el => el.label === 'Save');
  assert.equal((await driver.execute('browser_click', { observation_id:view.observation_id, element:button.id })).verified, false);
  assert.equal((await driver.execute('browser_verify', { text:'Saved report' })).verified, true);
  assert.equal((await driver.execute('browser_verify', { text:'not present' })).verified, false);
  await assert.rejects(driver.execute('browser_navigate', { url:'https://example.com' }), /outside/);
  await assert.rejects(driver.execute('shell', { command:'anything' }), /Unsupported/);
 } finally { await driver?.close(); await new Promise(resolve => server.close(resolve)); }
});
