'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),{EventEmitter}=require('node:events');
const {MaintenanceProcess,frame,validateMessage,MAX_RESULT}=require('../workspace-maintenance-process');
const id='a'.repeat(64);
const summary={files:1,bytes:8,application_version:'test',encryption:'age-v1',authenticated:true,published:true,database_checked:false};
function fixture(){
  const child=new EventEmitter();child.stdout=new EventEmitter();child.stderr={resume(){}};const sent=[];
  child.stdin=new EventEmitter();child.stdin.write=(data,done)=>{sent.push(Buffer.from(data));done?.();};child.stdin.end=data=>{if(data)sent.push(Buffer.from(data));};
  let spawnArgs;const process=new MaintenanceProcess({spawn:(...args)=>{spawnArgs=args;return child;},stopTimeoutMs:30});
  return {child,process,sent,spawn:()=>spawnArgs};
}
test('private process uses fixed command and waits for a valid terminal frame plus confirmed exit',async()=>{
  const f=fixture(),done=f.process.run('/owned/offgrid',{protocol:1,id,kind:'start',operation:'backup',passphrase:'isolated secret'},{PATH:'/approved'});
  assert.deepEqual(f.spawn()[1],['workspace','--desktop-stdio']);assert.equal(f.spawn()[2].shell,false);assert.equal(f.spawn()[2].windowsHide,true);
  assert.equal(JSON.parse(f.sent[0].subarray(4)).passphrase,'isolated secret');
  f.child.stdout.emit('data',frame({protocol:1,id,phase:'working'}));
  const final=frame({protocol:1,id,phase:'finished',outcome:'succeeded',summary});
  f.child.stdout.emit('data',final.subarray(0,7));f.child.stdout.emit('data',final.subarray(7));
  let settled=false;void done.then(()=>settled=true);await new Promise(resolve=>setImmediate(resolve));assert.equal(settled,false);
  f.child.emit('close',0,null);assert.equal((await done).phase,'succeeded');assert.equal(f.process.child,null);
});
test('mismatched identity, oversized messages and extra fields cannot become success',async()=>{
  for(const invalid of [{protocol:1,id:'b'.repeat(64),phase:'finished',outcome:'succeeded',summary},{protocol:1,id,phase:'finished',outcome:'succeeded',summary,secret:'not forwarded'},null]){
    const f=fixture(),done=f.process.run('/owned/offgrid',{protocol:1,id,kind:'start'},{});
    if(invalid)f.child.stdout.emit('data',frame(invalid));else{const header=Buffer.alloc(4);header.writeUInt32BE(MAX_RESULT+1);f.child.stdout.emit('data',header);}
    assert.equal(JSON.parse(f.sent[1].subarray(4)).kind,'cancel');
    f.child.emit('close',0,null);assert.equal((await done).phase,'uncertain');
  }
});
test('cancel is scoped, no arbitrary signal is sent and interrupted publication stays visible',async()=>{
  const f=fixture(),done=f.process.run('/owned/offgrid',{protocol:1,id,kind:'start'},{});
  void f.process.cancel();void f.process.cancel();assert.equal(f.sent.length,2);assert.deepEqual(JSON.parse(f.sent[1].subarray(4)),{protocol:1,kind:'cancel',id});
  f.child.stdout.emit('data',frame({protocol:1,id,phase:'finished',outcome:'cancelled',code:'cancelled',summary}));
  f.child.emit('close',130,null);const result=await done;assert.equal(result.phase,'cancelled');assert.equal(result.result.summary.published,true);
});
test('unconfirmed cancellation retains the child and reports uncertainty',async()=>{
  const f=fixture(),done=f.process.run('/owned/offgrid',{protocol:1,id,kind:'start'},{});
  const uncertain=new Promise(resolve=>f.process.once('uncertain',resolve));void f.process.cancel();await uncertain;
  assert.equal(f.process.child,f.child);assert.equal(f.process.uncertain,true);
  f.child.emit('close',null,'SIGKILL');assert.equal((await done).phase,'uncertain');
});
test('compact protocol does not accept manifests or false authentication claims',()=>{
  assert.throws(()=>validateMessage({protocol:1,id,phase:'finished',outcome:'succeeded',summary:{...summary,manifest:[]}},id));
  assert.throws(()=>validateMessage({protocol:1,id,phase:'finished',outcome:'succeeded',summary:{...summary,authenticated:false}},id));
});
