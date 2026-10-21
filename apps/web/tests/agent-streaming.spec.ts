import { openWorkDetails } from './helpers/workspace-actions';
import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page, durable = false) {
  const started = new Date().toISOString();
  let status = 'running', phase = 'generating', preview = 'Live partial answer 日本語';
  let creates = 0, cancels = 0, streams = 0;
  let disconnect = true;
  const replayCursors: string[] = [];
  const snapshot = () => ({ type: status === 'running' ? 'status' : status === 'completed' ? 'done' : 'error', run_id: 'run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', task_id:'run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', status, output: status === 'completed' ? 'Finished answer' : '', steps:[], pending_approval:null, resumable: false, started_at: started, progress:{phase,iteration:1,preview,truncated:false,started_at:started,updated_at:started} });
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.onboarding.complete','true'); });
  await page.route('**/health', r => r.fulfill({json:{status:'healthy'}}));
  await page.route('**/api/v2/system', r => r.fulfill({json:{product:'offgrid',version:'test',api_version:2,workspace_id:'test-workspace',capabilities:durable ? ['task-first-agents-v2','durable-agent-events-v2'] : ['task-first-agents-v2']}}));
  if (durable) await page.route('**/api/v2/jobs/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events', async route => {
    replayCursors.push(route.request().headers()['last-event-id'] ?? '');
    streams++;
    await new Promise(resolve => setTimeout(resolve,100));
    // Recovery changes the cursor; it must never create or rerun a task.
    const event = {...snapshot(),type:streams>1 ? 'snapshot_recovery' : 'snapshot',event_cursor:String(streams*10)};
    return route.fulfill({contentType:'text/event-stream',body:`id: ${streams*10}\nevent: activity\ndata: {"type":"activity","activity":{"sequence":${streams*10}}}\n\nid: ${streams*10}\nevent: ${event.type}\ndata: ${JSON.stringify(event)}\n\n`});
  });
  await page.route(/\/(?:v1\/|api\/v2\/jobs(?:\/|$))/, async route => {
    const path = new URL(route.request().url()).pathname;
    if (durable && path.endsWith('/events')) return route.fallback();
    let json: unknown = {};
    if (path === '/v1/users/me') json = {authenticated:false,user:null};
    else if (path === '/v1/models') json = {data:[{id:'model',type:'chat'}]};
    else if (path === '/api/v2/jobs' && route.request().method() === 'POST') { creates++; json = snapshot(); }
    else if (path === '/api/v2/jobs/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa') json = snapshot();
    else if (path === '/v1/agents/tasks/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/events') {
      streams++;
      if (!disconnect) await new Promise(resolve => setTimeout(resolve, 250));
      // EOF while running is a disconnection, not a completed run.
      return route.fulfill({contentType:'text/event-stream', body:`: heartbeat\r\ndata: ${JSON.stringify(snapshot())}\r\n\r\n`});
    } else if (path === '/api/v2/jobs/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/cancel') {cancels++;status='cancelled';json=snapshot();}
    else if (path === '/api/v2/jobs') json = [{id:'run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',prompt:'Do the task',status,created_at:started}];
    else if (path === '/v1/agents/tools') json = {tools:[],enabled_count:0};
    else if (path === '/v1/agents/mcp') json = {servers:[]};
    else if (path === '/v1/integrations') json = {integrations:[]};
    else if (path === '/v1/sessions') json = {sessions:[]};
    await route.fulfill({json});
  });
  return { creates:()=>creates,cancels:()=>cancels,streams:()=>streams,replayCursors, complete:()=>{status='completed';disconnect=false;}, tool:()=>{phase='tool';preview='';} };
}

test('saved agent preview appears before completion and reconnects without resubmitting', async ({page}) => {
  const state = await fixture(page);
  await page.goto('/ui/#/agents/task/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
  await openWorkDetails(page);
  await page.getByRole('checkbox',{name:'Live response preview'}).check();
  await expect(page.locator('.agent-live-preview')).toContainText('Live partial answer 日本語');
  await expect(page.locator('.agent-live-preview')).toContainText('not a completed answer');
  await expect(page.locator('.conversation-work > p').filter({hasText:'The connection is recovering.'})).toBeVisible();
  await expect.poll(state.streams).toBeGreaterThan(1);
  expect(state.creates()).toBe(0);
  await page.getByRole('checkbox',{name:'Live response preview'}).uncheck();
  await expect(page.locator('.agent-live-preview')).toHaveCount(0);
  expect(state.cancels()).toBe(0);
  await openWorkDetails(page);
  await page.getByRole('checkbox',{name:'Live response preview'}).check();
  await page.goto('/ui/#/chat');
  await page.goto('/ui/#/agents/task/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
  await openWorkDetails(page);
  await page.getByRole('checkbox',{name:'Live response preview'}).check();
  await expect(page.locator('.agent-live-preview')).toContainText('Live partial answer 日本語');
  expect(state.creates()).toBe(0);
  state.complete();
  await expect(page.locator('.task-result')).toContainText('Finished answer');
  await expect(page.locator('.agent-live-preview')).toHaveCount(0);
});

test('cancel retains a clearly incomplete preview without claiming completion', async ({page}) => {
  const state = await fixture(page);
  await page.goto('/ui/#/agents/task/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
  await openWorkDetails(page);
  await page.getByRole('checkbox',{name:'Live response preview'}).check();
  await expect(page.locator('.agent-live-preview')).toBeVisible();
  await page.getByRole('button',{name:'Stop',exact:true}).click();
  await expect(page.locator('.agent-live-preview')).toContainText('Incomplete response');
  expect(state.cancels()).toBe(1); expect(state.creates()).toBe(0);
  await expect(page.locator('.task-surface')).not.toContainText('Finished answer');
});

test('durable replay reconnects with the applied cursor and accepts snapshot recovery', async ({page}) => {
  const state = await fixture(page,true);
  await page.goto('/ui/#/agents/task/run-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
  await openWorkDetails(page);
  await page.getByRole('checkbox',{name:'Live response preview'}).check();
  await expect(page.locator('.agent-live-preview')).toBeVisible();
  await expect.poll(()=>state.replayCursors.length).toBeGreaterThan(1);
  expect(state.replayCursors[0]).toBe('');
  expect(state.replayCursors[1]).toBe('10');
  expect(state.creates()).toBe(0);
  state.complete();
  await expect(page.locator('.task-result')).toContainText('Finished answer');
  expect(state.creates()).toBe(0);
});
