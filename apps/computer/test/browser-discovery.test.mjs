import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserDriver,validateBrowserAction} from '../browser.mjs';

async function fixture(html,run) {
 const server=createServer((req,res)=>{res.setHeader('Content-Type','text/html; charset=utf-8');res.end(html);});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
 try {driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});await run(driver);}
 finally {await driver?.close();await new Promise(resolve=>server.close(resolve));}
}

test('large pages expose later controls and Unicode text through bounded observed cursors',async()=>{
 const text='語😀'.repeat(5000)+'END OF DOCUMENT';
 await fixture(`<p>${text}</p>${'<button hidden>Hidden</button>'.repeat(410)}${Array.from({length:105},(_,i)=>`<button onclick="document.body.dataset.clicked='${i}'">Control ${i}</button>`).join('')}`,async driver=>{
  let view=await driver.observe(), collected='',elements=[];const observation=view.observation_id;
  for(let page=0;;page++){
   assert.ok(page<30,'continuation must make progress');
   assert.ok(Buffer.byteLength(JSON.stringify(view))<48*1024);
   assert.equal(view.observation_id,observation);
   collected+=view.text;elements.push(...view.elements);
   if(!view.next_cursor)break;
   view=await driver.execute('browser_inspect',{observation_id:observation,cursor:view.next_cursor});
  }
  assert.ok(collected.includes(text));
  assert.equal(elements.length,105);assert.equal(new Set(elements.map(e=>e.id)).size,105);
  const last=elements.find(e=>e.label==='Control 104');assert.ok(last);
  const result=await driver.execute('browser_click',{observation_id:observation,element:last.id});
  assert.equal(result.dispatched,true);assert.equal(await driver.page.locator('body').getAttribute('data-clicked'),'104');
 });
});

test('unrelated DOM changes preserve targets; semantics, form values and replaced nodes invalidate them',async()=>{
 await fixture('<p id="clock">0</p><form action="/original"><label>Name <input></label><button type="button" onclick="window.clicks=(window.clicks||0)+1">Save</button></form>',async driver=>{
  let view=await driver.observe();let element=view.elements.find(e=>e.tag==='button').id;
  await driver.page.locator('#clock').evaluate(el=>el.textContent='1');
  const clicked=await driver.execute('browser_click',{observation_id:view.observation_id,element});
  assert.equal(clicked.dispatched,true);assert.equal(await driver.page.evaluate(()=>window.clicks),1);
  for(const change of [
   ()=>driver.page.locator('button').evaluate(el=>el.textContent='Different operation'),
   ()=>driver.page.locator('form').evaluate(el=>el.action='/different'),
   ()=>driver.page.locator('input').evaluate(el=>el.value='silent'),
   ()=>driver.page.locator('button').evaluate(el=>el.replaceWith(el.cloneNode(true))),
  ]) {
   view=await driver.observe();element=view.elements.find(e=>e.tag==='button').id;await change();
   await assert.rejects(driver.execute('browser_click',{observation_id:view.observation_id,element}),e=>e.code==='computer_browser_precondition_failed');
   assert.equal(await driver.page.evaluate(()=>window.clicks),1);
  }
 });
});

test('cursors reject guessed references, changed pages and malformed arguments without input',async()=>{
 validateBrowserAction('browser_inspect',{observation_id:'observed',cursor:'issued'});
 assert.throws(()=>validateBrowserAction('browser_inspect',{observation_id:'observed',cursor:'issued',offset:400}));
 await fixture(`<p>${'abc'.repeat(5000)}</p><button>Original</button>`,async driver=>{
  const view=await driver.observe();assert.ok(view.next_cursor);
  await assert.rejects(driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:'guessed'}),e=>e.code==='computer_browser_observation_failed');
  await driver.page.locator('button').evaluate(el=>el.textContent='Changed');
  await assert.rejects(driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:view.next_cursor}),e=>e.code==='computer_browser_observation_failed');
 });
});

test('long dropdowns preserve labels and require observing later option IDs before selection',async()=>{
 const label='A long option label with an identifying suffix ';
 await fixture(`<label>Choice<select>${Array.from({length:130},(_,i)=>`<option>${label}${i}</option>`).join('')}</select></label>`,async driver=>{
  let view=await driver.observe();let select=view.elements[0];
  assert.equal(select.options[0].label,label+'0');assert.equal(select.options_limited,true);
  await assert.rejects(driver.execute('browser_select',{observation_id:view.observation_id,element:select.id,option:'130'}),/enabled option/);
  view=await driver.observe();select=view.elements[0];
  const options=await driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:select.options_cursor});
  assert.equal(options.element,select.id);assert.equal(options.options.at(-1).label,label+'129');
  const result=await driver.execute('browser_select',{observation_id:view.observation_id,element:select.id,option:'130'});
  assert.equal(result.verified,true);assert.equal(await driver.page.locator('select').evaluate(el=>el.selectedIndex),129);
 });
});

test('same-URL document replacement invalidates capture and continuation even with identical form state',async()=>{
 await fixture(`<p>${'abc'.repeat(5000)}</p><button>Same control</button>`,async driver=>{
  const view=await driver.observe();await driver.page.reload({waitUntil:'domcontentloaded'});
  await assert.rejects(driver.execute('browser_inspect',{observation_id:view.observation_id,cursor:view.next_cursor}),e=>e.code==='computer_browser_observation_failed');
  await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),e=>e.code==='computer_browser_observation_failed');
  const fresh=await driver.observe();assert.notEqual(fresh.observation_id,view.observation_id);
 });
});

test('malformed page Unicode cannot create a non-advancing continuation',async()=>{
 await fixture('<p>Original</p>',async driver=>{
  await driver.page.locator('p').evaluate(el=>el.textContent='\ud800');
  const view=await driver.observe();assert.equal(view.text.length,1);assert.equal(view.text_limited,false);assert.equal(view.next_cursor,undefined);
 });
});
