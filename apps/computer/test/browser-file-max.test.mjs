import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash,randomUUID} from 'node:crypto';
import {createServer} from 'node:http';
import {mkdtemp,open,writeFile,rm,statfs} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join,resolve,dirname} from 'node:path';
import {BrowserDriver} from '../browser.mjs';
const approvedSize=512*1024*1024; // Approved product contract, independent of implementation constants.

// This explicit maximum-size qualification consumes about 1 GiB of temporary
// disk. Normal source suites keep it visible as unrun, not a smaller proxy pass.
test('maximum admitted upload verifies and transmits every approved byte after original replacement',{
 skip:process.env.OFFGRID_BROWSER_FILE_MAX_TEST!=='1',timeout:180000,
},async t=>{
 const available=await statfs(tmpdir(),{bigint:true});
 assert.ok(available.bavail*available.bsize>=2n*1024n*1024n*1024n,'maximum-size fixture requires 2 GiB free temporary disk');
 const directory=await mkdtemp(join(tmpdir(),'offgrid-browser-file-max-'));
 assert.equal(dirname(resolve(directory)),resolve(tmpdir()));
 const path=join(directory,'maximum.bin'),receipts=[];let driver,server,stage='creating_fixture',receivedBytes=0;
 try{
  const file=await open(path,'wx',0o600),hash=createHash('sha256'),chunk=Buffer.alloc(1024*1024,0x51);
  try{
   for(let offset=0;offset<approvedSize;offset+=chunk.length){
    let written=0;while(written<chunk.length){const result=await file.write(chunk,written,chunk.length-written);assert.ok(result.bytesWritten);written+=result.bytesWritten;}
    hash.update(chunk);
   }
   await file.sync();
  }finally{chunk.fill(0);await file.close();}
  const sha256=hash.digest('hex');
  server=createServer(async(req,res)=>{
   if(req.url==='/received'){
    const received=createHash('sha256');let size=0;
    try{for await(const bytes of req){size+=bytes.length;receivedBytes=size;received.update(bytes);}}
    catch(error){t.diagnostic(`Receiver failed after ${size} bytes: ${error.code??error.name}`);return;}
    receipts.push({size,sha256:received.digest('hex')});res.end('received');return;
   }
   res.setHeader('Content-Type','text/html');
   res.end('<input type="file" aria-label="Attach maximum"><script>document.querySelector("input").addEventListener("change",event=>{const file=event.target.files[0];window.sendSelected=()=>fetch("/received",{method:"POST",body:file});});</script>');
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const origin=`http://127.0.0.1:${server.address().port}`;
  driver=await BrowserDriver.open(origin,{headless:true,testLoopback:true,upload:{id:randomUUID().replaceAll('-',''),path,name:'maximum.bin',size:approvedSize,sha256}});
  driver.page.on('crash',()=>t.diagnostic(`Chromium page crashed during ${stage}`));
  driver.browser.on('disconnected',()=>t.diagnostic(`Chromium disconnected during ${stage}`));
  stage='verifying_selection';
  const observed=await driver.observe(),element=observed.elements.find(item=>item.label==='Attach maximum');assert.ok(element);
  const result=await driver.execute('browser_upload',{observation_id:observed.observation_id,element:element.id});
  assert.equal(result.file.size,approvedSize);assert.equal(result.file.sha256,sha256);assert.equal(result.verification_scope,'file_input_selection');
  assert.deepEqual(receipts,[],'selection receipt must precede any independent HTTP receipt');
  stage='selection_verified';t.diagnostic(`512 MiB native selection verified; Node memory ${JSON.stringify(process.memoryUsage())}`);
  await writeFile(path,'original source replaced after selection');
  stage='transmitting_selected_file';
  const received=driver.page.waitForResponse(response=>response.url()===origin+'/received',{timeout:120000});
  const [ok]=await Promise.all([driver.page.evaluate(()=>window.sendSelected().then(response=>response.ok)),received]);
  assert.equal(ok,true);
  assert.deepEqual(receipts,[{size:approvedSize,sha256}]);
 }finally{
  t.diagnostic(`Final stage ${stage}; independent receiver bytes ${receivedBytes}`);stage='closing_fixture';
  await driver?.close();
  if(server){server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
  await rm(directory,{recursive:true,force:true});
 }
});
