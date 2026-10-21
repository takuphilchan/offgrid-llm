// Negotiated document transport for both native session scopes. This composes
// with the existing worker/supervisor/journals; it never creates authority.
import {createHash} from 'node:crypto';
import {policyAllows,validActionClass} from './approval-policy.mjs';

const opaque=v=>typeof v==='string'&&/^[a-zA-Z0-9:_-]{1,256}$/.test(v);
const canonical=v=>v===null||typeof v!=='object'?JSON.stringify(v):Array.isArray(v)?`[${v.map(canonical).join(',')}]`:`{${Object.keys(v).sort().map(k=>`${JSON.stringify(k)}:${canonical(v[k])}`).join(',')}}`;
const same=(a,b)=>canonical(a)===canonical(b);
const exact=(v,keys)=>v&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===keys.length&&keys.every(k=>Object.hasOwn(v,k));
const digest=v=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const text=v=>typeof v==='string'&&v.isWellFormed()&&!v.includes('\0')&&Buffer.byteLength(v)<=16000;
const names=new Set(['computer_documents','computer_read_text','computer_create_document','computer_select_document','computer_select_text','computer_insert_text','computer_replace_selection']);

export function documentTool(name,args) {
  return names.has(name)||(name==='computer_replace_text'&&args&&Object.hasOwn(args,'document'));
}
export function documentAction(action) {
  return action.kind==='computer_prepare'?documentTool(action.arguments?.tool,action.arguments?.arguments):documentTool(action.kind,action.arguments);
}

function boundTarget(target,base,desktop) {
  if(!exact(target,['id','os_session','process_generation','surface','driver'])||!Object.values(target).every(opaque)||target.surface==='desktop')return false;
  return desktop?target.os_session===base.target.os_session&&target.driver===base.target.driver:same(target,base.target);
}
function documentBinding(binding,target) {
  return exact(binding,['target','document','generation','container','editor','observation','revision'])&&same(binding.target,target)&&['document','generation','container','editor','observation','revision'].every(k=>opaque(binding[k]));
}
function sameDocument(a,b){return ['target','document','generation','container','editor'].every(k=>same(a[k],b[k]));}

