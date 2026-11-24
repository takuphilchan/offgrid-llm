import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {createServer} from 'node:http';
import {mkdtemp,open,rm,statfs} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {dirname,join,resolve} from 'node:path';
import {chromium} from 'playwright';

const size=512*1024*1024;

// A diagnostic control, never an alternate product transport. This deliberately
// requires successful delivery in every mode: the intercepted raw-file case must
// remain red while Chromium cannot send the approved maximum file.
test('512 MiB native File transport controls with independent streamed receiver',{
 skip:process.env.OFFGRID_BROWSER_TRANSPORT_PROBE!=='1',timeout:240000,
},async t=>{
 const available=await statfs(tmpdir(),{bigint:true});
 assert.ok(available.bavail*available.bsize>=1024n*1024n*1024n);
 const directory=await mkdtemp(join(tmpdir(),'offgrid-file-transport-probe-'));
 assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
 const path=join(directory,'transport.bin');
 try{
  const file=await open(path,'wx',0o600),source=createHash('sha256'),chunk=Buffer.alloc(1024*1024,0x51);
  try{
   for(let offset=0;offset<size;offset+=chunk.length){
    let written=0;while(written<chunk.length){const r=await file.write(chunk,written,chunk.length-written);assert.ok(r.bytesWritten);written+=r.bytesWritten;}
    source.update(chunk);
   }
   await file.sync();
  }finally{chunk.fill(0);await file.close();}
  const sha256=source.digest('hex');
  for(const multipart of [false,true])for(const intercepted of [false,true]){
   await t.test(`${multipart?'multipart':'raw'}; interception=${intercepted}`,{timeout:55000},async child=>{
    let browser,server,wireBytes=0,payloadBytes=0,requests=0,interceptedPosts=0,receiverError='',stage='starting';
    const receipts=[],began=performance.now();
    try{
     server=createServer(async(req,res)=>{
      if(req.url!=='/received'){res.setHeader('Content-Type','text/html');res.end('<input type="file">');return;}
      requests++;
      const hash=createHash('sha256');let pending=Buffer.alloc(0),headerDone=!multipart,trailer;
      try{
       if(multipart){
        const contentType=req.headers['content-type']??'';
        const match=/^multipart\/form-data; boundary=([A-Za-z0-9-]{1,100})$/.exec(contentType);assert.ok(match,contentType);
        trailer=Buffer.from(`\r\n--${match[1]}--\r\n`);
       }
       for await(const bytes of req){
        wireBytes+=bytes.length;
        if(!multipart){payloadBytes+=bytes.length;hash.update(bytes);continue;}
        pending=Buffer.concat([pending,bytes]);
        if(!headerDone){
         const end=pending.indexOf('\r\n\r\n');
         if(end<0){assert.ok(pending.length<4096,'bounded multipart header');continue;}
         assert.ok(end<4096);assert.match(pending.subarray(0,end).toString('ascii'),/name="file"; filename="transport.bin"/);
         pending=pending.subarray(end+4);headerDone=true;
        }
        const count=Math.max(0,pending.length-trailer.length);
        if(count){hash.update(pending.subarray(0,count));payloadBytes+=count;pending=Buffer.from(pending.subarray(count));}
       }
       if(multipart){assert.ok(headerDone);assert.deepEqual(pending,trailer);}
       receipts.push({size:payloadBytes,sha256:hash.digest('hex')});res.end('received');
      }catch(error){receiverError=error.code??error.message;res.destroy();}
     });
     await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
     const origin=`http://127.0.0.1:${server.address().port}`;
     browser=await chromium.launch({headless:true});
     child.diagnostic(`Browser ${browser.version()}; ${new Date().toISOString()}`);
     browser.on('disconnected',()=>child.diagnostic(`Disconnected during ${stage}; ${Math.round(performance.now()-began)} ms`));
     const context=await browser.newContext({serviceWorkers:'block'});
     if(intercepted)await context.route('**/*',route=>{
      if(route.request().method()==='POST')interceptedPosts++;
      return route.continue();
     });
     const page=await context.newPage();
     page.on('crash',()=>child.diagnostic(`Page crash during ${stage}`));
     await page.goto(origin,{waitUntil:'domcontentloaded'});
     await page.locator('input').setInputFiles(path);
     assert.equal(await page.locator('input').evaluate(input=>input.files[0].size),size);
     stage='transmitting';
     const response=await page.evaluate(async multipart=>{
      const file=document.querySelector('input').files[0];
      const body=multipart?new FormData():file;
      if(multipart)body.append('file',file);
      const result=await fetch('/received',{method:'POST',body,signal:AbortSignal.timeout(35000)});
      return {ok:result.ok,text:await result.text()};
     },multipart);
     assert.deepEqual(response,{ok:true,text:'received'});
     assert.equal(requests,1);assert.equal(receiverError,'');
     assert.deepEqual(receipts,[{size,sha256}]);
     assert.equal(interceptedPosts,intercepted?1:0);
     stage='verified';
    }finally{
     child.diagnostic(JSON.stringify({stage,elapsedMs:Math.round(performance.now()-began),requests,interceptedPosts,wireBytes,payloadBytes,receipts,receiverError,memory:process.memoryUsage()}));
     stage='closing';
     try{await browser?.close();}
     finally{if(server){server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}}
    }
   });
  }
 }finally{assert.equal(dirname(resolve(directory)),resolve(tmpdir()));await rm(directory,{recursive:true,force:true});}
});
