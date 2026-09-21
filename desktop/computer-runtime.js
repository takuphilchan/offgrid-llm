const {EventEmitter} = require('node:events');
const path = require('node:path');

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
  async start(request) {
    if (this.pending || this.child || this.stopping) throw Error('session_active');
    if (!request || Object.keys(request).some(k=>!['origin','workspace','networkMode'].includes(k)) || typeof request.workspace !== 'string' || !request.workspace) throw Error('invalid_session');
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
      if (!await this.confirm(origin, service, networkMode)) return this.publish({state:'idle',code:'consent_declined'});
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
          if (message?.state === 'booted') child.postMessage({type:'start',service,origin,networkMode,code,workspace:request.workspace,directory:this.directory});
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
