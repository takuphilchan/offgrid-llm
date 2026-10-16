import { expect, test, type Page } from '@playwright/test';
import { taskAccessText } from '../src/i18n/task-access';
import { approvalPolicyText } from '../src/i18n/native-app';
import { taskDesktopText } from '../src/i18n/task-desktop';
import { taskSessionAccessText } from '../src/i18n/task-session-access';
import type { LocaleCode } from '../src/i18n';
import { installChatJob, openAgentChat, selectTaskAgent, sendAgentMessage } from './fixtures/chat-job';

const id = `run-${'c'.repeat(32)}`;
async function setup(page: Page, { lost = false, answer = false, desktop = false, deferredAccess = false } = {}) {
  const submissions: any[] = [];
  let policy: string | undefined;
  let computerScope: string | undefined;
  let attached=false;
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale', 'en');
    Object.assign(window, { electron: {
      isDesktop: true, platform: 'win32', getApiUrl: async () => location.origin,
      getVersion: async () => 'test', getServerStatus: async () => true, getSystemTheme: async () => 'light', onThemeChange: () => () => {},
      discoverComputerApps: async () => ({ state: 'selecting', targets: [{ id: 'opaque', title: 'Chrome', driver: 'windows-uia' }] }),
      getComputerStatus: async () => ({ state: 'idle', installed: true }),
      stopComputerAccess: async () => ({ state: 'stopped' }),
      startComputerApp: async (request: unknown) => { (window as any).hostRequest = request; return { state: 'ready', target: { id: 'host', origin: 'Chrome' } }; },
    }});
  });
  if(desktop)await page.addInitScript(()=>{
    Object.assign(window.electron!,{desktopComputerProtocol:3,startComputerDesktop:async(request:unknown)=>{(window as any).desktopRequests=[...((window as any).desktopRequests??[]),request];return {state:'ready',target:{id:'desktop-host',scope:'desktop'}};}});
  });
  const snapshot = () => ({ run_id: id, task_id: id, prompt: 'Open Chrome', model: 'test-model', status: answer ? 'completed' : 'waiting_for_input', steps: [], output: answer ? 'Please open Chrome yourself.' : '', pending_approval: null, resumable: false, requested_computer_approval_mode: policy,
    outcome: answer ? 'answer_only' : undefined, requested_computer_scope:computerScope,
    pending_input: answer || attached ? null : { id: 'input-1', call_id: desktop?'':'call-1', kind: 'computer', mode: desktop?'desktop':'app', target: 'Chrome' } });
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, async route => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = {};
    if (path === '/api/v2/system') data = { product: 'offgrid', api_version: 2, version: 'test', workspace_id: 'policy-test', capabilities: ['task-first-agents-v2', 'durable-agent-events-v2'] };
    else if (path === '/v1/system/config') data = { require_auth: false };
    else if (path === '/v1/users/me') data = { user: null, authenticated: false, guest: true };
    else if (path === '/v1/models') data = { data: [{ id: 'test-model', type: 'chat', context_window: 8192 }] };
    else if (path === '/api/v2/jobs' && route.request().method() === 'POST') {
      const body = route.request().postDataJSON(); submissions.push(body); policy = body.computer_approval_mode;computerScope=body.computer_scope;
      if (lost && submissions.length === 1) return route.abort('failed');
      return route.fulfill({ status: 202, json: deferredAccess ? {...snapshot(),status:'running',pending_input:null} : snapshot() });
    } else if(path===`/api/v2/jobs/${id}/input`){attached=true;return route.fulfill({json:{...snapshot(),status:'running'}});}
    else if (path === '/api/v2/jobs' || path === '/v1/agents/tasks') data = answer ? [{ id, prompt: 'Open Chrome', status: 'completed', outcome: 'answer_only', created_at: '2026-10-09T12:00:00Z', deletable: true }] : [];
    else if (path === `/api/v2/jobs/${id}/events`) return route.fulfill({ contentType: 'text/event-stream', body: `data: ${JSON.stringify({ ...snapshot(), type: 'snapshot' })}\n\n` });
    else if (path === `/api/v2/jobs/${id}`) data = snapshot();
    else if (path === '/api/v2/computer/sessions') data = { sessions: [] };
    else if (path === '/v1/catalog') data = { models: [] };
    else if (path === '/v1/sessions') data = { sessions: [] };
    return route.fulfill({ json: data });
  });
  await installChatJob(page, {workspace:'policy-test',model:'test-model',snapshot,submit:body=>{
    submissions.push(body); policy=body.computer_approval_mode; computerScope=body.computer_scope;
    if(lost && submissions.length===1)return 'lost';
    return deferredAccess ? {...snapshot(),status:'running',pending_input:null} : snapshot();
  }});
  return submissions;
}

