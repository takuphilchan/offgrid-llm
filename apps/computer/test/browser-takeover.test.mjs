import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserWorkspace} from '../browser-workspace.mjs';

async function fixture(run){
  let effects=0;
  const server=createServer((req,res)=>{
    res.setHeader('Content-Type','text/html; charset=utf-8');
    if(req.url==='/effect'){effects++;res.end('recorded');return;}
    if(req.url==='/signin'){res.setHeader('Set-Cookie','fixture-session=authenticated; HttpOnly; SameSite=Lax; Path=/');res.writeHead(302,{Location:'/account'});res.end();return;}
    if(req.url==='/account'){res.end(req.headers.cookie?.includes('fixture-session=authenticated')?'<p>Signed in fixture</p>':'<p>Signed out</p>');return;}
    res.end('<button onclick="fetch(\'/effect\')">Record</button><input aria-label="Note"><form method="POST" action="/signin"><input name="user" aria-label="Username"><input name="password" type="password"><button>Sign in</button></form>');
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let workspace;
  const origin=`http://127.0.0.1:${server.address().port}`;
  try{workspace=await BrowserWorkspace.open(origin,{headless:true,testLoopback:true});await run(workspace,origin,()=>effects);}
  finally{await workspace?.close();await new Promise(resolve=>server.close(resolve));}
}

test('takeover drains one admitted real effect and rejects new reads, preparation and input',async()=>{
  await fixture(async(workspace,origin,effects)=>{
    const view=await workspace.execute('browser_observe',{}),button=view.elements.find(el=>el.label==='Record');
    const driver=workspace.current(),execute=driver.execute.bind(driver);
    let release,entered;const boundary=new Promise(resolve=>{entered=resolve;}),gate=new Promise(resolve=>{release=resolve;});
    driver.execute=async(...args)=>{const result=await execute(...args);entered();await gate;return result;};
    const recorded=driver.page.waitForResponse(response=>response.url()===origin+'/effect');
    const effect=workspace.execute('browser_click',{observation_id:view.observation_id,element:button.id});
    await boundary;await recorded;
    const queuedRead=assert.rejects(workspace.execute('browser_observe',{}),/manual control/);
    const handoff=workspace.takeover();let handed=false;handoff.then(()=>{handed=true;});
    try{
      await assert.rejects(workspace.execute('browser_observe',{}),/manual control/);
      await assert.rejects(workspace.execute('browser_tabs',{}),/manual control/);
      await assert.rejects(workspace.prepare('browser_click',{observation_id:view.observation_id,element:button.id}),/manual control/);
      assert.equal(handed,false);assert.equal(workspace.control,'draining');
    }finally{release();}
    await effect;await queuedRead;assert.equal((await handoff).state,'manual');
    assert.equal(effects(),1);assert.equal(driver.observation,'');assert.equal(driver.elements.size,0);
    await assert.rejects(workspace.execute('browser_capture',{observation_id:view.observation_id}),/manual control/);
    assert.equal(effects(),1);
  });
});

test('a handoff timeout does not report manual readiness or reopen automation',async()=>{
  await fixture(async workspace=>{
    let entered,release;const started=new Promise(resolve=>{entered=resolve;}),gate=new Promise(resolve=>{release=resolve;});
    const admitted=workspace.serial(async()=>{entered();await gate;});await started;
    try{
      await assert.rejects(workspace.takeover({timeoutMs:5}),/could not be confirmed/);
      assert.equal(workspace.control,'draining');
      await assert.rejects(workspace.execute('browser_tabs',{}),/manual control/);
      await assert.rejects(workspace.resumeManual(),/unavailable/);
    }finally{release();}
    await admitted;assert.equal((await workspace.takeover()).state,'manual');
  });
});

test('manual authentication retains tabs and cookies while old evidence cannot resume input',async()=>{
  await fixture(async(workspace,origin)=>{
    const first=await workspace.execute('browser_observe',{}),input=first.elements.find(el=>el.label==='Note');
    await workspace.execute('browser_tab_open',{url:origin+'/'});
    const count=workspace.tabs.size,selected=workspace.selected,page=workspace.page;
    await workspace.takeover();
    // Explicit synthetic human input in this disposable browser only.
    await page.getByLabel('Username').fill('fixture-user');await page.locator('input[type=password]').fill('fixture-password');
    await page.getByRole('button',{name:'Sign in',exact:true}).click();
    await page.waitForURL(origin+'/account');
    await assert.rejects(workspace.execute('browser_observe',{}),/manual control/);
    await workspace.resumeManual();
    assert.equal(workspace.tabs.size,count);assert.equal(workspace.selected,selected);assert.equal(workspace.page,page);
    const view=await workspace.execute('browser_observe',{});assert.ok(view.text.includes('Signed in fixture'));
    assert.notEqual(view.observation_id,first.observation_id);
    await workspace.execute('browser_tab_select',{tab_id:first.tab_id});
    await assert.rejects(workspace.execute('browser_fill',{observation_id:first.observation_id,element:input.id,text:'stale'}),/Stale observation/);
    await workspace.execute('browser_navigate',{url:origin+'/account'});
    assert.ok((await workspace.execute('browser_observe',{})).text.includes('Signed in fixture'));
  });
});

test('manual selected-tab closure has no automatic fallback and Stop closes retained browser',async()=>{
  await fixture(async(workspace,origin)=>{
    await workspace.execute('browser_tab_open',{url:origin+'/'});
    await workspace.takeover();await workspace.page.close();
    assert.equal(workspace.selected,'');assert.equal(workspace.tabs.size,1);
    await workspace.resumeManual();
    await assert.rejects(workspace.execute('browser_observe',{}),/Selected task tab is unavailable/);
    const inventory=await workspace.execute('browser_tabs',{});assert.equal(inventory.selected_tab_id,'');
    await workspace.execute('browser_tab_select',{tab_id:inventory.tabs[0].tab_id});
    await workspace.takeover();await workspace.close();
    assert.equal(workspace.browser.isConnected(),false);
    await assert.rejects(workspace.resumeManual(),/unavailable/);
  });
});
