// Private host integration, not a renderer API or network listener. A worker
// grants authority only through its own local OS consent and journal.
const {spawn} = require('node:child_process');
const path = require('node:path');
const {randomUUID} = require('node:crypto');
const {verifyNativePack} = require('./pack.cjs');
const {EventEmitter} = require('node:events');
const MAX_FRAME = 1024 * 1024;

function frame(value) {
  const body = Buffer.from(JSON.stringify(value));
  if (!body.length || body.length > MAX_FRAME) throw Error('computer_invalid_action');
  const header = Buffer.alloc(4); header.writeUInt32BE(body.length);
  return Buffer.concat([header, body]);
}

class NativeWorker extends EventEmitter {
  constructor({root, directory, platform = process.platform, arch = process.arch, verify = verifyNativePack, spawnWorker = spawn, timeoutMs = 30000, approvalMs = 300000, stopMs = 3000}) {
    super(); Object.assign(this, {root, directory, platform, arch, verify, spawnWorker, timeoutMs, approvalMs, stopMs});
    this.generation = 0; this.buffer = Buffer.alloc(0); this.revoked = true;
  }
  async start() {
    if (this.child || this.starting || this.stopping) throw Error('computer_input_in_use');
    this.starting = true; const generation = ++this.generation;
    try {
      if (!{win32:['x64'],darwin:['x64','arm64'],linux:['x64']}[this.platform]?.includes(this.arch)) throw Error('computer_driver_unavailable');
      await this.verify(this.root, this.platform, this.arch);
      if (generation !== this.generation) throw Error('computer_stopped');
      const env = {...process.env};
      delete env.NODE_OPTIONS; delete env.NODE_PATH; delete env.ELECTRON_RUN_AS_NODE;
      const executable = this.platform === 'win32' ? 'offgrid-computer.exe' : 'offgrid-computer';
      const child = this.spawnWorker(path.join(this.root, 'native', executable), ['--state', path.resolve(this.directory)], {windowsHide:true, shell:false, stdio:['pipe','pipe','pipe'], env});
      this.child = child;
      this.revoked = false;
      this.booted = false;
      // Never forward provider stderr: it is diagnostics, not an instruction or
      // a reason to include application content in the service's event log.
      child.stderr?.resume();
      child.stdin.on('error', () => this.fail('computer_worker_unavailable'));
      child.stdout.on('data', data => this.receive(child, data));
      child.on('error', () => this.fail('computer_worker_unavailable'));
      child.once('exit', () => {
        if (this.child !== child) return;
        this.child = null; this.buffer = Buffer.alloc(0);
        this.rejectStartup('computer_worker_stopped');
        this.rejectPending(this.pending?.kind === 'execute' ? 'computer_uncertain_outcome' : 'computer_worker_stopped');
        this.emit('stopped');
      });
      await new Promise((resolve,reject)=>{
        const timer=setTimeout(()=>this.fail('computer_worker_timeout'),10000);
        this.startup={resolve,reject,timer};
      });
      return this;
    } finally { this.starting = false; }
  }
  request(kind, args = {}) {
    if (!this.child || this.stopping || this.revoked) return Promise.reject(Error('computer_worker_stopped'));
    if (!this.booted) return Promise.reject(Error('computer_worker_starting'));
    if (this.pending) return Promise.reject(Error('computer_busy'));
    const fields = {targets:[],select:['target','binding','approval_mode'],observe:[],prepare:['operation','observation'],verify:['operation','observation'],approve:['step','automatic'],execute:['step','grant']}[kind];
    if (!fields || !args || Object.keys(args).length !== fields.length || fields.some(field => !Object.hasOwn(args,field))) return Promise.reject(Error('computer_invalid_action'));
    const id = randomUUID();
    let encoded;
    try { encoded = frame({protocol:2,id,kind,...args}); } catch { return Promise.reject(Error('computer_invalid_action')); }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { this.fail(kind === 'execute' ? 'computer_uncertain_outcome' : 'computer_worker_timeout'); }, ['select','approve'].includes(kind) ? this.approvalMs : this.timeoutMs);
      this.pending = {id,kind,resolve,reject,timer};
      try { this.child.stdin.write(encoded); } catch { this.fail('computer_worker_unavailable'); }
    });
  }
  receive(child, data) {
    if (child !== this.child || this.stopping) return;
    if (data.length + this.buffer.length > MAX_FRAME + 4) { this.fail('computer_protocol_invalid'); return; }
    this.buffer = Buffer.concat([this.buffer,data]);
    if (this.buffer.length < 4) return;
    const length = this.buffer.readUInt32BE();
    if (!length || length > MAX_FRAME) { this.fail('computer_protocol_invalid'); return; }
    if (this.buffer.length < 4 + length) return;
    // One request is in flight; extra responses or unsolicited events cannot
    // be silently treated as the next approved call's acknowledgement.
    if (this.buffer.length !== 4 + length) { this.fail('computer_protocol_invalid'); return; }
    let value;
    try { value = JSON.parse(this.buffer.subarray(4).toString('utf8')); } catch { this.fail('computer_protocol_invalid'); return; }
    this.buffer = Buffer.alloc(0);
    if (!this.booted) {
      if (!value || value.protocol!==2 || value.id!=='startup' || !this.startup || Object.keys(value).some(k=>!['protocol','id','result','error'].includes(k))) {this.fail('computer_protocol_invalid');return;}
      if(value.error){this.fail(/^computer_[a-z_]+$/.test(value.error)?value.error:'computer_provider_unavailable');return;}
      if(value.result?.ready!==true){this.fail('computer_protocol_invalid');return;}
      const startup=this.startup;this.startup=null;clearTimeout(startup.timer);this.booted=true;startup.resolve();return;
    }
    if (!value || value.protocol !== 2 || value.id !== this.pending?.id || Object.keys(value).some(k=>!['protocol','id','result','error'].includes(k))) { this.fail('computer_protocol_invalid'); return; }
    const pending = this.pending; this.pending = null; clearTimeout(pending.timer);
    if (value.error) {
      const code = typeof value.error === 'string' && /^computer_[a-z_]+$/.test(value.error) ? value.error : 'computer_provider_unavailable';
      pending.reject(Object.assign(Error(code),{result:value.result}));
    } else {
      if (pending.kind === 'targets') this.targets = structuredClone(value.result);
      pending.resolve(value.result);
    }
  }
  rejectPending(code) {
    if (!this.pending) return;
    const pending=this.pending; this.pending=null; clearTimeout(pending.timer);
    pending.reject(Error(pending.kind==='execute' ? 'computer_uncertain_outcome' : code));
  }
  rejectStartup(code){if(!this.startup)return;const startup=this.startup;this.startup=null;clearTimeout(startup.timer);startup.reject(Error(code));}
  fail(code) { this.rejectStartup(code); this.rejectPending(code); void this.stop().catch(()=>{}); }
  stop() {
    this.revoked = true;
    this.rejectStartup('computer_stopped');
    if (this.stopping) return this.stopping;
    ++this.generation;
    this.stopping=this.stopOwned().finally(()=>{this.stopping=null;});
    return this.stopping;
  }
  async stopOwned() {
    const child=this.child;
    this.rejectPending('computer_stopped');
    if (!child) return;
    await new Promise((resolve,reject)=>{
      let timeout,settled=false;
      const finish=err=>{if(settled)return;settled=true;clearTimeout(timeout);child.removeListener('exit',exited);err?reject(Error(err)):resolve();};
      const exited=()=>finish();
      child.once('exit',exited);
      const force=()=>{
        timeout=setTimeout(()=>finish('computer_stop_unconfirmed'),this.stopMs);
        try {if(child.kill()===false)finish('computer_stop_unconfirmed');} catch {finish('computer_stop_unconfirmed');}
      };
      timeout=setTimeout(force,this.stopMs);
      try {child.stdin.write(frame({protocol:2,id:randomUUID(),kind:'stop'}));} catch {clearTimeout(timeout);force();}
    });
    // Do not clear ownership on a timer or on a response. Only process exit
    // proves this worker cannot dispatch another provider call.
  }
}
module.exports={NativeWorker,frame};