test('conversation with desktop available does not enable access or start a host task',async({page})=>{
  const submissions=await setup(page,{desktop:true,answer:true});
  await page.addInitScript(()=>Object.assign(window.electron!,{
    getDesktopAccess:async()=>({enabled:false,workspace:'policy-test'}),
    enableDesktopAccess:async()=>{(window as any).unexpectedConsent=true;return {enabled:true,workspace:'policy-test'};},
  }));
  await openAgentChat(page);
  await sendAgentMessage(page,'hello');
  await expect.poll(()=>submissions.length).toBe(1);
  await expect(page.locator('.conversation')).toContainText('Please open Chrome yourself.');
  expect(await page.evaluate(()=>({consent:(window as any).unexpectedConsent??false,requests:(window as any).desktopRequests??[]}))).toEqual({consent:false,requests:[]});
});

test('a live Start continues on-demand desktop access once; reopening does not',async({page})=>{
  await setup(page,{desktop:true,deferredAccess:true});
  await openAgentChat(page);
  await sendAgentMessage(page,'Open Chrome');
  await expect.poll(()=>page.evaluate(()=>(window as any).desktopRequests?.length??0)).toBe(1);
  await page.reload();
  await expect(page.locator('.conversation')).toContainText('Open Chrome');
  expect(await page.evaluate(()=>(window as any).desktopRequests??[])).toEqual([]);
});

test('desktop Start saves once then requests task consent without application selection',async({page})=>{
 const submissions=await setup(page,{desktop:true});await openAgentChat(page);
 await page.getByRole('button',{name:'Computer access',exact:true}).click();
 await expect(page.getByRole('combobox',{name:'Access scope',exact:true})).toHaveCount(0);
 await expect(page.getByRole('region',{name:'Computer access',exact:true})).toContainText('open and switch supported applications');
 await page.keyboard.press('Escape');
 await sendAgentMessage(page,'Open Chrome');
 await expect.poll(()=>page.evaluate(()=>(window as any).desktopRequests?.length??0)).toBe(1);
 expect(submissions).toHaveLength(1);expect(submissions[0].computer_scope).toBe('desktop');expect(submissions[0].computer_approval_mode).toBe('full_task');
 await expect(page.getByRole('button',{name:'Choose an application',exact:true})).toHaveCount(0);
 await page.reload();await expect(page.getByRole('button',{name:'Choose an application',exact:true})).toHaveCount(0);
 expect(await page.evaluate(()=>(window as any).desktopRequests??[])).toEqual([]);
});

test('Full access is an explicit host grant, reused by Start and revocable before another task',async({page})=>{
 let enabled=false,enableCalls=0,stops=0;
 const submissions=await setup(page,{desktop:true});
 await page.exposeFunction('accessStatus',()=>({enabled,workspace:'policy-test'}));
 await page.exposeFunction('accessEnable',()=>{enabled=true;enableCalls++;return {enabled,workspace:'policy-test'};});
 await page.exposeFunction('accessStop',()=>{enabled=false;stops++;return {state:'stopped'};});
 await page.addInitScript(()=>Object.assign(window.electron!,{getDesktopAccess:()=> (window as any).accessStatus(),enableDesktopAccess:()=> (window as any).accessEnable(),stopComputerBrowser:()=> (window as any).accessStop()}));
 await openAgentChat(page);
 await page.getByRole('button',{name:'Computer access',exact:true}).click();
 await expect(page.getByRole('status').filter({hasText:'Full access is not enabled'})).toBeVisible();
 expect(enableCalls).toBe(0);
 const full=page.getByRole('radio',{name:/Full task access/});
 await expect(full).not.toBeChecked();
 await expect(page.getByRole('button',{name:'Enable Full access',exact:true})).toHaveCount(0);
 await full.click();
 await expect(full).toBeChecked();
 expect(enableCalls).toBe(1);
 await page.keyboard.press('Escape');
 await sendAgentMessage(page,'Open Chrome');
 await expect.poll(()=>submissions.length).toBe(1);
 await expect.poll(()=>page.evaluate(()=>(window as any).desktopRequests?.length??0)).toBe(1);
 expect(enableCalls).toBe(1);
 await page.goto('/ui/#/chat');await page.reload();await selectTaskAgent(page);
 await page.getByRole('button',{name:'Computer access',exact:true}).click();
 await expect(page.getByRole('radio',{name:/Full task access/})).toBeChecked();
 expect(enableCalls).toBe(1);
 await page.getByRole('radio',{name:/Ask every time/}).click();
 await expect(page.getByRole('radio',{name:/Ask every time/})).toBeChecked();
 await expect.poll(()=>stops).toBe(1);expect(enabled).toBe(false);
});