export class DocumentSession {
  constructor(session){this.session=session;this.clear();}
  clear(){this.view=null;this.readback=null;this.preparation=null;}
  check(){this.session.checkActive();if(this.session.features?.native_documents!==1||!this.session.binding)throw Error('computer_document_unavailable');}
  scope(target){return boundTarget(target,this.session.binding,this.session.worker.protocol===3);}
  validateInventory(view){
    const obs=view?.observation;
    if(view?.version!==1||view.complete!==true||!opaque(obs?.id)||!this.scope(obs.target)||!Number.isSafeInteger(obs.sequence)||obs.sequence<1||!Number.isFinite(Date.parse(obs.captured_at))||!digest(obs.state_digest)||!Array.isArray(view.documents)||view.documents.length>256||!Array.isArray(view.commands)||view.commands.length>16)throw Error('computer_protocol_invalid');
    const ids=new Set(),containers=new Set();let selected;
    for(const doc of view.documents){
      if(!opaque(doc.id)||!opaque(doc.generation)||!opaque(doc.container)||ids.has(doc.id)||containers.has(doc.container)||typeof doc.title!=='string'||!doc.title.isWellFormed()||doc.title.includes('\0')||Buffer.byteLength(doc.title)>1024||typeof doc.selected!=='boolean'||!exact(doc.capabilities,['read','select','select_text','insert_text','replace_selection','replace_value'])||Object.values(doc.capabilities).some(v=>typeof v!=='boolean'))throw Error('computer_protocol_invalid');
      if(!doc.capabilities.read&&['select_text','insert_text','replace_selection','replace_value'].some(k=>doc.capabilities[k]))throw Error('computer_protocol_invalid');
      ids.add(doc.id);containers.add(doc.container);
      if(doc.selected){if(selected)throw Error('computer_protocol_invalid');selected=doc;}
    }
    if(selected){if(!documentBinding(view.active,obs.target)||view.active.observation!==obs.id||view.active.document!==selected.id||view.active.generation!==selected.generation||view.active.container!==selected.container)throw Error('computer_protocol_invalid');}
    else if(view.active!==undefined)throw Error('computer_protocol_invalid');
    const commands=new Set();
    for(const command of view.commands){if(!exact(command,['id','element','kind','profile','revision'])||!['new_document','new_tab'].includes(command.kind)||!['id','element','profile','revision'].every(k=>opaque(command[k]))||commands.has(command.id))throw Error('computer_protocol_invalid');commands.add(command.id);}
  }
  current(args){
    if(!this.view||args?.observation_id!==this.view.observation.id)throw Error('computer_stale_observation');
  }
  operation(tool,args){
    this.current(args);
    if(tool==='computer_create_document'){
      if(!exact(args,['observation_id','command'])||!this.view.commands.some(c=>c.id===args.command))throw Error('computer_invalid_action');
      return {kind:'document',document:{kind:'create_document',command:args.command}};
    }
    if(tool==='computer_select_document'){
      if(!exact(args,['observation_id','document'])||!this.view.documents.some(d=>d.id===args.document&&d.capabilities.select))throw Error('computer_invalid_action');
      return {kind:'document',document:{kind:'select_document',document:args.document}};
    }
    const mode={computer_select_text:'select_text',computer_insert_text:'insert_text',computer_replace_selection:'replace_selection',computer_replace_text:'replace_value'}[tool];
    const selection=mode==='select_text';
    if(!mode||!exact(args,['observation_id','document','snapshot',...(selection?['start','end']:['text'])]))throw Error('computer_invalid_action');
    const current=this.readback;
    const doc=this.view.documents.find(d=>d.selected&&d.id===args.document);
    if(!current||args.snapshot!==current.snapshot||current.binding.document!==args.document||!doc?.capabilities[mode])throw Error('computer_document_changed');
    if(selection?(!Number.isSafeInteger(args.start)||!Number.isSafeInteger(args.end)||args.start<0||args.end<args.start||args.end>65536):!text(args.text))throw Error('computer_invalid_action');
    return {kind:'document',document:{kind:mode,document:args.document,snapshot:args.snapshot,...(selection?{range:{start:args.start,end:args.end}}:{text:args.text})}};
  }
  async prepare(tool,args){
    const operation=this.operation(tool,args), session=this.session;
    const result=await session.worker.request('prepare',{operation,observation:this.view.observation});this.check();
    const prepared=session.worker.protocol===3?result:{action:result,binding:session.binding};
    const b=prepared?.binding,a=prepared?.action,base=session.binding;
    if(!exact(b,['actor','task','companion','session','target'])||['actor','task','companion','session'].some(k=>b[k]!==base[k])||!same(b.target,this.view.observation.target)||!same(a?.operation,operation)||!opaque(a?.id)||!validActionClass(a?.approval_class))throw Error('computer_protocol_invalid');
    return prepared;
  }
  validateRead(result,args){
    const b=result?.binding,p=result?.page;
    if(result?.version!==1||!opaque(result.snapshot)||!documentBinding(b,this.view.observation.target)||!same(b,this.view.active)||result.complete!==true||result.composition!=='stable'||!digest(result.text_sha256)||!Number.isSafeInteger(result.bytes)||result.bytes<0||result.bytes>65536)throw Error('computer_protocol_invalid');
    if(!p||p.snapshot!==result.snapshot||!same(p.binding,b)||p.source_complete!==true||typeof p.text!=='string'||!p.text.isWellFormed()||p.text.includes('\0')||Buffer.byteLength(p.text)>4096||!Number.isSafeInteger(p.start)||!Number.isSafeInteger(p.end)||!Number.isSafeInteger(p.total_scalars)||p.start<0||p.end<p.start||p.end>p.total_scalars||p.total_scalars>result.bytes||[...p.text].length!==p.end-p.start)throw Error('computer_protocol_invalid');
    if(p.end<p.total_scalars?(p.next!==p.end||p.end===p.start):p.next!==undefined)throw Error('computer_protocol_invalid');
    if(p.start!== (args.page?.start??0)||(args.page&&(result.snapshot!==args.page.snapshot||b.revision!==args.page.revision)))throw Error('computer_protocol_invalid');
    if(p.start===0&&p.next===undefined&&(Buffer.byteLength(p.text)!==result.bytes||createHash('sha256').update(p.text).digest('hex')!==result.text_sha256))throw Error('computer_protocol_invalid');
    if(result.selection!==undefined&&(!exact(result.selection,['start','end'])||!Number.isSafeInteger(result.selection.start)||!Number.isSafeInteger(result.selection.end)||result.selection.start<0||result.selection.end<result.selection.start||result.selection.end>p.total_scalars))throw Error('computer_protocol_invalid');
  }
  async execute(action){
    this.check();const session=this.session,args=action.arguments;
    session.preparation=null;session.view=null;
    if(action.kind==='computer_documents'){
      if(!exact(args,[]))throw Error('computer_invalid_action');
      this.clear();const result=await session.worker.request('documents');this.check();this.validateInventory(result);this.view=structuredClone(result);return result;
    }
    if(action.kind==='computer_read_text'){
      this.current(args);
      if(!exact(args,['observation_id','document',...(Object.hasOwn(args,'page')?['page']:[])])||args.document!==this.view.active?.document)throw Error('computer_invalid_action');
      if(args.page!==undefined&&(!exact(args.page,['snapshot','revision','start'])||!opaque(args.page.snapshot)||!opaque(args.page.revision)||!Number.isSafeInteger(args.page.start)||args.page.start<0||args.page.start>65536||args.page.snapshot!==this.readback?.snapshot||args.page.revision!==this.readback?.binding.revision))throw Error('computer_stale_observation');
      this.preparation=null;
      const result=await session.worker.request('read_document',{observation:this.view.observation,...(args.page?{page:args.page}:{})});this.check();this.validateRead(result,args);this.readback=structuredClone(result);return result;
    }
    if(action.kind==='computer_prepare'){
      if(!action.id.endsWith(':prepare')||!exact(args,['tool','arguments']))throw Error('computer_invalid_action');
      const prepared=await this.prepare(args.tool,args.arguments);
      this.preparation={call:action.id.slice(0,-8),tool:args.tool,args:structuredClone(args.arguments),prepared};
      return {prepared:true,approval_class:prepared.action.approval_class};
    }
    const cached=this.preparation;
    if(cached&&(cached.call!==action.id||cached.tool!==action.kind||!same(cached.args,args)))throw Error('computer_approval_invalid');
    const prepared=cached?.prepared??await this.prepare(action.kind,args);
    this.clear();this.check();
    if(prepared.action.approval_class==='forbidden')throw Error('computer_prohibited_action');
    if(!['auto','exact'].includes(action.authorization)||(action.authorization==='auto'&&!policyAllows(session.approvalMode,prepared.action.approval_class)))throw Error('computer_approval_invalid');
    const step={id:createHash('sha256').update(action.id).digest('hex'),binding:prepared.binding,actions:[prepared.action]};
    const grant=await session.worker.request('approve',{step,automatic:action.authorization==='auto'});this.check();
    const actions=await session.worker.request('execute',{step,grant:grant.id});this.check();
    if(!Array.isArray(actions)||actions.length!==1||actions[0].action_id!==prepared.action.id||actions[0].outcome!=='verified'||actions[0].uncertain)throw Error('computer_uncertain_outcome');
    const op=prepared.action.operation.document;
    if(!['select_document','select_text'].includes(op.kind)){
      const evidence=actions[0].document;
      if(evidence?.version!==1||evidence.action_id!==prepared.action.id||!documentBinding(evidence.after,prepared.binding.target)||!opaque(evidence.after_snapshot)||!digest(evidence.expected_sha256))throw Error('computer_uncertain_outcome');
      if(op.kind==='create_document'){
        if(!['new_document','new_tab'].includes(evidence.operation)||evidence.command!==op.command||evidence.expected_sha256!==createHash('sha256').update('').digest('hex')||['before','before_snapshot','preimage_sha256','input_sha256'].some(k=>evidence[k]!==undefined))throw Error('computer_uncertain_outcome');
      }else if(evidence.operation!==op.kind||evidence.command!==undefined||!documentBinding(evidence.before,prepared.binding.target)||!sameDocument(evidence.before,evidence.after)||evidence.before.observation===evidence.after.observation||evidence.before_snapshot===evidence.after_snapshot||evidence.before.document!==op.document||evidence.before_snapshot!==op.snapshot||evidence.input_sha256!==createHash('sha256').update(op.text).digest('hex')||!digest(evidence.preimage_sha256))throw Error('computer_uncertain_outcome');
      // The existing workflow evaluators consume canonical document evidence,
      // not an opaque nested action receipt or an echo of requested text.
      return evidence;
    }
    return {actions};
  }
}
