const {test}=require('node:test');const assert=require('node:assert/strict');const {EventEmitter}=require('node:events');const {PassThrough}=require('node:stream');const path=require('node:path');
const {ExecutionRuntime,frame}=require('../execution-runtime');const {workspaceChannelAllowed}=require('../backend');
const tick=()=>new Promise(resolve=>setImmediate(resolve));
const folder=path.resolve('disposable-fixture');
const identity={product:'offgrid',api_version:2,workspace_id:'workspace',version:'0.0.0-test',capabilities:['native-execution-preview-v1']};
const pair={protocol:1,code:'a'.repeat(64),actor:'local',workspace:'workspace',version:'0.0.0-test'};
const resource={reference:{host:'local-host',session:'session',grant:'grant',revision:1},backend:'local',root:folder,directory:'.',permissions:{read:true,write:true,processes:false}};
function fixture(overrides={}){
  const child=new EventEmitter();child.stdin=new PassThrough();child.stdout=new PassThrough();child.stderr=new PassThrough();let payload=Buffer.alloc(0),forks=0,pairs=0,confirms=0;
  child.stdin.on('data',data=>{payload=Buffer.concat([payload,data]);});
  const runtime=new ExecutionRuntime({root:path.resolve('trusted-pack'),directory:path.resolve('private-state'),version:'0.0.0-test',stopTimeout:40,service:()=> 'http://127.0.0.1:11611',identity:async()=>identity,principal:async()=> 'local',verify:async()=>{},choose:async()=>folder,confirm:async()=>{confirms++;return true;},pairing:async()=>{pairs++;return pair;},spawn:(_file,args,options)=>{forks++;assert.deepEqual(args,['--connect','--state-dir',path.resolve('private-state')]);assert.equal(options.shell,false);assert.equal(options.windowsHide,true);assert.equal(options.env.OFFGRID_API_KEY,undefined);assert.equal(options.env.OFFGRID_EXECUTOR_TOKEN,undefined);return child;},...overrides});
  return {runtime,child,counts:()=>({forks,pairs,confirms}),seed:()=>JSON.parse(payload.subarray(4).toString()),ready:()=>child.stdout.write(frame({protocol:1,state:'ready',resource})),exit:()=>{child.stdout.write(frame({protocol:1,state:'stopped'}));child.emit('exit',0);}};
}

test('native folder consent and pairing precede spawn; private seed is scoped and EOF stops',async()=>{
  const f=fixture();const start=f.runtime.start({access:'edit'});await tick();assert.deepEqual(f.counts(),{forks:1,pairs:1,confirms:1});
  assert.equal(f.seed().folder,folder);assert.equal(f.seed().pairing.code,pair.code);assert.deepEqual(f.seed().permissions,resource.permissions);assert.equal(f.seed().service,'http://127.0.0.1:11611');f.ready();assert.deepEqual((await start).resource,resource);
  const stopping=f.runtime.stop();assert.equal(f.runtime.state,'stopping');f.exit();assert.equal((await stopping).state,'stopped');assert.equal(f.runtime.child,null);
});
test('renderer cannot submit folder, authority, command or service; denial never pairs',async()=>{
  for(const request of [{access:'edit',folder},{access:'trusted',approved:true},{access:'read',service:'http://other'},{access:'shell'},{access:'read',command:'anything'}]){const f=fixture();await assert.rejects(f.runtime.start(request),/execution_request_invalid/);assert.equal(f.counts().forks,0);}
  const f=fixture({confirm:async()=>false});assert.equal((await f.runtime.start({access:'trusted'})).state,'idle');assert.equal(f.counts().pairs,0);assert.equal(f.counts().forks,0);
});
test('stop or identity change while native dialogs are pending cannot create late authority',async()=>{
  for(const stage of ['choose','confirm']){
    let release;const f=fixture({[stage]:()=>new Promise(resolve=>{release=resolve;})});const pending=f.runtime.start({access:'edit'});await tick();await f.runtime.stop();release(stage==='choose'?folder:true);await assert.rejects(pending,/execution_cancelled/);assert.equal(f.counts().forks,0);assert.equal(f.counts().pairs,0);
  }
  let account='first';const f=fixture({principal:async()=>account,confirm:async()=>{account='different';return true;}});await assert.rejects(f.runtime.start({access:'edit'}),/execution_scope_changed/);assert.equal(f.counts().forks,0);assert.equal(f.counts().pairs,0);
});
test('wrong worker scope fails closed and unconfirmed shutdown is never acknowledged',async()=>{
  const f=fixture();const pending=f.runtime.start({access:'edit'});await tick();f.child.stdout.write(frame({protocol:1,state:'ready',resource:{...resource,root:path.resolve('foreign')}}));await assert.rejects(pending,/execution_scope_changed/);assert.equal(f.runtime.state,'uncertain');assert.equal(f.runtime.child,f.child);await assert.rejects(f.runtime.start({access:'edit'}),/execution_host_in_use/);f.child.emit('exit',1);assert.equal(f.runtime.state,'uncertain');
});
test('existing lease checks do not spawn; account change revokes and crashes stay uncertain',async()=>{
  const f=fixture();const pending=f.runtime.start({access:'edit'});await tick();f.ready();await pending;assert.equal((await f.runtime.status()).state,'ready');assert.equal(f.counts().forks,1);
  f.runtime.principal=async()=> 'different';const status=f.runtime.status();await tick();f.exit();assert.equal((await status).state,'stopped');assert.equal(f.counts().forks,1);
  const crashed=fixture();const start=crashed.runtime.start({access:'edit'});await tick();crashed.ready();await start;crashed.child.emit('exit',1);assert.equal(crashed.runtime.state,'uncertain');
});
test('recovery pages can stop/inspect execution but cannot create grants',()=>{
  assert.equal(workspaceChannelAllowed('execution-start',false,true),false);assert.equal(workspaceChannelAllowed('execution-start',false,false),false);assert.equal(workspaceChannelAllowed('execution-start',true,false),true);
  for(const channel of ['execution-stop','execution-status'])assert.equal(workspaceChannelAllowed(channel,false,true),true);
});

