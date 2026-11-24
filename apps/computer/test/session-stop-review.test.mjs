import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {BrowserWorkspace} from '../browser-workspace.mjs';
import {CompanionSession} from '../session.mjs';
import {ManagedBrowserController} from '../managed-controller.mjs';

test('failed retained-browser close keeps ownership until explicit Stop retry confirms termination',async()=>{
 const server=createServer((_request,response)=>response.end('<p>Isolated close confirmation fixture</p>'));
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const driver=await BrowserWorkspace.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true});
 const close=driver.close.bind(driver),messages=[],exits=[];
 try{
  await driver.takeover();
  const owner=new CompanionSession({service:'http://127.0.0.1:1',directory:'unused'});
  owner.state='manual';owner.retained={driver};
  const controller=new ManagedBrowserController({send:value=>messages.push(value),exit:code=>exits.push(code)});controller.session=owner;
  driver.close=async()=>{throw Error('isolated close failure');};
  await assert.rejects(controller.stop(),/computer_stop_unconfirmed/);
  assert.equal(driver.browser.isConnected(),true,'injected failure leaves real Chromium alive');
  assert.equal(messages.some(value=>value.state==='stopped'),false);
  assert.deepEqual(exits,[],'utility must retain ownership while close is unconfirmed');
  assert.equal(owner.retained?.driver??owner.driver,driver);
  assert.equal(messages.at(-1).code,'computer_stop_unconfirmed');
  driver.close=close;
  await controller.command({type:'stop'});
  assert.equal(driver.browser.isConnected(),false);
  assert.equal(owner.retained,undefined);
  assert.deepEqual(exits,[0]);
  assert.equal(messages.at(-1).state,'stopped');
  await controller.command({type:'stop'});assert.deepEqual(exits,[0]);
 }finally{await close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
});
