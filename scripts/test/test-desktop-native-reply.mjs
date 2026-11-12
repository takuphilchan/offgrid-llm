// Real packaged desktop -> owned service -> local model, with copied offline
// artifacts and an isolated home. No installed service, account or model changes.
import assert from 'node:assert/strict';
import {createServer} from 'node:net';
import {createReadStream} from 'node:fs';
import {mkdtemp, mkdir, copyFile, readdir, readFile, writeFile, stat} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {createRequire} from 'node:module';
import {tmpdir} from 'node:os';
import {resolve, join, basename, dirname} from 'node:path';
import {runKnowledgeJourney} from './desktop-native-knowledge.mjs';
import {stageImageDraft,verifyImageDraftRestart} from './desktop-native-images.mjs';
import {runAgentJourney} from './desktop-native-agent.mjs';

const root=resolve(import.meta.dirname,'../..');
const require=createRequire(join(root,'apps/web/package.json'));
const {_electron:electron}=require('playwright');
const {expect}=require('@playwright/test');
const agentMode=process.argv.includes('--agent');
const gpuMode=process.argv.includes('--gpu');
const inputs=process.argv.slice(2).filter(value=>!['--agent','--gpu'].includes(value));
assert.ok([3,4].includes(inputs.length),'Usage: node scripts/test/test-desktop-native-reply.mjs DESKTOP_EXE MODEL_GGUF VERIFIED_RUNTIME_DIRECTORY [EMBEDDING_GGUF] [--agent] [--gpu]');
const [binary,modelSource,runtimeSource,embeddingSource]=inputs.map(value=>resolve(value));
assert.equal(process.platform,'win32','This qualification currently covers Windows only');
assert.match(modelSource,/\.gguf$/i);
const evidence=await mkdtemp(join(tmpdir(),'offgrid-desktop-native-reply-'));
console.log(`Native reply evidence: ${evidence}`);
const home=join(evidence,'home'),profile=join(evidence,'workspace');
const models=join(profile,'models'),bin=join(profile,'bin');
// USERPROFILE also controls Windows file-picker locations. A disposable home
// needs its standard folders before Electron starts; otherwise opening an image
// picker can show "Desktop is unavailable" over the qualification window.
const homeFolders=['Desktop','Documents','Downloads','Pictures','Music','Videos'];
for(const directory of [home,models,bin,join(home,'AppData','Local'),join(home,'AppData','Roaming'),...homeFolders.map(name=>join(home,name))]) await mkdir(directory,{recursive:true});
for(const name of homeFolders)assert.equal((await stat(join(home,name))).isDirectory(),true);
const digest=async file=>{const hash=createHash('sha256');for await(const bytes of createReadStream(file))hash.update(bytes);return hash.digest('hex');};
const originalHash=await digest(modelSource);
const desktopHash=await digest(binary);
const serviceHash=await digest(join(dirname(binary),'resources/bin/offgrid.exe'));
const modelFile=join(models,basename(modelSource));
await copyFile(modelSource,modelFile);
assert.equal(await digest(modelFile),originalHash,'Model copy must preserve bytes');
let embeddingHash;
if(embeddingSource){assert.match(embeddingSource,/\.gguf$/i);assert.notEqual(basename(embeddingSource),basename(modelSource));embeddingHash=await digest(embeddingSource);await copyFile(embeddingSource,join(models,basename(embeddingSource)));assert.equal(await digest(join(models,basename(embeddingSource))),embeddingHash);}
const manifest=JSON.parse(await readFile(join(runtimeSource,'runtime-dependencies.json'),'utf8'));
assert.equal(manifest.schema_version,1);assert.equal(manifest.platform,gpuMode?'windows-amd64-cuda':'windows-amd64-cpu');
assert.ok(manifest.files.some(file=>file.name==='llama-server.exe'));
for(const file of manifest.files) {
  assert.match(file.name,/^[a-z0-9_.-]+\.(exe|dll)$/i);
  const source=join(runtimeSource,file.name),destination=join(bin,file.name);
  assert.equal(await digest(source),file.sha256);
  await copyFile(source,destination);assert.equal(await digest(destination),file.sha256);
}
const socket=createServer();await new Promise(done=>socket.listen(0,'127.0.0.1',done));
const port=socket.address().port;await new Promise(done=>socket.close(done));
const origin=`http://127.0.0.1:${port}`;
// Do not inherit installed OffGrid pins, auth, external endpoints or config home.
const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.SystemRoot,COMSPEC:process.env.COMSPEC,
  // NVIDIA's Windows driver needs its machine installation roots for NVML.
  ProgramFiles:process.env.ProgramFiles,ProgramW6432:process.env.ProgramW6432,
  PATH:join(process.env.SystemRoot,'System32'),TEMP:evidence,TMP:evidence,
  USERPROFILE:home,HOME:home,LOCALAPPDATA:join(home,'AppData','Local'),APPDATA:join(home,'AppData','Roaming'),
  OFFGRID_DESKTOP_HOME:profile,OFFGRID_PORT:String(port),OFFGRID_ENABLE_GPU:String(gpuMode),OFFGRID_GPU_LAYERS:gpuMode?'999':'0',
  OFFGRID_MAX_MODELS:'1',OFFGRID_NUM_THREADS:'2',OFFGRID_MAX_CONTEXT:agentMode?'8192':'2048',OFFGRID_CHAT_CONTEXT:agentMode?'8192':'2048',
  OFFGRID_BATCH_SIZE:'128',OFFGRID_PREWARM_MODELS:'false',OFFGRID_SMART_MLOCK:'false',OFFGRID_KV_CACHE_TYPE:'f16',
  OFFGRID_FLASH_ATTENTION:'false',OFFGRID_MODEL_LOAD_TIMEOUT:'90',OFFGRID_REQUIRE_AUTH:'false'};
