import test from 'node:test';
import assert from 'node:assert/strict';
import {BrowserDriver} from '../browser.mjs';
import {startDemo} from '../demo.mjs';

test('successful input retains its receipt when follow-up observation fails', async () => {
  const demo = await startDemo(); let driver;
  try {
    driver = await BrowserDriver.open(demo.origin,{headless:true,testLoopback:true});
    let view = await driver.observe();
    const observe = driver.observe.bind(driver);
    driver.observe = async () => { throw Error('observation_changed'); };
    const filled = await driver.execute('browser_fill',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Report title').id,text:'Receipt test'});
    assert.equal(filled.verified,true);
    assert.equal(filled.effect_status,'completed');
    assert.equal(filled.observation_status,'unavailable');
    assert.equal(filled.observation_id,undefined);
    view = await observe();
    const handle = driver.elements.get(view.elements.find(e=>e.label==='Save draft').id);
    const click = handle.click.bind(handle); let dispatches=0;
    handle.click = async () => {dispatches++;await click();};
    const clicked = await driver.execute('browser_click',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Save draft').id});
    assert.equal(dispatches,1);
    assert.equal(clicked.dispatched,true);
    assert.equal(clicked.verified,false);
    assert.equal(clicked.effect_status,'dispatched');
    assert.equal(clicked.observation_status,'unavailable');
    assert.equal(await driver.page.locator('#status').innerText(),'Draft saved: Receipt test');
    await assert.rejects(driver.execute('browser_click',{observation_id:view.observation_id,element:'1'}),e=>e.code==='computer_browser_precondition_failed');
  } finally {await driver?.close();await demo.close();}
});

test('read failures carry safe stages but input exceptions remain uncertain',async()=>{
  const demo=await startDemo();let driver;
  try {
    driver=await BrowserDriver.open(demo.origin,{headless:true,testLoopback:true});
    const view=await driver.observe();
    const handle=driver.elements.get(view.elements.find(e=>e.label==='Save draft').id);
    let dispatches=0;
    handle.click=async()=>{dispatches++;throw Error('input transport failed');};
    await assert.rejects(driver.execute('browser_click',{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Save draft').id}),e=>!e.code && /input transport/.test(e.message));
    assert.equal(dispatches,1);
    driver.observe=async()=>{throw Error('read transport failed');};
    await assert.rejects(driver.execute('browser_observe',{}),e=>e.code==='computer_browser_observation_failed');
  } finally {await driver?.close();await demo.close();}
});

test('failed read-back preserves completed input without claiming the desired state',async()=>{
 const demo=await startDemo();let driver;
 try {
  driver=await BrowserDriver.open(demo.origin,{headless:true,testLoopback:true});
  const view=await driver.observe(),element=view.elements.find(e=>e.label==='Report title').id;
  driver.elements.get(element).inputValue=async()=>{throw Error('read-back unavailable');};
  const result=await driver.execute('browser_fill',{observation_id:view.observation_id,element,text:'Kept input'});
  assert.equal(result.dispatched,true);assert.equal(result.verified,false);assert.equal(result.effect_status,'dispatched');
  assert.equal(result.verification_status,'unavailable');
  assert.equal(await driver.page.locator('#report').inputValue(),'Kept input');
 } finally {await driver?.close();await demo.close();}
});
