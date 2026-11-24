import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserWorkspace} from '../browser-workspace.mjs';

async function fixture(run){
  const server=createServer((request,response)=>{response.setHeader('Content-Type','text/html');response.end('<p>Disposable takeover fixture</p>');});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let workspace;
  try{
    workspace=await BrowserWorkspace.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
    assert.equal((await workspace.execute('browser_tabs',{})).tabs.length,1,'the real automated read must work before handoff');
    await workspace.execute('browser_observe',{});
    await workspace.takeover();
    await run(workspace);
  }finally{await workspace?.close();await new Promise(resolve=>server.close(resolve));}
}

function holdEvidenceDisposal(workspace){
  const clear=workspace.clearObservations.bind(workspace);
  let entered,release;const started=new Promise(resolve=>{entered=resolve;}),gate=new Promise(resolve=>{release=resolve;});
  let calls=0;
  workspace.clearObservations=async()=>{calls++;await clear();entered();await gate;};
  return {started,release:()=>release(),calls:()=>calls};
}

test('a new takeover supersedes an unfinished resume instead of reporting stale manual readiness',async()=>{
  await fixture(async workspace=>{
    const boundary=holdEvidenceDisposal(workspace);
    let resumeError;
    const resumed=workspace.resumeManual().catch(error=>{resumeError=error;});
    await boundary.started;
    const handoff=workspace.takeover();
    boundary.release();
    try{
      assert.equal((await handoff).state,'manual');
      await resumed;
      assert.ok(boundary.calls()>0,'resume must reach the asynchronous evidence-disposal boundary');
      assert.equal(workspace.control,'manual','the later takeover must remain authoritative');
      await assert.rejects(workspace.execute('browser_tabs',{}),/manual control/);
      assert.ok(resumeError,'a superseded resume must not report successful automation');
    }finally{boundary.release();await resumed;}
  });
});

test('Stop while resume clears evidence closes the real retained browser without reopening automation',async()=>{
  await fixture(async workspace=>{
    const boundary=holdEvidenceDisposal(workspace);
    let resumeError;
    const resumed=workspace.resumeManual().catch(error=>{resumeError=error;});
    await boundary.started;
    try{
      await workspace.close();
    }finally{boundary.release();await resumed;}
    assert.ok(boundary.calls()>0);
    assert.ok(resumeError,'closed browser cannot report successful resumption');
    assert.equal(workspace.browser.isConnected(),false);
    await assert.rejects(workspace.execute('browser_tabs',{}));
    await assert.rejects(workspace.resumeManual(),/unavailable/);
  });
});
