import test from 'node:test';
import assert from 'node:assert/strict';
import {chromium} from 'playwright';
import {BrowserDriver} from '../browser.mjs';
import {installDocumentRevision} from '../browser-dom.mjs';

async function fixture(run){
  const browser=await chromium.launch({headless:true,args:['--site-per-process']});
  try{
    const context=await browser.newContext();await context.addInitScript(installDocumentRevision);
    const privateHTML='<style>input{position:fixed;left:0;top:0;width:100px;height:50px;background:red}</style><input aria-label="Secret token" value="PRIVATE">';
    await context.route('**/*',route=>route.fulfill({contentType:'text/html',body:route.request().url().startsWith('https://root.example/')?
      '<style>@media(min-width:1px){body{background:white}}</style><iframe name="separate" src="https://child.example.net/"></iframe><div id="host"></div><script>document.querySelector("#host").attachShadow({mode:"open"}).innerHTML='+JSON.stringify(privateHTML)+'</script>':privateHTML}));
    const page=await context.newPage();await page.goto('https://root.example/');await page.waitForLoadState('load');
    const driver=new BrowserDriver(browser,page,'https://root.example',{allowedOrigins:['https://root.example','https://child.example.net']});
    await run(driver,page.frame({name:'separate'}));
  }finally{await browser.close();}
}

async function witnessState(driver){
  const origins=await driver.frames.origins(),sessions=new Set(driver.frames.originSessions.values()),counts=[];
  for(const session of sessions)counts.push(['CSS.styleSheetChanged','CSS.styleSheetAdded','Animation.animationCreated'].map(event=>session.listenerCount(event)));
  const retained=[];
  for(const origin of origins){
    const session=driver.frames.originSessions.get(origin.id);
    const world=await session.send('Page.createIsolatedWorld',{frameId:origin.id,worldName:'offgrid-capture-witness-v1'});
    const result=await session.send('Runtime.evaluate',{contextId:world.executionContextId,expression:'Object.keys(globalThis).filter(key=>key.startsWith("capture_"))',returnByValue:true});
    assert.equal(result.exceptionDetails,undefined);retained.push(...result.result.value);
  }
  return {counts,retained};
}

for(const target of ['open-shadow','separate-process'])test(`capture witnesses transient CSSOM edits in ${target}`,async()=>{
  await fixture(async(driver,child)=>{
    const before=await witnessState(driver),view=await driver.observe(),screenshot=driver.page.screenshot.bind(driver.page);let captures=0;
    const mutate=()=>document.styleSheets[0].insertRule('input{left:100px}',1);
    const restore=()=>document.styleSheets[0].deleteRule(1);
    driver.page.screenshot=async options=>{
      captures++;
      if(target==='separate-process')await child.evaluate(mutate);
      else await driver.page.locator('#host').evaluate(host=>host.shadowRoot.querySelector('style').sheet.insertRule('input{left:100px}',1));
      try{return await screenshot(options);}
      finally{
        if(target==='separate-process')await child.evaluate(restore);
        else await driver.page.locator('#host').evaluate(host=>host.shadowRoot.querySelector('style').sheet.deleteRule(1));
      }
    };
    await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),error=>error.code==='computer_browser_observation_failed');
    assert.equal(captures,1);assert.deepEqual(await witnessState(driver),before,'failed capture must release every owned witness/listener');
  });
});

test('unchanged media-query pages capture repeatedly without accumulating witnesses',async()=>{
  await fixture(async driver=>{
    const before=await witnessState(driver);
    for(let index=0;index<3;index++){
      const view=await driver.observe(),capture=await driver.execute('browser_capture',{observation_id:view.observation_id});
      assert.equal(capture.captured,true);assert.deepEqual(await witnessState(driver),before);
    }
  });
});

test('transient viewport resize cannot erase its capture witness by restoring dimensions',async()=>{
  await fixture(async driver=>{
    const before=await witnessState(driver),view=await driver.observe(),screenshot=driver.page.screenshot.bind(driver.page),viewport=driver.page.viewportSize();let captures=0;
    driver.page.screenshot=async options=>{
      captures++;await driver.page.setViewportSize({...viewport,width:viewport.width+100});await driver.page.setViewportSize(viewport);
      return screenshot(options);
    };
    await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),error=>error.code==='computer_browser_observation_failed');
    assert.equal(captures,1);assert.deepEqual(await witnessState(driver),before);
  });
});
