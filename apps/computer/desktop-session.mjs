// Protocol-v3 adapter. Reuses the transport journal, queue, cancellation and
// private token lifecycle; it does not create another agent or task history.
import {mkdir,chmod} from 'node:fs/promises';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {NativeSession} from './native-session.mjs';
import {DispatchJournal} from './journal.mjs';
import {policyAllows,validActionClass,validApprovalMode} from './approval-policy.mjs';
import featureContract from './native-features.cjs';
import {documentAction} from './document-session.mjs';
const {requestedNativeFeatures,verifyNativeFeatures}=featureContract;

const canonical=v=>v===null||typeof v!=='object'?JSON.stringify(v):Array.isArray(v)?`[${v.map(canonical).join(',')}]`:`{${Object.keys(v).sort().map(k=>`${JSON.stringify(k)}:${canonical(v[k])}`).join(',')}}`;
const same=(a,b)=>canonical(a)===canonical(b);
const exact=(v,keys)=>v && typeof v==='object' && !Array.isArray(v) && Object.keys(v).length===keys.length && keys.every(k=>Object.hasOwn(v,k));
const opaque=v=>typeof v==='string' && /^[a-zA-Z0-9:_-]{1,256}$/.test(v);
const shortcuts=['copy','paste','undo','redo','select_all','save','enter','escape','tab','reverse_tab','left','right','up','down','page_up','page_down','home','end'];