test('old services, wrong packs and foreign tickets never create an executor',async()=>{
  for(const overrides of [{identity:async()=>({...identity,capabilities:[]})},{identity:async()=>({...identity,version:'older'})},{verify:async()=>{throw Error('pack mismatch');}},{pairing:async()=>({...pair,workspace:'foreign'})},{pairing:async()=>({...pair,actor:'foreign'})}]){
    const f=fixture(overrides);await assert.rejects(f.runtime.start({access:'edit'}));assert.equal(f.counts().forks,0);
  }
});

test('stop during native verification or pairing prevents a late helper spawn',async()=>{
  for(const stage of ['verify','pairing']){
    let release;const f=fixture({[stage]:()=>new Promise(resolve=>{release=resolve;})});const pending=f.runtime.start({access:'read'});await tick();await f.runtime.stop();release(stage==='pairing'?pair:undefined);await assert.rejects(pending,/execution_cancelled/);assert.equal(f.counts().forks,0);
  }
});

test('remembered native trust skips only review and still pairs a fresh checked host',async()=>{
  const pin={id:'fixture-scope'};let pins=0,lookups=0;
  const f=fixture({trust:{pin:async approval=>{pins++;assert.equal(approval.actor,'local');return pin;},matches:async value=>{lookups++;assert.equal(value,pin);return true;}}});
  const pending=f.runtime.start({access:'edit'});await tick();assert.deepEqual(f.counts(),{forks:1,pairs:1,confirms:0});assert.equal(pins,2);assert.equal(lookups,1);f.ready();await pending;
  const stopping=f.runtime.stop();f.exit();await stopping;
});

test('remembering is explicit; changed root, failed persistence and cancellation never pair',async()=>{
  for(const kind of ['remember','session','changed','failed','cancelled']){
    let pins=0,saves=0,release;
    const trust={pin:async()=>({id:kind==='changed'&&++pins>1?'replacement':'original'}),matches:async()=>false,remember:async()=>{saves++;if(kind==='failed')throw Error('storage_failed');if(kind==='cancelled')await new Promise(resolve=>{release=resolve;});}};
    const f=fixture({trust,confirm:async approval=>{assert.equal(approval.canRemember,true);return kind==='session'?true:'remember';}});
    const pending=f.runtime.start({access:'edit'});const rejected=['changed','failed','cancelled'].includes(kind)?assert.rejects(pending):null;await tick();
    if(kind==='cancelled'){await f.runtime.stop();release();}
    if(rejected){await rejected;assert.equal(f.counts().pairs,0);assert.equal(f.counts().forks,0);}
    else{assert.equal(saves,kind==='session'?0:1);f.ready();await pending;const stopping=f.runtime.stop();f.exit();await stopping;}
  }
});