let app,page;
const violations=[],submissions=[];
const deadline=setTimeout(()=>{console.error('Native reply qualification exceeded its deadline');process.exitCode=1;void app?.close().catch(()=>{});},embeddingSource||agentMode?600000:300000);
deadline.unref();
async function launch() {
  app=await electron.launch({executablePath:binary,cwd:home,env,timeout:30000});
  await app.evaluate(({BrowserWindow})=>{
    for(const window of BrowserWindow.getAllWindows()) {
      window.on('page-title-updated',event=>event.preventDefault());
      window.setTitle('OffGrid — isolated verification (not your workspace)');
    }
  });
  app.context().setDefaultTimeout(20000);
  await app.context().route(/^https?:\/\//,route=>{
    const request=route.request(),url=new URL(request.url());
    if(url.origin!==origin){violations.push(url.origin);return route.abort();}
    if(request.method()==='POST'&&/\/v1\/sessions\/[^/]+\/generate$/.test(url.pathname))submissions.push(url.pathname);
    return route.continue();
  });
  page=await app.firstWindow();await page.waitForURL(origin+'/ui/',{timeout:45000});
  const backend=await page.evaluate(()=>window.electron.getBackendInfo());
  assert.equal(backend.state,'ready');assert.equal(backend.managedByDesktop,true);
  const paths=await page.evaluate(()=>window.electron.getPaths());assert.equal(paths.models,models);assert.equal(paths.data,join(profile,'data'));
  return backend;
}
async function api(path){const response=await fetch(origin+path,{signal:AbortSignal.timeout(10000)});assert.equal(response.status,200,path);return response.json();}
async function closeOwned(){await app.close();app=null;await expect.poll(async()=>{try{await fetch(origin+'/health',{signal:AbortSignal.timeout(500)});return false;}catch{return true;}},{timeout:20000}).toBe(true);}
try {
  const first=await launch();
  await expect(page.locator('.onboarding')).toBeVisible();
  const engine=page.getByRole('region',{name:'Inference engine',exact:true});
  await engine.getByRole('button',{name:'Prepare engine',exact:true}).click();
  await expect(engine.getByRole('status')).toContainText('Engine ready.',{timeout:30000});
  const preparation=await api('/api/v2/runtime');
  assert.equal(preparation.state,'ready');assert.equal(preparation.mode,gpuMode?'nvidia':'cpu');
  assert.equal(await page.evaluate(()=>localStorage.getItem('offgrid.onboarding.complete')),null,'Engine preparation alone must not complete onboarding');
  await expect(page.locator('.onboarding .primary-button')).toHaveText('Start your first chat');
  await page.locator('.onboarding .primary-button').click();
  await expect(page.locator('.onboarding')).not.toBeVisible();
  assert.equal(await page.evaluate(()=>localStorage.getItem('offgrid.onboarding.complete')),null,'Connection/model alone must not complete onboarding');
  const library=await api('/v1/models');assert.equal(library.data.length,embeddingSource?2:1);
  const before=await api('/v1/sessions');assert.equal((before.sessions??[]).length,0);
  const prompt='Reply with a plain-text greeting in one short sentence. Do not use Markdown.';
  await page.locator('.composer textarea').fill(prompt);
  const started=Date.now();await page.getByRole('button',{name:'Send',exact:true}).click();
  await expect(page.locator('.message.assistant:not(.streaming-response) .message-body')).not.toBeEmpty({timeout:120000});
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await expect(page.locator('.composer textarea')).toBeEnabled();
  const elapsedMs=Date.now()-started;
  const answer=(await page.locator('.message.assistant:not(.streaming-response) .message-body').innerText()).trim();
  const saved=await api('/v1/sessions');assert.equal(saved.sessions.length,1);
  const session=await api('/v1/sessions/'+encodeURIComponent(saved.sessions[0].name));
  assert.equal(session.messages.filter(message=>message.role==='user').length,1);
  assert.equal(session.messages.filter(message=>message.role==='assistant').length,1);
  assert.equal(session.messages.find(message=>message.role==='user').content,prompt);
  assert.equal(session.messages.find(message=>message.role==='assistant').content.trim(),answer);
  assert.equal(await page.evaluate(()=>localStorage.getItem('offgrid.onboarding.complete')),'true');
  assert.equal(submissions.length,1,'One user submission must not become duplicate turns');
  const identity=await api('/api/v2/system');
  await page.screenshot({path:join(evidence,'first-reply.png')});
  const knowledge=embeddingSource?await runKnowledgeJourney({page,api,evidence,expect}):null;
  const agent=agentMode?await runAgentJourney({page,origin,api,evidence,expect}):null;
  const retained=agent??knowledge??{name:saved.sessions[0].name,session,answer};
  const expectedSubmissions=1+(knowledge?1:0)+(agent?1:0);
  assert.equal(submissions.length,expectedSubmissions);
  const imageDraft=!agent&&identity.capabilities?.includes('session-images-v1')?await stageImageDraft({page,origin,name:retained.name,api,expect,evidence}):null;
  await closeOwned();
  await launch();
  await expect(page.locator('.onboarding')).not.toBeVisible();
  await expect(page.locator('.message.assistant:not(.streaming-response) .message-body').last()).toHaveText(retained.answer);
  assert.equal((await api('/api/v2/system')).workspace_id,identity.workspace_id);
  assert.deepEqual(await api('/v1/sessions/'+encodeURIComponent(retained.name)),retained.session,'Restart must preserve the saved conversation without replay');
  assert.equal(submissions.length,expectedSubmissions);assert.deepEqual(violations,[]);
  const imageInput=imageDraft?await verifyImageDraftRestart({page,image:imageDraft,expect}):null;
  assert.deepEqual((await readdir(bin)).sort(),manifest.files.map(file=>file.name).sort(),'Runtime discovery must use the prepared worker');
  await closeOwned();
  assert.equal(await digest(modelSource),originalHash);assert.equal(await digest(modelFile),originalHash);
  if(embeddingSource){assert.equal(await digest(embeddingSource),embeddingHash);assert.equal(await digest(join(models,basename(embeddingSource))),embeddingHash);}
  const result={passed:true,platform:process.platform,backend:preparation.mode,preparation,model:basename(modelSource),modelSha256:originalHash,embeddingSha256:embeddingHash,desktopSha256:desktopHash,serviceSha256:serviceHash,uiBuildId:identity.ui_build_id,workerSha256:manifest.files.find(file=>file.name==='llama-server.exe').sha256,serviceVersion:first.version,elapsedMs,answer,knowledge,agent,imageInput,submissions:submissions.length,preparedOfflineArtifacts:true,installedWorkspaceChanged:false,evidence};
  await writeFile(join(evidence,'result.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
} catch(error) {
  await writeFile(join(evidence,'failure.txt'),String(error.stack??error));
  if(page)await page.screenshot({path:join(evidence,'failure.png')}).catch(()=>{});
  throw error;
} finally {clearTimeout(deadline);await app?.close();}
