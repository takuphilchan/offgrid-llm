const {randomUUID}=require('node:crypto');
const {savedBrowserTask,browserFileTask,supportsBrowserFileBinding}=require('./computer-browser-task');
const sessionID=value=>typeof value==='string'&&/^[a-zA-Z0-9_-]{1,128}$/.test(value);
const actorID=value=>typeof value==='string'&&/^[a-zA-Z0-9:_-]{1,256}$/.test(value);
const exact=(value,keys)=>value&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).sort().join(',')===keys;

async function computerPrincipal(response){
  if(response.status===401||response.status===403)throw Error('computer_scope_violation');
  if(!response.ok)throw Error('service_unavailable');
  const account=await response.json();
  if(account?.auth_required===false)return 'local-admin';
  if(account?.auth_required!==true||account.authenticated!==true||account.user?.role!=='admin'||!actorID(account.user.id))throw Error('computer_scope_violation');
  return account.user.id;
}

// Resource ownership remains with ComputerRuntime. A retained browser is not
// authority: each continuation needs current principal, saved task and consent.
class BrowserControl {
  constructor(runtime,{timeoutMs=65000,monitorMs=2000}={}){this.runtime=runtime;this.timeoutMs=timeoutMs;this.monitorMs=monitorMs;}
  bind(context){this.invalidate();this.context={...context,protocol:0,target:null};}
  get supported(){const c=this.context;return !!c&&c.protocol===1&&c.workspaceEnabled&&c.taskPairing&&actorID(c.actor);}
  status(){return {browserManualProtocol:this.supported?1:0};}
  current(c){return this.context===c&&this.runtime.child===c.child&&this.runtime.generation===c.generation&&!this.runtime.stopping;}
  requireCurrent(c){if(!this.current(c))throw Error('computer_stopped');}
  invalidate(){
    clearInterval(this.monitor);this.monitor=null;
    if(this.operation?.kind==='resume')this.runtime.pending=false;
    if(this.operation&&!this.operation.done)this.finish(this.operation,Error('computer_stopped'));
    this.context=null;this.operation=null;
  }
  initial(message){
    const c=this.context;if(!c||!this.current(c))return false;
    if(message?.state==='booted'){c.protocol=message.browserManualProtocol===1?1:0;return false;}
    if(message?.requestId!==undefined){this.receive(message);return true;}
    if(message?.state==='ready'&&message.target?.id){
      if(c.target)return true; // Only initial readiness is uncorrelated.
      if(!sessionID(message.target.id)||message.target.origin!==(c.origin==='demo'?'offgrid-demo://research':new URL(c.origin).origin)){
        void this.runtime.stop();return true;
      }
      c.target=Object.freeze({id:message.target.id,origin:message.target.origin});
      if(this.supported&&!this.monitor){
        this.monitor=setInterval(()=>{if(!this.checking)this.checking=this.checkContext(c,false).catch(()=>{}).finally(()=>{this.checking=null;});},this.monitorMs);
        this.monitor.unref?.();
      }
    }
    return false;
  }
  publish(c,state,code){
    return this.runtime.publish({state,...(code?{code}:{}),...(c.target?{target:c.target}:{}),...this.status()});
  }
  async checkContext(c,requireCapabilities=true){
    this.requireCurrent(c);
    // service() also enforces the owner's maintenance admission hold.
    const service=this.runtime.service();
    if(service!==c.service){await this.runtime.stop();throw Error('computer_browser_scope_changed');}
    const identity=await this.runtime.identity(service);this.requireCurrent(c);
    if(identity.product!=='offgrid'||identity.api_version!==2||identity.workspace_id!==c.workspace){
      await this.runtime.stop();throw Error('computer_browser_scope_changed');
    }
    let actor;
    try{actor=await this.runtime.principal(service);}
    catch(error){
      // A validated denial is revocation. Network/server failures retain local
      // cleanup ownership, but cannot authorize renewed automation.
      if(error.message==='computer_scope_violation'&&this.current(c))await this.runtime.stop();
      throw error;
    }
    this.requireCurrent(c);
    if(actor!==c.actor){await this.runtime.stop();throw Error('computer_browser_scope_changed');}
    if(requireCapabilities&&(!identity.capabilities?.includes('browser-workspace-v1')||!identity.capabilities?.includes('browser-task-pairing-v1')))throw Error('computer_upgrade_required');
    if(c.fileTask&&!supportsBrowserFileBinding(identity))throw Error('computer_upgrade_required');
    return identity;
  }
  begin(kind,key,c){
    let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});promise.catch(()=>{});
    const op={kind,key,context:c,id:randomUUID(),promise,resolve,reject,done:false,posted:false};
    this.operation=op;return op;
  }
  finish(op,error,result){
    if(op.done)return;op.done=true;
    if(error)op.reject(error);else op.resolve(result);
  }
  wait(op){
    if(op.done)return op.promise;
    let timer;
    return Promise.race([op.promise,new Promise((_,reject)=>{
      timer=setTimeout(()=>{
        const code=op.kind==='takeover'?'computer_handoff_unconfirmed':'computer_resume_unconfirmed';
        if(this.operation===op&&this.current(op.context))this.publish(op.context,op.kind==='takeover'?'taking_over':'resuming',code);
        reject(Error(code));
      },this.timeoutMs);
    })]).finally(()=>clearTimeout(timer));
  }
  receive(message){
    const op=this.operation,c=this.context;
    if(!op||op.done||op.context!==c||!this.current(c)||message.requestId!==op.id)return;
    if(message.state==='error'){
      const code=typeof message.code==='string'&&/^[a-z][a-z0-9_]{0,127}$/.test(message.code)?message.code:'browser_unavailable';
      if(op.kind==='resume')this.runtime.pending=false;
      this.publish(c,'error',code);this.finish(op,Error(code));return;
    }
    const expected=op.kind==='takeover'?'manual':'ready';
    if(message.state!==expected)return;
    const target=message.target;
    if(!sessionID(target?.id)||target.origin!==c.target.origin||(op.kind==='takeover'?target.id!==c.target.id:target.id===op.previousSession)){
      this.publish(c,'error','computer_browser_scope_changed');this.finish(op,Error('computer_browser_scope_changed'));void this.runtime.stop();return;
    }
    c.target=Object.freeze({id:target.id,origin:target.origin});
    if(op.kind==='resume')this.runtime.pending=false;
    this.finish(op,null,this.publish(c,expected));
  }
  takeover(request){
    if(!exact(request,'session')||!sessionID(request.session))throw Error('invalid_session');
    const c=this.context;
    if(!c&&this.runtime.child&&request.session===this.runtime.state.target?.id)return Promise.resolve({state:'unsupported',code:'computer_browser_manual_unsupported'});
    if(!c||!this.current(c)||request.session!==c.target?.id)return Promise.resolve({state:'not_owned'});
    if(!this.supported)return Promise.resolve({state:'unsupported',code:'computer_browser_manual_unsupported'});
    const key=request.session;
    if(this.operation?.kind==='takeover'&&this.operation.key===key)return this.wait(this.operation);
    if(this.runtime.pending||this.operation&&!this.operation.done)throw Error('session_active');
    if(this.runtime.state.state==='manual')return Promise.resolve(this.runtime.state);
    if(this.runtime.state.state!=='ready')throw Error('computer_session_unavailable');
    const op=this.begin('takeover',key,c);
    this.publish(c,'taking_over');
    try{op.posted=true;c.child.postMessage({type:'takeover',requestId:op.id,session:key});}
    catch{this.finish(op,Error('computer_handoff_unconfirmed'));}
    return this.wait(op);
  }
  resume(request){
    if(!exact(request,'requestId,runId,session,workspace')||!sessionID(request.session)||!/^run-[a-f0-9]{32}$/.test(request.runId)||!/^input-[a-zA-Z0-9_-]{1,122}$/.test(request.requestId)||typeof request.workspace!=='string'||!request.workspace)throw Error('invalid_session');
    request=Object.freeze({...request});
    const key=JSON.stringify([request.workspace,request.runId,request.requestId,request.session]),c=this.context;
    if(!c&&this.runtime.child&&request.session===this.runtime.state.target?.id)return Promise.resolve({state:'unsupported',code:'computer_browser_manual_unsupported'});
    if(this.operation?.kind==='resume'&&this.operation.key===key&&c&&this.current(c))return this.wait(this.operation);
    if(!c||!this.current(c)||request.session!==c.target?.id)return Promise.resolve({state:'not_owned'});
    if(!this.supported)return Promise.resolve({state:'unsupported',code:'computer_browser_manual_unsupported'});
    if(request.workspace!==c.workspace)throw Error('computer_browser_scope_changed');
    if(this.runtime.pending||this.operation&&!this.operation.done)throw Error('session_active');
    if(this.runtime.state.state!=='manual')throw Error('computer_session_unavailable');
    const op=this.begin('resume',key,c);op.previousSession=request.session;
    this.runtime.pending=true;
    void this.prepareResume(op,request).catch(error=>{
      if(this.current(c)&&!op.done){
        this.publish(c,op.posted?'resuming':'manual',error.message);
        if(!op.posted&&this.operation===op)this.operation=null;
      }
      this.finish(op,error);
    }).finally(()=>{if(this.current(c))this.runtime.pending=false;});
    return this.wait(op);
  }
  async savedRequest(c,request){
    const saved=await this.runtime.task(c.service,request.runId);this.requireCurrent(c);
    savedBrowserTask(saved,{runId:request.runId,inputId:request.requestId,previousSession:request.session,approvalMode:c.approvalMode,origin:c.origin});
    return saved;
  }
  async prepareResume(op,request){
    const c=op.context;
    await this.checkContext(c);
    await this.savedRequest(c,request);
    if(!await this.runtime.confirm(c.origin,c.service,c.networkMode,c.approvalMode,c.scope))throw Error('consent_declined');
    this.requireCurrent(c);
    await this.checkContext(c);
    await this.savedRequest(c,request);
    await this.runtime.verify(this.runtime.root);this.requireCurrent(c);
    const {code}=await this.runtime.pairing(c.service,{protocol_version:1,run_id:request.runId,input_id:request.requestId,previous_session:request.session,approval_mode:c.approvalMode});
    this.requireCurrent(c);
    if(!/^[a-f0-9]{64}$/.test(code))throw Error('pairing_failed');
    await this.checkContext(c);
    await this.savedRequest(c,request);
    const browserWorkspaceRoots=c.fileTask?await this.runtime.protectedBrowserWorkspaceRoots():null;this.requireCurrent(c);
    this.runtime.accessRequest=request.requestId;
    this.publish(c,'resuming');op.posted=true;
    c.child.postMessage({type:'resume',requestId:op.id,previousSession:request.session,service:c.service,directory:this.runtime.directory,browserWorkspaceRoots,origin:c.origin,code,workspace:c.workspace,networkMode:c.networkMode,approvalMode:c.approvalMode,additionalOrigins:[...c.additionalOrigins],expectedBrowserWorkspace:c.workspaceEnabled,...(c.fileTask?{fileTask:browserFileTask({actor:c.actor,workspace:c.workspace,runId:request.runId,inputId:request.requestId})}:{})});
    await op.promise;
  }
}
module.exports={BrowserControl,computerPrincipal};
