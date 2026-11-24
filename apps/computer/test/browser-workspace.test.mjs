import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserWorkspace} from '../browser-workspace.mjs';
import {safeBrowserFailure} from '../browser-outcome.mjs';

async function fixture(run) {
  const servers=[];const sites=[];let workspace;
  for(let i=0;i<3;i++) {
    const server=createServer((req,res)=>{
      res.setHeader('content-type','text/html');
      res.end(`<title>Site ${i}</title><h1>Site ${i}</h1><label>Note<input></label><button onclick="window.count=(window.count||0)+1">Add</button><a href="${sites[1]??'/'}" target="_blank">Other</a>`);
    });
    await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
    servers.push(server);sites.push(`http://127.0.0.1:${server.address().port}`);
  }
  try {
    workspace=await BrowserWorkspace.open(sites[0],{headless:true,testLoopback:true,additionalOrigins:[sites[1]]});
    await run(workspace,sites);
  } finally {
    await workspace?.close();
    await Promise.all(servers.map(server=>{server.closeAllConnections();return new Promise(resolve=>server.close(resolve));}));
  }
}

test('task tabs keep content and reject cross-tab references while navigating approved sites',async()=>fixture(async(w,sites)=>{
  const first=await w.execute('browser_observe',{});
  const input=first.elements.find(e=>e.tag==='input');
  await w.execute('browser_fill',{observation_id:first.observation_id,element:input.id,text:'retained'});
  const second=await w.execute('browser_tab_open',{url:sites[1]});
  assert.equal((await w.prepare('browser_tab_close',{tab_id:second.tab_id})).approval_class,'consequential');
  assert.notEqual(first.tab_id,second.tab_id);assert.match(second.text,/Site 1/);
  await assert.rejects(w.execute('browser_fill',{observation_id:first.observation_id,element:input.id,text:'wrong'}),safeBrowserFailure);
  assert.equal(await w.page.locator('input').inputValue(),'');
  const selected=await w.execute('browser_tab_select',{tab_id:first.tab_id});
  assert.equal(selected.tab_id,first.tab_id);assert.equal(await w.page.locator('input').inputValue(),'retained');
  assert.deepEqual((await w.execute('browser_tabs',{})).allowed_origins,sites.slice(0,2));
  await assert.rejects(w.execute('browser_navigate',{url:sites[2]}),safeBrowserFailure);
  await w.execute('browser_navigate',{url:sites[1]});
  assert.match((await w.execute('browser_observe',{})).text,/Site 1/);
}));

test('eight tab cap, closed references and manual last-tab closure are explicit',async()=>fixture(async(w,sites)=>{
  const initial=(await w.execute('browser_tabs',{})).selected_tab_id;
  for(let i=1;i<8;i++)await w.execute('browser_tab_open',{url:sites[0]});
  await assert.rejects(w.execute('browser_tab_open',{url:sites[0]}),safeBrowserFailure);
  assert.equal(w.tabs.size,8);
  await w.execute('browser_tab_close',{tab_id:initial});
  await assert.rejects(w.execute('browser_tab_select',{tab_id:initial}),safeBrowserFailure);
  const all=await w.execute('browser_tabs',{});
  for(const tab of all.tabs.slice(1))await w.execute('browser_tab_close',{tab_id:tab.tab_id});
  await assert.rejects(w.execute('browser_tab_close',{tab_id:w.selected}),safeBrowserFailure);
  let empty=false;w.onEmpty=()=>{empty=true;};await w.page.close();assert.ok(empty);
}));

test('serialized selection cannot redirect queued input and document IDs change on reload',async()=>fixture(async(w,sites)=>{
  const first=await w.execute('browser_observe',{});
  const second=await w.execute('browser_tab_open',{url:sites[1]});
  const before=second.document_id;
  await w.page.reload();const refreshed=await w.execute('browser_observe',{});
  assert.notEqual(refreshed.document_id,before);
  const result=await Promise.allSettled([
    w.execute('browser_tab_select',{tab_id:first.tab_id}),
    w.execute('browser_fill',{observation_id:refreshed.observation_id,element:refreshed.elements.find(e=>e.tag==='input').id,text:'wrong tab'}),
  ]);
  assert.equal(result[0].status,'fulfilled');assert.equal(result[1].status,'rejected');
  assert.equal(await w.page.locator('input').inputValue(),'');
}));

test('owned popups are discoverable and failed tab navigation is never labelled safe to replay',async()=>fixture(async(w,sites)=>{
  const obs=await w.execute('browser_observe',{});
  const popup=w.root.page.context().waitForEvent('page');
  await w.execute('browser_click',{observation_id:obs.observation_id,element:obs.elements.find(e=>e.tag==='a').id});
  await popup;
  assert.equal(w.tabs.size,2);
  const context=w.root.page.context(),original=context.newPage.bind(context);let calls=0;
  context.newPage=async()=>{calls++;const page=await original();page.goto=async()=>{throw Error('navigation transport lost');};return page;};
  await assert.rejects(w.execute('browser_tab_open',{url:sites[0]}),error=>!safeBrowserFailure(error)&&/transport lost/.test(error.message));
  assert.equal(calls,1);assert.equal(w.tabs.size,3);
}));

test('dense page observations and eight long tab descriptions stay separately below transport limits',async()=>fixture(async(w,sites)=>{
  const first=w.selected;
  for(let i=1;i<8;i++)await w.execute('browser_tab_open',{url:sites[0]+'/?q='+('a'.repeat(1900))+i});
  for(const driver of w.tabs.values())await driver.page.setContent(`<title>${'題'.repeat(500)}</title><p>${'text '.repeat(3000)}</p>${Array.from({length:105},(_,i)=>`<button>${i}${'項'.repeat(150)}</button>`).join('')}`);
  const reply=await w.execute('browser_tab_select',{tab_id:first});
  assert.equal(reply.dispatched,true);assert.equal(reply.observation_status,'available');
  assert.equal(reply.tabs,undefined);
  assert.ok(Buffer.byteLength(JSON.stringify(reply))<48*1024);
  const tabs=await w.execute('browser_tabs',{});
  assert.equal(tabs.tabs.length,8);assert.ok(tabs.tabs.every(tab=>tab.title_limited));
  assert.ok(Buffer.byteLength(JSON.stringify(tabs))<48*1024);
}));

test('manual selected-tab closure cannot retarget prepared navigation',async()=>fixture(async(w,sites)=>{
  const first=w.selected;
  await w.execute('browser_tab_open',{url:sites[1]});
  await w.prepare('browser_navigate',{url:sites[1]+'/new'});
  await w.page.close();
  await assert.rejects(w.execute('browser_navigate',{url:sites[1]+'/new'}),safeBrowserFailure);
  assert.equal(w.tabs.get(first).page.url(),sites[0]+'/');
  const listed=await w.execute('browser_tabs',{});assert.equal(listed.selected_tab_id,'');
  assert.equal((await w.execute('browser_tab_select',{tab_id:first})).tab_id,first);
}));
