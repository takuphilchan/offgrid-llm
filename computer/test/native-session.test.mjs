import test from 'node:test';
import assert from 'node:assert/strict';
import {EventEmitter} from 'node:events';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {NativeSession} from '../native-session.mjs';

function fixture(driver='windows-uia') {
  const target={title:'Owned editor',identity:{id:'window',os_session:'os',process_generation:'generation',surface:'surface',driver}};
  const binding={actor:'alice',task:'task',companion:'session',session:'session',target:target.identity};
  const worker=new EventEmitter();worker.calls=[];let seq=0;
  worker.stop=async()=>{worker.stopped=true;};
  worker.request=async(kind,args)=>{
    worker.calls.push({kind,args});
    if(kind==='targets')return [structuredClone(target)];
    if(kind==='select')return structuredClone(target);
    if(kind==='observe')return {observation:{id:`observation-${++seq}`,target:target.identity},elements:[{id:'field',name:'Report title',writable:true},{id:'check',name:'Include sources',checkable:true,checked:false}]};
    if(kind==='prepare')return {id:'prepared',operation:args.operation,control_identity:'stable',precondition:'a'.repeat(64),expected_change:'b'.repeat(64),approval_class:'reversible'};
    if(kind==='approve')return {id:'local-grant'};
    if(kind==='execute')return [{action_id:'prepared',outcome:'verified',evidence_id:'c'.repeat(64)}];
    if(kind==='verify')return args.operation.kind==='set_checked'
      ? {verified:true,check:'control_checked_equals',checked:args.operation.checked,element:args.operation.element,observation_id:args.observation.id,target:target.identity}
      : {verified:true,check:'control_text_equals',text:args.operation.text,element:args.operation.element,observation_id:args.observation.id,target:target.identity};
    throw Error('unexpected worker call');
  };
  const session=new NativeSession({service:'http://127.0.0.1:11611',directory:'unused',worker});
  session.target=target;session.session='session';
  const action=(kind,args={},extra={})=>({id:'task:call',kind,arguments:args,binding:structuredClone(binding),authorization:'exact',...extra});
  return {target,binding,worker,session,action};
}

for(const platform of ['windows-uia','macos-accessibility','linux-atspi']) {
  test(`native transport ${platform}: observed target, local consent and exact local approval`,async()=>{
    const {worker,session,action}=fixture(platform);
    await session.execute(action('computer_observe'));
    const value='Zimbabwe research — final (2026)';
    const result=await session.execute(action('computer_replace_text',{observation_id:'observation-1',element:'field',text:value}));
    assert.equal(result.actions[0].outcome,'verified');
    assert.deepEqual(worker.calls.map(c=>c.kind),['select','observe','prepare','approve','execute']);
    assert.equal(worker.calls[2].args.operation.text,value);
    assert.equal(worker.calls[4].args.grant,'local-grant');
    await assert.rejects(session.execute(action('computer_replace_text',{observation_id:'observation-1',element:'field',text:'again'})),/stale_observation/);
    assert.equal(worker.calls.filter(c=>c.kind==='execute').length,1);
    await session.stop();
  });
}

test('native transport exposes only bounded application shortcuts with fresh observation and approval',async()=>{
  const {worker,session,action}=fixture();
  await session.execute(action('computer_observe'));
  const result=await session.execute(action('computer_shortcut',{observation_id:'observation-1',element:'field',shortcut:'save'}));
  assert.equal(result.actions[0].outcome,'verified');
  assert.deepEqual(worker.calls[2].args.operation,{kind:'shortcut',element:'field',shortcut:'save'});
  assert.deepEqual(worker.calls.map(call=>call.kind),['select','observe','prepare','approve','execute']);
  await assert.rejects(session.execute(action('computer_shortcut',{observation_id:'observation-1',element:'field',shortcut:'alt_f4'})),/invalid_action/);
  await session.stop();
});

test('native transport sets and independently verifies explicit checkbox state',async()=>{
  const {worker,session,action}=fixture();
  await session.execute(action('computer_observe'));
  const result=await session.execute(action('computer_set_checked',{observation_id:'observation-1',element:'check',checked:true}));
  assert.equal(result.actions[0].outcome,'verified');
  assert.deepEqual(worker.calls[2].args.operation,{kind:'set_checked',element:'check',checked:true});
  await session.execute(action('computer_observe'));
  const verified=await session.execute(action('computer_verify_checked',{observation_id:'observation-2',element:'check',checked:true}));
  assert.equal(verified.verified,true);assert.equal(verified.check,'control_checked_equals');
  await assert.rejects(session.execute(action('computer_set_checked',{observation_id:'observation-2',element:'field',checked:true})),/invalid_action/);
  await session.stop();
});

