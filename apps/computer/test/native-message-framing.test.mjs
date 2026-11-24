import test from 'node:test';
import assert from 'node:assert/strict';
import {Readable,PassThrough,Writable} from 'node:stream';
import {endianness} from 'node:os';
import {spawn} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import {nativeMessageLimit,encodeNativeMessage,readNativeMessages,writeNativeMessage} from '../native-message-framing.mjs';

const header=length=>{const data=Buffer.alloc(4);if(endianness()==='LE')data.writeUInt32LE(length);else data.writeUInt32BE(length);return data;};
const decode=async chunks=>{const result=[];for await(const value of readNativeMessages(Readable.from(chunks)))result.push(value);return result;};
const invalid=code=>error=>error.code===code&&error.message===code;
const hello={protocol_version:1,type:'hello',extension_id:'a'.repeat(32),browser_version:'151.0.7922.34',client_nonce:'c'.repeat(64)};

test('native frames preserve fragmented Unicode and multiple messages without newline framing',async()=>{
  const messages=[{title:'已有会话',value:1},{id:'two',nested:{ready:true}}];
  const bytes=Buffer.concat(messages.map(encodeNativeMessage));
  assert.deepEqual(await decode([...bytes].map(byte=>Buffer.from([byte]))),messages);
  assert.deepEqual(await decode([bytes]),messages);
});

test('native decoder rejects oversize lengths, incomplete EOF, invalid UTF8 and deeply nested input',async()=>{
  await assert.rejects(decode([header(0xffffffff)]),invalid('native_message_oversize'));
  await assert.rejects(decode([header(0)]),invalid('native_message_invalid'));
  await assert.rejects(decode([header(20).subarray(0,2)]),invalid('native_message_truncated'));
  await assert.rejects(decode([header(20),Buffer.from('{}')]),invalid('native_message_truncated'));
  const malformed=Buffer.from([0x7b,0x22,0x78,0x22,0x3a,0x22,0xc0,0xaf,0x22,0x7d]);
  await assert.rejects(decode([header(malformed.length),malformed]),invalid('native_message_invalid'));
  for(const text of ['{"secret":"do-not-echo",BROKEN}','{"x":1e309}','{"x":"\\ud800"}','[1,2]','\ufeff{}',`{"x":${'['.repeat(34)}0${']'.repeat(34)}}`]){
    const data=Buffer.from(text);await assert.rejects(decode([header(data.length),data]),invalid('native_message_invalid'));
  }
});

test('native encoding has exact byte bounds and rejects non-JSON values without invoking getters',()=>{
  const atLimit={v:'a'.repeat(nativeMessageLimit-8)};
  assert.equal(encodeNativeMessage(atLimit).length,nativeMessageLimit+4);
  assert.throws(()=>encodeNativeMessage({v:atLimit.v+'a'}),invalid('native_message_oversize'));
  const cycle={};cycle.self=cycle;
  let invoked=false;const getter={get secret(){invoked=true;return 'private';}};
  const hiddenGetter={};Object.defineProperty(hiddenGetter,'toJSON',{get(){invoked=true;return ()=>({private:true});}});
  for(const value of [cycle,getter,hiddenGetter,{x:undefined},{x:NaN},{x:BigInt(1)},{x:new Date()},null,[]])assert.throws(()=>encodeNativeMessage(value));
  assert.equal(invoked,false);
});

test('blocked native read cancellation settles without forwarding the abort reason',async()=>{
  const stream=new PassThrough(),abort=new AbortController();
  const iterator=readNativeMessages(stream,{signal:abort.signal});
  const result=iterator.next();stream.write(header(5).subarray(0,2));
  abort.abort(Error('private-abort-reason'));
  await assert.rejects(result,invalid('native_message_cancelled'));
  assert.equal(stream.destroyed,true);
});

test('native write waits for actual sink callback and redacts IO failures',async()=>{
  let finish,copy;
  const output=new Writable({write(chunk,_encoding,callback){copy=Buffer.from(chunk);finish=callback;}});
  let done=false;const result=writeNativeMessage(output,{ok:true}).then(()=>{done=true;});
  await new Promise(resolve=>setImmediate(resolve));assert.equal(done,false);
  finish();await result;assert.deepEqual(await decode([copy]),[{ok:true}]);
  const failed=new Writable({write(_chunk,_encoding,callback){callback(Error('private-path-or-token'));}});
  await assert.rejects(writeNativeMessage(failed,{ok:true}),invalid('native_message_io'));
});

