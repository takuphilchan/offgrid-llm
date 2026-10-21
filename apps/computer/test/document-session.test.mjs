import test from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {createHash} from 'node:crypto';
import {NativeSession} from '../native-session.mjs';
import {DesktopSession} from '../desktop-session.mjs';

const hash=text=>createHash('sha256').update(text).digest('hex');
function fixture(protocol=3){
  const target={id:'window',os_session:'os',process_generation:'process',surface:'surface',driver:'windows-uia'};
  const base=protocol===3?{...target,id:'desktop',process_generation:'desktop',surface:'desktop'}:target;
  const binding={actor:'alice',task:'task',companion:'session',session:'session',target:base};
  const document={target,document:'doc',generation:'generation',container:'tab',editor:'editor',observation:'observed',revision:'revision'};
  const view={version:1,complete:true,observation:{id:'observed',target,sequence:1,captured_at:new Date().toISOString(),state_digest:hash('state')},active:document,documents:[{id:'doc',generation:'generation',container:'tab',title:'Untitled',selected:true,capabilities:{read:true,select:true,select_text:true,insert_text:true,replace_selection:true,replace_value:true}}],commands:[{id:'new',element:'button',kind:'new_tab',profile:'fixture',revision:'one'}]};
  const read={version:1,snapshot:'snapshot',binding:document,complete:true,composition:'stable',text_sha256:hash(''),bytes:0,selection:{start:0,end:0},page:{snapshot:'snapshot',binding:document,start:0,end:0,total_scalars:0,text:'',source_complete:true}};
  const worker=new EventEmitter();worker.protocol=protocol;worker.calls=[];worker.stop=async()=>{};
  worker.request=async(kind,args)=>{
    worker.calls.push({kind,args:structuredClone(args)});
    if(kind==='documents')return structuredClone(view);
    if(kind==='read_document')return structuredClone(read);
    if(kind==='prepare'){
      const action={id:'prepared',operation:args.operation,approval_class:'reversible'};
      return protocol===3?{action,binding:{...binding,target}}:action;
    }
    if(kind==='approve')return {id:'grant'};
    if(kind==='execute'){
      const op=args.step.actions[0].operation.document;
      const after={...document,observation:'after-observation',revision:'after-revision'};
      const evidence={version:1,action_id:'prepared',operation:op.kind,after,after_snapshot:'after-snapshot',expected_sha256:hash(op.text??'')};
      if(op.kind==='create_document')Object.assign(evidence,{operation:'new_tab',command:op.command,after:{...after,document:'created',container:'created-tab',generation:'created-generation'}});
      else Object.assign(evidence,{before:document,before_snapshot:'snapshot',preimage_sha256:hash(''),input_sha256:hash(op.text??'')});
      return [{action_id:'prepared',outcome:'verified',document:evidence}];
    }
    if(kind==='select')return {identity:target};
    if(kind==='applications')return [];
    throw Error('unexpected '+kind);
  };
  const session=new (protocol===3?DesktopSession:NativeSession)({service:'http://127.0.0.1:11611',directory:'unused',worker});
  session.features={native_documents:1};session.binding=binding;session.session='session';session.approvalMode='full_task';session.target={identity:target,title:'Owned fixture'};
  const action=(kind,args={},id='call')=>({kind,arguments:args,id,binding,authorization:'auto'});
  return {session,worker,action,view,read,binding,target};
}

for(const protocol of [2,3])test(`document discovery and edits keep the existing protocol-${protocol} grant`,async()=>{
  const {session,worker,action,target}=fixture(protocol);
  await session.execute(action('computer_documents'));
  const read=await session.execute(action('computer_read_text',{observation_id:'observed',document:'doc'}));
  assert.equal(read.snapshot,'snapshot');
  const args={observation_id:'observed',document:'doc',snapshot:'snapshot',text:'café 日本語 😀'};
  const prepared=await session.execute(action('computer_prepare',{tool:'computer_insert_text',arguments:args},'edit:prepare'));
  assert.equal(prepared.approval_class,'reversible');
  const edited=await session.execute(action('computer_insert_text',args,'edit'));
  assert.equal(edited.action_id,'prepared');assert.equal(edited.input_sha256,hash(args.text));assert.equal(edited.actions,undefined);
  assert.deepEqual(worker.calls.map(c=>c.kind),['documents','read_document','prepare','approve','execute']);
  assert.deepEqual(worker.calls.find(c=>c.kind==='approve').args.step.binding.target,target);
  await assert.rejects(session.execute(action('computer_insert_text',args,'second')),/stale_observation/);
  assert.equal(worker.calls.filter(c=>c.kind==='execute').length,1);
  await session.stop();
});