test('native transport refuses scope changes, arbitrary calls and model supplied grants',async()=>{
  const {worker,session,action}=fixture();
  await assert.rejects(session.execute(action('computer_activate',{observation_id:'x',element:'field'})),/stale_observation/);
  await session.execute(action('computer_observe'));
  for(const field of ['actor','task','companion','session']) {
    const changed=action('computer_observe');changed.binding[field]='other';
    await assert.rejects(session.execute(changed),/scope_violation/);
  }
  const changed=action('computer_observe');changed.binding.target.process_generation='replacement';
  await assert.rejects(session.execute(changed),/scope_violation/);
  await assert.rejects(session.execute(action('shell',{command:'test'})),/invalid_action/);
  await assert.rejects(session.execute(action('computer_replace_text',{observation_id:'observation-1',element:'field',text:'x',grant:'invented'})),/invalid_action/);
  assert.equal(worker.calls.length,2);
  await session.stop();
});

test('local approval refusal and takeover cannot dispatch native input',async()=>{
  for(const stop of [false,true]) {
    const {worker,session,action}=fixture();const request=worker.request;
    worker.request=async(kind,args)=>{
      if(kind==='approve') { if(stop){await session.stop();return {id:'grant'};}throw Error('computer_approval_invalid'); }
      return request(kind,args);
    };
    await session.execute(action('computer_observe'));
    await assert.rejects(session.execute(action('computer_replace_text',{observation_id:'observation-1',element:'field',text:'x'})),/computer_(approval_invalid|stopped)/);
    assert.equal(worker.calls.filter(c=>c.kind==='execute').length,0);await session.stop();
  }
});

test('stop during target discovery does not pair or leave a late journal',async()=>{
  const {worker,target}=fixture();const directory=await mkdtemp(join(tmpdir(),'offgrid-native-start-'));
  let release;worker.request=()=>new Promise(resolve=>{release=resolve;});
  const session=new NativeSession({service:'http://127.0.0.1:11611',directory,worker});
  session.request=async()=>({product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:['native-computer-sessions-v2']});
  try {
    const started=session.start({code:'a'.repeat(64),workspace:'workspace',target});
    await new Promise(resolve=>setImmediate(resolve));await session.stop();release([target]);
    await assert.rejects(started,/stopped/);assert.equal(session.journal,undefined);
  } finally {await rm(directory,{recursive:true,force:true});}
});

test('native queue replays a persisted reply without repeating input',async()=>{
  const {worker,target,action}=fixture();const directory=await mkdtemp(join(tmpdir(),'offgrid-native-replay-'));
  const session=new NativeSession({service:'http://127.0.0.1:11611',directory,worker});
  const replies=[];let polls=0;
  const read=action('computer_observe',{}, {id:'task:observe'});
  const edit=action('computer_replace_text',{observation_id:'observation-1',element:'field',text:'Hello'}, {id:'task:edit'});
  session.request=async(path,body)=>{
    if(path.endsWith('/system'))return {product:'offgrid',api_version:2,workspace_id:'workspace',capabilities:['native-computer-sessions-v2']};
    if(path.endsWith('/pair'))return {session:{id:'session',target:target.identity,driver:target.identity.driver},token:'b'.repeat(64)};
    if(path.endsWith('/poll')) { if(polls===3){await session.stop();return {};}return {action:[read,edit,edit][polls++]}; }
    if(path.endsWith('/reply')) {replies.push(body);return {accepted:true};}
    throw Error('unexpected request');
  };
  try {
    await session.start({code:'a'.repeat(64),workspace:'workspace',target});await session.running;
    assert.equal(worker.calls.filter(c=>c.kind==='execute').length,1);
    assert.equal(replies.length,3);assert.deepEqual(replies[1],replies[2]);assert.equal(session.token,'');
  } finally {await session.stop();await rm(directory,{recursive:true,force:true});}
});

