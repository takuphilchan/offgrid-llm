import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {CompanionSession} from '../session.mjs';
import {BrowserWorkspace} from '../browser-workspace.mjs';
import {ManagedBrowserController} from '../managed-controller.mjs';
import {DispatchJournal} from '../journal.mjs';

const deferred=()=>{let resolve;const promise=new Promise(done=>{resolve=done;});return {promise,resolve};};

async function fixture(run,{managed=false}={}){
  const directory=await mkdtemp(join(tmpdir(),'offgrid-session-takeover-'));
  const actions=[],received=new Map();let pairs=0,effects=0,replyGate,replyReceived,loseReply=false;
  const site=createServer((request,response)=>{
    response.setHeader('Content-Type','text/html');
    if(request.url==='/effect'){effects++;response.end('done');return;}
    response.end('<p>Disposable session</p><button onclick="fetch(\'/effect\')">Record</button>');
  });
  const service=createServer(async(request,response)=>{
    const path=request.url;response.setHeader('Content-Type','application/json');
    const chunks=[];let body={};for await(const chunk of request)chunks.push(chunk);
    if(chunks.length)body=JSON.parse(Buffer.concat(chunks).toString('utf8'));
    let result;
    if(path==='/api/v2/system')result={product:'offgrid',api_version:2,workspace_id:'fixture-workspace',capabilities:['browser-discovery-v1','browser-workspace-v1','browser-structured-v1']};
    else if(path.endsWith('/pair'))result={token:'fixture-'+ ++pairs,session:{id:'fixture-session-'+pairs,origin:'offgrid-demo://research'}};
    else if(path.endsWith('/poll'))result={action:actions.shift()??null};
    else if(path.endsWith('/reply')){
      received.get(body.id)?.resolve(body);result={};
      if(body.id==='effect'){replyReceived?.resolve();await replyGate?.promise;}
      if(body.id==='effect'&&loseReply){request.socket.destroy();return;}
    }else {response.statusCode=404;result={};}
    response.end(JSON.stringify(result));
  });
  await Promise.all([new Promise(resolve=>site.listen(0,'127.0.0.1',resolve)),new Promise(resolve=>service.listen(0,'127.0.0.1',resolve))]);
  const origin=`http://127.0.0.1:${site.address().port}`,serviceURL=`http://127.0.0.1:${service.address().port}`;
  const driver=await BrowserWorkspace.open(origin,{headless:true,testLoopback:true});
  const demo={origin,close:async()=>{}};let session,resumed,controller;
  const messages=[],exits=[];
  const options={origin:'demo',code:'a'.repeat(64),workspace:'fixture-workspace',approvalMode:'full_task'};
  const enqueue=action=>{const ready=deferred();received.set(action.id,ready);actions.push(action);return ready.promise;};
  try{
    if(managed){
      controller=new ManagedBrowserController({send:value=>messages.push(value),exit:code=>exits.push(code),
        createSession:config=>new CompanionSession({...config,prepared:config.prepared??{driver,demo}})});
      controller.boot();await controller.command({type:'start',service:serviceURL,directory,...options});session=controller.session;
    }else{session=new CompanionSession({service:serviceURL,directory,prepared:{driver,demo}});await session.start(options);}
    await run({session,driver,options,enqueue,controller,messages,exits,directory,effects:()=>effects,pairs:()=>pairs,
      holdReply:({lost=false}={})=>{loseReply=lost;replyGate=deferred();replyReceived=deferred();return {arrived:replyReceived.promise,release:replyGate.resolve};},
      resume:async()=>{
        if(managed){await controller.command({type:'resume',requestId:'resume-1',previousSession:'fixture-session-1',service:serviceURL,directory,...options});resumed=controller.session;}
        else {resumed=new CompanionSession({service:serviceURL,directory,prepared:session.takeRetained()});await resumed.start(options);}
        return resumed;
      },
    });
  }finally{
    replyGate?.resolve();await controller?.stop();await resumed?.stop();await session?.stop();await driver.close();
    service.closeAllConnections();site.closeAllConnections();
    await Promise.all([new Promise(resolve=>service.close(resolve)),new Promise(resolve=>site.close(resolve))]);
    await rm(directory,{recursive:true,force:true});
  }
}

test('handoff waits for admitted reply then retains the browser with no old polling authority',async()=>{
  await fixture(async({session,driver,enqueue,effects,holdReply})=>{
    const read=await enqueue({id:'observe',kind:'browser_observe',arguments:{},authorization:'auto'});
    const view=JSON.parse(read.result),button=view.elements.find(element=>element.label==='Record');
    const reply=holdReply(),recorded=driver.page.waitForResponse(response=>response.url().endsWith('/effect'));
    const effect=enqueue({id:'effect',kind:'browser_click',arguments:{observation_id:view.observation_id,element:button.id},authorization:'auto'});
    await reply.arrived;await recorded;
    const handoff=session.takeover();let manual=false;handoff.then(()=>{manual=true;});
    await assert.rejects(driver.execute('browser_tabs',{}),/manual control/);
    assert.equal(manual,false,'an unacknowledged admitted result is still settling');
    assert.equal(session.journal.prepare({id:'effect',kind:'browser_click',arguments:{observation_id:view.observation_id,element:button.id}},session.binding).execute,false,'actual journal has the result before handoff');
    reply.release();await effect;assert.equal((await handoff).state,'manual');
    assert.equal(effects(),1);assert.equal(session.token,'');assert.equal(session.driver,null);assert.equal(session.retained.driver,driver);
    assert.equal(driver.browser.isConnected(),true);
    await session.stop();assert.equal(driver.browser.isConnected(),false);
  });
});

