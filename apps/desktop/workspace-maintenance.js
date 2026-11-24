'use strict';
const {EventEmitter}=require('node:events'),crypto=require('node:crypto');
const files=require('./workspace-maintenance-files');
const {MaintenanceProcess}=require('./workspace-maintenance-process');
const validID=value=>typeof value==='string'&&/^[a-f0-9]{64}$/.test(value);
function requestOptions(value){
 if(!value||Object.keys(value).length!==2||!['backup','verify','restore'].includes(value.operation)||!['recipient','passphrase','legacy'].includes(value.encryption)||value.operation==='backup'&&value.encryption==='legacy')throw Error('maintenance_request_invalid');
 return {operation:value.operation,encryption:value.encryption};
}

// This owner receives only host-reviewed selections. No renderer can supply a
// filesystem path, credential, child process, command, service or identity.
class WorkspaceMaintenance extends EventEmitter{
 constructor(options){super();this.options={files,createProcess:()=>new MaintenanceProcess(),...options};this.runtime=options.runtime;this.epoch=0;this.revision=0;this.current=null;this.state={version:1,phase:'idle',active:false,serviceStopped:false,canResume:false};}
 snapshot(){return structuredClone({...this.state,...(this.current&&this.current.epoch!==this.epoch?{result:undefined,retainedPaths:undefined}:{}),revision:this.revision,available:this.runtime.state.state==='ready'&&!!this.runtime.child?.__ownedStdio&&!this.current?.active&&!this.runtime.maintenanceHeld});}
 publish(value){Object.assign(this.state,value);this.revision++;this.emit('status',this.snapshot());return this.snapshot();}
 start(value){
  const request=requestOptions(value);
  if(this.current?.active||this.runtime.maintenanceHeld||this.runtime.state.state!=='ready'||!this.runtime.child?.__ownedStdio||this.runtime.pending||this.runtime.closing)throw Error('maintenance_owned_service_required');
  const op={...request,id:crypto.randomBytes(32).toString('hex'),epoch:this.epoch,child:this.runtime.child,binary:this.runtime.child.__binaryPin,active:true,abort:new AbortController()};
  if(!op.binary)throw Error('maintenance_binary_changed');
  this.current=op;this.state={version:1,id:op.id,...request,phase:'selecting',active:true,serviceStopped:false,canResume:false};this.publish({});
  op.done=this.perform(op).catch(()=>this.fail(op));return this.snapshot();
 }
 valid(op){if(this.current!==op||op.epoch!==this.epoch||op.abort.signal.aborted)throw Error('maintenance_cancelled');}
 async recheck(op){
  this.valid(op);
  if(this.runtime.child!==op.child||this.runtime.state.state!=='ready'||JSON.stringify(await this.options.authority())!==op.authority)throw Error('maintenance_authority_changed');
  this.valid(op);
 }
 async paths(op){
  const f=this.options.files;
  await f.check(op.workspace);await f.checkBinary(op.binary);
  if(op.source)await f.check(op.source);if(op.destination)await f.checkDestination(op.destination);if(op.key)await f.check(op.key);
  this.valid(op);
 }
 async perform(op){
  const f=this.options.files;
  op.authority=JSON.stringify(await this.options.authority());this.valid(op);
  op.workspace=await f.existing(this.runtime.workspace.data,true);
  const selected=await this.options.select({...op,signal:op.abort.signal});this.valid(op);
  if(!selected){op.abort.abort();throw Error('maintenance_cancelled');}
  if(op.operation!=='backup')op.source=await f.existing(selected.source);
  if(op.operation!=='verify'){
   op.destination=await f.destination(selected.destination);
   if(!f.outside(op.workspace.path,op.destination.path))throw Error('maintenance_path_invalid');
  }
  if(op.encryption==='recipient'){
   const material=await f.readKeys(selected.key);op.key=material.pin;op.secret=material.text;
  }else if(op.encryption==='passphrase'){
   op.secret=await this.options.secret(op.operation,op.abort.signal);
   if(op.secret===null){op.abort.abort();throw Error('maintenance_cancelled');}
   if(typeof op.secret!=='string'||!op.secret||Buffer.byteLength(op.secret)>4096||/[\x00\r\n]/.test(op.secret))throw Error('maintenance_request_invalid');
  }
  await this.recheck(op);await this.paths(op);this.publish({phase:'confirming'});
  if(!await this.options.confirm({operation:op.operation,encryption:op.encryption,workspace:op.workspace.path,source:op.source?.path,destination:op.destination?.path},op.abort.signal)){op.abort.abort();throw Error('maintenance_cancelled');}
  await this.recheck(op);await this.paths(op);
  // Close host admission before draining companions, including the asynchronous
  // identity/filesystem checks after their stop acknowledgement.
  this.runtime.maintenanceAdmissionHold=true;this.publish({phase:'stopping'});
  await this.options.quiesce();await this.recheck(op);await this.paths(op);
  op.stopAttempted=true;
  await this.runtime.stopForMaintenance(op.child);
  this.publish({serviceStopped:true});await this.paths(op);
  const child=this.options.createProcess();op.process=child;
  child.on('uncertain',()=>{if(this.current===op)this.publish({phase:'uncertain',code:'maintenance_unconfirmed',active:true,canResume:false});});
  this.publish({phase:'running'});
  const request={protocol:1,kind:'start',id:op.id,operation:op.operation,encryption:op.encryption,
   ...(op.operation==='backup'?{data_dir:op.workspace.path}:{}),...(op.source?{source:op.source.path}:{}),...(op.destination?{destination:op.destination.path}:{}),
   ...(op.encryption==='passphrase'?{passphrase:op.secret}:op.encryption==='recipient'?{key_material:op.secret}:{})};
  let done;
  try{done=child.run(op.binary.path,request,this.options.environment());}finally{op.secret=undefined;delete request.passphrase;delete request.key_material;}
  const outcome=await done;
  if(this.current!==op)return;
  op.active=!!child.child;
  this.publish({phase:outcome.phase,active:op.active,code:outcome.code??outcome.result?.code,result:outcome.result?.summary,retainedPaths:outcome.result?.retained,
   canResume:!op.active&&this.runtime.maintenanceDrained&&op.epoch===this.epoch});
 }
 fail(op){
  op.secret=undefined;if(this.current!==op)return;
  if(!op.stopAttempted)this.runtime.maintenanceAdmissionHold=false;
  op.active=!!op.process?.child;
  const uncertain=op.active||op.stopAttempted&&!this.runtime.maintenanceDrained;
  this.publish({phase:uncertain?'uncertain':op.abort.signal.aborted?'cancelled':'failed',code:uncertain?'maintenance_unconfirmed':op.abort.signal.aborted?'cancelled':'maintenance_failed',active:op.active,
   serviceStopped:this.runtime.maintenanceDrained===true,canResume:!uncertain&&this.runtime.maintenanceDrained===true&&op.epoch===this.epoch});
 }
 cancel(value){
  const op=this.current;
  if(!value||Object.keys(value).length!==1||!validID(value.id)||op?.id!==value.id)throw Error('maintenance_request_invalid');
  op.abort.abort();if(op.process?.child)void op.process.cancel();return this.snapshot();
 }
 invalidate(){this.epoch++;const op=this.current;if(!op)return;op.abort.abort();if(op.process?.child)void op.process.cancel();this.publish({canResume:false});}
 async stop(){
  this.invalidate();const op=this.current;if(!op)return;
  if(op.process?.child)await op.process.cancel();
  if(op.process?.child)throw Error('maintenance_unconfirmed');
  let timer;
  try{await Promise.race([op.done,new Promise((_resolve,reject)=>{timer=setTimeout(()=>reject(Error('maintenance_unconfirmed')),this.options.stopTimeoutMs??45000);})]);}finally{clearTimeout(timer);}
 }
 async resume(value){
  const op=this.current;
  if(!value||Object.keys(value).length!==1||!validID(value.id)||op?.id!==value.id||op.active||!this.state.canResume||op.epoch!==this.epoch)throw Error('maintenance_resume_unavailable');
  this.publish({phase:'resuming',canResume:false});
  try{
   await this.options.files.check(op.workspace);await this.options.files.checkBinary(op.binary);
   if(op.epoch!==this.epoch)throw Error('maintenance_authority_changed');
   const result=await this.runtime.resumeAfterMaintenance();
   if(result.state!=='ready')throw Error('maintenance_resume_failed');
   this.publish({phase:'idle',serviceStopped:false,canResume:false});
  }catch{this.publish({phase:'failed',code:'maintenance_resume_failed',canResume:this.runtime.maintenanceDrained===true&&op.epoch===this.epoch});}
  return this.snapshot();
 }
}
module.exports={WorkspaceMaintenance,requestOptions};
