import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserDriver} from '../browser.mjs';

async function paintFixture(privateHTML,run){
  const html='<style>body{margin:0}</style>'+privateHTML+'<div style="position:fixed;left:250px;top:0;width:80px;height:50px;background:rgb(0,255,0)">Public</div>';
  const server=createServer((request,response)=>{response.setHeader('Content-Type','text/html');response.end(html);});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
  try{
    driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
    await run(driver);
  }finally{await driver?.close();await new Promise(resolve=>server.close(resolve));}
}

async function inspectPrivatePaint(privateHTML,point){
  await paintFixture(privateHTML,async driver=>{
    const view=await driver.observe();assert.ok(!view.text.includes('PRIVATE-SENTINEL'));
    const captured=await driver.execute('browser_capture',{observation_id:view.observation_id});
    const pixels=await driver.page.evaluate(async({data,points})=>{
      const image=new Image();image.src='data:image/png;base64,'+data;await image.decode();
      const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;
      const context=canvas.getContext('2d');context.drawImage(image,0,0);
      return points.map(([x,y])=>Array.from(context.getImageData(x,y,1,1).data));
    },{data:captured.data,points:[point,[275,40]]});
    assert.deepEqual(pixels[0],[0,0,0,255],'private descendant paint must be masked');
    assert.deepEqual(pixels[1],[0,255,0,255],'established public geometry remains useful');
  });
}

test('private display-contents container cannot lose its visible descendant mask',async()=>{
  await inspectPrivatePaint('<div data-offgrid-private style="display:contents"><span style="display:block;width:100px;height:50px;background:rgb(255,0,0)">PRIVATE-SENTINEL</span></div>',[20,40]);
});

test('private overflow paint outside a small ancestor is masked',async()=>{
  await inspectPrivatePaint('<div data-offgrid-private style="width:1px;height:1px"><span style="position:fixed;left:120px;top:0;width:100px;height:50px;background:rgb(255,0,0)">PRIVATE-SENTINEL</span></div>',[140,40]);
});

test('ordinary private box masking preserves unrelated public pixels',async()=>{
  await inspectPrivatePaint('<div data-offgrid-private style="width:100px;height:50px;background:rgb(255,0,0)">PRIVATE-SENTINEL</div>',[20,40]);
});

async function countRedGlyphs(driver,data){
  return driver.page.evaluate(async data=>{
    const image=new Image();image.src='data:image/png;base64,'+data;await image.decode();
    const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;
    const context=canvas.getContext('2d');context.drawImage(image,0,0);
    const pixels=context.getImageData(0,0,240,80).data;let red=0;
    for(let i=0;i<pixels.length;i+=4)if(pixels[i]>100&&pixels[i+1]<pixels[i]/2&&pixels[i+2]<pixels[i]/2&&pixels[i+3]>0)red++;
    return {red,publicPixel:Array.from(context.getImageData(275,40,1,1).data)};
  },data);
}

test('private direct text in display-contents masks glyph paint while preserving public pixels',async()=>{
  await paintFixture('<div data-offgrid-private style="display:contents;color:rgb(255,0,0);font:40px monospace">PRIVATE</div>',async driver=>{
    const raw=await driver.page.screenshot({type:'png'});
    try{assert.ok((await countRedGlyphs(driver,raw.toString('base64'))).red>100,'the positive control must contain visible private glyphs');}
    finally{raw.fill(0);}
    const view=await driver.observe();assert.ok(!view.text.includes('PRIVATE'));
    const captured=await driver.execute('browser_capture',{observation_id:view.observation_id});
    const pixels=await countRedGlyphs(driver,captured.data);
    assert.equal(pixels.red,0,'all private glyph pixels, including antialiasing, must be masked');
    assert.deepEqual(pixels.publicPixel,[0,255,0,255],'unrelated public paint must remain visible');
  });
});

for(const [name,html,message] of [
  ['generated pseudo-element','<style>#private::before{content:"PRIVATE";position:fixed;left:120px;top:0;color:red;font:40px monospace}</style><div id="private" data-offgrid-private style="width:1px;height:1px"></div>',/generated paint/],
  ['filter drop-shadow','<div data-offgrid-private style="width:80px;height:50px;background:red;filter:drop-shadow(120px 0 red)">PRIVATE</div>',/paint effects/],
  ['ancestor filter','<div style="filter:drop-shadow(120px 0 red)"><div data-offgrid-private style="width:80px;height:50px;background:red">PRIVATE</div></div>',/paint effects/],
  ['unbounded text shadow','<div data-offgrid-private style="color:red;font:40px monospace;text-shadow:120px 0 red">PRIVATE</div>',/paint effects/],
  ['unbounded box shadow','<div data-offgrid-private style="width:80px;height:50px;background:red;box-shadow:120px 0 20px 10px red">PRIVATE</div>',/paint effects/],
])test(`capture rejects protected ${name} without publishing an image`,async()=>{
  await paintFixture(html,async driver=>{
    const view=await driver.observe();
    await assert.rejects(driver.execute('browser_capture',{observation_id:view.observation_id}),error=>{
      assert.equal(error.code,'computer_browser_observation_failed');assert.match(error.message,message);return true;
    });
  });
});
