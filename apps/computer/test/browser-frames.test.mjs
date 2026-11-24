import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserDriver} from '../browser.mjs';
import {chromium} from 'playwright';
import {installDocumentRevision} from '../browser-dom.mjs';

async function fixture(run){
 let origin;
 const server=createServer((req,res)=>{
  res.setHeader('Content-Type','text/html; charset=utf-8');
  if(req.url==='/')res.end('<button>Top</button><iframe name="one" src="/child?one"></iframe><iframe name="two" src="/child?two"></iframe><iframe name="opaque" sandbox="allow-scripts" src="/opaque"></iframe><iframe name="blank"></iframe>');
  else if(req.url==='/opaque')res.end(`<script>window.origin=${JSON.stringify(origin)}</script><button>Do not read</button>`);
  else if(req.url==='/grandchild')res.end('<input aria-label="Same"><p>Nested document</p>');
  else res.end(`<input aria-label="Same"><button onclick="this.textContent='Changed'">Apply</button>${req.url.includes('one')?'<iframe name="nested" src="/grandchild"></iframe>':''}`);
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));origin=`http://127.0.0.1:${server.address().port}`;
 let driver;try{driver=await BrowserDriver.open(origin,{headless:true,testLoopback:true});await driver.page.waitForLoadState('load');await run(driver,origin);}
 finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
}

test('frames have separate issued identities, nested selection and exact observed input',async()=>{
 await fixture(async driver=>{
  const inventory=await driver.execute('browser_frames',{});assert.equal(inventory.frames.length,6);
  const one=inventory.frames.find(frame=>frame.name==='one'),two=inventory.frames.find(frame=>frame.name==='two'),nested=inventory.frames.find(frame=>frame.name==='nested');
  assert.equal(nested.parent_frame_id,one.frame_id);assert.notEqual(one.frame_id,two.frame_id);
  let view=await driver.execute('browser_frame_select',{frame_id:one.frame_id,document_id:one.document_id});
  assert.equal(view.frame_id,one.frame_id);assert.equal(view.frame_document_id,one.document_id);
  const element=view.elements.find(el=>el.label==='Same');
  await driver.execute('browser_fill',{observation_id:view.observation_id,element:element.id,text:'Only first'});
  assert.equal(await driver.page.frame({name:'one'}).locator('input').inputValue(),'Only first');
  assert.equal(await driver.page.frame({name:'two'}).locator('input').inputValue(),'');
  view=await driver.execute('browser_frame_select',{frame_id:nested.frame_id,document_id:nested.document_id});
  assert.ok(view.text.includes('Nested document'));assert.equal(view.elements.length,1);
  await assert.rejects(driver.execute('browser_fill',{observation_id:'guessed',element:element.id,text:'wrong'}),e=>e.code==='computer_browser_precondition_failed');
 });
});

test('same-address reload and detached frame require explicit current selection, never fallback',async()=>{
 await fixture(async driver=>{
  let inventory=await driver.execute('browser_frames',{}),one=inventory.frames.find(frame=>frame.name==='one');
  let view=await driver.execute('browser_frame_select',{frame_id:one.frame_id,document_id:one.document_id});
  const first=view;
  await driver.page.frame({name:'one'}).goto(driver.page.frame({name:'one'}).url());
  await assert.rejects(driver.execute('browser_fill',{observation_id:first.observation_id,element:first.elements[0].id,text:'wrong'}),/Stale observation/);
  await assert.rejects(driver.execute('browser_frame_select',{frame_id:one.frame_id,document_id:one.document_id}),/stale frame/);
  inventory=await driver.execute('browser_frames',{});one=inventory.frames.find(frame=>frame.name==='one');
  view=await driver.execute('browser_frame_select',{frame_id:one.frame_id,document_id:one.document_id});
  await driver.page.locator('iframe[name=one]').evaluate(el=>el.remove());
  await assert.rejects(driver.execute('browser_observe',{}),/Selected frame is unavailable/);
  assert.equal(driver.frames.selected,null);
  inventory=await driver.execute('browser_frames',{});
  const main=inventory.frames.find(frame=>!frame.parent_frame_id);
  view=await driver.execute('browser_frame_select',{frame_id:main.frame_id,document_id:main.document_id});
  assert.equal(view.elements[0].label,'Top');
 });
});

