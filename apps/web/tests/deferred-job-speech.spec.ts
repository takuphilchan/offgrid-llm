import {expect, test, type Page} from '@playwright/test';
import {openAgentChat, sendAgentMessage} from './fixtures/chat-job';

async function fixture(page: Page, delayAdmission = false) {
  const id='run-'+ 'd'.repeat(32), model='speech-model';
  const job: any={run_id:id,task_id:id,prompt:'Finish later',model,status:'waiting_for_input',steps:[],output:'',event_cursor:'1',resumable:false};
  const profile={id:'task-agent',revision:1,actor:'local',workspace:'speech-owner',settings:{name:'Task agent',model,instructions:'',tools:[],knowledge:[],computer:false,max_iterations:10}};
  const session: any={name:'Speech ownership',model_id:model,created_at:'2026-11-22T00:00:00Z',updated_at:'2026-11-22T00:00:00Z',messages:[{role:'assistant',content:'Historical answer.'}]};
  const spoken: string[]=[], writes: string[]=[];
  let submitted=false, accepted=false, release:()=>void=()=>{};
  const admission=new Promise<void>(resolve=>{release=resolve;});
  if(!delayAdmission)release();
  await page.addInitScript(()=>{
    localStorage.setItem('offgrid.onboarding.complete','true'); localStorage.setItem('offgrid.locale','en');
    const f=window as any; f.players=[];
    f.Audio=class extends EventTarget {
      onended: (()=>void)|null=null; onerror: (()=>void)|null=null; src=''; paused=false;
      constructor(){super();this.addEventListener('ended',()=>this.onended?.());}
      play(){f.players.push(this);return Promise.resolve();} pause(){this.paused=true;} load(){} removeAttribute(){this.src='';}
    };
  });
  await page.route('**/health',route=>route.fulfill({json:{status:'healthy'}}));
  await page.route(/\/(?:v1|api\/v2)\//,async route=>{
    const request=route.request(), path=decodeURIComponent(new URL(request.url()).pathname);
    if(request.method()!=='GET')writes.push(path);
    if(path==='/v1/audio/speech'){spoken.push(request.postDataJSON().input);return route.fulfill({contentType:'audio/wav',body:Buffer.alloc(100)});}
    if(path.endsWith('/generate')){
      submitted=true; const payload=request.postDataJSON(); await admission; accepted=true;
      session.turn={id:payload.request_id,prompt:payload.content,model,status:job.status,output:'',job_id:id,job_admission:'linked',agent:{id:profile.id,revision:1}};
      session.jobs=[{turn_id:payload.request_id,job_id:id}];
      session.messages.push({role:'user',content:payload.content,turn_id:payload.request_id});
      return route.fulfill({status:202,json:{turn:session.turn,job}});
    }
    if(path===`/api/v2/jobs/${id}`)return route.fulfill({json:job});
    if(path.endsWith('/turn'))return route.fulfill({json:{turn:session.turn??null}});
    if(path.startsWith('/v1/sessions/'))return route.fulfill({json:session});
    const bodies: Record<string,unknown>={
      '/api/v2/system':{product:'offgrid',api_version:2,workspace_id:'speech-owner',capabilities:['agent-profiles-v1','agent-conversations-v1','task-first-agents-v2','durable-agent-events-v2']},
      '/v1/users/me':{authenticated:false,user:null},'/v1/system/config':{require_auth:false},
      '/v1/models':{data:[{id:model,type:'chat'}]},'/v1/sessions':{sessions:[session]},
      '/api/v2/agent-profiles':{profiles:[profile]},'/api/v2/agent-profiles/resources':{tools:{}},
      '/v1/audio/status':{tts:{available:true}},'/api/v2/jobs':[], '/v1/agents/tasks':[],
      '/api/v2/computer/sessions':{sessions:[]},'/v1/agents/mcp':{servers:[]},'/v1/agents/tools':{tools:[]},
    };
    return route.fulfill({json:bodies[path]??{}});
  });
  await openAgentChat(page);
  await expect(page.getByRole('button',{name:'Read aloud',exact:true})).toBeVisible();
  const settings=page.locator('.composer-context-menu > button'); await settings.click();
  await page.getByRole('checkbox',{name:'Speak responses'}).click();
  await expect(page.getByRole('checkbox',{name:'Speak responses'})).toBeChecked(); await page.keyboard.press('Escape');
  await sendAgentMessage(page,'Finish later');
  await expect.poll(()=>submitted).toBe(true);
  if(!delayAdmission)await expect.poll(()=>accepted).toBe(true);
  return {spoken,writes,release,complete:()=>{
    job.status='completed';job.output='Deferred job answer.';job.event_cursor='2';
    session.turn.status='completed';session.turn.output=job.output;
    session.messages.push({role:'assistant',content:job.output,turn_id:session.turn.id,job_id:id});
  },accepted:()=>accepted};
}

