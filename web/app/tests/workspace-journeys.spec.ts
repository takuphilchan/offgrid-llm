import { createHash } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';

// Stateful API-boundary journeys. No model downloads, user recordings, native
// input or claims of model quality. Real-service storage tests run separately.
const id = 'run-' + 'c'.repeat(32), output = 'Journey output\n';
const artifact = {name:'result.txt',format:'txt',bytes:Buffer.byteLength(output),sha256:createHash('sha256').update(output).digest('hex'),verified:true,check:'stored_bytes_sha256_and_format'};
async function workspace(page: Page) {
  const state = { models:true, knowledge:true, documents:[] as any[], sessions:{} as Record<string, any>, turns:[] as any[], writes:[] as {path:string;data:any}[], lost:false,
    run:{run_id:id,prompt:'',model:'journey-model',status:'completed',output,steps:[],artifacts:[artifact],instructions:[],pending_input:null,pending_approval:null,can_steer:false} as any };
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete','true'); localStorage.setItem('offgrid.locale','en'); });
  await page.route('**/health', r=>r.fulfill({json:{status:'healthy'}}));
  await page.route(/\/(?:v1|api\/v2)\//, async r=>{
    const req=r.request(), path=new URL(req.url()).pathname, method=req.method();
    let data:any = {};
    if(method!=='GET') {
      if(req.headers()['content-type']?.includes('application/json')) data=req.postDataJSON();
      state.writes.push({path,data});
    }
    if(path==='/v1/sessions' && method==='POST') return r.fulfill({json:state.sessions[data.name]={...data,messages:[],updated_at:'2026-10-08T00:00:00Z'}});
    if(path.endsWith('/generate')) {
      const name=decodeURIComponent(path.split('/')[3]); state.turns.push(data);
      const session=state.sessions[name], message={role:'assistant',content:`Saved reply ${state.turns.length}: ${data.content}`};
      session.messages.push({role:'user',content:data.content},message);
      return r.fulfill({contentType:'text/event-stream',body:`data: ${JSON.stringify({type:'done',session,message})}\n\n`});
    }
    if(path.endsWith('/turn')) return r.fulfill({json:{turn:null}});
    if(path.startsWith('/v1/sessions/')) return r.fulfill({json:state.sessions[decodeURIComponent(path.split('/')[3])]??{}});
    if(path==='/api/v2/jobs' && method==='POST') {
      state.run.prompt=data.prompt;
      if(state.lost) { state.lost=false; return r.abort('failed'); }
      return r.fulfill({status:202,json:state.run});
    }
    if(path===`/api/v2/jobs/${id}/events`) return r.fulfill({contentType:'text/event-stream',body:`data: ${JSON.stringify({...state.run,type:'snapshot'})}\n\n`});
    if(path===`/api/v2/jobs/${id}/artifact`) return r.fulfill({contentType:'application/octet-stream',body:output});
    if(path.startsWith(`/api/v2/jobs/${id}/`) && method==='POST') {
      if(path.endsWith('/input')) {state.run.pending_input=null;state.run.status='waiting_for_approval';}
      if(path.endsWith('/approve')) {state.run.status='completed';state.run.pending_approval=null;}
      if(path.endsWith('/pause')) {state.run.status='interrupted';state.run.can_steer=true;state.run.resumable=true;}
      if(path.endsWith('/steer')) state.run.instructions.push({request_id:data.request_id,text:data.instruction});
      if(path.endsWith('/resume')) state.run.status='running';
      if(path.endsWith('/cancel')) state.run.status='cancelled';
      return r.fulfill({status:202,json:state.run});
    }
    if(path===`/api/v2/jobs/${id}` && method==='DELETE') {state.run.prompt='';return r.fulfill({json:{success:true}});}
    if(path==='/v1/documents/ingest') state.documents=[{id:'doc',name:'Journey.txt',size:16,chunk_count:1,index_status:'ready',source_retained:true}];
    if(path==='/v1/documents/delete') state.documents=[];
    const bodies:Record<string,unknown>={
      '/api/v2/system':{product:'offgrid',version:'journey',api_version:2,workspace_id:'journey',capabilities:['task-first-agents-v2','durable-agent-events-v2']},
      '/v1/users/me':{authenticated:false,auth_required:false,user:null}, '/v1/system/config':{version:'journey',inference_slots:1},
      '/v1/models':{data:state.models?[{id:'journey-model',type:'chat'},{id:'embedding',type:'embedding'}]:[]},
      '/v1/catalog':{models:[]},'/v1/models/download/progress':{},'/v1/sessions':{sessions:Object.values(state.sessions)},
      '/api/v2/jobs':state.run.prompt?[{id,prompt:state.run.prompt,status:state.run.status,deletable:state.run.status==='completed',created_at:'2026-10-08T00:00:00Z'}]:[],
      [`/api/v2/jobs/${id}`]:state.run,'/v1/rag/status':{enabled:state.knowledge,stats:{},embedding_model:'embedding'},
      '/v1/documents':{documents:state.documents},'/v1/documents/source':{document:state.documents[0],content:'Journey source.',truncated:false},
      '/v1/runs':{runs:state.run.prompt?[{id,status:state.run.status,updated_at:'2026-10-08T00:00:00Z',data:{prompt:state.run.prompt}}]:[]},
      '/api/v2/computer/status':{available:false},'/api/v2/computer/sessions':{sessions:[]},
      '/api/v2/models':{models:[]},'/api/v2/models/operations':{operations:[]},'/v1/stats':{server:{version:'journey'}},
      '/v1/audio/status':{profiles:[{id:'asr',revision:'r1',name:'Whisper fixture',capabilities:['transcription'],available:true},{id:'tts',revision:'r1',name:'Piper fixture',capabilities:['speech_synthesis'],available:true,voices:[{id:'en',language:'en'}]}]},
      '/v1/audio/transcriptions':{text:'Dictated draft.'},
    };
    return r.fulfill({json:bodies[path]??{}});
  });
  return state;
}
const nav=async(page:Page,route:string)=>page.locator(`.primary-nav a[href="#/${route}"]`).click();
async function send(page:Page,text:string) {await page.locator('.composer textarea').fill(text);await page.getByRole('button',{name:'Send',exact:true}).click();}
async function start(page:Page) {await page.goto('/ui/#/agents/new');await page.getByRole('textbox',{name:'Task',exact:true}).fill('Prepare the journey result');await page.getByRole('button',{name:'Start task',exact:true}).click();}