test('explicit fresh pairing reuses the retained context without old callbacks closing it',async()=>{
  await fixture(async({session,driver,resume,enqueue,pairs})=>{
    const before=driver.browser.listenerCount('disconnected');
    await session.takeover();assert.equal(session.state,'manual');
    await driver.page.context().addCookies([{name:'synthetic-session',value:'retained',url:driver.page.url()}]);
    const next=await resume();assert.equal(pairs(),2);assert.equal(next.driver,driver);assert.equal(session.retained,undefined);
    assert.equal(driver.browser.listenerCount('disconnected'),before);
    assert.ok((await driver.page.context().cookies()).some(cookie=>cookie.name==='synthetic-session'&&cookie.value==='retained'));
    const reply=await enqueue({id:'new-observe',kind:'browser_observe',arguments:{},authorization:'auto'});
    assert.ok(JSON.parse(reply.result).text.includes('Disposable session'));
    await session.stop();assert.equal(driver.browser.isConnected(),true,'released old owner cannot close new owner browser');
    await next.stop();assert.equal(driver.browser.isConnected(),false);
  });
});

test('closing the retained last tab releases the manual session instead of leaving stale ownership',async()=>{
  await fixture(async({session,driver})=>{
    await session.takeover();await driver.page.close();
    await session.closing;
    assert.equal(driver.browser.isConnected(),false);assert.equal(session.state,'stopped');
    assert.throws(()=>session.takeRetained(),/unavailable/);
  });
});

test('managed handoff replies carry exact identity and duplicate resume never pairs or navigates again',async()=>{
  await fixture(async({controller,driver,messages,exits,resume,pairs})=>{
    assert.equal(messages[0].browserManualProtocol,1);
    const command={type:'takeover',requestId:'handoff-1',session:'fixture-session-1'};
    await Promise.all([controller.command(command),controller.command(command)]);
    assert.equal(exits.length,0);assert.equal(driver.control,'manual');
    assert.equal(messages.filter(value=>value.state==='manual').length,2);
    assert.ok(messages.filter(value=>value.state==='manual').every(value=>value.requestId==='handoff-1'&&value.target.id==='fixture-session-1'));
    const page=driver.page;await resume();
    await controller.command({type:'resume',requestId:'resume-1',previousSession:'fixture-session-1'});
    assert.equal(pairs(),2);assert.equal(driver.page,page);assert.equal(driver.control,'automated');
    assert.equal(messages.filter(value=>value.state==='ready'&&value.requestId==='resume-1').length,2);
    await controller.command({type:'takeover',requestId:'wrong-target',session:'fixture-session-other'});
    assert.equal(messages.at(-1).code,'computer_takeover_unavailable');assert.equal(driver.control,'automated');
  },{managed:true});
});

test('managed Stop supersedes an unacknowledged handoff and closes the retained browser',async()=>{
  await fixture(async({controller,driver,messages,exits,enqueue,holdReply})=>{
    const read=JSON.parse((await enqueue({id:'observe',kind:'browser_observe',arguments:{},authorization:'auto'})).result);
    const gate=holdReply();void enqueue({id:'effect',kind:'browser_click',arguments:{observation_id:read.observation_id,element:read.elements.find(value=>value.label==='Record').id},authorization:'auto'});
    await gate.arrived;
    const handoff=controller.command({type:'takeover',requestId:'handoff-stop',session:'fixture-session-1'});
    await controller.command({type:'stop'});gate.release();await handoff;
    assert.deepEqual(exits,[0]);assert.equal(driver.browser.isConnected(),false);
    assert.equal(messages.some(value=>value.state==='manual'),false);
  },{managed:true});
});

test('lost result acknowledgement at handoff retains uncertainty and forbids redispatch across pairing',async()=>{
  await fixture(async({session,driver,directory,enqueue,effects,holdReply,resume})=>{
    const view=JSON.parse((await enqueue({id:'observe',kind:'browser_observe',arguments:{},authorization:'auto'})).result);
    const action={id:'effect',kind:'browser_click',arguments:{observation_id:view.observation_id,element:view.elements.find(value=>value.label==='Record').id},authorization:'auto'};
    const gate=holdReply({lost:true}),oldBinding=session.binding;
    const recorded=driver.page.waitForResponse(response=>response.url().endsWith('/effect'));void enqueue(action);
    await gate.arrived;await recorded;
    const handoff=session.takeover();gate.release();await handoff;
    assert.equal(session.handoffError,'connection_lost');assert.equal(effects(),1);assert.equal(session.state,'manual');
    const journal=new DispatchJournal(join(directory,'dispatch.sqlite'));
    try{assert.throws(()=>journal.prepare(action,oldBinding),/action_uncertain/);}finally{journal.close();}
    const next=await resume();assert.throws(()=>next.journal.prepare(action,next.binding),/action_conflict/);
    assert.equal(effects(),1,'a renewed session never replays the old uncertain effect');
  });
});