test('document commands and selections are typed and cannot inject key macros',async()=>{
  for(const [tool,args,kind] of [
    ['computer_create_document',{observation_id:'observed',command:'new'},'create_document'],
    ['computer_select_document',{observation_id:'observed',document:'doc'},'select_document'],
    ['computer_select_text',{observation_id:'observed',document:'doc',snapshot:'snapshot',start:0,end:0},'select_text'],
    ['computer_replace_selection',{observation_id:'observed',document:'doc',snapshot:'snapshot',text:'hi'},'replace_selection'],
    ['computer_replace_text',{observation_id:'observed',document:'doc',snapshot:'snapshot',text:'hi'},'replace_value'],
  ]){
    const {session,worker,action}=fixture();
    await session.execute(action('computer_documents'));
    await session.execute(action('computer_read_text',{observation_id:'observed',document:'doc'}));
    await assert.rejects(session.execute(action(tool,{...args,shortcut:'ctrl+n'})),/invalid_action/);
    await session.execute(action(tool,args));
    const op=worker.calls.find(c=>c.kind==='prepare').args.operation;
    assert.equal(op.kind,'document');assert.equal(op.document.kind,kind);
    await session.stop();
  }
});

test('document transport rejects unnegotiated features, foreign identity and changed approval payloads',async()=>{
  const {session,worker,action,binding,view}=fixture();
  session.features={};await assert.rejects(session.execute(action('computer_documents')),/document_unavailable/);assert.equal(worker.calls.length,0);
  session.features={native_documents:1};
  await assert.rejects(session.execute({...action('computer_documents'),binding:{...binding,actor:'bob'}}),/scope_violation/);
  view.observation.target.os_session='foreign';
  await assert.rejects(session.execute(action('computer_documents')),/protocol_invalid/);
  view.observation.target.os_session='os';
  await session.execute(action('computer_documents'));
  await session.execute(action('computer_read_text',{observation_id:'observed',document:'doc'}));
  const args={observation_id:'observed',document:'doc',snapshot:'snapshot',text:'approved'};
  await session.execute(action('computer_prepare',{tool:'computer_insert_text',arguments:args},'edit:prepare'));
  await assert.rejects(session.execute(action('computer_insert_text',{...args,text:'different'},'edit')),/approval_invalid/);
  assert.equal(worker.calls.some(c=>c.kind==='approve'||c.kind==='execute'),false);await session.stop();
});

test('document text rejects lossy Unicode, stale snapshots and mismatched read-back pages',async()=>{
  const {session,worker,action,read}=fixture();
  await session.execute(action('computer_documents'));
  read.page.text='not blank';
  await assert.rejects(session.execute(action('computer_read_text',{observation_id:'observed',document:'doc'})),/protocol_invalid/);
  read.page.text='';
  await session.execute(action('computer_read_text',{observation_id:'observed',document:'doc'}));
  for(const invalid of [{text:'\ud800'},{text:'a\0b'},{snapshot:'old'},{document:'other'},{observation_id:'old'}]){
    await assert.rejects(session.execute(action('computer_insert_text',{observation_id:'observed',document:'doc',snapshot:'snapshot',text:'valid',...invalid})),/invalid_action|document_changed|stale_observation/);
  }
  assert.equal(worker.calls.some(c=>c.kind==='execute'),false);await session.stop();
});

test('lost document result and Stop never dispatch a replacement input',async()=>{
  for(const stop of [false,true]){
    const {session,worker,action}=fixture();const original=worker.request;
    worker.request=async(kind,args)=>{
      if(kind==='approve'&&stop){await session.stop();return {id:'grant'};}
      if(kind==='execute'){worker.calls.push({kind,args});throw Error('computer_uncertain_outcome');}
      return original(kind,args);
    };
    await session.execute(action('computer_documents'));
    const args={observation_id:'observed',command:'new'};
    await assert.rejects(session.execute(action('computer_create_document',args)),/uncertain_outcome|stopped/);
    await assert.rejects(session.execute(action('computer_create_document',args)),/stale_observation|stopped/);
    assert.equal(worker.calls.filter(c=>c.kind==='execute').length,stop?0:1);await session.stop();
  }
});

test('selected-window document discovery uses consent once, not a new desktop grant',async()=>{
  const {session,worker,action}=fixture(2);session.binding=null;
  await session.execute(action('computer_documents'));
  await session.execute(action('computer_documents'));
  assert.equal(worker.calls.filter(c=>c.kind==='select').length,1);
  assert.equal(worker.calls.some(c=>c.kind==='grant_desktop'),false);await session.stop();
});

test('a verified receipt without matching document evidence stays uncertain and is not retried',async()=>{
  for(const corrupt of [
    result=>delete result.document,
    result=>result.document.after.document='different-document',
    result=>result.document.after.generation='replaced',
    result=>result.document.after.editor='reused-editor',
    result=>result.document.after_snapshot=result.document.before_snapshot,
    result=>result.document.after.observation=result.document.before.observation,
    result=>result.document.input_sha256=hash('not requested'),
  ]){
    const {session,worker,action}=fixture();const original=worker.request;
    worker.request=async(kind,args)=>{const value=await original(kind,args);if(kind==='execute')corrupt(value[0]);return value;};
    await session.execute(action('computer_documents'));
    await session.execute(action('computer_read_text',{observation_id:'observed',document:'doc'}));
    const args={observation_id:'observed',document:'doc',snapshot:'snapshot',text:'requested'};
    await assert.rejects(session.execute(action('computer_insert_text',args)),/uncertain_outcome/);
    await assert.rejects(session.execute(action('computer_insert_text',args)),/stale_observation/);
    assert.equal(worker.calls.filter(c=>c.kind==='execute').length,1);await session.stop();
  }
});