test('access options scroll with wheel and keyboard without moving the page or resetting position',async({page})=>{
 await setup(page,{desktop:true});
 await page.addInitScript(()=>Object.assign(window.electron!,{getDesktopAccess:async()=>({enabled:false,workspace:'policy-test'}),enableDesktopAccess:async()=>({enabled:true,workspace:'policy-test'})}));
 await page.setViewportSize({width:650,height:420});await openAgentChat(page);
 for(const locale of ['en','ar'] as LocaleCode[]){
  await page.evaluate(locale=>localStorage.setItem('offgrid.locale',locale),locale);await page.reload();await selectTaskAgent(page);
  await page.getByRole('button',{name:taskAccessText(locale).title,exact:true}).click();
  const panel=page.getByRole('region',{name:taskAccessText(locale).title,exact:true});
  await expect.poll(()=>panel.evaluate(el=>el.scrollHeight>el.clientHeight)).toBe(true);
  const before=await page.evaluate(()=>window.scrollY);const bounds=await panel.boundingBox();
  await page.mouse.move(bounds!.x+30,bounds!.y+30);await page.mouse.wheel(0,400);
  await expect.poll(()=>panel.evaluate(el=>el.scrollTop)).toBeGreaterThan(0);
  expect(await page.evaluate(()=>window.scrollY)).toBe(before);
  await panel.focus();await page.keyboard.press('End');
  await expect.poll(()=>panel.evaluate(el=>el.scrollHeight-el.clientHeight-el.scrollTop)).toBeLessThan(2);
  await expect(panel.getByText(taskSessionAccessText(locale).hint,{exact:true})).toBeInViewport();
  await page.keyboard.press('Home');await expect.poll(()=>panel.evaluate(el=>el.scrollTop)).toBe(0);
  await page.keyboard.press('Escape');
 }
});

test('denied Full access remains unchecked and host revocation clears the selection',async({page})=>{
 const submissions=await setup(page,{desktop:true});let enabled=false,allow=false,calls=0;
 await page.exposeFunction('statusFixture',()=>({enabled,workspace:'policy-test'}));
 await page.exposeFunction('enableFixture',()=>{calls++;enabled=allow;return {enabled,workspace:'policy-test'};});
 await page.addInitScript(()=>Object.assign(window.electron!,{getDesktopAccess:()=> (window as any).statusFixture(),enableDesktopAccess:()=> (window as any).enableFixture()}));
 await openAgentChat(page);
 const trigger=page.getByRole('button',{name:'Computer access',exact:true});await trigger.click();
 const full=page.getByRole('radio',{name:/Full task access/});await expect(full).not.toBeChecked();
 await full.click();await expect(page.getByRole('alert')).toContainText('computer_scope_violation');
 await expect(full).not.toBeChecked();expect(calls).toBe(1);expect(submissions).toEqual([]);
 allow=true;await full.click();await expect(full).toBeChecked();expect(calls).toBe(2);
 enabled=false;await expect(full).not.toBeChecked({timeout:5000});await expect(trigger).toHaveText('Computer access');
 expect(submissions).toEqual([]);
});

test('in-app Stop remains available between tasks, across navigation and offline',async({page})=>{
 await setup(page,{desktop:true});let active=true,stops=0;
 await page.exposeFunction('hostState',()=>({state:active?'access_ready':'stopped',active,installed:true}));
 await page.exposeFunction('localStop',()=>{active=false;stops++;return {state:'stopped'};});
 await page.addInitScript(()=>Object.assign(window.electron!,{getComputerStatus:()=> (window as any).hostState(),stopComputerBrowser:()=> (window as any).localStop()}));
 await openAgentChat(page);
 const banner=page.getByRole('complementary',{name:'Full access enabled for this desktop session',exact:true});
 await expect(banner).toBeVisible();
 await page.goto('/ui/#/chat');await expect(banner).toBeVisible();
 await page.route('**/api/v2/computer/**',route=>route.abort('failed'));
 await banner.getByRole('button',{name:'Stop control',exact:true}).click();
 await expect.poll(()=>stops).toBe(1);expect(active).toBe(false);
});

