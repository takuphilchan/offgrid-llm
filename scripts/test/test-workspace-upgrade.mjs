// Rehearse upgrade, crash/restart and snapshot rollback on disposable data only.
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,readFile,readdir,cp} from 'node:fs/promises';
import {createWriteStream} from 'node:fs';
import {resolve,join} from 'node:path';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import {createServer} from 'node:net';
import {createHash,randomUUID} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';

assert(process.argv[2]&&process.argv[3],'Pass baseline and candidate service executables');
const [baseline,candidate]=process.argv.slice(2).map(path=>resolve(path));
const root=resolve(import.meta.dirname,'../..'),evidence=await mkdtemp(join(root,'build/workspace-upgrade-'));
for(const folder of ['data','models','bin'])await mkdir(join(evidence,folder));
await writeFile(join(evidence,'config.yaml'),'{}');
const listener=createServer();listener.listen(0,'127.0.0.1');await once(listener,'listening');const port=listener.address().port;await new Promise(resolve=>listener.close(resolve));
const base=`http://127.0.0.1:${port}`,name='Upgrade fixture',sessionPath='/v1/sessions/'+encodeURIComponent(name);
const history=[{role:'user',content:'Fixture policy 17.25; exception July 31.'},{role:'assistant',content:'Recorded fixture answer, not a model result.'}];
const original=Buffer.from('Licensed synthetic test record.\nException: rural applications close July 31.\nBudget: USD 17.25.\n');
let service,stopped,log;
async function start(binary,data,label){
 const env=Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('OFFGRID_')));
 Object.assign(env,{OFFGRID_CONFIG:join(evidence,'config.yaml'),OFFGRID_HOST:'127.0.0.1',OFFGRID_PORT:String(port),OFFGRID_DATA_DIR:data,OFFGRID_MODELS_DIR:join(evidence,'models'),OFFGRID_BIN_DIR:join(evidence,'bin'),OFFGRID_UI_DIR:join(root,'apps/web/dist'),OFFGRID_REQUIRE_AUTH:'false',OFFGRID_ENABLE_P2P:'false',OFFGRID_PREWARM_MODELS:'false',OFFGRID_ENABLE_CONVERSATION_PREVIEW:'true'});
 log=createWriteStream(join(evidence,label+'.log'));service=spawn(binary,['serve'],{cwd:evidence,env,windowsHide:true,stdio:['ignore','pipe','pipe']});service.stdout.pipe(log,{end:false});service.stderr.pipe(log,{end:false});stopped=once(service,'exit');
 for(let i=0;i<150;i++){if(service.exitCode!==null)throw Error(label+' exited');try{const response=await fetch(base+'/api/v2/system',{signal:AbortSignal.timeout(500)});if(response.ok)return response.json();}catch{}await delay(200);}
 throw Error(label+' did not start');
}
async function stop(){if(!service)return;service.kill();await stopped;await new Promise(resolve=>log.end(resolve));service=undefined;}
async function request(path,body,method=body?'POST':'GET'){
 const response=await fetch(base+path,{method,headers:body instanceof FormData?{}:{'Content-Type':'application/json'},body:body instanceof FormData?body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(10000)});
 const value=await response.json();assert(response.ok,`${response.status} ${path}: ${JSON.stringify(value)}`);return value;
}
async function hashes(directory,prefix=''){
 const values={};for(const entry of await readdir(directory,{withFileTypes:true})){const path=join(directory,entry.name),key=prefix+entry.name;if(entry.isDirectory())Object.assign(values,await hashes(path,key+'/'));else if(entry.isFile())values[key]=createHash('sha256').update(await readFile(path)).digest('hex');}return values;
}
let attachment,identity,accepted,conversation;
async function inspect(additional=[]){
 const session=await request(sessionPath),files=await request(sessionPath+'/attachments');
 assert.deepEqual(session.messages.map(({role,content})=>({role,content})),[...history,...additional]);
 assert.equal(files.attachments.length,1);assert.equal(files.attachments[0].sha256,attachment.sha256);assert.equal(files.attachments[0].revision,attachment.revision);
 const response=await fetch(base+sessionPath+'/attachments/'+attachment.id+'/original');assert(response.ok);assert.deepEqual(Buffer.from(await response.arrayBuffer()),original);
 return {owner:session.owner_id??'',messages:session.messages.length,attachments:files.attachments.length,sha256:attachment.sha256,revision:attachment.revision};
}
try{
 identity=await start(baseline,join(evidence,'data'),'baseline');
 await request('/v1/sessions',{name,model_id:'unavailable-fixture-model',history});
 const form=new FormData();form.append('file',new Blob([original],{type:'text/plain'}),'policy.txt');attachment=(await request(sessionPath+'/attachments',form)).attachment;
 const before=await inspect();await stop();
 await cp(join(evidence,'data'),join(evidence,'baseline-snapshot'),{recursive:true,errorOnExist:true});const snapshot=await hashes(join(evidence,'baseline-snapshot'));
 const upgradedIdentity=await start(candidate,join(evidence,'data'),'candidate');assert.equal(upgradedIdentity.workspace_id,identity.workspace_id);assert.deepEqual(await inspect(),before);
 const imported=await request('/api/v2/conversations',{request_id:randomUUID(),target_id:'',expected_revision:0,kind:'import_conversation',payload:{legacy_name:name}});conversation=imported.conversation;
 assert(conversation?.id);const command={request_id:randomUUID(),target_id:conversation.id,expected_revision:conversation.revision,kind:'submit_input',payload:{content:'Explain the retained exception.',source_session:name,attachments:[{id:attachment.id,revision:attachment.revision}]}};
 accepted=await request('/api/v2/conversations/'+conversation.id+'/inputs',command);assert(accepted.input?.id);await stop();
 await start(candidate,join(evidence,'data'),'candidate-restarted');const retained=await request('/api/v2/conversations/'+conversation.id+'/inputs/'+accepted.input.id);assert.deepEqual(retained.input,command.payload);
 const duplicate=await request('/api/v2/conversations/'+conversation.id+'/inputs',command);assert.equal(duplicate.input.id,accepted.input.id);assert.deepEqual(await inspect([{role:'user',content:command.payload.content}]),{...before,messages:before.messages+1});await stop();
 // Older software gets its matching data snapshot, never the newer store.
 await cp(join(evidence,'baseline-snapshot'),join(evidence,'rollback-data'),{recursive:true,errorOnExist:true});assert.deepEqual(await hashes(join(evidence,'rollback-data')),snapshot);
 const rolledBack=await start(baseline,join(evidence,'rollback-data'),'rollback');assert.equal(rolledBack.workspace_id,identity.workspace_id);assert.deepEqual(await inspect(),before);assert(!(await request(sessionPath)).canonical_id);await stop();
 assert.deepEqual(await hashes(join(evidence,'baseline-snapshot')),snapshot);
 await writeFile(join(evidence,'receipt.json'),JSON.stringify({baseline,candidate,before,workspace:identity.workspace_id,snapshotFiles:Object.keys(snapshot).length,sourceSnapshot:snapshot,input:accepted.input.id,upgrade:true,restart:true,duplicateAdmission:false,rollback:true,modelInference:false},null,2));
 console.log('Upgrade/restart/rollback passed: '+evidence);
}finally{await stop();}
