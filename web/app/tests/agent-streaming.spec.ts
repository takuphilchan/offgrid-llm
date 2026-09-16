import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const started = new Date().toISOString();
  let status = 'running', phase = 'generating', preview = 'Live partial answer 日本語';
  let creates = 0, cancels = 0, streams = 0;
  let disconnect = true;
  const snapshot = () => ({ type: status === 'running' ? 'status' : status === 'completed' ? 'done' : 'error', run_id: 'live-run', task_id:'live-run', status, output: status === 'completed' ? 'Finished answer' : '', steps:[], pending_approval:null, resumable: false, started_at: started, progress:{phase,iteration:1,preview,truncated:false,started_at:started,updated_at:started} });
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.onboarding.complete','true'); });
  await page.route('**/health', r => r.fulfill({json:{status:'healthy'}}));
  await page.route('**/api/v2/system', r => r.fulfill({json:{product:'offgrid',version:'test',api_version:2,capabilities:[]}}));
  await page.route('**/v1/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let json: unknown = {};
    if (path === '/v1/users/me') json = {authenticated:false,user:null};
    else if (path === '/v1/models') json = {data:[{id:'model',type:'chat'}]};
    else if (path === '/v1/agents/run') { creates++; json = snapshot(); }
    else if (path === '/v1/agents/tasks/live-run') json = snapshot();
    else if (path === '/v1/agents/tasks/live-run/events') {
      streams++;
      if (!disconnect) await new Promise(resolve => setTimeout(resolve, 250));
      // EOF while running is a disconnection, not a completed run.
      return route.fulfill({contentType:'text/event-stream', body:`: heartbeat\r\ndata: ${JSON.stringify(snapshot())}\r\n\r\n`});
    } else if (path === '/v1/agents/tasks/live-run/cancel') {cancels++;status='cancelled';json=snapshot();}
    else if (path === '/v1/agents/tasks') json = creates ? [{id:'live-run',prompt:'Do the task',status,created_at:started}] : [];
    else if (path === '/v1/agents/tools') json = {tools:[],enabled_count:0};
    else if (path === '/v1/agents/mcp') json = {servers:[]};
    else if (path === '/v1/integrations') json = {integrations:[]};
    else if (path === '/v1/sessions') json = {sessions:[]};
    await route.fulfill({json});
  });
  return { creates:()=>creates,cancels:()=>cancels,streams:()=>streams, complete:()=>{status='completed';disconnect=false;}, tool:()=>{phase='tool';preview='';} };
}

test('agent preview appears before completion and reconnects without resubmitting', async ({page}) => {
  const state = await fixture(page);
  await page.goto('/ui/#/agents');
  await page.getByLabel('Task', {exact:true}).fill('Do the task');
  await page.getByRole('button',{name:'Run task',exact:true}).click();
  await expect(page.locator('.agent-live-preview')).toContainText('Live partial answer 日本語');
  await expect(page.locator('.agent-live-preview')).toContainText('not a completed answer');
  await expect(page.getByText('The connection is recovering.',{exact:false})).toBeVisible();
  await expect.poll(state.streams).toBeGreaterThan(1);
  expect(state.creates()).toBe(1);
  await page.getByRole('checkbox',{name:'Live response preview'}).uncheck();
  await expect(page.locator('.agent-live-preview')).toHaveCount(0);
  expect(state.cancels()).toBe(0);
  await page.getByRole('checkbox',{name:'Live response preview'}).check();
  await page.getByRole('link',{name:'Chat',exact:true}).click();
  await page.getByRole('link',{name:'Agents',exact:true}).click();
  await expect(page.locator('.agent-live-preview')).toContainText('Live partial answer 日本語');
  expect(state.creates()).toBe(1);
  state.complete();
  await expect(page.locator('.result-card')).toContainText('Finished answer');
  await expect(page.locator('.agent-live-preview')).toHaveCount(0);
});

test('cancel retains a clearly incomplete preview without claiming completion', async ({page}) => {
  const state = await fixture(page);
  await page.goto('/ui/#/agents');
  await page.getByLabel('Task',{exact:true}).fill('Do the task');
  await page.getByRole('button',{name:'Run task',exact:true}).click();
  await expect(page.locator('.agent-live-preview')).toBeVisible();
  await page.getByRole('button',{name:'Cancel',exact:true}).click();
  await expect(page.locator('.agent-live-preview')).toContainText('Incomplete response');
  expect(state.cancels()).toBe(1); expect(state.creates()).toBe(1);
  await expect(page.locator('.result-card')).not.toContainText('Finished answer');
});
