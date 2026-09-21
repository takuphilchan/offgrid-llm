import {createRequire} from 'node:module';
const require=createRequire(import.meta.url);
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {EventEmitter}=require('node:events');
const {PassThrough}=require('node:stream');
const {NativeWorker,frame}=require('../native-worker.cjs');

function fixture(options={}) {
  const child=new EventEmitter();child.stdin=new PassThrough();child.stdout=new PassThrough();child.stderr=new PassThrough();child.requests=[];
  child.stdin.on('data',data=>{const value=JSON.parse(data.subarray(4));child.requests.push(value);if(value.kind==='stop' && !options.hang)queueMicrotask(()=>child.emit('exit',0));});
  child.kill=()=>{child.killed=true;if(!options.hang)queueMicrotask(()=>child.emit('exit',2));return true;};
  const worker=new NativeWorker({root:'test-pack',directory:'test-journal',verify:async()=>{},spawnWorker:(file,args,config)=>{assert.equal(config.shell,false);assert.equal(config.windowsHide,true);assert.deepEqual(args.slice(0,1),['--state']);child.executable=file;queueMicrotask(()=>child.stdout.write(frame({protocol:2,id:'startup',result:{ready:true}})));return child;},timeoutMs:50,stopMs:20,...options});
  const reply=(result)=>{const {id}=child.requests.at(-1);child.stdout.write(frame({protocol:2,id,result}));};
  return {worker,child,reply};
}

test('native IPC frames split responses; validates operations and serializes requests',async()=>{
  const {worker,child}=fixture();await worker.start();
  await assert.rejects(worker.request('shell',{command:'x'}),/invalid_action/);
  const pending=worker.request('targets');
  await assert.rejects(worker.request('observe'),/busy/);
  const response=frame({protocol:2,id:child.requests.at(-1).id,result:[]});
  child.stdout.write(response.subarray(0,3));child.stdout.write(response.subarray(3));
  assert.deepEqual(await pending,[]);await worker.stop();assert.equal(worker.child,null);
});
test('native IPC refuses stale/malformed responses and retains uncertain mutations',async()=>{
  const {worker,child}=fixture();await worker.start();
  const pending=worker.request('execute',{step:{},grant:'exact-local-grant'});
  child.stdout.write(frame({protocol:2,id:'wrong-action',result:{}}));
  await assert.rejects(pending,/uncertain_outcome/);await worker.stop();
});
test('native IPC cannot start after verification was cancelled',async()=>{
  let finish,spawns=0;
  const {worker}=fixture({verify:()=>new Promise(resolve=>{finish=resolve;}),spawnWorker:()=>{spawns++;throw Error('must not spawn');}});
  const starting=worker.start();await worker.stop();finish();
  await assert.rejects(starting,/stopped/);assert.equal(spawns,0);
});
test('native worker crash never repeats a dispatched action',async()=>{
  const {worker,child}=fixture();await worker.start();
  const pending=worker.request('execute',{step:{},grant:'exact-local-grant'});
  child.emit('exit',2);await assert.rejects(pending,/uncertain_outcome/);
  assert.equal(child.requests.filter(x=>x.kind==='execute').length,1);
});
test('native stop requires owned-process exit acknowledgement',async()=>{
  const {worker,child}=fixture({hang:true});await worker.start();
  await assert.rejects(worker.stop(),/stop_unconfirmed/);
  assert.equal(child.killed,true);assert.equal(worker.child,child);
  await assert.rejects(worker.start(),/input_in_use/);
  await assert.rejects(worker.request('observe'),/worker_stopped/);child.emit('exit',2);
});

for(const [platform,arch] of [['win32','x64'],['darwin','x64'],['darwin','arm64'],['linux','x64']])test(`shared private transport uses the ${platform}/${arch} host pack`,async()=>{
  let checked=false;
  const {worker,child}=fixture({platform,arch,verify:async(root,actualPlatform,actualArch)=>{assert.equal(actualPlatform,platform);assert.equal(actualArch,arch);checked=true;}});
  await worker.start();assert.equal(checked,true);assert.ok(child.executable.endsWith(platform==='win32'?'offgrid-computer.exe':'offgrid-computer'));await worker.stop();
});
