// Run with Electron, not Node. Real utility process + bundled Chromium; only our
// owned ephemeral demo is automated by default. The explicit public-docs flag
// performs read-only navigation against one public site, using consented VPN
// routing. Neither mode qualifies model planning or general computer control.
const {app,utilityProcess}=require('electron');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {tmpdir}=require('node:os');
const {createServer}=require('node:http');
const {ComputerRuntime}=require('../../desktop/computer-runtime');
let runtime,server;
const publicDocs=process.argv.includes('--public-docs-trusted-vpn');
const target=publicDocs?'https://playwright.dev/docs/intro':'demo';
const origin=publicDocs?'https://playwright.dev':'offgrid-demo://research';
app.whenReady().then(async()=>{
 const directory=await fs.mkdtemp(path.join(tmpdir(),'offgrid-desktop-browser-'));
 app.setPath('userData',path.join(directory,'electron'));
 const root=path.resolve(__dirname,'../../build/computer-runtime',`${{win32:'win',darwin:'mac',linux:'linux'}[process.platform]}-${process.arch}`);
 let next={id:'observe-1',kind:'browser_observe',arguments:{}};
 let complete,fail;const completed=new Promise((resolve,reject)=>{complete=resolve;fail=reject;});
 let paired=0,checks=0;const replies=[];const code='c'.repeat(64);
 server=createServer(async(req,res)=>{
  try{
   let raw='';for await(const chunk of req)raw+=chunk;const body=raw?JSON.parse(raw):{};
   let result={};
   if(req.url==='/api/v2/system')result={product:'offgrid',api_version:2,workspace_id:'owned-fixture'};
   else if(req.url.endsWith('/pair')){assert.equal(body.code,code);assert.equal(body.origin,origin);paired++;result={token:'synthetic-local-token',session:{id:'owned-session',origin:body.origin}};}
   else {
    assert.equal(req.headers.authorization,'Bearer synthetic-local-token');
    if(req.url.endsWith('/poll')){result={action:next};next=null;}
    else if(req.url.endsWith('/reply')){
     assert.equal(body.error,undefined);const view=JSON.parse(body.result);replies.push(body.id);
     if(publicDocs && body.id==='observe-1') {
      assert.match(view.text,/Installation/);
      const link=view.elements.find(e=>e.href==='https://playwright.dev/docs/writing-tests');assert.ok(link,'Public link must be observed, not invented');
      next={id:'navigate-1',kind:'browser_navigate',arguments:{url:link.href}};
     }
     else if(publicDocs && body.id==='navigate-1') {
      assert.equal(view.url,'https://playwright.dev/docs/writing-tests');
      next={id:'verify-1',kind:'browser_verify',arguments:{text:'Writing tests'}};
     }
     else if(body.id==='observe-1')next={id:'fill-1',kind:'browser_fill',arguments:{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Report title').id,text:'Desktop integrated test'}};
     else if(body.id==='fill-1')next={id:'click-1',kind:'browser_click',arguments:{observation_id:view.observation_id,element:view.elements.find(e=>e.label==='Save draft').id}};
     else if(body.id==='click-1'){assert.match(view.text,/Draft saved: Desktop integrated test/);next={id:'verify-1',kind:'browser_verify',arguments:{text:'Draft saved: Desktop integrated test'}};}
     else if(body.id==='verify-1'){assert.equal(view.verified,true);complete();}
    }
   }
   res.setHeader('Content-Type','application/json');res.end(JSON.stringify(result));
  }catch(error){fail(error);res.statusCode=500;res.end('{}');}
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const service=`http://127.0.0.1:${server.address().port}`;
 runtime=new ComputerRuntime({root,directory,service:()=>service,fork:(...args)=>utilityProcess.fork(...args),
  identity:async()=>({product:'offgrid',api_version:2,workspace_id:'owned-fixture'}),
  pairing:async()=>({code}),
  verify:folder=>require(path.join(folder,'pack.cjs')).verifyPack(folder),
  // Public network trust requires the explicit operator flag. Never turn this
  // into wildcard consent or use it with a user's workspace/service.
  confirm:async(selected,url,networkMode)=>{assert.equal(selected,target);assert.equal(url,service);assert.equal(networkMode,publicDocs?'trusted-vpn':'direct');checks++;return true;}});
 const timeout=setTimeout(()=>fail(Error('Real desktop browser test timed out')),45000);
 try{
  const state=await runtime.start({origin:target,workspace:'owned-fixture',networkMode:publicDocs?'trusted-vpn':'direct'});
  assert.equal(state.state,'ready');await completed;
  await runtime.stop();assert.equal(runtime.child,null);assert.equal(runtime.state.state,'stopped');
  assert.equal(paired,1);assert.equal(checks,1);assert.deepEqual(replies,publicDocs?['observe-1','navigate-1','verify-1']:['observe-1','fill-1','click-1','verify-1']);
  console.log(publicDocs?'PASS: bundled runtime, Electron utility IPC, real HTTPS documentation observe/link navigation/verify through trusted VPN, owned shutdown':'PASS: packaged runtime integrity, private IPC, real Chromium observe/fill/save/verify, owned shutdown');
  console.log(`Isolated evidence: ${directory}`);
 }finally{clearTimeout(timeout);await runtime.stop();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
 app.exit(0);
}).catch(async error=>{console.error(error.message);await runtime?.stop();server?.closeAllConnections();server?.close();app.exit(1);});
