const {EventEmitter}=require('node:events');
const path=require('node:path');

const modes={read:{read:true,write:false,processes:false},edit:{read:true,write:true,processes:false},trusted:{read:true,write:true,processes:true}};
const safeID=value=>typeof value==='string'&&value.length>0&&value.length<=256&&!/[\x00\r\n]/.test(value);
function frame(value){const data=Buffer.from(JSON.stringify(value));if(data.length>1024*1024)throw Error('execution_protocol_invalid');const header=Buffer.alloc(4);header.writeUInt32BE(data.length);return Buffer.concat([header,data]);}

// Native main owns choosing, consent, pairing and the fixed helper. Renderer
// input is only an enum: never a path, command, service URL, grant or approval.
class ExecutionRuntime extends EventEmitter{
  constructor(options){super();Object.assign(this,options);this.child=null;this.pending=false;this.generation=0;this.state='idle';this.resource=null;}
  publish(state){this.state=state;this.emit('status',this.snapshot());}
  snapshot(){return {state:this.state,...(this.resource?{resource:this.resource}:{})};}
  async start(request){
    if(!request||Object.keys(request).length!==1||!Object.hasOwn(modes,request.access))throw Error('execution_request_invalid');
    if(this.pending||this.child||this.state==='uncertain')throw Error('execution_host_in_use');
    const generation=++this.generation;this.pending=true;this.publish('selecting');
    const current=()=>{if(generation!==this.generation)throw Error('execution_cancelled');};
    try{
      const service=this.service(),identity=await this.identity(service);current();
      if(identity.product!=='offgrid'||identity.api_version!==2||!safeID(identity.workspace_id)||!identity.capabilities?.includes('native-execution-preview-v1')||String(identity.version).replace(/^v/,'')!==this.version)throw Error('execution_upgrade_required');
      const principal=await this.principal(service);current();
      await this.verify(this.root);current();
      const folder=await this.choose();current();if(!folder){this.publish('idle');return this.snapshot();}
      const permissions={...modes[request.access]};
      const approval={folder,permissions,service,workspace:identity.workspace_id,actor:principal};
      let pin=null,remembered=false;
      if(this.trust){try{pin=await this.trust.pin(approval);remembered=await this.trust.matches(pin);}catch{pin=null;}current();}
      if(!remembered){
        const decision=await this.confirm({...approval,canRemember:!!pin});current();
        if(decision!==true&&decision!=='remember'){this.publish('idle');return this.snapshot();}
        if(decision==='remember'){if(!pin)throw Error('execution_scope_changed');await this.trust.remember(pin);current();}
      }
      if(pin&&(await this.trust.pin(approval)).id!==pin.id)throw Error('execution_scope_changed');current();
      const fresh=await this.identity(service);current();
      if(this.service()!==service||fresh.workspace_id!==identity.workspace_id||await this.principal(service)!==principal)throw Error('execution_scope_changed');current();
      const pair=await this.pairing(service);current();
      if(pair.protocol!==1||pair.workspace!==identity.workspace_id||pair.actor!==principal||!safeID(pair.actor)||!/^[a-f0-9]{64}$/.test(pair.code))throw Error('execution_pairing_failed');
      this.publish('starting');
      const file=path.join(this.root,process.platform==='win32'?'offgrid-executor.exe':'offgrid-executor');
      const env={};for(const key of ['PATH','SystemRoot','WINDIR','COMSPEC','PATHEXT','TEMP','TMP','TMPDIR','HOME','USERPROFILE','LOCALAPPDATA','APPDATA','LANG','LC_ALL'])if(process.env[key])env[key]=process.env[key];
      const child=this.spawn(file,['--connect','--state-dir',this.directory],{stdio:['pipe','pipe','pipe'],shell:false,windowsHide:true,env});this.child=child;
      let buffer=Buffer.alloc(0),ready=false,stopped=false,settled=false;
      const result=await new Promise((resolve,reject)=>{
        const finish=(error,value)=>{if(settled)return;settled=true;clearTimeout(timer);error?reject(error):resolve(value);};
        const timer=setTimeout(()=>{child.stdin.end();finish(Error('execution_start_timeout'));},20000);
        child.stdout.on('data',chunk=>{
          if(buffer.length+chunk.length>1024*1024+4){child.stdin.end();finish(Error('execution_protocol_invalid'));return;}
          buffer=Buffer.concat([buffer,chunk]);
          while(buffer.length>=4){
            const size=buffer.readUInt32BE();if(size<1||size>1024*1024){child.stdin.end();finish(Error('execution_protocol_invalid'));return;}if(buffer.length<size+4)return;
            let message;try{message=JSON.parse(buffer.subarray(4,size+4).toString('utf8'));}catch{child.stdin.end();finish(Error('execution_protocol_invalid'));return;}buffer=buffer.subarray(size+4);
            if(message.protocol!==1){child.stdin.end();finish(Error('execution_protocol_invalid'));return;}
            if(message.state==='stopped'){stopped=true;continue;}
            const resource=message.resource,ref=resource?.reference;
            if(message.state!=='ready'||ready||!ref||!['host','session','grant'].every(key=>safeID(ref[key]))||ref.revision!==1||resource.backend!=='local'||!path.isAbsolute(resource.root)||path.resolve(resource.root)!==path.resolve(folder)||JSON.stringify(resource.permissions)!==JSON.stringify(permissions)){child.stdin.end();finish(Error('execution_scope_changed'));return;}
            if(generation!==this.generation){child.stdin.end();finish(Error('execution_cancelled'));return;}
            ready=true;this.owner={service,workspace:identity.workspace_id,principal};this.resource=resource;this.publish('ready');finish(null,this.snapshot());
          }
        });
        // Diagnostics can contain private paths. Drain without forwarding or
        // accumulating arbitrary helper content in renderer state/log exports.
        child.stderr.resume();
        child.once('error',()=>{if(this.child===child)this.child=null;this.publish('uncertain');finish(Error('execution_host_failed'));});
        child.once('exit',code=>{child.__executionExited=true;if(this.child===child)this.child=null;this.resource=null;this.owner=null;this.publish(code===0&&stopped?'stopped':'uncertain');finish(Error('execution_host_failed'));});
        child.stdin.on('error',()=>finish(Error('execution_host_failed')));
        child.stdin.write(frame({protocol:1,service,pairing:pair,folder,permissions}));
      });
      return result;
    }catch(error){if(this.child){try{await this.stop();}catch{/* Keep uncertain stopping visible. */}}else if(generation===this.generation&&this.state!=='uncertain')this.publish('error');throw error;}
    finally{if(generation===this.generation)this.pending=false;}
  }
  async status(){
    if(this.owner){const owner=this.owner;try{if(this.service()!==owner.service||await this.principal(owner.service)!==owner.principal||(await this.identity(owner.service)).workspace_id!==owner.workspace)await this.stop();}catch{await this.stop();}}
    return this.snapshot();
  }
  async stop(){
    ++this.generation;this.pending=false;this.resource=null;this.owner=null;
    const child=this.child;if(!child){if(this.state!=='uncertain')this.publish('stopped');return this.snapshot();}
    this.publish('stopping');
    await new Promise((resolve,reject)=>{
      const done=()=>{clearTimeout(timer);this.state==='stopped'?resolve():reject(Error('execution_stop_unconfirmed'));};
      const timer=setTimeout(()=>{child.removeListener('exit',done);this.publish('uncertain');reject(Error('execution_stop_unconfirmed'));},this.stopTimeout??5000);
      child.once('exit',done);child.stdin.end();if(child.__executionExited)done();
    });
    return this.snapshot();
  }
}
module.exports={ExecutionRuntime,frame};
