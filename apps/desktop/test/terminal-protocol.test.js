const test = require('node:test');
const assert = require('node:assert/strict');
const {createTerminalProtocol,MAX_INPUT_BYTES,MAX_PASTE_BYTES}=require('../terminal-protocol');

function fixture() {
  const url='file:///trusted/terminal.html',calls=[];
  const frame={url};
  const contents={mainFrame:frame,getURL:()=>url,isDestroyed:()=>false};
  const event={sender:contents,senderFrame:frame};
  let ready=true;
  const operations={create:async request=>{calls.push(['create',request]);return{id:'session-1'};}};
  for(const key of ['input','resize','ack','close']) operations[key]=async(...args)=>{calls.push([key,...args]);};
  operations.paste=async()=>{calls.push(['paste']);return 'café 日本語';};
  const bridge=createTerminalProtocol({trustedURL:url,contents:()=>contents,operations,available:()=>ready});
  return {bridge,event,calls,operations,setReady:value=>{ready=value;}};
}
async function connect(f){const hello=await f.bridge.dispatch(f.event,{protocol:1,action:'handshake'});return {protocol:1,generation:hello.generation};}
test('terminal requires the exact bundled main frame, owner and protocol',async()=>{
  const f=fixture();
  for(const event of [{...f.event,sender:{}},{...f.event,senderFrame:{url:'http://localhost/ui'}},{...f.event,senderFrame:{url:f.event.senderFrame.url}}]) {
    await assert.rejects(f.bridge.dispatch(event,{protocol:1,action:'handshake'}),/untrusted/);
  }
  await assert.rejects(f.bridge.dispatch(f.event,{protocol:2,action:'handshake'}),/protocol/);
  assert.deepEqual(f.calls,[]);
});
test('unavailable runtime, malformed create and arbitrary executable never spawn',async()=>{
  const f=fixture(),base=await connect(f);
  f.setReady(false);
  await assert.rejects(f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24}),/unavailable/);
  f.setReady(true);
  for(const patch of [{cols:0},{rows:201},{cols:NaN},{kind:'cmd /c injected'},{executable:'evil.exe'}]) {
    await assert.rejects(f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24,...patch}));
  }
  assert.deepEqual(f.calls,[]);
});
test('only reviewed terminal entry modes reach the supervisor',async()=>{
  for(const kind of ['shell','cli','agent']){
    const f=fixture(),base=await connect(f);
    await f.bridge.dispatch(f.event,{...base,action:'create',kind,cols:80,rows:24});
    assert.deepEqual(f.calls,[['create',{kind,cols:80,rows:24}]]);
  }
});
test('input, resize, ACK and close stay scoped and bounded; Ctrl+C is input',async()=>{
  const f=fixture(),base=await connect(f);
  await f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24});
  const session={...base,id:'session-1'};
  for(const text of ['hello 日本語','\x03']) await f.bridge.dispatch(f.event,{...session,action:'input',text});
  await assert.rejects(f.bridge.dispatch(f.event,{...session,action:'input',text:'x'.repeat(MAX_INPUT_BYTES+1)}),/input/);
  await assert.rejects(f.bridge.dispatch(f.event,{...session,action:'resize',cols:80.5,rows:24}),/size/);
  await assert.rejects(f.bridge.dispatch(f.event,{...session,action:'ack',sequence:0}),/ack/);
  await assert.rejects(f.bridge.dispatch(f.event,{...session,id:'another',action:'input',text:'x'}),/unknown/);
  await f.bridge.dispatch(f.event,{...session,action:'close'});
  await assert.rejects(f.bridge.dispatch(f.event,{...session,action:'input',text:'x'}),/unknown/);
  assert.deepEqual(f.calls.map(call=>call[0]),['create','input','input','close']);
});
test('clipboard paste requires an owned live session and remains bounded',async()=>{
  const f=fixture(),base=await connect(f),paste={...base,id:'session-1',action:'paste'};
  await assert.rejects(f.bridge.dispatch(f.event,paste),/unknown_session/);
  await f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24});
  assert.equal(await f.bridge.dispatch(f.event,paste),'café 日本語');
  for(const value of ['x'.repeat(MAX_PASTE_BYTES+1),'\x1b[31m','\x00']){
    f.operations.paste=async()=>value;
    await assert.rejects(f.bridge.dispatch(f.event,paste),/invalid_paste/);
  }
  f.bridge.invalidate();
  await assert.rejects(f.bridge.dispatch(f.event,paste),/stale_renderer/);
});
test('reload fences old input and stops a spawn completed after renderer loss',async()=>{
  const f=fixture(),base=await connect(f);let resolve;
  f.operations.create=()=>new Promise(done=>{resolve=done;});
  const pending=f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24});
  f.bridge.invalidate();resolve({id:'late'});
  await assert.rejects(pending,/stale/);
  assert.deepEqual(f.calls,[['close','late']]);
  await connect(f);
  await assert.rejects(f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24}),/stale/);
});
test('failed close retains ownership and never reports stopped',async()=>{
  const f=fixture(),base=await connect(f);
  await f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24});
  f.operations.close=async()=>{throw Error('stop_unconfirmed');};
  await assert.rejects(f.bridge.dispatch(f.event,{...base,id:'session-1',action:'close'}),/unconfirmed/);
  await f.bridge.dispatch(f.event,{...base,id:'session-1',action:'input',text:'\x03'});
});

test('dock controls work without a runtime, while layout and clipboard stay bounded',async()=>{
  const f=fixture(),base=await connect(f);f.setReady(false);
  for(const key of ['hide','layout','copy'])f.operations[key]=value=>f.calls.push([key,value]);
  await f.bridge.dispatch(f.event,{...base,action:'hide'});
  await f.bridge.dispatch(f.event,{...base,action:'layout',height:320});
  await f.bridge.dispatch(f.event,{...base,action:'copy',text:'selected output'});
  await assert.rejects(f.bridge.dispatch(f.event,{...base,action:'layout',height:Infinity}),/height/);
  await assert.rejects(f.bridge.dispatch(f.event,{...base,action:'copy',text:'x'.repeat(1024*1024+1)}),/copy/);
  assert.equal(f.calls.length,3);
});

test('selected context requires an owned terminal and strict bounded payload',async()=>{
 const f=fixture(),base=await connect(f),contexts=[];
 f.operations.share=async value=>{contexts.push(value);return {opened:true};};
 const context={version:1,source:'terminal_selection',command:'ls',cwd:'C:\\test',output:'selected',question:'Explain this',truncated:false};
 const request={...base,action:'share',id:'session-1',context};
 await assert.rejects(f.bridge.dispatch(f.event,request),/unknown_session/);
 await f.bridge.dispatch(f.event,{...base,action:'create',kind:'shell',cols:80,rows:24});
 for(const patch of [{output:'x'.repeat(32769)},{output:'\x1b[1m'},{question:''},{source:'trusted_receipt'},{secret:'extra'}])await assert.rejects(f.bridge.dispatch(f.event,{...request,context:{...context,...patch}}));
 assert.equal(contexts.length,0);assert.deepEqual(await f.bridge.dispatch(f.event,request),{opened:true});
 f.bridge.invalidate();await assert.rejects(f.bridge.dispatch(f.event,request),/stale/);
 assert.equal(contexts.length,1);
});
