const {EventEmitter} = require('node:events');
const path = require('node:path');
const APPROVAL_MODES = new Set(['ask_every_time','scoped_changes','full_task']);

function validateTarget(value) {
  if (value === 'demo') return 'demo';
  if (typeof value !== 'string' || value.length > 2048) throw Error('target_invalid');
  const url = new URL(value);
  if (url.protocol !== 'https:' || url.username || url.password || require('node:net').isIP(url.hostname) || url.hostname.startsWith('[') || !url.hostname.includes('.') || /\.(localhost|local|internal|test|invalid)$/.test(url.hostname)) throw Error('target_invalid');
  return url.pathname==='/' && !url.search && !url.hash ? url.origin : url.href;
}
function isComputerLink(value) { return value === 'offgrid://computer' || value === 'offgrid://computer/'; }

// One owned worker. Never use ports/PIDs to terminate unrelated processes.
class ComputerRuntime extends EventEmitter {
  constructor({fork, root, directory, service, confirm, identity, pairing, verify = async () => {}, stopGraceMs = 35000, stopAckMs = 5000}) {
    super(); Object.assign(this,{fork,root,directory,service,confirm,identity,pairing,verify,stopGraceMs,stopAckMs});
    this.state = {state:'idle'}; this.generation = 0;
  }
  publish(value) { this.state = value; this.emit('status',value); return value; }
  async discoverNative(request) {
    if(this.pending || this.child || this.stopping) throw Error('session_active');
    if(!request || Object.keys(request).length!==1 || typeof request.workspace!=='string' || !request.workspace) throw Error('invalid_session');
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
  waitNativeState(expected,generation) {
    return new Promise((resolve,reject)=>{
      const check=()=>{
        if(generation!==this.generation || this.state.state==='stopped')return finish(Error('computer_stopped'));
        if(this.state.state==='error')return finish(Error(this.state.code || 'computer_worker_unavailable'));
        if(this.state.state===expected)return finish();
      };
      const finish=error=>{clearTimeout(timer);this.removeListener('status',check);error?reject(error):resolve(this.state);};
      const timer=setTimeout(()=>{void this.stop();finish(Error('computer_worker_timeout'));},60000);
      this.on('status',check);check();
    });
  }
  async startNative(request) {
    if(this.pending || this.stopping || !this.child || !this.nativeContext || this.state.state!=='selecting') throw Error('computer_session_unavailable');
    request={...request,approvalMode:request?.approvalMode??'scoped_changes'};
    if(!request || Object.keys(request).length!==3 || request.workspace!==this.nativeContext.workspace || typeof request.target!=='string' || !this.state.targets.some(t=>t.id===request.target) || !APPROVAL_MODES.has(request.approvalMode)) throw Error('computer_stale_target');
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
    if (!request || Object.keys(request).some(k=>!['origin','workspace','networkMode','upload','approvalMode'].includes(k)) || typeof request.workspace !== 'string' || !request.workspace || !APPROVAL_MODES.has(request.approvalMode)) throw Error('invalid_session');
    if (request.upload && (Object.keys(request.upload).sort().join(',')!=='id,name,path,sha256,size' || typeof request.upload.path!=='string' || !path.isAbsolute(request.upload.path) || typeof request.upload.name!=='string' || request.upload.name.length>255 || !Number.isSafeInteger(request.upload.size) || request.upload.size<0 || request.upload.size>512*1024*1024 || typeof request.upload.id!=='string' || !/^[a-f0-9]{32}$/.test(request.upload.id) || typeof request.upload.sha256!=='string' || !/^[a-f0-9]{64}$/.test(request.upload.sha256))) throw Error('computer_upload_invalid');
    const origin = validateTarget(request.origin);
    const networkMode = request.networkMode ?? 'direct';
    if (!['direct','trusted-vpn'].includes(networkMode) || (origin === 'demo' && networkMode !== 'direct')) throw Error('network_mode_invalid');
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
  stop() {
    if (this.stopping) return this.stopping;
    this.stopping = this.stopOwned().finally(() => { this.stopping = null; });
    return this.stopping;
  }
  async stopOwned() {
    ++this.generation;
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
module.exports = {ComputerRuntime, validateTarget, isComputerLink};