test('manual historical playback invalidates deferred job speech without cancelling the job',async({page})=>{
  const f=await fixture(page);
  await page.getByRole('button',{name:'Read aloud',exact:true}).click();
  await expect.poll(()=>f.spoken).toEqual(['Historical answer.']);
  f.complete();
  await expect(page.locator('.message.assistant').last()).toContainText('Deferred job answer.');
  expect(f.spoken).toEqual(['Historical answer.']);
  expect(await page.evaluate(()=>(window as any).players[0].paused)).toBe(false);
  expect(f.writes.filter(path=>/cancel|stop/.test(path))).toEqual([]);
});

test('uninterrupted deferred intent still speaks the completed job exactly once',async({page})=>{
  const f=await fixture(page); f.complete();
  await expect.poll(()=>f.spoken).toEqual(['Deferred job answer.']);
  await expect(page.locator('.message.assistant').last()).toContainText('Deferred job answer.');
  expect(f.spoken).toEqual(['Deferred job answer.']);
});

for(const boundary of ['audio-stop','dismiss','hidden','toggle-off-on'] as const) test(`${boundary} revokes deferred speech while the job continues`,async({page})=>{
  const f=await fixture(page);
  if(boundary==='toggle-off-on'){
    await page.locator('.composer-context-menu > button').click();
    const toggle=page.getByRole('checkbox',{name:'Speak responses'});
    await toggle.click(); await expect(toggle).not.toBeChecked();
    await toggle.click(); await expect(toggle).toBeChecked(); await page.keyboard.press('Escape');
  }else await page.evaluate(boundary=>{
    if(boundary==='hidden') {Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));Object.defineProperty(document,'hidden',{configurable:true,value:false});document.dispatchEvent(new Event('visibilitychange'));}
    else window.dispatchEvent(new Event(boundary==='dismiss'?'offgrid-desktop-dismiss':'offgrid-stop-read-aloud'));
  },boundary);
  f.complete(); await expect(page.locator('.message.assistant').last()).toContainText('Deferred job answer.');
  expect(f.spoken).toEqual([]);expect(f.writes.filter(path=>/cancel|stop/.test(path))).toEqual([]);
});

for(const boundary of ['manual','dismiss'] as const) test(`late admission cannot recreate speech intent revoked by ${boundary}`,async({page})=>{
  const f=await fixture(page,true);
  if(boundary==='manual'){
    await page.getByRole('button',{name:'Read aloud',exact:true}).click();
    await expect.poll(()=>f.spoken).toEqual(['Historical answer.']);
  }else await page.evaluate(()=>window.dispatchEvent(new Event('offgrid-desktop-dismiss')));
  f.release();await expect.poll(f.accepted).toBe(true);
  f.complete();await expect(page.locator('.message.assistant').last()).toContainText('Deferred job answer.');
  expect(f.spoken).toEqual(boundary==='manual'?['Historical answer.']:[]);
  expect(f.writes.filter(path=>/cancel|stop/.test(path))).toEqual([]);
});

test('navigation revokes deferred speech without stopping or replaying the job',async({page})=>{
  const f=await fixture(page);
  await page.getByRole('link',{name:'Models',exact:true}).click();
  await expect(page).toHaveURL(/#\/models$/);
  f.complete();
  await page.getByRole('link',{name:'Chat',exact:true}).click();
  await expect(page.locator('.message.assistant').last()).toContainText('Deferred job answer.');
  expect(f.spoken).toEqual([]);expect(f.writes.filter(path=>/cancel|stop/.test(path))).toEqual([]);
});
