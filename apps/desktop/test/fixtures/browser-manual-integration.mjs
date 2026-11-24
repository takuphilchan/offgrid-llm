import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {EventEmitter} from 'node:events';
import {createRequire} from 'node:module';
import {BrowserWorkspace} from '../../../computer/browser-workspace.mjs';
import {CompanionSession} from '../../../computer/session.mjs';
import {ManagedBrowserController} from '../../../computer/managed-controller.mjs';
const require=createRequire(import.meta.url);
const {ComputerRuntime}=require('../../computer-runtime.js');
const {computerPrincipal}=require('../../computer-browser-control.js');
const service=process.env.OFFGRID_FIXTURE_SERVICE,directory=process.env.OFFGRID_FIXTURE_DIRECTORY,key=process.env.OFFGRID_FIXTURE_KEY;
const diagnostics=process.env.OFFGRID_BROWSER_MANUAL_DIAGNOSTICS==='1';
if(process.env.OFFGRID_BROWSER_MANUAL_INTEGRATION!=='1'||!service||!directory||!key)throw Error('Explicit isolated fixture required');
const request=async(path,body,{allowError=false}={})=>{
 const response=await fetch(service+path,{method:body===undefined?'GET':'POST',redirect:'error',headers:{Authorization:`Bearer ${key}`,'Content-Type':'application/json',...(body===undefined?{}:{Origin:service})},...(body===undefined?{}:{body:JSON.stringify(body)}),signal:AbortSignal.timeout(15000)});
 const data=await response.json();if(!allowError)assert.ok(response.ok,`${path}: ${response.status} ${JSON.stringify(data)}`);return {response,data};
};
let effects=0,pairs=0,driver,runtime,controller,run;
const trace=[],epoch=performance.now();
const mark=(event)=>{if(diagnostics)trace.push({event,ms:Math.round(performance.now()-epoch)});};
function traceMethod(owner,name,label){
 if(!diagnostics)return;
 const original=owner[name].bind(owner);
 owner[name]=async(...args)=>{mark(label+':begin');try{const result=await original(...args);mark(label+':end');return result;}catch(error){mark(label+':error:'+error.message);throw error;}};
}
const site=createServer((req,res)=>{
 res.setHeader('Content-Type','text/html; charset=utf-8');
 if(req.url==='/effect'){effects++;res.end('recorded');return;}
 if(req.url==='/signin'){res.setHeader('Set-Cookie','fixture-session=authenticated; HttpOnly; SameSite=Lax; Path=/');res.writeHead(302,{Location:'/account'});res.end();return;}
 if(req.url==='/account'){res.end(req.headers.cookie?.includes('fixture-session=authenticated')?'<h1>Signed in fixture account</h1>':'<h1>Signed out</h1>');return;}
 res.end('<button onclick="fetch(\'/effect\')">Record</button><form method="POST" action="/signin"><input name="user" aria-label="Username"><input type="password" name="password"><button>Sign in</button></form>');
});
await new Promise(resolve=>site.listen(0,'127.0.0.1',resolve));
const origin=`http://127.0.0.1:${site.address().port}`;
try{
 driver=await BrowserWorkspace.open(origin,{headless:true,testLoopback:true});
 traceMethod(driver,'close','workspace.close');traceMethod(driver.root,'close','root.close');traceMethod(driver.browser,'close','browser.close');
 driver.browser.on('disconnected',()=>mark('browser.disconnected'));
 const child=new EventEmitter(),messages=[],consents=[];
 let owners=0;
 controller=new ManagedBrowserController({send:message=>{messages.push(message);mark('child.message:'+message.state);queueMicrotask(()=>child.emit('message',message));},exit:code=>{mark('child.exit:'+code);queueMicrotask(()=>child.emit('exit',code));},
  createSession:options=>{
   const label='owner'+ ++owners,owner=new CompanionSession({...options,prepared:options.prepared??{driver,demo:{origin,close:async()=>{mark('demo.close');}}}});
   if(!diagnostics)return owner;
   traceMethod(owner,'stop',label+'.stop');const start=owner.start.bind(owner);
   owner.start=async(...args)=>{
    const result=await start(...args);
    for(const method of ['forgetResults','close']){
     const original=owner.journal[method].bind(owner.journal);
     owner.journal[method]=(...values)=>{mark(label+'.journal.'+method+':begin');try{return original(...values);}finally{mark(label+'.journal.'+method+':end');}};
    }
    void owner.running.then(()=>mark(label+'.running:end'),()=>mark(label+'.running:error'));return result;
   };
   return owner;
  }});
 child.postMessage=message=>{const pending=controller.command(structuredClone(message));if(diagnostics){mark('child.command:'+message.type);void pending.then(()=>mark('child.command:'+message.type+':end'),error=>mark('child.command:'+message.type+':error:'+error.message));}else void pending.catch(()=>{});};
 child.kill=()=>false; // A forced kill would invalidate this ownership fixture.
 runtime=new ComputerRuntime({root:'fixture-pack',directory,service:()=>service,
  identity:async()=> (await request('/api/v2/system')).data,
  principal:async()=>computerPrincipal(await fetch(service+'/v1/users/me',{headers:{Authorization:`Bearer ${key}`}})),
  task:async(_service,id)=>(await request('/api/v2/jobs/'+id)).data,
  pairing:async(_service,body={})=>{pairs++;return (await request('/api/v2/computer/pairing',body)).data;},
  verify:async()=>{},confirm:async(...scope)=>{consents.push(scope);return true;},
  fork:()=>{queueMicrotask(()=>controller.boot());return child;},browserMonitorMs:60000});
 const started=await runtime.start({origin:'demo',workspace:'manual-host-fixture',approvalMode:'full_task',requestId:'input-fixture'});
 const old=started.target.id;
 run=(await request('/fixture/task',{session:old})).data.run_id;
 const tool=async(session,tool,arguments_,{otherRun,allowError=false}={})=>{
  const result=await request('/fixture/tool',{Session:session,Run:otherRun??run,ID:crypto.randomUUID(),Tool:tool,Arguments:arguments_},{allowError});
  return allowError?result:JSON.parse(result.data.result);
 };
 const first=await tool(old,'browser_observe',{}),record=first.elements.find(value=>value.label==='Record');assert.ok(record);
 const recorded=driver.page.waitForResponse(response=>response.url()===origin+'/effect');
 await tool(old,'browser_click',{observation_id:first.observation_id,element:record.id});await recorded;assert.equal(effects,1);
 await tool(old,'browser_tab_open',{url:origin+'/'});
 const page=driver.page,selected=driver.selected,count=driver.tabs.size;
 assert.equal(count,2);
 assert.equal((await runtime.browserControl.takeover({session:old})).state,'manual');
 await request('/api/v2/jobs/'+run+'/takeover',{});
 for(const name of ['browser_observe','browser_tabs'])await assert.rejects(driver.execute(name,{}),/manual control/);
 assert.equal((await tool(old,'browser_observe',{}, {allowError:true})).response.status,409);
 // Synthetic human input only, on the retained disposable browser fixture.
 await page.getByLabel('Username').fill('fixture-user');await page.locator('input[type=password]').fill('fixture-password');
 await page.getByRole('button',{name:'Sign in',exact:true}).click();await page.waitForURL(origin+'/account');
 const cookie=(await page.context().cookies()).find(value=>value.name==='fixture-session');assert.equal(cookie?.value,'authenticated');
 assert.equal(effects,1);await assert.rejects(driver.execute('browser_observe',{}),/manual control/);
 const pending=(await request('/api/v2/jobs/'+run+'/reconnect',{mode:'browser',target:'Disposable fixture',url:'https://fixture.example'})).data;
 assert.equal(pending.pending_input.previous_session,old);
 const resume={workspace:'manual-host-fixture',runId:run,requestId:pending.pending_input.id,session:old};
 const next=await runtime.browserControl.resume(resume);
 assert.equal(next.state,'ready');assert.notEqual(next.target.id,old);assert.deepEqual(consents[1],consents[0]);
 assert.equal(driver.page,page);assert.equal(driver.selected,selected);assert.equal(driver.tabs.size,count);
 assert.equal((await tool(next.target.id,'browser_observe',{}, {otherRun:'run-foreign',allowError:true})).response.status,409);
 await request('/api/v2/jobs/'+run+'/input',{input_id:pending.pending_input.id,computer_session:next.target.id});
 const admitted=(await request('/api/v2/jobs/'+run)).data;
 assert.ok(['pending','running'].includes(admitted.status),JSON.stringify(admitted));
 let resumed;
 try{resumed=await tool(next.target.id,'browser_observe',{});}
 catch(error){throw Error(JSON.stringify({host:runtime.state,session:controller.session.state,control:driver.control,messages,task:(await request('/api/v2/jobs/'+run)).data}),{cause:error});}
 assert.ok(resumed.text.includes('Signed in fixture account'));
 assert.notEqual(resumed.observation_id,first.observation_id);
 assert.deepEqual(await runtime.browserControl.resume({...resume}),next);assert.equal(pairs,2);
 assert.equal((await tool(old,'browser_observe',{}, {allowError:true})).response.status,409);
 assert.equal(effects,1);assert.equal(messages.filter(value=>value.state==='ready').length,2);
 const samePage=driver.page===page;
 await request('/api/v2/jobs/'+run+'/cancel',{});
 const stoppingAt=performance.now();
 const diagnostic=diagnostics?setTimeout(()=>process.stderr.write(JSON.stringify({trace,controller:{starting:!!controller.starting,operationPending:controller.operation?.pending,stopping:controller.stopping},owner:{state:controller.session.state,closePending:controller.session.closePending},resources:process.getActiveResourcesInfo()})+'\n'),10000):undefined;diagnostic?.unref();
 let stopped;try{stopped=await runtime.stopAccess({session:next.target.id});}finally{clearTimeout(diagnostic);}
 assert.equal(stopped.state,'stopped',JSON.stringify({stopped,messages,session:controller.session.state,connected:driver.browser.isConnected()}));
 assert.equal(driver.browser.isConnected(),false);
 process.stdout.write(JSON.stringify({run,effects,pairs,same_page:samePage,authenticated:cookie.value==='authenticated',stopped:runtime.child===null,stop_ms:Math.round(performance.now()-stoppingAt)}));
}catch(error){process.stderr.write(String(error.stack)+'\n'+JSON.stringify({trace})+'\n');throw error;}
finally{
 if(run)await request('/api/v2/jobs/'+run+'/cancel',{}).catch(()=>{});
 await runtime?.stop();await controller?.stop();await driver?.close();site.closeAllConnections();await new Promise(resolve=>site.close(resolve));
}