test('retry with the same preselected policy keeps request identity and does not reselect it during access', async ({ page }) => {
  const submissions = await setup(page, { lost: true });
  await openAgentChat(page);
  const trigger = page.getByRole('button', { name: 'Computer access', exact: true });
  await expect(trigger).toHaveText('Full task access');
  await trigger.click();
  const panel = page.getByRole('region', { name: 'Computer access', exact: true });
  await expect(panel).toContainText('Approve once in the desktop dialog');
  await panel.getByRole('radio', { name: /Ask every time/ }).check();
  await page.keyboard.press('Escape');
  await expect(panel).toBeHidden();
  await expect(trigger).toBeFocused();
  await trigger.click();
  // Click an uncovered part of the composer; the panel may open above its
  // anchor and intentionally overlay the middle of the textarea.
  await page.locator('.composer textarea').click({ position: { x: 10, y: 10 } });
  await expect(panel).toBeHidden();
  await sendAgentMessage(page,'Open Chrome');
  await expect(page.getByRole('alert').first()).toBeVisible();
  await page.reload();
  await selectTaskAgent(page);
  // Profile and policy selections are session-memory settings; deliberately
  // restore the same choices before retrying the retained draft.
  await trigger.click();
  await page.getByRole('radio', {name:/Ask every time/}).check();
  await page.keyboard.press('Escape');
  await expect(trigger).toHaveText('Ask every time');
  await page.locator('.composer-actions').getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.locator('.conversation-work')).toContainText('Access needed');
  expect(submissions).toHaveLength(2);
  expect(submissions[0]).toEqual(submissions[1]);
  expect(submissions[0].computer_approval_mode).toBe('ask_every_time');
  const access = page.locator('.task-access');
  await expect(access).toContainText('Policy selected before starting');
  await expect(access.getByRole('combobox', { name: 'Action approvals' })).toHaveCount(0);
  await access.getByRole('button', { name: 'Choose an application', exact: true }).click();
  await access.getByRole('button', { name: 'Allow access and continue', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).hostRequest?.approvalMode)).toBe('ask_every_time');
  expect(submissions).toHaveLength(2);
});

test('a deliberate policy change after failed submission has a different request payload identity', async ({ page }) => {
  const submissions = await setup(page, { lost: true });
  await openAgentChat(page);
  await sendAgentMessage(page,'Open Chrome');
  await expect(page.getByRole('alert').first()).toBeVisible();
  await page.getByRole('button', { name: 'Computer access', exact: true }).click();
  await page.getByRole('radio', { name: /Ask every time/ }).check();
  await page.keyboard.press('Escape');
  await page.locator('.composer-actions').getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => submissions.length).toBe(2);
  expect(submissions[0].request_id).not.toBe(submissions[1].request_id);
  expect(submissions.map(s => s.computer_approval_mode)).toEqual(['full_task', 'ask_every_time']);
});

test('answer-only history and result never claim completed application work', async ({ page }) => {
  await setup(page, { answer: true });
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.work-status')).toContainText('Answered');
  await expect(page.locator('.task-list')).toHaveCount(0);
  await expect(page.locator('.task-result')).toContainText('No external tool action was performed');
  await expect(page.locator('.task-result')).toContainText('Please open Chrome yourself.');
});

test('retired task-composer links lead to Chat without creating or replaying a job', async ({ page }) => {
  const submissions = await setup(page);
  await page.goto('/ui/#/agents/new');
  await expect(page.locator('.task-composer')).toHaveCount(0);
  await expect(page.getByRole('button', {name:'Start task',exact:true})).toHaveCount(0);
  await page.reload();
  await page.locator('.primary-nav').getByRole('link',{name:'Chat',exact:true}).click();
  await expect(page.locator('.composer textarea')).toBeVisible();
  expect(submissions).toEqual([]);
});

test('policy control preserves all locales, RTL, and narrow-screen layout', async ({ page }, info) => {
  const submissions = await setup(page, { desktop: true });
  await page.setViewportSize({ width: 390, height: 640 });
  await openAgentChat(page);
  for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
    await page.evaluate(locale => localStorage.setItem('offgrid.locale', locale), locale);
    await page.reload();
    await selectTaskAgent(page);
    const copy = taskAccessText(locale), approval = approvalPolicyText(locale);
    await page.getByRole('button', { name: copy.title, exact: true }).click();
    const panel = page.getByRole('region', { name: copy.title, exact: true });
    await expect(panel.getByRole('combobox', { name: taskDesktopText(locale).scope, exact: true })).toHaveCount(0);
    await expect(panel.getByRole('radio', { name: approval.full + ' ' + approval.fullHelp, exact: true })).toBeChecked();
    await expect(panel).not.toContainText(/Ã|Ù|Ø/);
    const box = await panel.boundingBox();
    expect(box!.x).toBeGreaterThanOrEqual(0); expect(box!.x + box!.width).toBeLessThanOrEqual(391);
    expect(box!.y).toBeGreaterThanOrEqual(0); expect(box!.y + box!.height).toBeLessThanOrEqual(641);
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
    if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`access-${locale}.png`), fullPage: true });
    await page.keyboard.press('Escape');
    await expect(panel).toBeHidden();
  }
  expect(submissions).toEqual([]);
});
