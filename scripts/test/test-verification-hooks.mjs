import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import vm from 'node:vm';
import test from 'node:test';
const require = createRequire(new URL('../../apps/web/package.json', import.meta.url));
const ts = require('typescript');
const source = readFileSync(new URL('../../internal/integrations/plugins/openclaw/offgrid/verification.ts',import.meta.url),'utf8');
const compiled = ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
function fixture({active=true,reject=false,unavailable=false,mode='full'}={}) {
  const env = active ? {OFFGRID_VERIFY_URL:'http://127.0.0.1:12345',OFFGRID_VERIFY_TOKEN:'a'.repeat(64),OFFGRID_VERIFY_RUN:'b'.repeat(48),OFFGRID_VERIFY_ADAPTER:'openclaw-2026.10.4-files-v1'} : {};
  const hooks={},requests=[],exports={};
  const sandbox=vm.createContext({exports,process:{env},Buffer,AbortSignal,fetch:async(url,options)=>{
    if(unavailable) throw Error('offline');
    const event=JSON.parse(options.body);requests.push({url,event,options});
    return new Response(JSON.stringify({allowed:!(reject&&url.endsWith('/before')),code:'fixture',message:'Specific shared rejection.'}));
  }});
  vm.runInContext(`(function(exports){${compiled}\n})(exports)`,sandbox);
  exports.registerVerification({registrationMode:mode,on(name,fn){hooks[name]=fn;}});
  return {hooks,requests,env,register:exports.registerVerification,reload(){
    sandbox.exports={};
    vm.runInContext(`(function(exports){${compiled}\n})(exports)`,sandbox);
    return sandbox.exports.registerVerification;
  }};
}
const ctx={sessionId:'s',runId:'t',toolCallId:'call',sandboxed:false};
const event=(name,params,extra={})=>({toolName:name,params,runId:'t',toolCallId:'call',...extra});
test('ordinary run has no hooks and credentials are consumed only from invocation environment',()=>{
  assert.equal(Object.keys(fixture({active:false}).hooks).length,0);
  assert.equal(Object.keys(fixture().env).length,0);
});
test('metadata registration cannot consume execution credentials',()=>{
  const {hooks,env,register}=fixture({mode:'cli-metadata'});
  assert.equal(Object.keys(hooks).length,0);
  assert.equal(env.OFFGRID_VERIFY_TOKEN,'a'.repeat(64));
  register({registrationMode:'full',on(name,fn){hooks[name]=fn;}});
  assert.equal(Object.keys(hooks).length,4);
  assert.equal(Object.keys(env).length,0);
});
test('fresh execution module retains credentials after a discarded discovery registry',()=>{
  const {env,reload}=fixture({mode:'discovery'});
  assert.equal(Object.keys(env).length,0);
  const hooks={};
  reload()({registrationMode:'discovery',on(name,fn){hooks[name]=fn;}});
  assert.equal(Object.keys(hooks).length,4);
});
test('reviewed real read payload yields complete metadata; results and arguments are untouched',async()=>{
  const {hooks,requests}=fixture();await hooks.before_prompt_build({},ctx);
  const params=Object.freeze({path:'source.txt'});
  assert.equal(await hooks.before_tool_call(event('read',params),ctx),undefined);
  const result=Object.freeze({content:[{type:'text',text:'1| café 日本語'}],details:{kind:'text',content:'1| café 日本語'}});
  assert.equal(await hooks.after_tool_call(event('read',params,{result}),ctx),undefined);
  await hooks.agent_end({success:true},ctx);
  assert.deepEqual(requests.map(r=>new URL(r.url).pathname),['/bind','/before','/after','/end']);
  assert.equal(requests[2].event.complete_read,true);
  assert.equal(requests[2].event.content,undefined,'private tool results never forwarded');
  assert.equal(result.content[0].text,'1| café 日本語');
});
test('partial, truncated and unknown results cannot count as complete reads',async()=>{
  for(const [params,result] of [
    [{path:'x',offset:2},{content:[],details:{kind:'text',content:'x'}}],
    [{path:'x',limit:1},{content:[],details:{kind:'text',content:'x'}}],
    [{path:'x'},{content:[],details:{kind:'truncated',content:'x',continuation:{offset:2}}}],
    [{path:'x'},{content:[{type:'text',text:'success'}]}],
  ]) {
    const {hooks,requests}=fixture();await hooks.before_prompt_build({},ctx);
    await hooks.before_tool_call(event('read',params),ctx);await hooks.after_tool_call(event('read',params,{result}),ctx);
    assert.equal(requests.at(-1).event.complete_read,false);
  }
});
test('veto preserves proposed numbered content and cannot report missing completion as success',async()=>{
  const {hooks,requests}=fixture({reject:true});await hooks.before_prompt_build({},ctx);
  const params=Object.freeze({path:'out.txt',content:'1| café 日本語'});
  const decision=await hooks.before_tool_call(event('write',params),ctx);
  assert.equal(decision.block,true);assert.equal(params.content,'1| café 日本語');
  assert.ok(decision.blockReason.includes('Specific shared rejection.'));
  assert.ok(!decision.blockReason.includes('absolute local'));
  assert.equal(requests.at(-1).event.status,'blocked');
  await hooks.after_tool_call(event('write',params,{error:'hook veto'}),ctx);
  assert.equal(requests.length,3,'own veto is not double-completed');
  const other=fixture();await other.hooks.before_prompt_build({},ctx);
  await other.hooks.before_tool_call(event('read',{path:'x'}),ctx);
  await other.hooks.agent_end({success:true},ctx);
  assert.equal(other.requests.some(r=>r.url.endsWith('/end')),false);
});
test('transport failure, absent session and foreign identities veto reviewed operations; unknown tools stay unchanged',async()=>{
  for(const setup of [{unavailable:true},{}]) {
    const {hooks,requests}=fixture(setup);
    assert.equal((await hooks.before_tool_call(event('write',{path:'x',content:'y'}),ctx)).block,true);
    assert.equal(await hooks.before_tool_call(event('web_search',{private:'not-forwarded'}),ctx),undefined);
    await hooks.agent_end({success:true},ctx);
    assert.equal(requests.some(r=>r.url.endsWith('/end')),false);
  }
});
