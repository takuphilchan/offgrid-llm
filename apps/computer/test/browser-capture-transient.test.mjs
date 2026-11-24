import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserDriver} from '../browser.mjs';

// Independent review reproduction: a field exists only while the image is
// captured, then the DOM and every retained property return to their old values.
// Comparing only final markup/property hashes cannot witness this change.
test('capture rejects a transient sensitive field absent from both endpoint snapshots',async()=>{
  const server=createServer((request,response)=>{response.setHeader('Content-Type','text/html');response.end('<p>Public content</p>');});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  let driver;
  try{
    driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
    const view=await driver.observe(),screenshot=driver.page.screenshot.bind(driver.page);
    let captures=0;
    driver.page.screenshot=async options=>{
      captures++;
      await driver.page.evaluate(()=>{
        const node=document.createElement('input');node.id='review-transient-sensitive';
        node.setAttribute('aria-label','Secret token');node.value='REVIEW-PRIVATE-SENTINEL';
        node.style.cssText='position:fixed;top:0;left:0;width:100px;height:50px;background:rgb(255,0,0);z-index:999999';
        document.body.append(node);
      });
      try{return await screenshot(options);}
      finally{await driver.page.locator('#review-transient-sensitive').evaluate(node=>node.remove());}
    };
    try{
      await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),error=>error.code==='computer_browser_observation_failed');
    }finally{
      // If capture changes transport, move this fixture's hook to that actual
      // boundary. A never-executed injection is not privacy evidence.
      assert.equal(captures,1,'the transient mutation must surround the actual capture');
    }
  }finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
});

async function transientRenderFixture(html,mutate,restore,{animationPreflight=false}={}){
  const server=createServer((request,response)=>{response.setHeader('Content-Type','text/html');response.end(html);});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
  try{
    driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
    const view=await driver.observe(),screenshot=driver.page.screenshot.bind(driver.page);let captures=0,error;
    driver.page.screenshot=async options=>{
      captures++;await driver.page.evaluate(mutate);
      try{return await screenshot(options);}finally{await driver.page.evaluate(restore);}
    };
    try{await driver.execute('browser_capture',{observation_id:view.observation_id});}catch(caught){error=caught;}
    assert.equal(error?.code,'computer_browser_observation_failed','transient movement must not expose unmasked content');
    if(captures===0&&animationPreflight){
      // Explicit rejection of this already-present animation is a valid closed
      // boundary; any unrelated early error must not count as injected evidence.
      assert.match(error.message,/animation/i);
    }else assert.equal(captures,1,'the transient rendering change must surround the actual capture');
  }finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
}

test('capture rejects transient CSSOM geometry changes invisible to DOM observers',async()=>{
  await transientRenderFixture(
    '<style>input{position:fixed;top:0;left:0;width:100px;height:50px;background:rgb(255,0,0)}</style><input aria-label="Secret token" value="REVIEW-PRIVATE-SENTINEL">',
    ()=>document.styleSheets[0].insertRule('input{transform:translateX(100px)}',1),
    ()=>document.styleSheets[0].deleteRule(1),
  );
});

test('capture rejects transient paused-animation movement with unchanged endpoint geometry',async()=>{
  await transientRenderFixture(
    '<input aria-label="Secret token" value="REVIEW-PRIVATE-SENTINEL" style="position:fixed;top:0;left:0;width:100px;height:50px;background:rgb(255,0,0)"><script>window.reviewAnimation=document.querySelector("input").animate([{transform:"translateX(0px)"},{transform:"translateX(200px)"}],{duration:1000,fill:"both"});reviewAnimation.pause();reviewAnimation.currentTime=0;</script>',
    ()=>{window.reviewAnimation.currentTime=500;},
    ()=>{window.reviewAnimation.currentTime=0;},
    {animationPreflight:true},
  );
});