test('unconfirmed native stop never publishes success and can be retried',async()=>{
  const {session,worker}=fixture();const events=[];session.emit=event=>events.push(event);let attempts=0;
  worker.stop=async()=>{if(++attempts===1)throw Error('computer_stop_unconfirmed');};
  await assert.rejects(session.stop(),/stop_unconfirmed/);
  assert.equal(events.some(event=>event.state==='stopped'),false);
  assert.equal(events.at(-1).code,'computer_stop_unconfirmed');
  await session.stop();assert.equal(attempts,2);
  await assert.rejects(session.execute({}),/stopped/);
});

test('approval review uses the prepared exact action, not an expired observation',async()=>{
  const {worker,session,action}=fixture();
  await session.execute(action('computer_observe'));
  const args={observation_id:'observation-1',element:'field',text:'Approved text'};
  await session.execute(action('computer_prepare',{tool:'computer_replace_text',arguments:args},{id:'task:call:prepare'}));
  const request=worker.request;
  worker.request=async(kind,args)=>{if(kind==='prepare')throw Error('computer_stale_observation');return request(kind,args);};
  await session.execute(action('computer_replace_text',args));
  assert.equal(worker.calls.filter(c=>c.kind==='prepare').length,1);
  assert.equal(worker.calls.filter(c=>c.kind==='execute').length,1);
  await session.stop();
});

test('preparation cannot authorize changed arguments or a different action ID',async()=>{
  for(const change of ['text','id']){
    const {worker,session,action}=fixture();await session.execute(action('computer_observe'));
    const args={observation_id:'observation-1',element:'field',text:'Approved text'};
    await session.execute(action('computer_prepare',{tool:'computer_replace_text',arguments:args},{id:'task:call:prepare'}));
    const edit=action('computer_replace_text',{...args});
    if(change==='text')edit.arguments.text='Not approved';else edit.id='another';
    await assert.rejects(session.execute(edit),/approval_invalid/);
    assert.equal(worker.calls.filter(c=>c.kind==='execute').length,0);await session.stop();
  }
});

test('read-only verification asks the native provider and reobservation discards preparations',async()=>{
  const {worker,session,action}=fixture();await session.execute(action('computer_observe'));
  const args={observation_id:'observation-1',element:'field',text:'Requested text'};
  await session.execute(action('computer_prepare',{tool:'computer_replace_text',arguments:args},{id:'task:call:prepare'}));
  await session.execute(action('computer_observe'));assert.equal(session.preparation,null);
  const result=await session.execute(action('computer_verify',{...args,observation_id:'observation-2'}));
  assert.equal(result.verified,true);assert.equal(result.observation_id,'observation-2');
  assert.equal(worker.calls.filter(c=>['approve','execute'].includes(c.kind)).length,0);
  await assert.rejects(session.execute(action('computer_verify',args)),/stale_observation/);
  await session.stop();
});

test('native approval modes auto-run only the locally classified scope',async()=>{
  for(const [mode,kind,approvalClass,automatic] of [
    ['ask_every_time','computer_replace_text','reversible',false],
    ['scoped_changes','computer_replace_text','reversible',true],
    ['scoped_changes','computer_activate','consequential',false],
    ['full_task','computer_activate','consequential',true],
  ]) {
    const {worker,session,action}=fixture();session.approvalMode=mode;
    const request=worker.request;worker.request=async(type,args)=>type==='prepare'?{...await request(type,args),approval_class:approvalClass}:request(type,args);
    await session.execute(action('computer_observe'));
    const args={observation_id:'observation-1',element:'field',...(kind==='computer_replace_text'?{text:'value'}:{})};
    await session.execute(action(kind,args,{authorization:automatic?'auto':'exact'}));
    assert.equal(worker.calls.find(call=>call.kind==='approve').args.automatic,automatic);
    await session.stop();
  }
  const {worker,session,action}=fixture();session.approvalMode='full_task';const request=worker.request;
  worker.request=async(type,args)=>type==='prepare'?{...await request(type,args),approval_class:'forbidden'}:request(type,args);
  await session.execute(action('computer_observe'));
  await assert.rejects(session.execute(action('computer_activate',{observation_id:'observation-1',element:'field'},{authorization:'auto'})),/prohibited/);
  assert.equal(worker.calls.some(call=>call.kind==='execute'),false);await session.stop();
});