test('opaque and inherited documents remain unavailable despite page-authored origin claims',async()=>{
 await fixture(async driver=>{
  const inventory=await driver.execute('browser_frames',{});
  assert.equal(inventory.frames.filter(frame=>!frame.available).length,2,JSON.stringify({inventory,origins:await driver.frames.origins()}));
  for(const frame of inventory.frames.filter(frame=>!frame.available)){
   assert.equal(frame.url,undefined);assert.equal(frame.name,undefined);
   await assert.rejects(driver.execute('browser_frame_select',{frame_id:frame.frame_id,document_id:frame.document_id}),/stale frame/);
  }
  const opaque=driver.page.frame({name:'opaque'});
  assert.equal(await opaque.evaluate(()=>window.origin),new URL(driver.page.url()).origin);
  assert.ok(!JSON.stringify(inventory).includes('Do not read'));
 });
});

test('failed post-input observation keeps selected frame and its uncertain receipt',async()=>{
 await fixture(async driver=>{
  const inventory=await driver.execute('browser_frames',{}),one=inventory.frames.find(frame=>frame.name==='one');
  const view=await driver.execute('browser_frame_select',{frame_id:one.frame_id,document_id:one.document_id});
  const selected=driver.context();driver.observe=async()=>{throw Error('fixture observation unavailable');};
  const result=await driver.execute('browser_fill',{observation_id:view.observation_id,element:view.elements[0].id,text:'Retained'});
  assert.equal(result.dispatched,true);assert.equal(result.observation_status,'unavailable');assert.equal(driver.context(),selected);
 });
});

test('parent replacement invalidates nested selection and old frame inventory',async()=>{
 await fixture(async driver=>{
  const inventory=await driver.execute('browser_frames',{}),nested=inventory.frames.find(frame=>frame.name==='nested');
  await driver.execute('browser_frame_select',{frame_id:nested.frame_id,document_id:nested.document_id});
  await driver.page.locator('iframe[name=one]').evaluate(el=>el.outerHTML=el.outerHTML);
  await driver.page.waitForLoadState('load');
  await assert.rejects(driver.execute('browser_observe',{}),/Selected frame is unavailable/);
  await assert.rejects(driver.execute('browser_frame_select',{frame_id:nested.frame_id,document_id:nested.document_id}),/stale frame/);
 });
});

test('approved cross-origin child is usable and unapproved child cannot expand network scope',async()=>{
 let deniedRequests=0;
 const allowed=createServer((req,res)=>{res.setHeader('Content-Type','text/html; charset=utf-8');res.end('<input aria-label="Cross origin">');});
 const denied=createServer((req,res)=>{deniedRequests++;res.end('Unapproved');});
 await Promise.all([new Promise(resolve=>allowed.listen(0,'127.0.0.1',resolve)),new Promise(resolve=>denied.listen(0,'127.0.0.1',resolve))]);
 const approved=`http://127.0.0.1:${allowed.address().port}`,unapproved=`http://127.0.0.1:${denied.address().port}`;
 const main=createServer((req,res)=>{res.setHeader('Content-Type','text/html; charset=utf-8');res.end(`<iframe src="${approved}" name="approved"></iframe><iframe src="${unapproved}" name="unapproved"></iframe>`);});
 await new Promise(resolve=>main.listen(0,'127.0.0.1',resolve));let driver;
 try{
  driver=await BrowserDriver.open(`http://127.0.0.1:${main.address().port}`,{headless:true,testLoopback:true,additionalOrigins:[approved]});
  await driver.page.waitForLoadState('load');
  const inventory=await driver.execute('browser_frames',{}),target=inventory.frames.find(frame=>frame.name==='approved');
  assert.equal(target.available,true);assert.equal(inventory.frames.filter(frame=>!frame.available).length,1);assert.equal(deniedRequests,0);
  const view=await driver.execute('browser_frame_select',{frame_id:target.frame_id,document_id:target.document_id});
  const result=await driver.execute('browser_fill',{observation_id:view.observation_id,element:view.elements[0].id,text:'Scoped'});assert.equal(result.verified,true);
 }finally{await driver?.close();await Promise.all([main,allowed,denied].map(server=>new Promise(resolve=>server.close(resolve))));}
});

