const {EventEmitter} = require('node:events');
const path = require('node:path');
const APPROVAL_MODES = new Set(['ask_every_time','scoped_changes','full_task']);
const validAccessRequest = value => value === undefined || (typeof value === 'string' && /^input-[a-zA-Z0-9_-]{1,122}$/.test(value));

function validateTarget(value) {
  if (value === 'demo') return 'demo';
  if (typeof value !== 'string' || value.length > 2048) throw Error('target_invalid');
  const url = new URL(value);
  if (url.protocol !== 'https:' || url.username || url.password || require('node:net').isIP(url.hostname) || url.hostname.startsWith('[') || !url.hostname.includes('.') || /\.(localhost|local|internal|test|invalid)$/.test(url.hostname)) throw Error('target_invalid');
  return url.pathname==='/' && !url.search && !url.hash ? url.origin : url.href;
}
// Deep links identify saved work only. They never carry prompts, service
// addresses, credentials, targets, approval policies or executable commands.
function parseComputerLink(value) {
  if (typeof value !== 'string') return null;
  try {
    const url = new URL(value);
    if (url.protocol !== 'offgrid:' || url.hostname !== 'computer' || url.port || url.username || url.password || url.hash || !['','/'].includes(url.pathname)) return null;
    const entries = [...url.searchParams.entries()];
    if (!entries.length) return {task:null};
    if (entries.length !== 1 || entries[0][0] !== 'task' || !/^run-[a-f0-9]{32}$/.test(entries[0][1])) return null;
    return {task:entries[0][1]};
  } catch { return null; }
}
function isComputerLink(value) { return parseComputerLink(value) !== null; }

