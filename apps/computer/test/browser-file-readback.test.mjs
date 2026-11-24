import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash,randomUUID} from 'node:crypto';
import {createServer} from 'node:http';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,dirname,resolve} from 'node:path';
import {chromium} from 'playwright';
import {BrowserFileSnapshot} from '../browser-file-snapshot.mjs';
import {registerFileReadback,verifySelectedBrowserFile} from '../browser-file-readback.mjs';

const digest=value=>createHash('sha256').update(value).digest('hex');
await registerFileReadback();
async function fixture(run,{size=1024}={}){
  const directory=await mkdtemp(join(tmpdir(),'offgrid-readback-'));
  assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
  const content=Buffer.alloc(size,0x51),path=join(directory,'selected.txt');await writeFile(path,content);
  const snapshot=new BrowserFileSnapshot(join(directory,'staging'));
  const grant={id:randomUUID().replaceAll('-',''),path,name:'selected.txt',size,sha256:digest(content)};
  const server=createServer((req,res)=>{res.setHeader('Content-Type','text/html');res.end('<input type="file"><span>sentinel</span>');});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let browser;
  try{
    await snapshot.prepare(grant);browser=await chromium.launch({headless:true});
    const page=await browser.newPage();await page.goto(`http://127.0.0.1:${server.address().port}`);
    const handle=await page.$('input');await handle.setInputFiles(snapshot.path);
    await run({page,handle,snapshot,content,path});
  }finally{
    await browser?.close();await snapshot.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));
    await rm(directory,{recursive:true,force:true});
  }
}

test('isolated readback verifies actual selected bytes without DOM mutation or byte publication',async()=>{
  await fixture(async({page,handle,snapshot})=>{
    await page.evaluate(()=>{window.mutations=0;new MutationObserver(records=>window.mutations+=records.length).observe(document,{childList:true,attributes:true,characterData:true,subtree:true});});
    assert.deepEqual(await verifySelectedBrowserFile(handle,snapshot),{name:snapshot.metadata.name,size:snapshot.metadata.size,sha256:snapshot.metadata.sha256});
    assert.equal(await page.evaluate(()=>window.mutations),0);
  });
});

test('empty selected files use complete empty-content evidence',async()=>{
  await fixture(async({handle,snapshot})=>{
    assert.deepEqual(await verifySelectedBrowserFile(handle,snapshot),{name:'selected.txt',size:0,sha256:digest(Buffer.alloc(0))});
  },{size:0});
});

test('isolated readback ignores page File, crypto, Element and own-input getter spoofing',async()=>{
  await fixture(async({page,handle,snapshot})=>{
    await page.evaluate(()=>{
      const input=document.querySelector('input');
      Object.defineProperty(input,'files',{get:()=>[{name:'fake',size:0}]});
      Object.defineProperty(HTMLInputElement.prototype,'files',{get:()=>[{name:'fake',size:0}]});
      Object.defineProperty(File.prototype,'name',{get:()=> 'fake'});
      Object.defineProperty(Blob.prototype,'size',{get:()=>0});
      Blob.prototype.slice=()=>{throw Error('hostile slice');};
      FileReader.prototype.readAsArrayBuffer=()=>{throw Error('hostile reader');};
      Element.prototype.getAttribute=()=> 'spoof';
      Object.defineProperty(Node.prototype,'isConnected',{get:()=>false});
      crypto.subtle.digest=()=>Promise.resolve(new Uint8Array(32).buffer);
    });
    assert.equal((await verifySelectedBrowserFile(handle,snapshot)).sha256,snapshot.metadata.sha256);
  });
});

test('same-name same-size replacement bytes reject even when main-world getters claim the approved file',async()=>{
  await fixture(async({page,handle,snapshot,content})=>{
    await handle.setInputFiles({name:'selected.txt',mimeType:'text/plain',buffer:Buffer.alloc(content.length,0x58)});
    await page.evaluate(({size})=>{Object.defineProperty(document.querySelector('input'),'files',{get:()=>[{name:'selected.txt',size,arrayBuffer:()=>new Uint8Array(size).fill(0x51).buffer}]});},{size:content.length});
    assert.equal(await handle.evaluate((input,name)=>input.files?.length===1&&input.files[0].name===name,'selected.txt'),true,'the previous name-only readback accepts this wrong-byte selection');
    await assert.rejects(verifySelectedBrowserFile(handle,snapshot),/content differs/);
  });
});

