import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page, desktop = false) {
  const state = { supported: true, identityFailed: false, storageFailed: false, reads: 0, mutations: [] as string[] };
  await page.addInitScript(({desktop}) => {
    localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en');
    if (desktop) Object.assign(window, { electron: { isDesktop: true, platform: 'win32', getApiUrl: async () => location.origin, getSystemTheme: async () => 'light', onThemeChange: () => () => {} } });
  }, {desktop});
  await page.route('**/health', r => r.fulfill({json:{status:'healthy'}}));
  await page.route(/\/(?:v1|api\/v2)\//, r => {
    const path = new URL(r.request().url()).pathname;
    if (r.request().method() !== 'GET') state.mutations.push(path);
    if (path === '/api/v2/system') {
      state.reads++;
      return r.fulfill(state.identityFailed ? {status:503,json:{error:'Fixture identity failure'}} : {json:{product:'offgrid',version:'test',api_version:2,workspace_id:'availability',capabilities:state.supported?['task-first-agents-v2','durable-agent-events-v2']:[]}});
    }
    if (path === '/api/v2/jobs' && state.storageFailed) return r.fulfill({status:503,json:{error:'Agent task storage unavailable'}});
    const bodies: Record<string, unknown> = {
      '/v1/users/me': {authenticated:false,auth_required:false,user:null},
      '/v1/models': {data:[{id:'model',type:'chat'}]}, '/v1/system/config': {require_auth:false},
      '/api/v2/jobs': [], '/v1/agents/tools':{tools:[],enabled_count:0}, '/v1/agents/mcp':{servers:[]},
      '/v1/integrations':{integrations:[]}, '/api/v2/computer/sessions':{sessions:[]}, '/v1/sessions':{sessions:[]}
    };
    return r.fulfill({json:bodies[path]??{}});
  });
  return state;
}

for (const desktop of [false, true]) {
  test(`current ${desktop?'desktop':'web'} renderer survives storage loss and identity refresh without losing drafts`, async ({page}) => {
    const state = await fixture(page, desktop);
    await page.goto('/ui/#/chat');
    const draft = page.locator('.composer textarea');
    await draft.fill('Keep this draft 日本語');
    await page.goto('/ui/#/agents/workspace');
    state.storageFailed = true;
    // Older services drop their feature flag when the task store fails.
    state.supported = false;
    await refreshWorkspace(page);
    await expect(page.getByRole('alert').filter({hasText:'Agent task storage unavailable'})).toBeVisible();
    await page.goto('/ui/#/chat');
    await expect(draft).toHaveValue('Keep this draft 日本語');
    await page.goto('/ui/#/agents/workspace');
    await expect(page.getByRole('button',{name:'Start task',exact:true})).toHaveCount(0);
    await expect(page.locator('.task-card, .result-card, .computer-mode-picker')).toHaveCount(0);
    state.identityFailed = true;
    await refreshWorkspace(page);
    await expect(page.getByRole('alert').filter({hasText:'Agent task storage unavailable'})).toBeVisible();
    await page.goto('/ui/#/chat');
    await expect(draft).toHaveValue('Keep this draft 日本語');
    await page.goto('/ui/#/agents/connections');
    await page.getByLabel('Connection name',{exact:true}).fill('Keep connection draft');
    await page.getByRole('link',{name:'Available tools',exact:true}).click();
    await expect(page.locator('#agent-tools-panel')).toBeVisible();
    await page.getByRole('link',{name:'Connections',exact:true}).click();
    await expect(page.getByLabel('Connection name',{exact:true})).toHaveValue('Keep connection draft');
    await page.goto('/ui/#/activity');
    await page.goto('/ui/#/chat');
    await expect(draft).toHaveValue('Keep this draft 日本語');
    await page.goto('/ui/#/agents/workspace');
    state.identityFailed=false;state.supported=true;state.storageFailed=false;
    await page.getByRole('alert').filter({hasText:'Agent task storage unavailable'}).getByRole('button',{name:'Try again',exact:true}).click();
    await expect(page.getByRole('button',{name:'Start task',exact:true})).toHaveCount(0);
    expect(state.mutations).toEqual([]);
  });
}

test('cold unsupported service shows recovery and management, never a legacy task runner', async ({page}) => {
  const state=await fixture(page);state.supported=false;
  await page.goto('/ui/#/agents');
  await expect(page.getByRole('alert')).toContainText('does not support agent profiles');
  await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveCount(0);
  await expect(page.locator('.task-card, .result-card, .computer-mode-picker')).toHaveCount(0);
  const previous=state.reads;
  await page.getByRole('button',{name:'Try again',exact:true}).click();
  await expect.poll(()=>state.reads).toBeGreaterThan(previous);
  await page.getByRole('link',{name:'Connections',exact:true}).click();
  await expect(page.locator('.connector-panel')).toBeVisible();
  state.supported=true;
  await page.goto('/ui/#/activity');
  await refreshWorkspace(page);
  await expect(page.getByRole('heading',{name:'Recent tasks'})).toBeVisible();
  expect(state.mutations).toEqual([]);
});
import { refreshWorkspace } from './helpers/workspace-actions';
