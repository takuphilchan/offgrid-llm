// Uses installed OpenClaw's public hook runner with deterministic native-shaped
// events. The Go test owns the verifier and disposable files. No inference.
import assert from 'node:assert/strict';
import {readFile,writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import {createRequire} from 'node:module';
import vm from 'node:vm';
import path from 'node:path';
const require=createRequire(new URL('../../apps/web/package.json',import.meta.url));
const ts=require('typescript');
const root=process.env.OFFGRID_OPENCLAW_TEST_ROOT;
const {initializeGlobalHookRunner,getGlobalHookRunner}=await import(pathToFileURL(path.join(root,'dist/plugins/hook-runner-global.js')));
const source=await readFile(new URL('../../internal/integrations/plugins/openclaw/offgrid/verification.ts',import.meta.url),'utf8');
const exports={};
vm.runInNewContext(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports,process,Buffer,fetch,AbortSignal});
const registry={plugins:[],hooks:[],typedHooks:[],trustedToolPolicies:[]};
exports.registerVerification({registrationMode:'full',on(hookName,handler){registry.typedHooks.push({pluginId:'offgrid',hookName,handler,priority:0});}});
initializeGlobalHookRunner(registry);
const runner=getGlobalHookRunner(), ctx={sessionId:'fixture-session',runId:'fixture-turn',sandboxed:false};
await runner.runBeforePromptBuild({prompt:'fixture',messages:[]},ctx);
const content=await readFile('source.txt','utf8');
for (const [call,toolName,params] of [
  ['read-source','read',{path:'source.txt'}],['bad-write','write',{path:'copy.txt',content:'wrong'}],
  ['good-write','write',{path:'copy.txt',content}],['read-copy','read',{path:'copy.txt'}],
]) {
  params.path=path.resolve(params.path);
  const event={toolName,params,toolCallId:call,runId:ctx.runId};
  const decision=await runner.runBeforeToolCall(event,ctx);
  if(call==='bad-write'){assert.equal(decision?.block,true);continue;}
  assert.notEqual(decision?.block,true,decision?.blockReason);
  let result;
  if(toolName==='write'){await writeFile(params.path,params.content);result={content:[{type:'text',text:'Written'}]};}
  else {const text=await readFile(params.path,'utf8');result={content:[{type:'text',text}],details:{kind:'text',content:text}};}
  await runner.runAfterToolCall({...event,result},ctx);
}
await runner.runAgentEnd({success:true,messages:[],runId:ctx.runId},ctx);
assert.equal(await readFile('copy.txt','utf8'),content);
console.log('installed OpenClaw public hook dispatch, veto and lifecycle passed (deterministic tool fixtures)');