test('detached or replaced retained input cannot be silently rebound to a matching control',async()=>{
  for(const replacement of [false,true])await fixture(async({page,handle,snapshot})=>{
    await page.evaluate(replace=>{const old=document.querySelector('input');if(replace)old.replaceWith(old.cloneNode());else old.remove();},replacement);
    await assert.rejects(verifySelectedBrowserFile(handle,snapshot),/target changed/);
  });
});

test('readback supports delayed native File content larger than Playwright buffer limit',async()=>{
  await fixture(async({handle,snapshot,path})=>{
    await writeFile(path,'changed original source');
    assert.equal((await verifySelectedBrowserFile(handle,snapshot)).size,51*1024*1024);
  },{size:51*1024*1024});
});

test('abort and timeout retire isolated readback state so another explicit verification can run',async()=>{
  await fixture(async({handle,snapshot})=>{
    const abort=new AbortController();abort.abort();
    await assert.rejects(verifySelectedBrowserFile(handle,snapshot,{signal:abort.signal}),error=>error.name==='AbortError');
    await assert.rejects(verifySelectedBrowserFile(handle,snapshot,{timeoutMs:1}),/timed out/);
    assert.equal((await verifySelectedBrowserFile(handle,snapshot)).sha256,snapshot.metadata.sha256);
  },{size:2*1024*1024});
});

test('active chunk cancellation releases all eight bounded isolated state slots',async()=>{
  await fixture(async({handle,snapshot})=>{
    const query=handle.$.bind(handle);let activeAbort,started=0;
    handle.$=async selector=>{
      const result=await query(selector);
      if(JSON.parse(selector.slice(selector.indexOf('=')+1)).op==='chunk'&&activeAbort){started++;activeAbort.abort();}
      return result;
    };
    for(let index=0;index<10;index++){
      activeAbort=new AbortController();
      await assert.rejects(verifySelectedBrowserFile(handle,snapshot,{signal:activeAbort.signal}),error=>error.name==='AbortError');
    }
    assert.equal(started,10);activeAbort=undefined;
    assert.equal((await verifySelectedBrowserFile(handle,snapshot)).sha256,snapshot.metadata.sha256);
  },{size:2*1024*1024});
});

test('selection replacement during native chunk read invalidates evidence without reselecting',async()=>{
  await fixture(async({page,handle,snapshot})=>{
    const query=handle.$.bind(handle);let replaced=false;
    handle.$=async selector=>{
      const result=await query(selector);
      if(!replaced&&JSON.parse(selector.slice(selector.indexOf('=')+1)).op==='chunk'){
        replaced=true;await page.evaluate(()=>{const replacement=document.createElement('input');replacement.type='file';document.querySelector('input').replaceWith(replacement);});
      }
      return result;
    };
    await assert.rejects(verifySelectedBrowserFile(handle,snapshot),/target changed|content differs/);
    assert.equal(replaced,true);assert.equal(await page.locator('input').inputValue(),'');
  },{size:2*1024*1024});
});

test('exact retained input in a child frame is verified without requerying another frame',async()=>{
  await fixture(async({page,handle,snapshot})=>{
    const navigated=page.waitForEvent('framenavigated',{predicate:frame=>frame!==page.mainFrame()});
    await page.evaluate(()=>{const frame=document.createElement('iframe');frame.srcdoc='<input type="file">';document.body.append(frame);});
    const child=await navigated;
    const childHandle=await child.waitForSelector('input');await childHandle.setInputFiles(snapshot.path);
    await handle.setInputFiles({name:'selected.txt',mimeType:'text/plain',buffer:Buffer.alloc(snapshot.metadata.size,0x58)});
    assert.equal((await verifySelectedBrowserFile(childHandle,snapshot)).sha256,snapshot.metadata.sha256);
    await assert.rejects(verifySelectedBrowserFile(handle,snapshot),/content differs/);
    await childHandle.dispose();
  });
});