test('01 ready Chat saves a reply, survives navigation and saves a follow-up once',async({page})=>{
  const s=await workspace(page);await page.goto('/ui/#/chat');await send(page,'First question');
  await expect(page.locator('.message.assistant')).toContainText('Saved reply 1');await nav(page,'settings');await nav(page,'chat');
  await send(page,'Follow up');await expect(page.locator('.message.assistant').last()).toContainText('Saved reply 2');
  expect(Object.values(s.sessions)).toHaveLength(1);expect(Object.values(s.sessions)[0].messages).toHaveLength(4);expect(s.turns.map(t=>t.content)).toEqual(['First question','Follow up']);
});
test('02 missing model setup returns to the preserved draft before the first reply',async({page})=>{
  const s=await workspace(page);s.models=false;await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Preserve this');
  await page.getByRole('button',{name:'Choose a chat model',exact:true}).click();expect(s.writes).toEqual([]);s.models=true;
  await page.getByRole('button',{name:'Return to your draft'}).click();await expect(page.locator('.composer textarea')).toHaveValue('Preserve this');
  await page.getByRole('button',{name:'Send',exact:true}).click();await expect(page.locator('.message.assistant')).toContainText('Preserve this');expect(s.turns).toHaveLength(1);
});
test('03 Knowledge setup and source inspection preserve the question and require explicit retrieval',async({page})=>{
  const s=await workspace(page);s.knowledge=false;await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Use the source');
  await page.getByRole('button',{name:'Context & response',exact:true}).click();await page.getByRole('link',{name:'Set up Knowledge'}).click();
  s.knowledge=true;s.documents=[{id:'doc',name:'Journey.txt',source_retained:true,index_status:'ready'}];
  await page.getByRole('button',{name:'Return to your draft'}).click();await nav(page,'knowledge');
  await page.getByRole('button',{name:'View source',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('Journey source.');await page.keyboard.press('Escape');
  await page.getByRole('button',{name:'Ask using Knowledge',exact:true}).click();await expect(page.locator('.composer-retrieval-scope')).toContainText('not just one selected document');
  expect(s.writes).toEqual([]);await page.getByRole('button',{name:'Send',exact:true}).click();await expect.poll(()=>s.turns.length).toBe(1);expect(s.turns[0].use_knowledge_base).toBe(true);
});
test('04 synthetic dictation is editable, chosen speech models persist and playback can stop',async({page})=>{
  const s=await workspace(page);
  await page.addInitScript(()=>{
    Object.defineProperty(navigator,'mediaDevices',{value:{getUserMedia:async()=>({getTracks:()=>[{stop(){(window as any).trackStopped=true;}}]})}});
    class Recorder {static isTypeSupported(){return false;} state='inactive';mimeType='audio/wav';ondataavailable:any;onstop:any;start(){this.state='recording';}stop(){this.state='inactive';this.ondataavailable?.({data:new Blob(['synthetic'],{type:'audio/wav'})});this.onstop?.();}}
    (window as any).MediaRecorder=Recorder;
    const players=new Set<HTMLMediaElement>();
    HTMLMediaElement.prototype.play=function(){players.add(this);(window as any).playing=players.size>0;return Promise.resolve();};
    HTMLMediaElement.prototype.pause=function(){players.delete(this);(window as any).playing=players.size>0;};HTMLMediaElement.prototype.load=function(){};
  });
  const wav=Buffer.alloc(364);wav.write('RIFF');wav.writeUInt32LE(356,4);wav.write('WAVEfmt ',8);wav.writeUInt32LE(16,16);wav.writeUInt16LE(1,20);wav.writeUInt16LE(1,22);wav.writeUInt32LE(16000,24);wav.writeUInt32LE(32000,28);wav.writeUInt16LE(2,32);wav.writeUInt16LE(16,34);wav.write('data',36);wav.writeUInt32LE(320,40);
  await page.route('**/v1/audio/speech',r=>r.fulfill({contentType:'audio/wav',body:wav}));
  await page.goto('/ui/#/chat');await page.getByRole('button',{name:'Voice settings',exact:true}).click();
  await page.getByLabel('Recognition model',{exact:true}).selectOption('asr@r1');await page.getByLabel('Speech model',{exact:true}).selectOption('tts@r1');await page.keyboard.press('Escape');
  await page.getByRole('button',{name:'Use microphone',exact:true}).click();await page.locator('.voice-input-control[data-capture-phase="recording"] button').click();
  await expect(page.locator('.composer textarea')).toHaveValue('Dictated draft.');expect(s.turns).toEqual([]);
  await page.getByText('Speak responses',{exact:true}).click();await expect(page.getByRole('checkbox',{name:'Speak responses',exact:true})).toBeChecked();await send(page,'Edited dictated draft.');
  await expect.poll(()=>page.evaluate(()=>(window as any).playing)).toBe(true);await page.getByRole('button',{name:'Stop reading',exact:true}).first().click();
  await expect.poll(()=>page.evaluate(()=>(window as any).playing)).toBe(false);expect(s.turns).toHaveLength(1);expect(await page.evaluate(()=>(window as any).trackStopped)).toBe(true);
  await nav(page,'agents');await page.getByRole('button',{name:'Voice settings',exact:true}).click();await expect(page.getByLabel('Recognition model',{exact:true})).toHaveValue('asr@r1');
});
test('05 ordinary task creates once and exposes saved output with an integrity-checked artifact',async({page})=>{
  const s=await workspace(page);await start(page);await expect(page.getByRole('heading',{name:'Saved answer'})).toBeVisible();
  const downloaded=page.waitForEvent('download');await page.getByRole('button',{name:'Download: result.txt'}).click();
  const stream=await(await downloaded).createReadStream();const chunks:Buffer[]=[];for await(const chunk of stream!)chunks.push(chunk);expect(Buffer.concat(chunks).toString()).toBe(output);
  expect(s.writes.map(w=>w.path)).toEqual(['/api/v2/jobs']);
});
test('06 application access is scoped, followed by a separate exact-action confirmation',async({page})=>{
  const s=await workspace(page);Object.assign(s.run,{status:'waiting_for_input',pending_input:{id:'input-1',kind:'computer',mode:'app',target:'Editor'},pending_approval:{id:'approval-1',tool:'write_file',arguments:{path:'/selected/result.txt'},expires_at:'2099-01-01T00:00:00Z'}});
  await page.addInitScript(()=>Object.assign(window,{electron:{isDesktop:true,platform:'win32',getApiUrl:async()=>location.origin,getVersion:async()=>'journey',getServerStatus:async()=>true,getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},getComputerStatus:async()=>({state:'idle',installed:true}),discoverComputerApps:async()=>({state:'selecting',targets:[{id:'opaque',title:'Editor',driver:'windows-uia'}]}),startComputerApp:async(request:unknown)=>{(window as any).grant=request;return{state:'ready',target:{id:'session',origin:'Editor'}};}}}));
  await page.addInitScript(()=>{(window as any).electron.stopComputerAccess=async()=>({state:'stopped'});});
  await start(page);await page.getByRole('button',{name:'Choose an application',exact:true}).click();expect(s.writes).toHaveLength(1);
  await page.getByRole('button',{name:'Allow access and continue'}).click();await expect(page.locator('.approval-card')).toContainText('/selected/result.txt');
  await nav(page,'models');await nav(page,'agents');await page.getByRole('button',{name:'Approve exact call',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Saved answer'})).toBeVisible();expect(s.writes.map(w=>[w.path,w.data.approval_id??w.data.input_id])).toEqual([['/api/v2/jobs',undefined],[`/api/v2/jobs/${id}/input`,'input-1'],[`/api/v2/jobs/${id}/approve`,'approval-1']]);
  expect(await page.evaluate(()=>(window as any).grant)).toMatchObject({workspace:'journey',target:'opaque',requestId:'input-1'});
});
test('07 known active work returns to the same task for pause, steering and stop',async({page})=>{
  const s=await workspace(page);s.run.status='running';await start(page);await nav(page,'models');
  await page.getByRole('button',{name:'Known work',exact:true}).click();await page.getByRole('link',{name:'Prepare the journey result',exact:true}).click();
  await page.getByRole('button',{name:'Pause',exact:true}).click();await page.getByRole('textbox',{name:'Update instruction'}).fill('Use three sections');await page.getByRole('button',{name:'Save instruction'}).click();
  await page.getByRole('button',{name:'Resume',exact:true}).click();await page.getByRole('button',{name:'Stop',exact:true}).click();
  expect(s.writes.map(w=>w.path.split('/').pop())).toEqual(['jobs','pause','steer','resume','cancel']);expect(s.run.instructions[0].text).toBe('Use three sections');
});
test('08 lost acceptance is recovered by saved identity without repeating uncertain effects',async({page})=>{
  const s=await workspace(page);s.lost=true;Object.assign(s.run,{status:'uncertain',uncertain_call_id:'call',uncertain_call:{tool:'external_submission',arguments:{recipient:'Selected recipient'}}});await start(page);
  await nav(page,'activity');await page.getByRole('button',{name:/^Prepare the journey result/}).click();await page.getByRole('link',{name:'Open task',exact:true}).click();
  await expect(page.locator('.approval-card')).toContainText('Outcome unknown');await expect(page.getByRole('button',{name:'Resume',exact:true})).toHaveCount(0);
  await page.locator('.topbar').getByRole('button',{name:'Refresh',exact:true}).click();expect(s.writes.map(w=>w.path)).toEqual(['/api/v2/jobs']);
});
test('09 discovery keeps meaningful variants in one review and transfers only the chosen file',async({page})=>{
  const s=await workspace(page);await page.route('**/v1/search?*',r=>r.fulfill({json:{results:[{id:'fixture/repo',name:'Fixture model',author:'fixture',size_bytes:12288}]}}));
  await page.route('**/v1/search/files?*',r=>r.fulfill({json:{files:[{id:'small',file:'small.gguf',size_bytes:4096,supported:true},{id:'large',file:'large.gguf',size_bytes:8192,supported:true}]}}));
  await page.goto('/ui/#/models');await page.getByRole('tab',{name:'Discover models',exact:true}).click();await page.getByRole('searchbox').fill('fixture');await page.getByRole('button',{name:'Search',exact:true}).click();
  await page.getByRole('button',{name:'Review download',exact:true}).click();
  await page.getByRole('combobox').last().selectOption('large');expect(s.writes).toEqual([]);await page.getByRole('button',{name:'Download',exact:true}).click();
  expect(s.writes).toHaveLength(1);expect(JSON.stringify(s.writes[0].data)).toContain('large.gguf');
});
test('10 interrupted model transfer survives navigation, resumes explicitly and removal targets only that model',async({page})=>{
  const s=await workspace(page);const op:any={id:'operation',state:'interrupted',bytes_done:1024,bytes_total:4096,retained_bytes:1024,artifacts:[],provenance:{repository:'fixture/speech',revision:'pinned'},target:{kind:'package',package:{id:'speech',revision:'r1',name:'Speech',capabilities:['transcription']}}};let removed=false;
  await page.route('**/api/v2/models?*',r=>r.fulfill({json:{models:removed?[]:[{id:'speech',revision:'r1',name:'Speech',kind:'package',category:'speech_recognition',provenance:{kind:'import'},readiness:{},package:{installed:true,integrity:'checked',manifest:{artifacts:[],runtime:{adapter:'fixture'}}}}]}}));
  await page.route('**/api/v2/models/operations',r=>r.fulfill({json:{operations:[op]}}));
  await page.route('**/api/v2/models/operations/operation/resume',r=>{s.writes.push({path:'resume',data:{}});op.state='complete';return r.fulfill({json:op});});
  await page.route('**/api/v2/models/packages/speech/r1/remove',r=>{removed=true;s.writes.push({path:'remove',data:{}});return r.fulfill({json:{removed:true}});});
  await page.goto('/ui/#/models');await page.getByRole('button',{name:'Speech recognition',exact:true}).click();await nav(page,'chat');await nav(page,'models');
  await page.getByRole('button',{name:'Resume',exact:true}).click();await page.locator('.package-card').getByRole('button',{name:'Delete',exact:true}).click();expect(removed).toBe(false);
  await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();expect(removed).toBe(true);expect(s.writes.map(w=>w.path)).toEqual(['resume','remove']);expect(s.models).toBe(true);
});
test('11 document import, source inspection, disabled indexing and explicit deletion form one journey',async({page})=>{
  const s=await workspace(page);await page.goto('/ui/#/knowledge');await page.locator('input[type=file]').setInputFiles({name:'Journey.txt',mimeType:'text/plain',buffer:Buffer.from('Journey source.')});
  await expect(page.getByRole('heading',{name:'Journey.txt'})).toBeVisible();await page.getByRole('button',{name:'View source',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('Journey source.');await page.keyboard.press('Escape');
  await nav(page,'chat');s.knowledge=false;await nav(page,'knowledge');await page.getByRole('button',{name:'Manage document: Journey.txt'}).click();await expect(page.getByRole('button',{name:'Reindex',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'Delete',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();expect(s.documents).toEqual([]);expect(s.writes.map(w=>w.path)).toEqual(['/v1/documents/ingest','/v1/documents/delete']);
});
test('12 Activity opens canonical work; history deletion and preferences preserve an unrelated draft',async({page})=>{
  const s=await workspace(page);s.run.prompt='Completed journey';await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Keep my private draft');await nav(page,'activity');
  await page.getByRole('button',{name:/^Completed journey/}).click();await page.getByRole('link',{name:'Open task',exact:true}).click();await expect(page).toHaveURL(new RegExp(id));await nav(page,'agents');
  await page.getByRole('button',{name:'Delete task: Completed journey',exact:true}).click();await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();
  await nav(page,'settings');await page.getByRole('button',{name:'Dark',exact:true}).click();await expect(page.locator('html')).toHaveAttribute('data-theme','dark');await nav(page,'chat');await expect(page.locator('.composer textarea')).toHaveValue('Keep my private draft');
  expect(s.run.prompt).toBe('');expect(s.writes.map(w=>w.path)).toEqual([`/api/v2/jobs/${id}`]);
});
