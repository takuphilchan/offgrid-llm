// Explicit opt-in; installed model/runtime are read-only. No host tools or downloads.
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,link} from 'node:fs/promises';
import {createWriteStream} from 'node:fs';
import {resolve,join,basename} from 'node:path';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {randomUUID} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';
const [binary,modelPath,runtime,temporaryRoot]=process.argv.slice(2);
assert(binary&&modelPath&&runtime&&temporaryRoot,'Pass service, installed GGUF, runtime and same-volume temporary directory');
const root=resolve(import.meta.dirname,'../..'),evidence=await mkdtemp(join(root,'build/workspace-real-model-'));
const models=await mkdtemp(join(resolve(temporaryRoot),'offgrid-model-smoke-'));
await link(resolve(modelPath),join(models,basename(modelPath)));
for(const folder of ['data','bin'])await mkdir(join(evidence,folder));
await writeFile(join(evidence,'config.yaml'),'{}');
const listener=createServer();listener.listen(0,'127.0.0.1');await once(listener,'listening');const port=listener.address().port;await new Promise(resolve=>listener.close(resolve));
const env=Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('OFFGRID_')));
Object.assign(env,{OFFGRID_CONFIG:join(evidence,'config.yaml'),OFFGRID_HOST:'127.0.0.1',OFFGRID_PORT:String(port),OFFGRID_DATA_DIR:join(evidence,'data'),OFFGRID_MODELS_DIR:models,OFFGRID_BIN_DIR:join(evidence,'bin'),OFFGRID_UI_DIR:join(root,'apps/web/dist'),OFFGRID_LLAMA_SERVER_PATH:resolve(runtime),OFFGRID_ENABLE_GPU:'true',OFFGRID_GPU_LAYERS:'99',OFFGRID_MAX_CONTEXT:'8192',OFFGRID_CHAT_CONTEXT:'4096',OFFGRID_NUM_THREADS:'2',OFFGRID_MAX_MEMORY_MB:'4096',OFFGRID_REQUIRE_AUTH:'false',OFFGRID_ENABLE_P2P:'false',OFFGRID_PREWARM_MODELS:'false'});
const log=createWriteStream(join(evidence,'service.log')),service=spawn(resolve(binary),['serve'],{cwd:evidence,env,windowsHide:true,stdio:['ignore','pipe','pipe']});service.stdout.pipe(log,{end:false});service.stderr.pipe(log,{end:false});const stopped=once(service,'exit');
const base=`http://127.0.0.1:${port}`,results=[];
async function request(path,body){const start=Date.now();const response=await fetch(base+path,{method:body?'POST':'GET',headers:body instanceof FormData?{}:{'Content-Type':'application/json'},body:body instanceof FormData?body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(240000)});let value;if(response.headers.get('content-type')?.includes('text/event-stream')){const events=(await response.text()).split('\n').filter(line=>line.startsWith('data: ')).map(line=>JSON.parse(line.slice(6)));await writeFile(join(evidence,body.request_id+'.json'),JSON.stringify(events,null,2));assert(!events.some(event=>event.type==='error'),JSON.stringify(events));value=events.findLast(event=>event.type==='done');assert(value,'No persisted completion');}else value=await response.json();assert(response.ok,`${response.status}: ${JSON.stringify(value)}`);return {value,ms:Date.now()-start};}
try{
 let ready=false;for(let i=0;i<100;i++){if(service.exitCode!==null)throw Error('Service exited');try{ready=(await fetch(base+'/api/v2/system',{signal:AbortSignal.timeout(500)})).ok;}catch{}if(ready)break;await delay(200);}assert(ready);
 const catalog=(await request('/v1/models')).value;await writeFile(join(evidence,'models.json'),JSON.stringify(catalog,null,2));
 const model=basename(modelPath,'.gguf');
 const cases=[
  {name:'Greeting',prompt:'Reply with a short greeting.'},
  {name:'Policy exception',prompt:'According to the attached policy, when do rural applications close? State the exception and the budget. Cite the document.',check:/July\s+31|31\s+July/i},
  {name:'Unsupported chart',prompt:'What was the exact value shown in Figure 9 of the attached policy? If the document does not provide it, say so without inventing a value.',check:/not|no |cannot|unable|doesn.t/i},
 ];
 for(const item of cases){
  const path='/v1/sessions/'+encodeURIComponent(item.name);await request('/v1/sessions',{name:item.name,model_id:model});let attachments;
  if(item.name!=='Greeting'){const form=new FormData();form.append('file',new Blob(['Synthetic public test record.\nRegional procurement policy.\nApplications close June 30. Exception: rural applications close July 31.\nApproved budget: USD 17.25.\nThis record contains no figures or charts.\n'],{type:'text/plain'}),'policy.txt');const upload=(await request(path+'/attachments',form)).value;attachments=[{id:upload.attachment.id,revision:upload.attachment.revision}];}
  const result=await request(path+'/generate',{content:item.prompt,model_id:model,attachments,max_tokens:256,profile:'interactive',stream:true,durable:true,request_id:randomUUID()});
  const answer=result.value.message?.content;assert.equal(typeof answer,'string');assert(answer.trim());if(item.check)assert.match(answer,item.check);if(item.name==='Policy exception')assert.match(answer,/17\.25/);
  results.push({name:item.name,prompt:item.prompt,answer,latencyMs:result.ms,attachments,response:result.value});await writeFile(join(evidence,'answers.json'),JSON.stringify(results,null,2));console.log(JSON.stringify({name:item.name,latencyMs:result.ms,answer}));
 }
 console.log('PASS: real local inference smoke (not corpus/model qualification): '+evidence);
}finally{service.kill();await stopped;await new Promise(resolve=>log.end(resolve));}