test('distinct-site Chromium frame processes retain explicit scope and document identity',async()=>{
 const browser=await chromium.launch({headless:true,args:['--site-per-process']});
 try{
  const context=await browser.newContext();await context.addInitScript(installDocumentRevision);
  // Every URL is fulfilled locally. No DNS, external site or personal session
  // participates; distinct test sites force Chromium's process boundary.
  await context.route('**/*',route=>route.fulfill({contentType:'text/html',body:route.request().url().startsWith('https://root.example/')?'<iframe src="https://child.example.net/" name="separate"></iframe>':'<input aria-label="Separate process"><input aria-label="Verification code" value="PRIVATE" style="background:red;width:100px;height:40px">'}));
  const page=await context.newPage();await page.goto('https://root.example/');
  const child=page.frame({name:'separate'}),session=await context.newCDPSession(child);
  const ownTree=await session.send('Page.getFrameTree');assert.equal(ownTree.frameTree.frame.url,'https://child.example.net/');await session.detach();
  const driver=new BrowserDriver(browser,page,'https://root.example',{allowedOrigins:['https://root.example','https://child.example.net']});
  const inventory=await driver.execute('browser_frames',{}),target=inventory.frames.find(frame=>frame.name==='separate');
  assert.ok(target?.available,'approved out-of-process frame must have origin proof');
  const view=await driver.execute('browser_frame_select',{frame_id:target.frame_id,document_id:target.document_id});
  const result=await driver.execute('browser_fill',{observation_id:view.observation_id,element:view.elements[0].id,text:'Owned exact process'});assert.equal(result.verified,true);
  const box=await child.locator('input').nth(1).boundingBox();
  const capture=await driver.execute('browser_capture',{observation_id:result.observation_id});
  const pixel=await page.evaluate(async({data,x,y})=>{const image=new Image();image.src='data:image/png;base64,'+data;await image.decode();const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;const ctx=canvas.getContext('2d');ctx.drawImage(image,0,0);return Array.from(ctx.getImageData(x,y,1,1).data);},{data:capture.data,x:Math.floor(box.x+8),y:Math.floor(box.y+8)});
  assert.deepEqual(pixel,[0,0,0,255]);
  await child.evaluate(()=>{const host=document.createElement('div');document.body.append(host);host.attachShadow({mode:'closed'}).innerHTML='<input value="PRIVATE">';});
  const fresh=await driver.observe();await assert.rejects(driver.execute('browser_capture',{observation_id:fresh.observation_id}),/Closed shadow/);
 }
 finally{await browser.close();}
});

test('same-address replacement during a fresh observation cannot publish mixed document evidence',async()=>{
 let current='Original document';const server=createServer((req,res)=>{res.setHeader('Content-Type','text/html');res.end(`<p>${current}</p>`);});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
 try{
  driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
  const stateDigest=driver.stateDigest.bind(driver);let calls=0;
  driver.stateDigest=async()=>{if(++calls===2){current='Replacement document';await driver.page.goto(driver.page.url());}return stateDigest();};
  await assert.rejects(driver.observeOnce(),/observation_changed/);assert.equal(driver.observation,'');
  const view=await driver.observe();assert.equal(view.text,'Replacement document');assert.equal(view.frame_document_id,driver.frames.provenance().frame_document_id);
 }finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
});
