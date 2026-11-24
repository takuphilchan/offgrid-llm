import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserDriver} from '../browser.mjs';

async function fixture(run){
 const server=createServer((req,res)=>{
  res.setHeader('Content-Type','text/html; charset=utf-8');
  if(req.url==='/')res.end('<style>body{margin:0}iframe{border:0;display:block;width:300px;height:150px}</style><iframe src="/child"></iframe><div id="host"></div><script>document.querySelector("#host").attachShadow({mode:"open"}).innerHTML=\'<div data-offgrid-private style="width:100px;height:40px;background:red">SHADOW-SECRET</div>\'</script>');
  else res.end('<style>body{margin:0}</style><div data-offgrid-private style="width:100px;height:40px;background:red">CHILD-SECRET</div><input style="display:block;background:red;width:100px;height:40px;border:0" aria-label="Verification code" value="123456"><p>Public document</p>');
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
 try{driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});await driver.page.waitForLoadState('load');await run(driver);}
 finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
}

async function pixels(driver,data,points){
 return driver.page.evaluate(async({data,points})=>{
  const image=new Image();image.src='data:image/png;base64,'+data;await image.decode();
  const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;const context=canvas.getContext('2d');context.drawImage(image,0,0);
  return points.map(([x,y])=>Array.from(context.getImageData(x,y,1,1).data));
 },{data,points});
}

test('capture masks protected fields in child documents and open shadow trees',async()=>{
 await fixture(async driver=>{
  const view=await driver.observe();assert.ok(!view.text.includes('SHADOW-SECRET'));
  const captured=await driver.execute('browser_capture',{observation_id:view.observation_id});
  assert.equal(captured.privacy.protected_controls_masked,true);
  assert.deepEqual(await pixels(driver,captured.data,[[10,10],[10,60],[10,160]]),Array(3).fill([0,0,0,255]));
  const frames=await driver.execute('browser_frames',{}),child=frames.frames.find(frame=>frame.parent_frame_id);
  const selected=await driver.execute('browser_frame_select',{frame_id:child.frame_id,document_id:child.document_id});
  assert.ok(selected.text.includes('Public document'));assert.ok(!selected.text.includes('CHILD-SECRET'));assert.ok(!selected.text.includes('123456'));
 });
});

test('a capture changed after screenshot is rejected instead of returning stale private coverage',async()=>{
 await fixture(async driver=>{
  const view=await driver.observe(),screenshot=driver.page.screenshot.bind(driver.page);
  driver.page.screenshot=async options=>{const image=await screenshot(options);await driver.page.frames()[1].locator('input').evaluate(el=>el.value='Changed silently');return image;};
  await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),e=>e.code==='computer_browser_observation_failed'&&/changed/.test(e.message));
 });
});

test('opaque child surfaces are wholly masked without reading their controls',async()=>{
 await fixture(async driver=>{
  await driver.page.locator('iframe').evaluate(el=>{el.setAttribute('sandbox','allow-scripts');el.src='/child?opaque';});
  await driver.page.waitForEvent('framenavigated',frame=>frame.url().includes('opaque'));
  const view=await driver.observe(),captured=await driver.execute('browser_capture',{observation_id:view.observation_id});
  assert.equal(captured.privacy.whole_frame_groups_masked,1);
  assert.deepEqual(await pixels(driver,captured.data,[[10,120]]),[[0,0,0,255]]);
 });
});

test('closed shadow content fails capture instead of silently exposing hidden protected fields',async()=>{
 await fixture(async driver=>{
  await driver.page.frames()[1].evaluate(()=>{const host=document.createElement('div');document.body.append(host);host.attachShadow({mode:'closed'}).innerHTML='<input aria-label="Secret" value="CLOSED-SECRET">';});
  const view=await driver.observe();
  await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),e=>e.code==='computer_browser_observation_failed'&&/Closed shadow/.test(e.message));
 });
});
