import { expect, test, type Page } from '@playwright/test';
import { agentProfilesText } from '../src/i18n/agent-profiles';
import { conversationWorkText } from '../src/i18n/conversation-work';
import type { LocaleCode } from '../src/i18n';

const jobID = `run-${'b'.repeat(32)}`, model = 'local-chat-model';
const profile = { id:'writer', revision:1, actor:'local', workspace:'chat-agent-fixture', digest:'fixture', archived:false, created_at:'2026-10-13T00:00:00Z', settings:{ name:'Writer', description:'Write with evidence', model, max_iterations:10, instructions:'Keep it clear.', tools:[], knowledge:[], computer:false } };

async function fixture(page: Page, startStatus?: string, computer = false) {
  await page.addInitScript(() => { if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale','en'); localStorage.setItem('offgrid.onboarding.complete','true'); });
  let workspace='chat-agent-fixture', actor='', selectedProfile:any={...profile,settings:{...profile.settings,computer}};
  let status = startStatus ?? 'running', requestID = 'saved-turn-0001', prompt = 'Prepare an outline.', supported = true;
  let conversation: any = startStatus ? { name:'Existing work', model_id:model, messages:[], jobs:[{turn_id:requestID,job_id:jobID}], created_at:'2026-10-13T00:00:00Z', updated_at:'2026-10-13T00:00:00Z' } : null;
  let pending: any[] = [], extra: any = {}, cursor = 1;
  const submissions: any[] = [], commands: any[] = [], deletes: string[] = [];
  const run = () => ({ run_id:jobID, task_id:jobID, prompt, model, status, steps:[], output:status === 'completed' ? 'A saved, verified outline.' : '', can_steer:status === 'running', event_cursor:String(cursor), pending_instructions:pending, ...extra });
  const turn = () => ({ id:requestID, prompt, model, profile:'interactive', status, output:run().output, job_id:jobID, job_admission:'linked', agent:{id:profile.id,revision:profile.revision} });
  const commit = () => { conversation.messages = [{role:'user',content:prompt,turn_id:requestID,job_id:jobID}, {role:'assistant',content:run().output,turn_id:requestID,job_id:jobID}]; };
  await page.route('**/health', route => route.fulfill({json:{status:'healthy'}}));
  await page.route(/\/(?:api\/v2|v1)\//, async route => {
    const req = route.request(), url = new URL(req.url()), path = decodeURIComponent(url.pathname);
    let data: any = {};
    if (path === '/api/v2/system') data = { product:'offgrid',api_version:2,version:'fixture',workspace_id:workspace,capabilities:['agent-profiles-v1','task-first-agents-v2','durable-agent-events-v2', ...(supported ? ['agent-conversations-v1'] : [])] };
    else if (path === '/v1/system/config') data = {require_auth:false,version:'fixture'};
    else if (path === '/v1/users/me') data = actor ? {user:{id:actor,username:actor,role:'admin'},authenticated:true,auth_required:true,guest:false} : {user:null,authenticated:false,guest:true};
    else if (path === '/v1/models') data = {data:[{id:model,type:'chat',context_window:8192}]};
    else if (path === '/v1/rag/status') data = {enabled:true};
    else if (path === '/v1/audio/status') data = {tts:{available:true}};
    else if (path === '/api/v2/agent-profiles') data = {profiles:[selectedProfile]};
    else if (path === '/api/v2/jobs' || path === '/v1/agents/tasks') data = startStatus ? [{...run(),id:jobID,created_at:'2026-10-13T00:00:00Z',deletable:true}] : [];
    else if (path === '/api/v2/computer/sessions') data = {sessions:[]};
    else if (path === '/v1/sessions') {
      if (req.method() === 'POST') { const body=req.postDataJSON(); conversation = {name:body.name, model_id:model, messages:[],jobs:[],created_at:'2026-10-13T00:00:00Z',updated_at:'2026-10-13T00:00:00Z'}; data=conversation; }
      else data = {sessions:conversation ? [conversation] : []};
    } else if (path.startsWith('/v1/sessions/')) {
      if (path.endsWith('/generate')) {
        const body = req.postDataJSON(); submissions.push(body); requestID=body.request_id; prompt=body.content;
        conversation.jobs=[{turn_id:requestID,job_id:jobID}];
        return route.fulfill({status:202,json:{turn:turn(),job:run()}});
      }
      if (path.endsWith('/turn/recover')) { commands.push({action:'recover'}); commit(); data={turn:turn(),job:run()}; }
      else if (path.endsWith('/turn')) data = {turn:conversation?.jobs.length ? turn() : null};
      else if (req.method() === 'DELETE') { deletes.push(url.searchParams.get('work') ?? ''); if (!url.searchParams.has('work') && !['completed','failed','cancelled'].includes(status)) return route.fulfill({status:409,json:{error:'Active work requires an explicit choice'}}); conversation=null; data={success:true}; }
      else data = conversation;
    } else if (path.startsWith(`/api/v2/jobs/${jobID}`)) {
      if (req.method() === 'POST') {
        const action = path.split('/').pop()!, body = req.postDataJSON(); commands.push({action,...body});
        if (action === 'steer') pending=[{request_id:body.request_id,text:body.instruction,at:'2026-10-13T00:00:00Z'}];
        if (action === 'cancel') status='cancelled';
        if (action === 'pause') status='interrupted';
        if (action === 'input') { status='running'; extra={}; }
        cursor++; data=run();
      } else if (path.endsWith('/events')) return route.fulfill({contentType:'text/event-stream',body:`data: ${JSON.stringify({...run(),type:'snapshot'})}\n\n`});
      else data=run();
    }
    return route.fulfill({json:data});
  });
  return { submissions, commands, deletes, setMessages:(messages:any[])=>{if(conversation)conversation.messages=messages;}, setStatus:(value:string,fields:any={})=>{status=value;extra=fields;cursor++;}, complete:(saved=true)=>{status='completed';cursor++;if(saved)commit();}, unsupported:()=>{supported=false;}, profile:(settings:any)=>{selectedProfile={...selectedProfile,settings:{...selectedProfile.settings,...settings}};}, identity:(nextWorkspace:string,nextActor='')=>{workspace=nextWorkspace;actor=nextActor;conversation=null;} };
}

test('Chat follows growing activity without scrolling the page or stealing a manual scroll-up', async ({page}) => {
  const f=await fixture(page,'completed');
  f.setMessages(Array.from({length:18},(_,i)=>({role:i%2?'assistant':'user',content:`Message ${i}: ${'content '.repeat(30)}`})));
  await page.setViewportSize({width:1280,height:800});
  await page.goto('/ui/#/chat');
  await expect(page.getByText('Message 17:',{exact:false})).toBeAttached();
  const pane=page.locator('.conversation');
  await expect.poll(()=>pane.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeLessThan(2);
  await page.evaluate(()=>{
    const notice=document.createElement('aside'); notice.className='computer-activity'; notice.textContent='Application connected'; notice.style.height='96px';
    document.querySelector('.workspace > .topbar')?.after(notice);
  });
  await expect.poll(()=>pane.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeLessThan(2);
  expect(await page.evaluate(()=>document.documentElement.scrollHeight-document.documentElement.clientHeight)).toBeLessThanOrEqual(1);
  expect(await pane.evaluate(el=>getComputedStyle(el).scrollbarWidth)).toBe('none');
  await pane.hover(); await page.mouse.wheel(0,-1500);
  await expect.poll(()=>pane.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeGreaterThan(1000);
  const pausedAt=await pane.evaluate(el=>el.scrollTop);
  await page.evaluate(()=>{
    const growth=document.createElement('div');growth.style.height='220px';growth.textContent='New activity details';
    document.querySelector('.conversation-content')?.append(growth);
  });
  await expect.poll(()=>pane.evaluate(el=>el.scrollTop)).toBe(pausedAt);
  await pane.hover(); await page.mouse.wheel(0,5000);
  await expect.poll(()=>pane.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeLessThan(2);
  await page.evaluate(()=>{
    const growth=document.createElement('div');growth.style.height='220px';growth.textContent='More activity details';
    document.querySelector('.conversation-content')?.append(growth);
  });
  await expect.poll(()=>pane.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeLessThan(2);
});

test('Chat pins the selected profile, steers the same job, and reload is observation-only', async ({ page }) => {
  const f = await fixture(page);
  await page.goto('/ui/#/chat');
  const selector = page.getByRole('button',{name:'Agent',exact:true});
  await selector.click();
  await page.getByRole('radio',{name:/Writer Write with evidence/}).check();
  await page.keyboard.press('Escape');
  await expect(selector).toBeFocused();
  await page.locator('.composer textarea').fill('Prepare an outline.');
  await page.locator('.composer-actions').getByRole('button',{name:'Send',exact:true}).click();
  await expect(page.getByRole('region',{name:'Agent activity'})).toContainText('Running');
  expect(f.submissions).toHaveLength(1);
  expect(f.submissions[0]).toMatchObject({agent:{id:'writer',revision:1},model_id:model});
  await page.locator('.composer textarea').fill('Use three sections.');
  await page.getByRole('button',{name:'Send instruction',exact:true}).click();
  await expect(page.getByRole('region',{name:'Agent activity'})).toContainText('Instruction queued');
  expect(f.commands[0]).toMatchObject({action:'steer',instruction:'Use three sections.'});
  await page.reload();
  await expect(page.getByRole('region',{name:'Agent activity'})).toContainText('Running');
  expect(f.submissions).toHaveLength(1); expect(f.commands).toHaveLength(1);
  await expect(page.getByRole('heading',{name:'Prepare an outline.',exact:true})).toHaveCount(0);
  f.complete();
  await page.reload();
  await expect(page.getByText('A saved, verified outline.',{exact:true})).toHaveCount(1);
  await expect(page.getByRole('textbox',{name:/outcome/i})).toHaveCount(0);
  expect(f.commands).toHaveLength(1); expect(f.submissions).toHaveLength(1);
});

test('Chat exposes specialist handoffs without expanding technical payloads', async ({page}) => {
  const f=await fixture(page,'waiting_for_children');
  f.setStatus('waiting_for_children',{children:[
    {id:`run-${'c'.repeat(32)}`,spec:{key:'research',goal:'Research sources',depends_on:[],tools:[],profile_id:'researcher'}},
    {id:`run-${'d'.repeat(32)}`,spec:{key:'draft',goal:'Draft the document',depends_on:['research'],tools:[],profile_id:'writer'}},
  ]});
  await page.goto('/ui/#/chat');
  const activity=page.getByRole('region',{name:'Agent activity'});
  await expect(activity.getByRole('link',{name:'Research sources'})).toBeVisible();
  await expect(activity.getByRole('link',{name:'Draft the document'})).toBeVisible();
  await expect(activity.locator('.conversation-work-details')).not.toHaveAttribute('open');
  expect(f.submissions).toEqual([]);
});

async function desktopFixture(page: Page, deferred = false) {
  await page.addInitScript(({ deferred }) => {
    const state = { starts:0, enables:0, stops:0, release: (() => {}) as () => void };
    Object.assign(window, { desktopFixture:state, electron:{
      isDesktop:true, platform:'win32', desktopComputerProtocol:3,
      getApiUrl:async()=>location.origin, getVersion:async()=>'fixture', getServerStatus:async()=>true,
      getSystemTheme:async()=>'light', onThemeChange:()=>()=>{}, getComputerStatus:async()=>({state:'idle',installed:true}),
      getDesktopAccess:async()=>({enabled:!deferred,workspace:'chat-agent-fixture'}),
      enableDesktopAccess:async()=>{state.enables++; await new Promise<void>(resolve=>{state.release=resolve;}); return {enabled:true,workspace:'chat-agent-fixture'};},
      startComputerDesktop:async()=>{state.starts++;return {state:'ready',target:{id:'desktop-host',scope:'desktop'}};},
      stopComputerAccess:async()=>{state.stops++;return {state:'stopped'};},
      stopComputerBrowser:async()=>{state.stops++;return {state:'stopped'};},
    }});
  }, {deferred});
}

const desktopInput = {requested_computer_scope:'desktop',requested_computer_approval_mode:'full_task',pending_input:{id:'desktop-input',kind:'computer',mode:'desktop'}};
async function sendWithWriter(page: Page, prompt = 'Open an application.') {
  await page.getByRole('button',{name:'Agent',exact:true}).click();
  await page.getByRole('radio',{name:/Writer Write with evidence/}).check();
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill(prompt);
  await page.locator('.composer-actions').getByRole('button',{name:'Send',exact:true}).click();
}

for (const mode of ['desktop', 'app']) test(`Chat continues a live ${mode} request once with remembered access, without another consent`, async ({page}) => {
  const f=await fixture(page,undefined,true); await desktopFixture(page);
  f.setStatus('waiting_for_input',{...desktopInput,pending_input:{...desktopInput.pending_input,mode,target:'Microsoft Edge'}});
  await page.goto('/ui/#/chat'); await sendWithWriter(page);
  await expect.poll(()=>f.commands.filter(c=>c.action==='input').length).toBe(1);
  expect(await page.evaluate(()=>({starts:(window as any).desktopFixture.starts,enables:(window as any).desktopFixture.enables}))).toEqual({starts:1,enables:0});
  await page.reload(); await expect(page.getByRole('region',{name:'Agent activity'})).toContainText('Running');
  expect(await page.evaluate(()=>(window as any).desktopFixture.starts)).toBe(0);
  expect(f.submissions).toHaveLength(1); expect(f.commands).toHaveLength(1);
});

for (const variant of ['selected-window', 'managed-browser', 'recovery']) test(`remembered Full access does not automatically attach ${variant} work`, async ({page}) => {
  const f=await fixture(page,undefined,true); await desktopFixture(page);
  f.setStatus('waiting_for_input',{
    ...desktopInput,
    requested_computer_scope:variant==='selected-window'?'selected_window':'desktop',
    pending_input:{...desktopInput.pending_input,mode:variant==='managed-browser'?'browser':'app',target:'Microsoft Edge',
      ...(variant==='recovery'?{previous_session:'previous-host'}:{})},
  });
  await page.goto('/ui/#/chat'); await sendWithWriter(page);
  await expect(page.getByRole('region',{name:'Agent activity'})).toContainText('Access needed');
  expect(await page.evaluate(()=>({starts:(window as any).desktopFixture.starts,enables:(window as any).desktopFixture.enables}))).toEqual({starts:0,enables:0});
  expect(f.commands).toEqual([]); expect(f.submissions).toHaveLength(1);
});

test('reopening pending desktop access is observation-only even with remembered Full access', async ({page}) => {
  const f=await fixture(page,'waiting_for_input',true); await desktopFixture(page);
  f.setStatus('waiting_for_input',desktopInput);
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('button',{name:'Allow desktop access',exact:true})).toBeVisible();
  expect(await page.evaluate(()=>(window as any).desktopFixture.starts)).toBe(0);
  expect(f.commands).toEqual([]); expect(f.submissions).toEqual([]);
});

test('a conversational Chat response never requests desktop access', async ({page}) => {
  const f=await fixture(page,undefined,true); await desktopFixture(page);
  f.setStatus('completed',{outcome:'answer_only'});
  await page.goto('/ui/#/chat'); await sendWithWriter(page,'Hello');
  await expect(page.getByRole('region',{name:'Agent activity'})).toBeVisible();
  expect(await page.evaluate(()=>({starts:(window as any).desktopFixture.starts,enables:(window as any).desktopFixture.enables}))).toEqual({starts:0,enables:0});
  expect(f.commands).toEqual([]);
});

test('Stop while Chat waits for consent cannot start input after a late consent reply', async ({page}) => {
  const f=await fixture(page,undefined,true); await desktopFixture(page,true);
  f.setStatus('waiting_for_input',desktopInput);
  await page.goto('/ui/#/chat'); await sendWithWriter(page);
  await expect.poll(()=>page.evaluate(()=>(window as any).desktopFixture.enables)).toBe(1);
  await page.getByRole('region',{name:'Agent activity'}).getByRole('button',{name:'Stop',exact:true}).first().click();
  await expect.poll(()=>f.commands.some(c=>c.action==='cancel')).toBe(true);
  await page.evaluate(()=>(window as any).desktopFixture.release());
  await expect(page.getByRole('region',{name:'Agent activity'})).toContainText('Cancelled');
  expect(await page.evaluate(()=>(window as any).desktopFixture.starts)).toBe(0);
  expect(f.commands.filter(c=>c.action==='input')).toEqual([]);
});

test('lost final persistence attaches the saved result without another submission', async ({ page }) => {
  const f = await fixture(page,'completed');
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('button',{name:'Attach saved result',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Attach saved result',exact:true}).click();
  await expect(page.getByText('A saved, verified outline.',{exact:true})).toHaveCount(1);
  await expect(page.getByRole('button',{name:'Attach saved result',exact:true})).toHaveCount(0);
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([{action:'recover'}]);
});

for (const state of ['pending','waiting_for_children','waiting_for_approval','failed','uncertain','interrupted','cancelled']) test(`inline ${state} work does not dispatch or hide its recovery boundary`, async ({ page }) => {
  const f = await fixture(page,state);
  if(state==='waiting_for_approval') f.setStatus(state,{pending_approval:{id:'approval',tool:'write_file',arguments:{path:'allowed/file'},expires_at:'2099-01-01T00:00:00Z'}});
  if(state==='uncertain') f.setStatus(state,{uncertain_call:{tool:'type_text',arguments:{text:'exact text'}},uncertain_call_id:'call-1'});
  if(state==='failed') f.setStatus(state,{error:'Provider is unavailable. No retry was sent.'});
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('region',{name:'Agent activity'})).toBeVisible();
  if(state==='uncertain') { await expect(page.getByText('An action was sent, but its outcome is not confirmed. It will not be repeated automatically.',{exact:true})).toBeVisible(); await expect(page.getByRole('textbox',{name:'Report what you observed',exact:true})).not.toBeVisible(); }
  await page.reload();
  await expect(page.getByRole('region',{name:'Agent activity'})).toBeVisible();
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([]);
});

test('an old service never silently receives a profile-enabled message as plain Chat', async ({ page }) => {
  const f = await fixture(page);
  await page.goto('/ui/#/chat');
  await page.getByRole('button',{name:'Agent',exact:true}).click();
  await page.getByRole('radio',{name:/Writer Write with evidence/}).check();
  await page.keyboard.press('Escape');
  f.unsupported();
  await page.locator('.composer textarea').fill('Do not fall back.');
  await page.locator('.composer-actions').getByRole('button',{name:'Send',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('does not support agent conversations');
  expect(f.submissions).toEqual([]);
  await expect(page.locator('.composer textarea')).toHaveValue('Do not fall back.');
});

test('agent and Tools selection preserve the draft and dismiss without submitting', async ({page}) => {
  const f=await fixture(page); f.profile({tools:[{provider:'fixture',tool:'read_docs',schema_revision:'1',location:'remote',policy:'read'}]});
  await page.goto('/ui/#/chat');
  const draft=page.locator('.composer textarea'); await draft.fill('Draft — café 日本語');
  const agent=page.getByRole('button',{name:'Agent',exact:true}); await agent.click();
  await page.getByRole('radio',{name:/Writer Write with evidence/}).check();
  await page.keyboard.press('Escape'); await expect(agent).toBeFocused();
  const tools=page.getByRole('button',{name:'Tools and context',exact:true}); await tools.click();
  await page.getByRole('checkbox',{name:/read_docs/}).uncheck();
  await page.keyboard.press('Escape'); await expect(tools).toBeFocused();
  await agent.click(); await draft.click();
  await expect(page.getByRole('region',{name:'Agent',exact:true})).not.toBeVisible();
  await expect(draft).toBeFocused(); await expect(draft).toHaveValue('Draft — café 日本語');
  await draft.dispatchEvent('keydown',{key:'Enter',code:'Enter',keyCode:229,isComposing:true});
  expect(f.submissions).toEqual([]);
  await page.locator('.composer-actions').getByRole('button',{name:'Send',exact:true}).click();
  await expect.poll(()=>f.submissions.length).toBe(1);
  expect(f.submissions[0].restrictions.tools).toEqual([]);
});

test('profile and tool choices do not leak into another workspace or actor',async({page})=>{
  const f=await fixture(page); await page.goto('/ui/#/chat');
  for(const [workspace,actor] of [['other-workspace',''],['other-workspace','second-user']]) {
    await page.getByRole('button',{name:'Agent',exact:true}).click();
    await page.getByRole('radio',{name:/Writer Write with evidence/}).check(); await page.keyboard.press('Escape');
    f.identity(workspace,actor);
    await page.getByRole('button', { name: 'Workspace options', exact: true }).click();
    await page.getByRole('region', { name: 'Workspace options', exact: true }).getByRole('button',{name:'Refresh',exact:true}).click();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button',{name:'Agent',exact:true})).toContainText('No agent');
    await expect(page.getByRole('button',{name:'Tools and context',exact:true})).toHaveCount(0);
  }
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([]);
});

test('localized agent controls fit short screens, scroll and preserve IME in both themes', async ({page},info)=>{
  test.setTimeout(120000);
  const f=await fixture(page,'interrupted');
  f.profile({tools:Array.from({length:18},(_,i)=>({provider:'fixture',tool:`read_${i}`,schema_revision:'1',location:'remote',policy:'read'}))});
  await page.setViewportSize({width:600,height:500}); await page.goto('/ui/#/chat');
  for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as LocaleCode[]) {
    await page.evaluate(locale=>localStorage.setItem('offgrid.locale',locale),locale); await page.reload();
    const copy=agentProfilesText[locale];
    await expect(page.getByRole('region',{name:conversationWorkText[locale].activity,exact:true})).toBeVisible();
    if(locale==='ar') await expect(page.locator('html')).toHaveAttribute('dir','rtl');
    const selector=page.getByRole('button',{name:copy.selector,exact:true}); await selector.click();
    await page.getByRole('radio',{name:/Writer Write with evidence/}).check(); await page.keyboard.press('Escape');
    await expect(selector).toBeFocused();
    const trigger=page.getByRole('button',{name:copy.resources,exact:true});
    for(const theme of ['light','dark']) {
      await page.evaluate(theme=>{document.documentElement.dataset.theme=theme;},theme);
      await trigger.click();
      const panel=page.getByRole('region',{name:copy.resources,exact:true});
      await panel.focus(); await page.keyboard.press('End');
      await expect.poll(()=>panel.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeLessThan(2);
      const box=await panel.boundingBox(); expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x+box!.width).toBeLessThanOrEqual(600); expect(box!.y+box!.height).toBeLessThanOrEqual(500);
      if(locale==='en'||locale==='ar') await page.screenshot({path:info.outputPath(`chat-tools-${locale}-${theme}.png`)});
      await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
    }
    const draft=page.locator('.composer textarea'); await draft.fill('日本語');
    await draft.dispatchEvent('keydown',{key:'Enter',code:'Enter',keyCode:229,isComposing:true});
    await expect(draft).toHaveValue('日本語');
  }
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([]);
});

test('Chat resource popovers keep separated rows, fields and recovery actions', async ({page},info)=>{
  const f=await fixture(page);
  f.profile({knowledge:['brief','notes'],tools:[{provider:'fixture',tool:'read_docs',schema_revision:'1',location:'remote'}],computer:true});
  await page.route('**/v1/documents',route=>route.fulfill({json:{documents:[{id:'brief',name:'Project brief'},{id:'notes',name:'Notes 日本語'}]}}));
  await page.route('**/v1/audio/status',route=>route.fulfill({status:503,json:{error:'Speech service unavailable'}}));
  await page.goto('/ui/#/chat');
  await page.getByRole('button',{name:'Agent',exact:true}).click();
  await page.getByRole('radio',{name:/Writer Write with evidence/}).check();
  await page.keyboard.press('Escape');
  for(const size of [{width:1280,height:800},{width:390,height:600}]) {
    await page.setViewportSize(size);
    for(const theme of ['light','dark']) {
      await page.evaluate(theme=>{document.documentElement.dataset.theme=theme;},theme);
      const trigger=page.getByRole('button',{name:'Tools and context',exact:true});
      await trigger.click();
      const panel=page.getByRole('region',{name:'Tools and context',exact:true});
      const details=panel.locator('details');
      if(!(await details.evaluate(el=>(el as HTMLDetailsElement).open))) await details.locator('summary').click();
      const rows=panel.locator('.agent-resource-options .profile-option');
      await expect(rows).toHaveCount(2);
      const gap=await rows.evaluateAll(rows=>rows[1].getBoundingClientRect().top-rows[0].getBoundingClientRect().bottom);
      expect(gap).toBeGreaterThanOrEqual(12);
      expect(await panel.locator('.agent-resource-options').evaluate(el=>el.getBoundingClientRect().top-el.previousElementSibling!.getBoundingClientRect().bottom)).toBeGreaterThanOrEqual(12);
      await panel.focus(); await page.keyboard.press('End');
      const box=await panel.boundingBox();
      expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x+box!.width).toBeLessThanOrEqual(size.width);
      expect(box!.y+box!.height).toBeLessThanOrEqual(size.height);
      await page.screenshot({path:info.outputPath(`resource-spacing-${size.width}-${theme}.png`)});
      await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
    }
  }
  const voice=page.getByRole('button',{name:'Voice settings',exact:true}); await voice.click();
  const voicePanel=page.getByRole('region',{name:'Voice settings',exact:true});
  const alert=voicePanel.getByRole('alert'); await expect(alert).toBeVisible();
  const gap=await alert.evaluate(el=>el.querySelector('button')!.getBoundingClientRect().top-el.querySelector('small')!.getBoundingClientRect().bottom);
  expect(gap).toBeGreaterThanOrEqual(12);
  await page.keyboard.press('Escape'); await expect(voice).toBeFocused();
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([]);
});

test('partial results and attachments stay inline with collapsed technical payloads',async({page},info)=>{
  const f=await fixture(page,'completed');
  f.setStatus('completed',{verification:{version:1,revision:1,state:'partial',checks:[]},output:'The outline is ready; the requested export is unavailable.',steps:[{id:1,type:'tool',tool_name:'read_docs',tool_result:'technical fixture payload'}],artifacts:[{name:'outline.txt',format:'txt',bytes:12,sha256:'a'.repeat(64),verified:true,check:'stored_bytes_sha256_and_format'}]});
  await page.goto('/ui/#/chat');
  await expect(page.getByText('The outline is ready; the requested export is unavailable.',{exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:/Download.*outline.txt/})).toBeVisible();
  await expect(page.getByText('technical fixture payload',{exact:true})).not.toBeVisible();
  await expect(page.getByRole('textbox',{name:/outcome/i})).toHaveCount(0);
  await expect(page.getByRole('region',{name:'Agent activity'}).locator('header')).not.toContainText('Completed');
  for(const width of [1280,390]) {
    await page.setViewportSize({width,height:720});
    await page.getByRole('button',{name:/Download.*outline.txt/}).scrollIntoViewIfNeeded();
    await page.screenshot({path:info.outputPath(`chat-partial-${width}.png`),fullPage:true});
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }
  expect(f.commands).toEqual([]); expect(f.submissions).toEqual([]);
});

test('speech waits for the parent response and never narrates child activity or reconnect',async({page})=>{
  const f=await fixture(page);const speech:any[]=[];
  await page.route('**/v1/audio/speech',route=>{speech.push(route.request().postDataJSON());return route.fulfill({status:503,json:{error:{message:'Fixture playback unavailable'}}});});
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Voice settings', exact: true }).click();
  await page.getByRole('checkbox',{name:'Speak responses',exact:true}).focus();
  await page.keyboard.press('Space');
  await expect(page.getByRole('checkbox',{name:'Speak responses',exact:true})).toBeChecked();
  f.setStatus('waiting_for_children',{progress:{preview:'PRIVATE child draft'},steps:[{id:1,type:'tool',tool_name:'read_docs',tool_result:'PRIVATE tool payload'}]});
  await sendWithWriter(page);
  await expect(page.getByRole('region',{name:'Agent activity'})).toBeVisible();
  expect(speech).toEqual([]);
  // A five-minute research task must not consume the audio preparation deadline.
  await page.clock.install();await page.clock.fastForward(301000);
  expect(speech).toEqual([]);
  f.complete();
  await page.getByRole('button',{name:'Refresh',exact:true}).first().click();
  await expect.poll(()=>speech.length).toBe(1);
  expect(JSON.stringify(speech[0])).toContain('A saved, verified outline.');
  expect(JSON.stringify(speech)).not.toContain('PRIVATE');
  await page.reload();await expect(page.getByText('A saved, verified outline.',{exact:true})).toHaveCount(1);
  expect(speech).toHaveLength(1);
});

test('Activity keeps saved work but never offers a second task composer', async ({page}) => {
  const f=await fixture(page,'completed');
  await page.goto('/ui/#/agents/workspace');
  await expect(page.locator('.primary-nav a[href="#/activity"]')).toHaveAttribute('aria-current','page');
  await expect(page.locator('.task-composer')).toHaveCount(0);
  await expect(page.getByRole('button',{name:'Start task',exact:true})).toHaveCount(0);
  await expect(page.getByRole('link',{name:'Chat',exact:true}).first()).toBeVisible();
  await page.locator('.task-list-item').filter({hasText:'Prepare an outline.'}).click();
  await expect(page.locator('.task-surface')).toContainText('A saved, verified outline.');
  await page.goto('/ui/#/agents/new');
  await expect(page.locator('.task-composer')).toHaveCount(0);
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([]);
});

test('agent activity and composer controls keep their spacing as work changes',async({page},info)=>{
  const f=await fixture(page,'waiting_for_approval');
  f.setStatus('waiting_for_approval',{pending_approval:{id:'approval',tool:'write_file',arguments:{path:'approved/notes.txt'},canonical_arguments:'reviewed write',expires_at:'2099-01-01T00:00:00Z'},profile:{name:'Writer',revision:1}});
  await page.goto('/ui/#/chat');
  await page.getByRole('button',{name:'Agent',exact:true}).click();
  await page.getByRole('radio',{name:/Writer Write with evidence/}).check();
  await page.keyboard.press('Escape');
  for(const width of [390,600,900]) {
    await page.setViewportSize({width,height:560});
    const region=page.getByRole('region',{name:'Agent activity'});
    await expect(region).toBeVisible();
    await expect(page.locator('.composer-actions')).toBeInViewport();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth-window.innerWidth)).toBeLessThanOrEqual(1);
    const card=await region.boundingBox(), composer=await page.locator('.composer').boundingBox();
    expect(card && composer && card.x >= 0 && card.x + card.width <= width + 1 && composer.x >= 0 && composer.x + composer.width <= width + 1).toBeTruthy();
    const approvalActions=region.locator('.approval-card .button-row');
    expect(await approvalActions.evaluate(el=>getComputedStyle(el).flexWrap)).toBe('wrap');
    expect(await approvalActions.evaluate(el=>parseFloat(getComputedStyle(el).gap))).toBeGreaterThanOrEqual(8);
    await page.screenshot({path:info.outputPath(`agent-spacing-${width}.png`)});
  }
  expect(f.submissions).toEqual([]); expect(f.commands).toEqual([]);
});

test('response actions align and agent notices, results and steps share readable spacing',async({page},info)=>{
  const f=await fixture(page,'failed');
  f.setMessages([{role:'assistant',content:'## Previous result\n\nA clear answer with **evidence**.',job_id:`run-${'a'.repeat(32)}`}]);
  f.setStatus('failed',{
    output:'## What happened\n\nThe document is still available.\n\n- No edit was repeated.\n- Review the result before continuing.',
    error:'The selected application could not be inspected. Your earlier result is retained; refresh to check its saved state before taking another action.',
    context:{estimated_tokens:1495,window:8192,archived_messages:0},
    steps:[{id:1,type:'tool',tool_name:'read_document',tool_result:'A recorded observation, not a new action.'}],
  });
  await page.goto('/ui/#/chat');
  for(const width of [1280,390]) {
    await page.setViewportSize({width,height:900});
    for(const theme of ['light','dark']) {
      await page.evaluate(theme=>{document.documentElement.dataset.theme=theme;},theme);
      const actions=page.locator('.message-actions').first();
      await actions.scrollIntoViewIfNeeded();
      // Capture all geometry in one browser turn. Follow-latest can change the
      // scroll offset between separate boundingBox calls; comparing those would
      // report misalignment even when the controls share the same row.
      const positions=await actions.locator(':scope > button, :scope > a').evaluateAll(elements=>elements.map(el=>{const box=el.getBoundingClientRect();return {tag:el.tagName,label:el.getAttribute('aria-label'),text:el.textContent?.trim(),top:box.top,bottom:box.bottom,left:box.left,right:box.right};}));
      expect(positions.length).toBeGreaterThanOrEqual(2);
      const inspect=positions.find(position=>position.tag==='A'&&position.text==='Inspect work');
      const copy=positions.find(position=>position.tag==='BUTTON'&&position.label==='Copy response');
      expect(inspect).toBeDefined(); expect(copy).toBeDefined();
      if(width===1280) expect(Math.abs(inspect!.top-copy!.top)).toBeLessThan(1);
      for(let i=1;i<positions.length;i++) {
        const a=positions[i-1],b=positions[i];
        if(Math.abs(a.top-b.top)<1) expect(b.left-a.right).toBeGreaterThanOrEqual(8);
        else expect(b.top-a.bottom).toBeGreaterThanOrEqual(8);
      }
      await page.screenshot({path:info.outputPath(`response-actions-${width}-${theme}.png`)});
      const region=page.getByRole('region',{name:'Agent activity'});
      const result=region.locator('.task-result .message-body');
      await expect(result).toBeVisible();
      expect(await result.evaluate(el=>parseFloat(getComputedStyle(el).lineHeight))).toBeGreaterThanOrEqual(24);
      const notice=region.getByRole('alert');
      const gap=await notice.evaluate(el=>{const text=el.firstElementChild!.getBoundingClientRect(),action=el.lastElementChild!.getBoundingClientRect();return action.top>=text.bottom? action.top-text.bottom:action.left-text.right;});
      expect(gap).toBeGreaterThanOrEqual(12);
      const activity=region.locator('.conversation-work-details');
      if(!await activity.evaluate(el=>(el as HTMLDetailsElement).open)) await activity.locator(':scope > summary').click();
      const context=activity.locator('details').filter({has:page.locator('summary',{hasText:'Context usage'})}).first();
      if(!await context.evaluate(el=>(el as HTMLDetailsElement).open)) await context.locator('summary').click();
      expect(await context.locator('button').evaluate(el=>el.getBoundingClientRect().top-el.previousElementSibling!.getBoundingClientRect().bottom)).toBeGreaterThanOrEqual(12);
      const step=activity.locator('details').filter({has:page.locator('summary',{hasText:'read_document'})});
      if(!await step.evaluate(el=>(el as HTMLDetailsElement).open)) await step.locator('summary').click();
      expect(await step.locator('pre').evaluate(el=>el.getBoundingClientRect().top-el.previousElementSibling!.getBoundingClientRect().bottom)).toBeGreaterThanOrEqual(12);
      await step.scrollIntoViewIfNeeded();
      expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
      await page.screenshot({path:info.outputPath(`response-details-${width}-${theme}.png`)});
    }
  }
  expect(f.commands).toEqual([]); expect(f.submissions).toEqual([]);
});
