// Native transport adapter for the existing computer task queue. No UI or
// model receives a worker handle, credential, selector, or raw execution API.
import {mkdir, chmod} from 'node:fs/promises';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {localService} from './session.mjs';
import {DispatchJournal} from './journal.mjs';
import {policyAllows, validActionClass, validApprovalMode} from './approval-policy.mjs';

const opaque = value => typeof value === 'string' && /^[a-zA-Z0-9:_-]{1,256}$/.test(value);
const canonical = value => value === null || typeof value !== 'object' ? JSON.stringify(value) : Array.isArray(value) ? `[${value.map(canonical).join(',')}]` : `{${Object.keys(value).sort().map(k=>`${JSON.stringify(k)}:${canonical(value[k])}`).join(',')}}`;
const same = (a,b) => canonical(a) === canonical(b);
const exact = (value, fields) => value && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === fields.length && fields.every(k=>Object.hasOwn(value,k));

export class NativeSession {
  constructor({service, directory, worker, emit = () => {}, fetchRequest = fetch}) {
    Object.assign(this, {service:localService(service),directory,worker,emit,fetchRequest});
    this.abort = new AbortController(); this.token = ''; this.stopping = false;
  }
  async request(path, body) {
    const response = await this.fetchRequest(new URL(path,this.service), {method:body===undefined?'GET':'POST',redirect:'error',
      headers:{'Content-Type':'application/json',...(this.token?{Authorization:`Bearer ${this.token}`}:{})},
      ...(body===undefined?{}:{body:JSON.stringify(body)}), signal:AbortSignal.any([this.abort.signal,AbortSignal.timeout(20000)])});
    if(!response.ok) throw Error('computer_connection_lost');
    return response.json();
  }
  companion(path,body={}) { return this.request(`/api/v2/computer/companion/${path}`,body); }
  checkActive() { if(this.stopping) throw Error('computer_stopped'); }
  async start({code,workspace,target,approvalMode='scoped_changes'}) {
    if(this.started || this.stopping) throw Error('computer_session_active');
    this.started=true;
    try {
      if(!/^[a-f0-9]{64}$/.test(code) || typeof workspace!=='string' || !workspace || !target?.identity || typeof target.title!=='string' || !validApprovalMode(approvalMode)) throw Error('computer_invalid_session');
      this.approvalMode=approvalMode;
      const identity=await this.request('/api/v2/system');
      this.checkActive();
      if(identity.product!=='offgrid' || identity.api_version!==2 || identity.workspace_id!==workspace) throw Error('computer_workspace_changed');
      if(!identity.capabilities?.includes('native-computer-sessions-v2'))throw Error('computer_upgrade_required');
      // Select only a target issued by this live worker, not renderer metadata.
      const targets=this.worker.targets ?? await this.worker.request('targets');
      this.checkActive();
      const selected=targets.find(item=>same(item.identity,target.identity));
      if(!selected || !['windows-uia','macos-accessibility','linux-atspi'].includes(selected.identity.driver)) throw Error('computer_stale_target');
      this.target=structuredClone(selected);
      await mkdir(this.directory,{recursive:true,mode:0o700});
      this.checkActive();
      const journalPath=join(this.directory,'native-transport.sqlite');
      this.journal=new DispatchJournal(journalPath);
      await chmod(journalPath,0o600);
      this.checkActive();
      const paired=await this.companion('pair',{code,protocol_version:2,target:this.target.identity,title:this.target.title,approval_mode:approvalMode});
      this.checkActive();
      if(!opaque(paired.session?.id) || !same(paired.session.target,this.target.identity) || paired.session.driver!==this.target.identity.driver || !/^[a-f0-9]{64}$/.test(paired.token)) throw Error('computer_protocol_invalid');
      this.session=paired.session.id; this.token=paired.token;
      this.scope=canonical([this.service.origin,workspace,this.session,this.target.identity]);
      this.journal.discardPreviousSessionResults(this.scope);
      this.worker.once('stopped',()=>{ void this.stop().catch(()=>{}); });
      this.emit({state:'ready',target:{id:this.session,origin:this.target.title,driver:this.target.identity.driver}});
      // The queue must remain live while the user considers native OS consent.
      this.heartbeat=setInterval(()=>{ void this.companion('heartbeat').catch(()=>this.stop()).catch(()=>{}); },5000);
      this.running=this.loop();
      return {id:this.session,origin:this.target.title,driver:this.target.identity.driver};
    } catch(error) { await this.stop(); throw error; }
  }
  async execute(action) {
    this.checkActive();
    const binding=action.binding;
    if(!exact(binding,['actor','task','companion','session','target']) || !opaque(binding.actor) || !opaque(binding.task) || binding.session!==this.session || binding.companion!==this.session || !same(binding.target,this.target.identity)) throw Error('computer_scope_violation');
    if(this.binding && !same(binding,this.binding)) throw Error('computer_scope_violation');
    if(action.kind==='computer_prepare') {
      if(!this.binding || !exact(action.arguments,['tool','arguments']) || !['computer_replace_text','computer_activate','computer_shortcut','computer_set_checked'].includes(action.arguments.tool) || !action.id.endsWith(':prepare'))throw Error('computer_invalid_action');
      const {tool,arguments:args}=action.arguments;
      const fields=tool==='computer_replace_text'?['observation_id','element','text']:tool==='computer_activate'?['observation_id','element']:tool==='computer_shortcut'?['observation_id','element','shortcut']:['observation_id','element','checked'];
      if(!exact(args,fields) || !this.view || args.observation_id!==this.view.observation.id || !this.view.elements.some(e=>e.id===args.element))throw Error('computer_stale_observation');
      if(tool==='computer_replace_text' && (typeof args.text!=='string'||Buffer.byteLength(args.text)>16000||args.text.includes('\0')))throw Error('computer_invalid_action');
      if(tool==='computer_shortcut' && (!['copy','paste','undo','redo','select_all','save','enter','escape','tab','reverse_tab','left','right','up','down','page_up','page_down','home','end'].includes(args.shortcut)||(args.shortcut==='paste'&&!this.view.elements.find(e=>e.id===args.element)?.writable)))throw Error('computer_invalid_action');
      if(tool==='computer_set_checked' && (typeof args.checked!=='boolean'||!this.view.elements.find(e=>e.id===args.element)?.checkable))throw Error('computer_invalid_action');
      const operation=tool==='computer_shortcut'?{kind:'shortcut',element:args.element,shortcut:args.shortcut}:tool==='computer_set_checked'?{kind:'set_checked',element:args.element,checked:args.checked}:{kind:tool==='computer_replace_text'?'replace_text':'activate',element:args.element,...(tool==='computer_replace_text'?{text:args.text}:{})};
      const prepared=await this.worker.request('prepare',{operation,observation:this.view.observation});this.checkActive();
      if(!same(prepared.operation,operation))throw Error('computer_protocol_invalid');
      this.preparation={call:action.id.slice(0,-8),tool,args:structuredClone(args),prepared};
      if(!validActionClass(prepared.approval_class))throw Error('computer_protocol_invalid');
      return {prepared:true,approval_class:prepared.approval_class};
    }
    const args=action.arguments;
    const fields={computer_observe:[],computer_replace_text:['observation_id','element','text'],computer_verify:['observation_id','element','text'],computer_activate:['observation_id','element'],computer_shortcut:['observation_id','element','shortcut'],computer_set_checked:['observation_id','element','checked'],computer_verify_checked:['observation_id','element','checked']}[action.kind];
    if(!fields || !exact(args,fields)) throw Error('computer_invalid_action');
    if(action.kind==='computer_shortcut' && !['copy','paste','undo','redo','select_all','save','enter','escape','tab','reverse_tab','left','right','up','down','page_up','page_down','home','end'].includes(args.shortcut)) throw Error('computer_invalid_action');
    if(action.kind!=='computer_observe' && (!this.view || args.observation_id!==this.view.observation.id || !this.view.elements.some(e=>e.id===args.element))) throw Error('computer_stale_observation');
    if(action.kind==='computer_shortcut' && args.shortcut==='paste' && !this.view.elements.find(e=>e.id===args.element)?.writable)throw Error('computer_invalid_action');
    if(['computer_set_checked','computer_verify_checked'].includes(action.kind) && (typeof args.checked!=='boolean'||!this.view.elements.find(e=>e.id===args.element)?.checkable))throw Error('computer_invalid_action');
    if(['computer_replace_text','computer_verify'].includes(action.kind) && (typeof args.text!=='string' || Buffer.byteLength(args.text)>16000 || args.text.includes('\0'))) throw Error('computer_invalid_action');
    if(!this.binding) {
      if(action.kind!=='computer_observe') throw Error('computer_observation_required');
      // The Go worker owns the local consent dialog. A paired token or service
      // approval cannot replace this independent check.
      const selected=await this.worker.request('select',{target:this.target.identity.id,binding,approval_mode:this.approvalMode});
      this.checkActive();
      if(!same(selected.identity,this.target.identity)) throw Error('computer_stale_target');
      this.binding=structuredClone(binding);
    }
    if(this.stopping) throw Error('computer_stopped');
    if(action.kind==='computer_observe') {
      this.preparation=null;
      this.view=null;
      const view=await this.worker.request('observe');
      this.checkActive();
      if(!opaque(view?.observation?.id) || !same(view.observation.target,this.target.identity) || !Array.isArray(view.elements)) throw Error('computer_protocol_invalid');
      this.view=view; return view;
    }
    const observation=this.view.observation;
    if(action.kind==='computer_verify')return this.worker.request('verify',{operation:{kind:'replace_text',element:args.element,text:args.text},observation});
    if(action.kind==='computer_verify_checked')return this.worker.request('verify',{operation:{kind:'set_checked',element:args.element,checked:args.checked},observation});
    this.view=null; // Every proposed mutation requires a fresh observation.
    const operation=action.kind==='computer_shortcut'?{kind:'shortcut',element:args.element,shortcut:args.shortcut}:action.kind==='computer_set_checked'?{kind:'set_checked',element:args.element,checked:args.checked}:{kind:action.kind==='computer_replace_text'?'replace_text':'activate',element:args.element,...(action.kind==='computer_replace_text'?{text:args.text}:{})};
    const cached=this.preparation;
    if(cached && (cached.call!==action.id || cached.tool!==action.kind || !same(cached.args,args)))throw Error('computer_approval_invalid');
    const prepared=cached?.prepared ?? await this.worker.request('prepare',{operation,observation});
    this.preparation=null;
    this.checkActive();
    // No model-generated step, binding, approval ID or native operation enters
    // this boundary. Only the worker's locally prepared exact operation does.
    if(!same(prepared.operation,operation)) throw Error('computer_protocol_invalid');
    if(!validActionClass(prepared.approval_class) || prepared.approval_class==='forbidden')throw Error('computer_prohibited_action');
    if(action.authorization==='auto'&&!policyAllows(this.approvalMode,prepared.approval_class) || !['auto','exact'].includes(action.authorization))throw Error('computer_approval_invalid');
    const step={id:createHash('sha256').update(action.id).digest('hex'),binding:this.binding,actions:[prepared]};
    const grant=await this.worker.request('approve',{step,automatic:action.authorization==='auto'});
    if(this.stopping) throw Error('computer_stopped');
    const result=await this.worker.request('execute',{step,grant:grant.id});
    return {actions:result};
  }
  async loop() {
    try {
      while(!this.stopping) {
        const {action}=await this.companion('poll');
        if(this.stopping) break;
        if(!action) { await new Promise(resolve=>setTimeout(resolve,300)); continue; }
        // Include immutable actor/task/target binding in duplicate protection.
        const dispatch=this.journal.prepare(action,canonical([this.scope,action.binding]));
        if(!dispatch.execute) { await this.companion('reply',dispatch.reply); continue; }
        let result;
        try { result=await this.execute(action); }
        catch(error) {
          if(!this.stopping) {
            // Record and acknowledge failure before closing, so the runner does
            // not wait for a heartbeat timeout or repeat an uncertain input.
            const code=/^computer_[a-z_]+$/.test(error.message)?error.message:'computer_uncertain_outcome';
            const reply={id:action.id,result:'',error:code};
            this.journal.complete(action.id,reply);
            await this.companion('reply',reply);
          }
          throw error;
        }
        if(this.stopping) break;
        const reply={id:action.id,result:JSON.stringify(result)};
        this.journal.complete(action.id,reply);
        await this.companion('reply',reply);
      }
    } catch(error) {
      if(!this.stopping) { this.failed=true; this.emit({state:'error',code:/^computer_[a-z_]+$/.test(error.message)?error.message:'computer_uncertain_outcome'}); }
    } finally { await this.stop().catch(()=>{}); }
  }
  stop() {
    if(this.closing) return this.closing;
    this.stopping=true; this.abort.abort(); clearInterval(this.heartbeat);
    this.closing=(async()=>{
      await this.worker.stop();
      // Keep tombstones; erase observation results only after dispatch stops.
      if(this.journal) { this.journal.db.exec('UPDATE dispatch SET reply=NULL'); this.journal.close(); this.journal=null; }
      this.token='';this.view=null;this.preparation=null;if(!this.failed)this.emit({state:'stopped'});
    })().catch(error=>{
      this.failed=true;this.closing=null;
      this.emit({state:'error',code:'computer_stop_unconfirmed'});
      throw error;
    });
    return this.closing;
  }
}