export class DesktopSession extends NativeSession {
  async start({code,workspace,run,input,prompt,approvalMode}) {
    if(this.started||this.stopping)throw Error('computer_session_active');
    this.started=true;
    try {
      if(!/^[a-f0-9]{64}$/.test(code)||!opaque(workspace)||!opaque(run)||!opaque(input)||typeof prompt!=='string'||!prompt||prompt.length>32000||!validApprovalMode(approvalMode)||this.worker.protocol!==3)throw Error('computer_invalid_session');
      const identity=await this.request('/api/v2/system');this.checkActive();
      if(identity.product!=='offgrid'||identity.workspace_id!==workspace||identity.api_version!==2)throw Error('computer_workspace_changed');
      if(!identity.capabilities?.includes('task-desktop-sessions-v3'))throw Error('computer_upgrade_required');
      const target=await this.worker.request('desktop_identity');this.checkActive();
      if(!target||target.surface!=='desktop'||target.process_generation!==target.id||!opaque(target.id)||!opaque(target.os_session))throw Error('computer_protocol_invalid');
      await mkdir(this.directory,{recursive:true,mode:0o700});this.checkActive();
      const file=join(this.directory,'native-transport.sqlite');this.journal=new DispatchJournal(file);await chmod(file,0o600);this.checkActive();
      const features=requestedNativeFeatures(identity,this.worker);
      const paired=await this.companion('pair',{code,protocol_version:3,target,approval_mode:approvalMode,...(Object.keys(features).length?{native_features:features}:{})});this.checkActive();
      const session=paired.session,authority=session?.desktop,binding=authority?.binding;
      if(!opaque(session?.id)||session.run_id!==run||session.input_id!==input||session.approval_mode!==approvalMode||session.state!=='awaiting_consent'||authority?.workspace!==workspace||!exact(binding,['actor','task','companion','session','target'])||!opaque(binding.actor)||binding.task!==run||binding.session!==session.id||binding.companion!==session.id||!same(binding.target,target)||!same(session.target,target)||!/^[a-f0-9]{64}$/.test(paired.token))throw Error('computer_protocol_invalid');
      this.session=session.id;this.token=paired.token;this.binding=structuredClone(binding);this.approvalMode=approvalMode;
      this.features=verifyNativeFeatures(features,session.native_features);
      this.scope=canonical([this.service.origin,workspace,run,input,authority]);this.journal.discardPreviousSessionResults(this.scope);
      this.workerStopped=()=>{void this.stop().catch(()=>{});};this.worker.once('stopped',this.workerStopped);
      this.heartbeat=setInterval(()=>{void this.companion('heartbeat').catch(()=>this.stop()).catch(()=>{});},5000);
      if(!this.releaseWorker)this.emit({state:'consent',target:{id:this.session,origin:'Task desktop',scope:'desktop'}});
      const granted=await this.worker.request('grant_desktop',{desktop:authority,approval_mode:approvalMode,prompt,...(this.releaseWorker?{service:this.service.origin}:{})});this.checkActive();
      if(granted?.granted!==true||!same(granted.binding,binding))throw Error('computer_protocol_invalid');
      await this.companion('activate');this.checkActive();
      const result={id:this.session,origin:'Task desktop',driver:target.driver,scope:'desktop'};
      this.emit({state:'ready',target:result});this.running=this.loop();return result;
    }catch(error){await this.stop();throw error;}
  }
  operation(tool,args) {
    const fields={computer_open_application:['application_id'],computer_focus_window:['target_id'],computer_replace_text:['observation_id','element','text'],computer_activate:['observation_id','element'],computer_shortcut:['observation_id','element','shortcut'],computer_set_checked:['observation_id','element','checked']}[tool];
    if(!fields||!exact(args,fields))throw Error('computer_invalid_action');
    if(tool==='computer_open_application'){if(!opaque(args.application_id))throw Error('computer_invalid_action');return {kind:'open_application',application:args.application_id};}
    if(tool==='computer_focus_window'){if(!opaque(args.target_id))throw Error('computer_invalid_action');return {kind:'focus_surface',target:args.target_id};}
    if(!this.view||args.observation_id!==this.view.observation.id)throw Error('computer_stale_observation');
    const element=this.view.elements.find(e=>e.id===args.element);if(!element)throw Error('computer_stale_observation');
    if(tool==='computer_replace_text'){
      if(!element.writable||typeof args.text!=='string'||Buffer.byteLength(args.text)>16000||args.text.includes('\0'))throw Error('computer_invalid_action');
      return {kind:'replace_text',element:args.element,text:args.text};
    }
    if(tool==='computer_set_checked'){
      if(!element.checkable||typeof args.checked!=='boolean')throw Error('computer_invalid_action');
      return {kind:'set_checked',element:args.element,checked:args.checked};
    }
    if(tool==='computer_shortcut'){
      if(!shortcuts.includes(args.shortcut)||(args.shortcut==='paste'&&!element.writable))throw Error('computer_invalid_action');
      return {kind:'shortcut',element:args.element,shortcut:args.shortcut};
    }
    if(!element.invokable)throw Error('computer_invalid_action');
    return {kind:'activate',element:args.element};
  }
  async prepare(tool,args){
    const operation=this.operation(tool,args);
    const lifecycle=['open_application','focus_surface'].includes(operation.kind);
    const prepared=await this.worker.request(lifecycle?'prepare_lifecycle':'prepare',lifecycle?{operation}:{operation,observation:this.view.observation});this.checkActive();
    const b=prepared?.binding,a=prepared?.action,base=this.binding;
    if(!exact(b,['actor','task','companion','session','target'])||b.actor!==base.actor||b.task!==base.task||b.companion!==base.companion||b.session!==base.session||b.target?.os_session!==base.target.os_session||b.target?.driver!==base.target.driver||!same(a?.operation,operation)||!validActionClass(a?.approval_class))throw Error('computer_protocol_invalid');
    return prepared;
  }
  async execute(action){
    this.checkActive();if(!same(action.binding,this.binding))throw Error('computer_scope_violation');
    if(documentAction(action))return this.documents.execute(action);
    this.documents.clear();
    const args=action.arguments;
    if(action.kind==='computer_prepare'){
      if(!action.id.endsWith(':prepare')||!exact(args,['tool','arguments']))throw Error('computer_invalid_action');
      const prepared=await this.prepare(args.tool,args.arguments);
      this.preparation={call:action.id.slice(0,-8),tool:args.tool,args:structuredClone(args.arguments),prepared};
      return {prepared:true,approval_class:prepared.action.approval_class};
    }
    if(['computer_applications','computer_windows','computer_observe'].includes(action.kind)){
      if(!exact(args,[]))throw Error('computer_invalid_action');
      this.preparation=null;
      if(action.kind==='computer_applications')return {applications:await this.worker.request('applications')};
      this.view=null;
      if(action.kind==='computer_windows')return {surfaces:await this.worker.request('surfaces')};
      const view=await this.worker.request('observe');this.checkActive();this.validateView(view);this.view=view;return view;
    }
    if(action.kind==='computer_verify_application'){
      if(!exact(args,['application_id'])||!opaque(args.application_id))throw Error('computer_invalid_action');
      this.preparation=null;this.view=null;
      const result=await this.worker.request('verify_application',{application:args.application_id});this.checkActive();
      if(result.application_id!==args.application_id||result.check!=='application_open'||typeof result.verified!=='boolean')throw Error('computer_protocol_invalid');
      if(result.readiness!==undefined){
        if(result.verified?result.readiness!=='ready':!['starting','surface_unavailable','unsupported_identity','observation_unavailable'].includes(result.readiness)||result.target!==undefined||result.view!==undefined)throw Error('computer_protocol_invalid');
      }
      if(result.verified){this.validateView(result.view);if(!same(result.target,result.view.observation.target))throw Error('computer_protocol_invalid');this.view=result.view;}
      return result;
    }
    if(['computer_verify','computer_verify_checked'].includes(action.kind)){
      const checked=action.kind==='computer_verify_checked';
      if(!exact(args,['observation_id','element',checked?'checked':'text'])||!this.view||args.observation_id!==this.view.observation.id||!this.view.elements.some(e=>e.id===args.element))throw Error('computer_stale_observation');
      if(checked?typeof args.checked!=='boolean':typeof args.text!=='string'||Buffer.byteLength(args.text)>16000||args.text.includes('\0'))throw Error('computer_invalid_action');
      return this.worker.request('verify',{operation:{kind:checked?'set_checked':'replace_text',element:args.element,...(checked?{checked:args.checked}:{text:args.text})},observation:this.view.observation});
    }
    const cached=this.preparation;
    if(cached&&(cached.call!==action.id||cached.tool!==action.kind||!same(cached.args,args)))throw Error('computer_approval_invalid');
    const prepared=cached?.prepared??await this.prepare(action.kind,args);
    this.preparation=null;this.view=null;this.checkActive();
    if(prepared.action.approval_class==='forbidden')throw Error('computer_prohibited_action');
    if(!['auto','exact'].includes(action.authorization)||(action.authorization==='auto'&&!policyAllows(this.approvalMode,prepared.action.approval_class)))throw Error('computer_approval_invalid');
    const step={id:createHash('sha256').update(action.id).digest('hex'),binding:prepared.binding,actions:[prepared.action]};
    const grant=await this.worker.request('approve',{step,automatic:action.authorization==='auto'});this.checkActive();
    return {actions:await this.worker.request('execute',{step,grant:grant.id})};
  }
  validateView(view){
    if(!opaque(view?.observation?.id)||view.observation.target?.os_session!==this.binding.target.os_session||view.observation.target?.driver!==this.binding.target.driver||!Array.isArray(view.elements))throw Error('computer_protocol_invalid');
  }
}