// One owned worker. Never use ports/PIDs to terminate unrelated processes.
class ComputerRuntime extends EventEmitter {
  constructor({fork, root, directory, service, confirm, identity, pairing, task, principal, verify = async () => {}, stopGraceMs = 35000, stopAckMs = 5000}) {
    super(); Object.assign(this,{fork,root,directory,service,confirm,identity,pairing,task,principal,verify,stopGraceMs,stopAckMs});
    this.state = {state:'idle'}; this.generation = 0;
  }
  publish(value) { this.state = value; this.emit('status',value); return value; }
  async desktopContext(workspace) {
    const service=this.service(),url=new URL(service);
    if(!['127.0.0.1','[::1]'].includes(url.hostname)||!['http:','https:'].includes(url.protocol)||url.username||url.password||url.pathname!=='/'||url.search||url.hash)throw Error('service_invalid');
    const identity=await this.identity(service);
    if(identity.product!=='offgrid'||identity.api_version!==2||identity.workspace_id!==workspace)throw Error('workspace_changed');
    if(!identity.capabilities?.includes('task-desktop-sessions-v3'))throw Error('computer_upgrade_required');
    const actor=await this.principal(service);
    if(typeof actor!=='string'||!/^[a-zA-Z0-9:_-]{1,256}$/.test(actor))throw Error('computer_scope_violation');
    return {service,workspace,actor};
  }
  async desktopAccessStatus() {
    const access=this.desktopAccess,generation=this.generation;
    if(access){
      try{const current=await this.desktopContext(access.workspace);if(JSON.stringify(current)!==JSON.stringify(access))throw Error('computer_scope_violation');}
      catch{if(generation===this.generation)await this.stop();}
    }
    return {enabled:!!this.desktopAccess,workspace:this.desktopAccess?.workspace};
  }
  async enableDesktopAccess(request) {
    if(!request||Object.keys(request).join(',')!=='workspace'||typeof request.workspace!=='string'||!request.workspace)throw Error('invalid_session');
    if(this.pending||this.child||this.stopping)throw Error('session_active');
    this.pending=true;const generation=++this.generation;
    try{
      const context=await this.desktopContext(request.workspace);if(generation!==this.generation)throw Error('computer_stopped');
      await this.verify(this.root);if(generation!==this.generation)throw Error('computer_stopped');
      const env={...process.env};delete env.NODE_OPTIONS;delete env.NODE_PATH;delete env.ELECTRON_RUN_AS_NODE;
      const child=this.fork(path.join(this.root,'native-managed.cjs'),[],{stdio:'ignore',serviceName:'OffGrid desktop access',env});this.child=child;
      child.on('message',message=>{
        if(this.child!==child||generation!==this.generation)return;
        if(message?.state==='booted')child.postMessage({type:'enable-desktop',...context,directory:this.directory});
        if(message?.state==='consent')this.publish({state:'consent'});
        if(message?.state==='access_ready'){this.desktopAccess=context;this.accessRequest=undefined;this.publish({state:'access_ready'});}
        if(message?.state==='ready'&&message.target?.id)this.publish({state:'ready',target:message.target});
        if(message?.state==='error')this.publish({state:'error',code:message.code});
      });
      child.once('exit',()=>{if(this.child===child){this.desktopAccess=null;clearInterval(this.accessMonitor);this.child=null;this.publish({state:'stopped'});}});
      this.publish({state:'starting'});
      await this.waitNativeState('access_ready',generation,300000);
      // Recheck after the human response, including account changes during consent.
      await this.desktopAccessStatus();if(generation!==this.generation||!this.desktopAccess)throw Error('computer_scope_violation');
      this.accessMonitor=setInterval(()=>{if(!this.accessChecking){this.accessChecking=this.desktopAccessStatus().finally(()=>{this.accessChecking=null;});}},2000);
      this.accessMonitor.unref?.();
      return {enabled:true,workspace:context.workspace};
    }catch(error){if(generation===this.generation)await this.stop();throw error;}
    finally{this.pending=false;}
  }
  async startDesktop(request) {
    const reuse=!!this.desktopAccess&&!!this.child&&this.state.state==='access_ready';
    if(this.pending||this.child&&!reuse||this.stopping)throw Error('session_active');
    if(!request||Object.keys(request).sort().join(',')!=='requestId,runId,workspace'||!/^run-[a-f0-9]{32}$/.test(request.runId)||!request.requestId||!validAccessRequest(request.requestId)||typeof request.workspace!=='string'||!request.workspace)throw Error('invalid_session');
    this.pending=true;this.accessRequest=request.requestId;const generation=reuse?this.generation:++this.generation;
    try {
      const service=this.service(),url=new URL(service);
      if(!['127.0.0.1','[::1]'].includes(url.hostname)||!['http:','https:'].includes(url.protocol)||url.username||url.password||url.pathname!=='/'||url.search||url.hash)throw Error('service_invalid');
      const identity=await this.identity(service);if(generation!==this.generation)throw Error('computer_stopped');
      if(identity.product!=='offgrid'||identity.api_version!==2||identity.workspace_id!==request.workspace)throw Error('workspace_changed');
      if(!identity.capabilities?.includes('task-desktop-sessions-v3'))throw Error('computer_upgrade_required');
      // Prompt and policy come from the authenticated saved task, not IPC text
      // that could misdescribe what the user is consenting to.
      const saved=await this.task(service,request.runId);if(generation!==this.generation)throw Error('computer_stopped');
      if(saved.run_id!==request.runId||saved.status!=='waiting_for_input'||saved.pending_input?.id!==request.requestId||!['desktop','app'].includes(saved.pending_input?.mode)||saved.requested_computer_scope==='selected_window'||!APPROVAL_MODES.has(saved.requested_computer_approval_mode)||typeof saved.prompt!=='string'||!saved.prompt)throw Error('computer_scope_violation');
      if(reuse){
        await this.desktopAccessStatus();
        if(generation!==this.generation||!this.desktopAccess||this.desktopAccess.service!==service||this.desktopAccess.workspace!==request.workspace||saved.requested_computer_approval_mode!=='full_task')throw Error('computer_scope_violation');
      }
      await this.verify(this.root);if(generation!==this.generation)throw Error('computer_stopped');
      const {code}=await this.pairing(service,{protocol_version:3,run_id:request.runId,input_id:request.requestId});
      if(generation!==this.generation)throw Error('computer_stopped');
      if(!/^[a-f0-9]{64}$/.test(code))throw Error('pairing_failed');
      if(reuse){
        this.publish({state:'starting'});
        this.child.postMessage({type:'start-desktop',service,directory:this.directory,workspace:request.workspace,actor:this.desktopAccess.actor,run:request.runId,input:request.requestId,prompt:saved.prompt,approvalMode:saved.requested_computer_approval_mode,code});
        return await this.waitNativeState('ready',generation,60000);
      }
      const env={...process.env};delete env.NODE_OPTIONS;delete env.NODE_PATH;delete env.ELECTRON_RUN_AS_NODE;
      const child=this.fork(path.join(this.root,'native-managed.cjs'),[],{stdio:'ignore',serviceName:'OffGrid desktop task',env});
      this.child=child;
      child.on('message',message=>{
        if(this.child!==child||generation!==this.generation)return;
        if(message?.state==='booted')child.postMessage({type:'start-desktop',service,directory:this.directory,workspace:request.workspace,run:request.runId,input:request.requestId,prompt:saved.prompt,approvalMode:saved.requested_computer_approval_mode,code});
        if(message?.state==='consent')this.publish({state:'consent',target:message.target});
        if(message?.state==='ready'&&message.target?.id)this.publish({state:'ready',target:message.target});
        if(message?.state==='error')this.publish({state:'error',code:message.code});
        if(message?.state==='stopped')this.publish({state:'stopped'});
      });
      child.once('exit',()=>{if(this.child===child){this.child=null;if(this.state.state!=='error')this.publish({state:'stopped'});}});
      this.publish({state:'starting'});
      return await this.waitNativeState('ready',generation,300000);
    }catch(error){if(generation===this.generation)this.publish({state:'error',code:error.message});throw error;}
    finally{this.pending=false;}
  }
  async discoverNative(request) {
    if(this.pending || this.child || this.stopping) throw Error('session_active');
    if(!request || Object.keys(request).some(k=>!['workspace','requestId'].includes(k)) || typeof request.workspace!=='string' || !request.workspace || !validAccessRequest(request.requestId)) throw Error('invalid_session');
    this.accessRequest=request.requestId;
    this.pending=true;const generation=++this.generation;
    try {
      const service=this.service(),url=new URL(service);
      if(!['127.0.0.1','[::1]'].includes(url.hostname) || !['http:','https:'].includes(url.protocol) || url.username || url.password || url.pathname!=='/' || url.search || url.hash) throw Error('service_invalid');
      const identity=await this.identity(service);
      if(generation!==this.generation)return this.state;
      if(identity.product!=='offgrid' || identity.api_version!==2 || identity.workspace_id!==request.workspace) throw Error('workspace_changed');
      if(!identity.capabilities?.includes('native-computer-sessions-v2'))throw Error('computer_upgrade_required');
      // Check the signed-in service administrator before exposing local target
      // titles. This unused short-lived code is replaced after target selection.
      await this.pairing(service);
      if(generation!==this.generation)return this.state;
      await this.verify(this.root);
      if(generation!==this.generation)return this.state;
      const env={...process.env};delete env.NODE_OPTIONS;delete env.NODE_PATH;delete env.ELECTRON_RUN_AS_NODE;
      const child=this.fork(path.join(this.root,'native-managed.cjs'),[],{stdio:'ignore',serviceName:'OffGrid application assistant',env});
      this.child=child;this.nativeContext={service,workspace:request.workspace,generation};
      child.on('message',message=>{
        if(this.child!==child || generation!==this.generation)return;
        if(message?.state==='booted')child.postMessage({type:'discover',service,workspace:request.workspace,directory:this.directory});
        if(message?.state==='selecting' && Array.isArray(message.targets))this.publish({state:'selecting',targets:message.targets});
        if(message?.state==='ready' && message.target?.id)this.publish({state:'ready',target:message.target});
        if(message?.state==='error')this.publish({state:'error',code:message.code});
        if(message?.state==='stopped')this.publish({state:'stopped'});
      });
      child.once('exit',()=>{if(this.child===child){this.child=null;this.nativeContext=null;if(this.state.state!=='error')this.publish({state:'stopped'});}});
      this.publish({state:'starting'});
      return await this.waitNativeState('selecting',generation);
    } catch(error) {if(generation===this.generation)this.publish({state:'error',code:error.message});throw error;}
    finally {this.pending=false;}
  }
  waitNativeState(expected,generation,timeout=60000) {
    return new Promise((resolve,reject)=>{
      const check=()=>{
        if(generation!==this.generation || this.state.state==='stopped')return finish(Error('computer_stopped'));
        if(this.state.state==='error')return finish(Error(this.state.code || 'computer_worker_unavailable'));
        if(this.state.state===expected)return finish();
      };
      const finish=error=>{clearTimeout(timer);this.removeListener('status',check);error?reject(error):resolve(this.state);};
      const timer=setTimeout(()=>{void this.stop();finish(Error('computer_worker_timeout'));},timeout);
      this.on('status',check);check();
    });
  }
  async startNative(request) {
    if(this.pending || this.stopping || !this.child || !this.nativeContext || this.state.state!=='selecting') throw Error('computer_session_unavailable');
    request={...request,approvalMode:request?.approvalMode??'scoped_changes'};
    if(!request || Object.keys(request).some(k=>!['workspace','target','approvalMode','requestId'].includes(k)) || request.requestId!==this.accessRequest || request.workspace!==this.nativeContext.workspace || typeof request.target!=='string' || !this.state.targets.some(t=>t.id===request.target) || !APPROVAL_MODES.has(request.approvalMode)) throw Error('computer_stale_target');
    this.pending=true;const {service,workspace,generation}=this.nativeContext;
    try {
      const identity=await this.identity(service);
      if(generation!==this.generation)return this.state;
      if(identity.product!=='offgrid' || identity.workspace_id!==workspace || identity.api_version!==2)throw Error('workspace_changed');
      if(!identity.capabilities?.includes('native-computer-sessions-v2'))throw Error('computer_upgrade_required');
      const {code}=await this.pairing(service);
      if(generation!==this.generation)return this.state;
      if(!/^[a-f0-9]{64}$/.test(code))throw Error('pairing_failed');
      this.publish({state:'starting'});
      this.child.postMessage({type:'start-native',target:request.target,code,approvalMode:request.approvalMode});
      return await this.waitNativeState('ready',generation);
    } finally {this.pending=false;}
  }
  async start(request) {
    if (this.pending || this.child || this.stopping) throw Error('session_active');
    request={...request,approvalMode:request?.approvalMode??'scoped_changes'};
    if (!request || Object.keys(request).some(k=>!['origin','workspace','networkMode','upload','approvalMode','requestId'].includes(k)) || typeof request.workspace !== 'string' || !request.workspace || !APPROVAL_MODES.has(request.approvalMode) || !validAccessRequest(request.requestId)) throw Error('invalid_session');
    if (request.upload && (Object.keys(request.upload).sort().join(',')!=='id,name,path,sha256,size' || typeof request.upload.path!=='string' || !path.isAbsolute(request.upload.path) || typeof request.upload.name!=='string' || request.upload.name.length>255 || !Number.isSafeInteger(request.upload.size) || request.upload.size<0 || request.upload.size>512*1024*1024 || typeof request.upload.id!=='string' || !/^[a-f0-9]{32}$/.test(request.upload.id) || typeof request.upload.sha256!=='string' || !/^[a-f0-9]{64}$/.test(request.upload.sha256))) throw Error('computer_upload_invalid');
    const origin = validateTarget(request.origin);
    const networkMode = request.networkMode ?? 'direct';
    if (!['direct','trusted-vpn'].includes(networkMode) || (origin === 'demo' && networkMode !== 'direct')) throw Error('network_mode_invalid');
    this.accessRequest=request.requestId;
    this.pending = true; const generation = ++this.generation;
    try {
      const service = this.service();
      const url = new URL(service);
      if (!['127.0.0.1','[::1]'].includes(url.hostname) || !['http:','https:'].includes(url.protocol) || url.username || url.password || url.pathname !== '/' || url.search || url.hash) throw Error('service_invalid');
      const identity = await this.identity(service);
      if (generation !== this.generation) return this.state;
      if (identity.product !== 'offgrid' || identity.api_version !== 2 || identity.workspace_id !== request.workspace) throw Error('workspace_changed');
      this.publish({state:'consent'});
      if (!await this.confirm(origin, service, networkMode, request.approvalMode)) return this.publish({state:'idle',code:'consent_declined'});
      if (generation !== this.generation) return this.state;
      this.publish({state:'starting'});
      await this.verify(this.root);
      if (generation !== this.generation) return this.state;
      // Enroll only after consent and integrity checks, so time spent reading
      // the native prompt cannot expire a code or create an unused enrollment.
      const {code}=await this.pairing(service);
      if (generation !== this.generation) return this.state;
      if (!/^[a-f0-9]{64}$/.test(code)) throw Error('pairing_failed');
      const env = {...process.env, PLAYWRIGHT_BROWSERS_PATH:path.join(this.root,'browsers')};
      delete env.NODE_OPTIONS; delete env.NODE_PATH; delete env.ELECTRON_RUN_AS_NODE;
      const child = this.fork(path.join(this.root,'managed.cjs'),[],{stdio:'ignore',serviceName:'OffGrid browser assistant',env});
      this.child = child;
      return await new Promise((resolve, reject) => {
        let settled = false;
        const finish = (error) => { if (settled) return; settled = true; clearTimeout(timer); error ? reject(Error(error)) : resolve(this.state); };
        const timer = setTimeout(()=>{ void this.stop(); finish('startup_timeout'); },60000);
        child.on('message', message => {
          if (this.child !== child || generation !== this.generation) return;
        if (message?.state === 'booted') child.postMessage({type:'start',service,origin,networkMode,code,workspace:request.workspace,directory:this.directory,upload:request.upload,approvalMode:request.approvalMode});
          else if (message?.state === 'ready' && message.target?.id) { this.publish({state:'ready',target:{id:message.target.id,origin:message.target.origin}}); finish(); }
          else if (message?.state === 'error') { this.publish({state:'error',code:message.code}); finish(message.code); }
          else if (message?.state === 'stopped') this.publish({state:'stopped'});
        });
        child.once('exit', () => {
          if (this.child === child) { this.child = null; if (this.state.state !== 'error') this.publish({state:'stopped'}); }
          finish('companion_stopped');
        });
      });
    } catch (error) { if (generation !== this.generation) return this.state; this.publish({state:'error',code:error.message}); throw error; }
    finally { this.pending = false; }
  }
  // Check and revoke in the same main-process turn. Renderer status checks alone
  // race with a different task acquiring the controller. No match is a no-op.
  stopAccess(request) {
    if (!request || Object.keys(request).length!==1) throw Error('invalid_session');
    if (Object.hasOwn(request,'requestId')) {
      if (!request.requestId || !validAccessRequest(request.requestId)) throw Error('invalid_session');
      if (request.requestId!==this.accessRequest) return Promise.resolve({state:'not_owned'});
    } else if (Object.hasOwn(request,'session')) {
      if (typeof request.session!=='string' || !request.session || request.session.length>128) throw Error('invalid_session');
      if (request.session!==this.state.target?.id) return Promise.resolve({state:'not_owned'});
    } else throw Error('invalid_session');
    return this.stop();
  }
  stop() {
    if (this.stopping) return this.stopping;
    this.stopping = this.stopOwned().finally(() => { this.stopping = null; });
    return this.stopping;
  }
  async stopOwned() {
    ++this.generation;
    this.desktopAccess=null;clearInterval(this.accessMonitor);
    const child = this.child;
    if (child) {
      const stopped = await new Promise(resolve => {
        let timer, acknowledgement, settled=false;
        const finish=ok=>{if(settled)return;settled=true;clearTimeout(timer);clearTimeout(acknowledgement);child.removeListener('exit',exited);resolve(ok);};
        const exited=()=>finish(true);
        const force=()=>{
          acknowledgement=setTimeout(()=>finish(false),this.stopAckMs);
          try {if(child.kill()===false)finish(false);} catch {finish(false);}
        };
        child.once('exit',exited);
        timer=setTimeout(force,this.stopGraceMs);
        try {child.postMessage({type:'stop'});} catch {clearTimeout(timer);force();}
      });
      // A lost exit acknowledgement is not success. Retain ownership and refuse
      // new sessions; users can retry Stop or close the browser manually.
      if (!stopped) return this.publish({state:'error',code:'stop_unconfirmed'});
    }
    if (this.child === child) this.child = null;
    return this.publish({state:'stopped'});
  }
}
module.exports = {ComputerRuntime, validateTarget, isComputerLink, parseComputerLink};