test('native write cancellation destroys only its owned sink and settles while blocked',async()=>{
  const output=new Writable({write(_chunk,_encoding,_callback){}}),abort=new AbortController();
  const result=writeNativeMessage(output,{ok:true},{signal:abort.signal});
  abort.abort(Error('private-abort-reason'));
  await assert.rejects(result,invalid('native_message_cancelled'));
  assert.equal(output.destroyed,true);
});

function subprocess(t){
  const child=spawn(process.execPath,[fileURLToPath(new URL('./fixtures/attachment-framing-child.mjs',import.meta.url))],{stdio:['pipe','pipe','pipe'],windowsHide:true,env:{}});
  const exit=new Promise((resolve,reject)=>{child.once('error',reject);child.once('close',(code,signal)=>resolve({code,signal}));});
  let stderr='';child.stderr.on('data',chunk=>{stderr+=chunk;if(stderr.length>2048)child.kill();});
  t.after(async()=>{if(child.exitCode===null&&child.signalCode===null)child.kill();await exit;});
  return {child,exit,stderr:()=>stderr};
}

test('actual native-framing subprocess round-trips fragmented hello and exits on EOF',{timeout:10000},async t=>{
  const {child,exit,stderr}=subprocess(t);
  const received=(async()=>{const result=[];for await(const message of readNativeMessages(child.stdout))result.push(message);return result;})();
  const frame=encodeNativeMessage(hello);
  for(const piece of [frame.subarray(0,1),frame.subarray(1,3),frame.subarray(3,11)])child.stdin.write(piece);
  child.stdin.end(Buffer.concat([frame.subarray(11),frame]));
  const messages=await received;
  assert.equal(messages.length,2);assert.ok(messages.every(value=>value.type==='hello_ack'&&value.client_nonce===hello.client_nonce));
  assert.deepEqual(await exit,{code:0,signal:null});assert.equal(stderr(),'');
});

test('actual subprocess rejects oversized framing before body allocation and emits no protocol stdout',{timeout:10000},async t=>{
  const {child,exit,stderr}=subprocess(t);const chunks=[];child.stdout.on('data',chunk=>chunks.push(chunk));
  child.stdin.end(header(0x40000000));
  assert.deepEqual(await exit,{code:2,signal:null});assert.equal(Buffer.concat(chunks).length,0);
  assert.equal(stderr(),'native_message_oversize');
});

test('actual subprocess rejects partial EOF and malformed private payloads with fixed errors',{timeout:10000},async t=>{
  for(const [data,expected] of [
    [header(10).subarray(0,2),'native_message_truncated'],
    [Buffer.concat([header(10),Buffer.from('{"a"')]),'native_message_truncated'],
    [Buffer.from('{"private":"do-not-echo",BROKEN}'),'native_message_invalid'],
  ]){
    const {child,exit,stderr}=subprocess(t);const chunks=[];child.stdout.on('data',chunk=>chunks.push(chunk));
    child.stdin.end(expected==='native_message_invalid'?Buffer.concat([header(data.length),data]):data);
    assert.deepEqual(await exit,{code:2,signal:null});assert.equal(Buffer.concat(chunks).length,0);
    assert.equal(stderr(),expected);assert.ok(!stderr().includes('do-not-echo'));
  }
});

test('actual subprocess rejects duplicate root, nested and escaped-equivalent metadata keys',{timeout:10000},async t=>{
  const valid=JSON.stringify(hello);
  for(const raw of [
    valid.replace('"type":"hello"','"type":"release","type":"hello"'),
    valid.replace('"type":"hello"','"t\\u0079pe":"hello","type":"hello"'),
    valid.slice(0,-1)+',"scope":{"session_id":"one","session_id":"two"}}',
    valid.slice(0,-1)+',"extra":[{"session_id":"one","session_\\u0069d":"two"}]}',
  ]){
    const {child,exit,stderr}=subprocess(t);const chunks=[];child.stdout.on('data',chunk=>chunks.push(chunk));
    const data=Buffer.from(raw);child.stdin.end(Buffer.concat([header(data.length),data]));
    assert.deepEqual(await exit,{code:2,signal:null});assert.equal(Buffer.concat(chunks).length,0);
    assert.equal(stderr(),'native_message_invalid');
  }
});
