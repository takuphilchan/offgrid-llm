'use strict';
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path');
const expected={'recovery-running':'interrupted','recovery-dispatched':'uncertain','recovery-uncertain':'uncertain'};
const marker='original effect sentinel';

// Seed historical snapshots through the supported legacy import. The production
// service must recover their statuses without dispatching any saved write intent.
exports.seed=async data=>{
 const directory=path.join(data,'agent_tasks');await fs.mkdir(directory);
 await fs.writeFile(path.join(data,'effect-sentinel.txt'),marker);
 for(const [id,status] of Object.entries(expected)){
  const call={id:'unconfirmed-write',type:'function',function:{name:'write_file',arguments:JSON.stringify({path:path.join(data,'effect-sentinel.txt'),content:'UNEXPECTED REPLAY'})}};
  const task={id,actor:'local-admin',prompt:'Retain historical isolated recovery evidence',status:id==='recovery-uncertain'?'uncertain':'running',model:'isolated-unloaded-model',config:{max_iterations:4,max_tokens:64},created_at:'2026-11-20T00:00:00Z',scheduler_ready:true,
   checkpoint:{iteration:1,messages:[],...(status==='uncertain'?{calls:[call],executing_call:call.id}:{})}};
  await fs.writeFile(path.join(directory,id+'.json'),JSON.stringify(task));
 }
};

exports.checkService=async url=>{
 for(const [id,status] of Object.entries(expected)){
  const response=await fetch(url+'/v1/agents/tasks/'+id,{signal:AbortSignal.timeout(5000)});assert.equal(response.status,200);
  const task=await response.json();assert.equal(task.status,status);assert.equal(task.steps.length,0);
  if(status==='uncertain'){assert.equal(task.uncertain_call_id,'unconfirmed-write');assert.equal(task.resumable,false);}
 }
};

exports.checkRestored=async directory=>{
 // Read only the disconnected restored copy. node:sqlite is available in the
 // explicitly required Node 24 test runtime; no extension or worker is loaded.
 const {DatabaseSync}=require('node:sqlite');const db=new DatabaseSync(path.join(directory,'agent-state.sqlite'),{readOnly:true});
 try{
  for(const [id,status] of Object.entries(expected)){
   const row=db.prepare('SELECT snapshot FROM agent_tasks WHERE id=?').get(id);assert.ok(row);
   const task=JSON.parse(Buffer.from(row.snapshot).toString());assert.equal(task.status,status);assert.notEqual(task.scheduler_ready,true);assert.equal(task.steps?.length??0,0);
   if(status==='uncertain'){assert.equal(task.checkpoint.executing_call,'unconfirmed-write');assert.equal(task.checkpoint.calls[0].function.name,'write_file');}
  }
 }finally{db.close();}
 assert.equal(await fs.readFile(path.join(directory,'effect-sentinel.txt'),'utf8'),marker);
};
exports.checkSentinel=async directory=>assert.equal(await fs.readFile(path.join(directory,'effect-sentinel.txt'),'utf8'),marker);
