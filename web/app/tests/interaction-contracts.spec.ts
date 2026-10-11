import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const state = { role: 'admin', authRequired: true, failHistory: false, slowHistory: false, failStats: false, knowledge: false, deletes: [] as string[], calls: [] as string[] };
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.onboarding.complete', 'true'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2, workspace_id: 'legacy', capabilities: ['task-first-agents-v2'] } }));
  await page.route(/\/(?:v1\/|api\/v2\/(?:computer\/|jobs(?:\/|$)))/, async r => {
    const path = new URL(r.request().url()).pathname;
    state.calls.push(path);
    if (path === '/api/v2/jobs' && state.slowHistory) await new Promise(resolve => setTimeout(resolve, 700));
    if ((path === '/api/v2/jobs' && state.failHistory) || (path === '/v1/stats' && state.failStats)) return r.fulfill({ status: 503, json: { error: 'Fixture unavailable' } });
    if (r.request().method() === 'DELETE') {
      await new Promise(resolve => setTimeout(resolve, 600)); state.deletes.push(path.split('/').at(-1)!);
      return r.fulfill({ json: { success: true } });
    }
    const tasks = ['one', 'two', 'three'].filter(id => !state.deletes.includes(id)).map(id => ({ id, prompt: `Task ${id}`, status: 'completed', deletable: true, created_at: '2026-09-19T10:00:00Z' }));
    const bodies: Record<string, unknown> = {
      '/v1/users/me': { authenticated: true, auth_required: state.authRequired, user: { id: 'alice', username: 'alice', role: state.role } },
      '/v1/models': { data: [{ id: 'model-a', type: 'chat' }, { id: 'model-b', type: 'chat' }] },
      '/v1/catalog': { models: [{ id: 'suggestion', name: 'Suggested model', type: 'chat', repo: 'test/repo', file: 'test.gguf', quant: 'Q4_K_M', size_bytes: 1000, min_ram_gb: 2 }] },
      '/v1/models/download/progress': {}, '/v1/sessions': { sessions: [] },
      '/v1/rag/status': { enabled: state.knowledge, stats: {} },
      '/v1/documents': { documents: [{ id: 'document', name: 'Document without index status', chunk_count: 1, source_retained: true, size: 100 }] },
      '/api/v2/jobs': tasks, '/v1/agents/tools': { tools: [], enabled_count: 0 }, '/v1/agents/mcp': { servers: [] },
      '/api/v2/computer/status': { available: false }, '/v1/integrations': { integrations: [] },
      '/v1/runs': { runs: [{ id: 'one', status: 'completed', updated_at: '2026-09-19T10:00:00Z', event_count: 0, data: { prompt: 'Saved run' } }] },
      '/v1/runs/one/events': { events: [] }, '/v1/stats': {}, '/v1/system/config': { require_auth: state.authRequired }
    };
    return r.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}


test('agent selections from another workspace are never fetched', async ({ page }) => {
 const state = await fixture(page);
 await page.route('**/api/v2/system', r => r.fulfill({json:{product:'offgrid', api_version:2, workspace_id:'new-workspace',capabilities:['task-first-agents-v2']}}));
 await page.addInitScript(() => localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Aold-workspace:agent-run:', 'old-run'));
 await page.goto('/ui/#/agents');
 await expect(page.locator('.task-list li')).toHaveCount(3);
 expect(state.calls).not.toContain('/v1/agents/tasks/old-run');
});

test('Stop revokes the service even when local shutdown is unconfirmed', async ({ page }) => {
 await fixture(page);let stops=0;
 await page.addInitScript(()=>{(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',stopComputerBrowser:async()=>({state:'error',code:'stop_unconfirmed'})};});
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:stops?[]:[{id:'paired',origin:'offgrid-demo://research',state:'in_use',remaining_actions:10}]}}));
 await page.route('**/api/v2/computer/stop',r=>{stops++;return r.fulfill({status:202,json:{status:'stopping'}})});
 await page.goto('/ui/#/agents');
 const strip=page.locator('.computer-activity');await strip.getByRole('button',{name:'Stop browser'}).click();
 await expect(strip.getByRole('alert')).toContainText('Browser shutdown was not confirmed');
 await expect.poll(()=>stops).toBe(1);
 await expect(strip.getByRole('button',{name:'Stop browser'})).toBeEnabled();
 await expect(strip.getByRole('button',{name:'Close',exact:true})).toHaveCount(0);
});

test('Stop still closes the local browser when the service is offline', async ({ page }) => {
 await fixture(page);
 await page.addInitScript(()=>{(window as any).__localStops=0;(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',stopComputerBrowser:async()=>{(window as any).__localStops++;return {state:'stopped'};}};});
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[{id:'paired',origin:'offgrid-demo://research',state:'in_use',remaining_actions:10}]}}));
 await page.route('**/api/v2/computer/stop',r=>r.fulfill({status:503,json:{error:'Offline fixture'}}));
 await page.goto('/ui/#/agents');const strip=page.locator('.computer-activity');
 await strip.getByRole('button',{name:'Stop browser'}).click();
 await expect(strip.getByRole('alert')).toContainText('The browser is closed locally');
 expect(await page.evaluate(()=>(window as any).__localStops)).toBe(1);
});

test('form approval cards show the observed option label and explicit checkbox state', async ({ page }) => {
 await fixture(page);
 await page.addInitScript(()=>localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:','browser-form'));
 let checked=false;
 await page.route('**/api/v2/jobs/browser-form',r=>r.fulfill({json:{run_id:'browser-form',task_id:'browser-form',status:'waiting_for_approval',output:'',computer_session:'paired',
  steps:[{id:'observe',tool_name:'browser_observe',tool_result:JSON.stringify({observation_id:'current',elements:[{id:'1',label:'Region',options:[{id:'2',label:'Zimbabwe — 2026'}]},{id:'2',label:'Include sources'}]})}],
  pending_approval:{id:'approval',tool:checked?'browser_set_checked':'browser_select',arguments:checked?{observation_id:'current',element:'2',checked:false}:{observation_id:'current',element:'1',option:'2'},expires_at:new Date(Date.now()+60000).toISOString()}}}));
 await page.goto('/ui/#/agents');
 const card=page.locator('.browser-action-summary');
 await expect(card).toContainText('Select an option');await expect(card).toContainText('Region');await expect(card).toContainText('Zimbabwe — 2026');
 checked=true;await page.reload();
 await expect(card).toContainText('Set a checkbox');await expect(card).toContainText('Include sources');await expect(card).toContainText('Unchecked');
});

test('computer task history distinguishes automatic authorization from verification', async ({page})=>{
 await fixture(page);
 await page.addInitScript(()=>localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:','approval-audit'));
 await page.route('**/api/v2/jobs/approval-audit',r=>r.fulfill({json:{run_id:'approval-audit',task_id:'approval-audit',status:'completed',output:'Draft saved.',computer_session:'paired',computer_approval_mode:'scoped_changes',steps:[{id:1,tool_name:'browser_fill',tool_result:'{"verified":true}',authorization:'automatic:scoped_changes'}]}}));
 await page.goto('/ui/#/agents');
 const timeline=page.locator('.browser-task-timeline');
 await page.locator('.task-activity > summary').click();
 await expect(timeline).toContainText('Automatically approved · Approve scoped changes');
 await expect(timeline).not.toContainText('Verified · scoped changes');
});

test('current task history keeps search and rows separated on desktop and mobile', async ({page}, info) => {
  await fixture(page);await page.goto('/ui/#/agents');
  await expect(page.locator('.task-list li')).toHaveCount(3);
  for (const width of [1440,390]) {
    await page.setViewportSize({width,height:900});
    const search=await page.locator('.workspace-history-toolbar').boundingBox();
    const rows=await page.locator('.task-list ul').boundingBox();
    expect(rows!.y-search!.y-search!.height).toBeGreaterThanOrEqual(12);
    expect(await page.locator('.task-list').evaluate(e=>e.scrollWidth-e.clientWidth)).toBeLessThanOrEqual(1);
    await page.locator('.task-list').screenshot({path:info.outputPath(`history-${width}.png`)});
  }
});

test('command palette contains focus, handles Escape outside input and ignores composition Enter', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: /Quick actions/ }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox').dispatchEvent('keydown', { key: 'Enter', isComposing: true });
  await expect(dialog).toBeVisible();
  for (let i = 0; i < 18; i++) { await page.keyboard.press('Tab'); expect(await page.evaluate(() => !!document.activeElement?.closest('dialog'))).toBe(true); }
  await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Quick actions/ })).toBeFocused();
});

test('mobile conversation drawer contains focus and Escape restores its trigger', async ({ page }) => {
  await fixture(page); await page.setViewportSize({ width: 390, height: 844 }); await page.goto('/ui/#/chat');
  const trigger = page.getByRole('button', { name: 'Show conversations', exact: true });
  await trigger.click();
  await expect(page.locator('#conversation-history')).toBeVisible();
  await expect(page.locator('#conversation-history .history-close')).toBeFocused();
  for (let i = 0; i < 8; i++) { await page.keyboard.press('Tab'); const focus = await page.evaluate(() => ({ inside: !!document.activeElement?.closest('#conversation-history'), element: document.activeElement?.outerHTML.slice(0, 1000) })); expect(focus.inside, `Tab ${i + 1}: ${focus.element}`).toBe(true); }
  await page.keyboard.press('Escape'); await expect(page.locator('#conversation-history')).not.toBeVisible(); await expect(trigger).toBeFocused();
});

test('member UI does not call administrator endpoints or offer privileged actions', async ({ page }) => {
  const state = await fixture(page); state.role = 'user';
  await page.goto('/ui/#/agents'); await expect(page.getByText('Administrator access required')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Start task', exact: true })).toHaveCount(0);
  await page.goto('/ui/#/settings'); await expect(page.locator('.settings-page')).toContainText('Administrator access required');
  await page.goto('/ui/#/models'); await page.getByRole('tab', { name: 'Discover models', exact: true }).click(); await expect(page.locator('.catalog-card .primary-button')).toBeDisabled();
  await page.goto('/ui/#/knowledge'); await expect(page.getByRole('button', { name: 'Add document' })).toBeDisabled();
  expect(state.calls.filter(path => path.startsWith('/v1/agents/') || path === '/api/v2/computer/status' || path === '/v1/models/download/progress')).toEqual([]);
});

test('anonymous local workspace remains usable when authentication is explicitly disabled', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/users/me', r => r.fulfill({ json: { authenticated: false, guest: true, auth_required: false, user: null } }));
  await page.goto('/ui/#/agents'); await expect(page.getByRole('button', { name: 'Start task', exact: true })).toBeVisible();
});

test('history loading, failure and successful recovery stay distinct', async ({ page }) => {
  const state = await fixture(page); state.slowHistory = true; state.failHistory = true;
  await page.goto('/ui/#/agents'); await expect(page.locator('.task-list')).toContainText('Loading…');
  await expect(page.locator('.task-list')).not.toContainText('No agent tasks');
  await expect(page.locator('.task-list')).toContainText('Fixture unavailable');
  state.failHistory = false; state.slowHistory = false;
  await page.locator('.topbar').getByRole('button', { name: 'Refresh' }).click();
  await expect(page.locator('.task-list')).toContainText('Task one'); await expect(page.getByRole('alert')).toHaveCount(0);
});

test('statistics failure does not hide available run history', async ({ page }) => {
  const state = await fixture(page); state.failStats = true;
  await page.goto('/ui/#/activity'); await page.getByRole('button', { name: /^Saved run/ }).click();
  await expect(page.locator('.event-list')).toContainText('No events recorded');
});

test('connector drafts and selected Activity survive navigation without refetching online searches', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/agents/connections');
  await page.locator('.connector-panel input').nth(0).fill('Private connector'); await page.locator('.connector-panel input').nth(1).fill('http://localhost:3000/mcp');
  await page.locator('.primary-nav a[href="#/models"]').click(); await page.getByRole('tab', { name: 'Discover models', exact: true }).click(); await page.getByLabel('Model name or publisher', { exact: true }).fill('Research model');
  await page.locator('.primary-nav a[href="#/agents"]').click(); await page.getByRole('link', { name: 'Connections', exact: true }).click();
  await expect(page.locator('.connector-panel input').nth(0)).toHaveValue('Private connector');
  await page.locator('.primary-nav a[href="#/models"]').click(); await expect(page.getByLabel('Model name or publisher', { exact: true })).toHaveValue('Research model');
  await page.locator('.primary-nav a[href="#/activity"]').click(); await page.getByRole('button', { name: /^Saved run/ }).click();
  await page.locator('.primary-nav a[href="#/models"]').click(); await page.locator('.primary-nav a[href="#/activity"]').click();
  await expect(page.locator('.run-row.selected')).toContainText('Saved run');
});

test('bulk deletion can stop after the in-flight item without deleting remaining entries', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/agents');
  await page.getByRole('button', { name: 'Manage history', exact: true }).click();
  await page.getByRole('button', { name: 'Clear removable tasks' }).click();
  const dialog = page.getByRole('dialog'); await dialog.getByRole('button', { name: 'Confirm delete' }).click();
  await expect(dialog.getByRole('status')).toHaveText('0 of 3 processed');
  await dialog.getByRole('button', { name: 'Stop after current item' }).click();
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeEnabled();
  expect(state.deletes).toHaveLength(1); await expect(dialog).not.toContainText('Task one');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.locator('.task-list li')).toHaveCount(2);
});

test('unknown index status is never presented as ready and unavailable knowledge cannot be enabled in chat', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/knowledge');
  await expect(page.locator('.resource-card')).toContainText('Status unavailable');
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).toBeDisabled();
});

test('late provider setup cannot overwrite instructions for a newly selected model', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/integrations?*', r => r.fulfill({ json: { integrations: [{ id: 'hermes', name: 'Hermes', provider_id: 'offgrid', transport: 'openai-chat-completions', model_id: new URL(r.request().url()).searchParams.get('model') || 'model-a', context_window: 8192, minimum_context: 8192, warnings: [], ready: true }] } }));
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let requested = false;
  await page.route('**/v1/integrations/hermes/setup*', async r => {
    requested = true; await held;
    await r.fulfill({ json: { setup: { install_command: 'STALE MODEL A', verify: [], environment: {} } } });
  });
  await page.goto('/ui/#/agents/connections');
  await page.locator('.provider-card button').click();
  await expect.poll(() => requested).toBe(true);
  await page.getByRole('link', { name: 'Work', exact: true }).click();
  await page.getByRole('combobox', { name: 'Model', exact: true }).selectOption('model-b');
  release();
  await page.getByRole('link', { name: 'Connections', exact: true }).click();
  await expect(page.locator('.provider-card')).toContainText('model-b');
  await expect(page.locator('.provider-setup')).toHaveCount(0);
});
test('obsolete verification settings cannot enter new job submissions',async({page})=>{
  await fixture(page);
  const obsolete='offgrid.draft.v1:alice%3Aworkspace%3Alegacy:computer-expected-text:';
  await page.addInitScript(key=>localStorage.setItem(key,'Do not use this legacy assertion'),obsolete);
  let submitted:any;
  await page.route('**/api/v2/jobs',r=>{
    if(r.request().method()==='GET')return r.fulfill({json:[]});
    submitted=r.request().postDataJSON();return r.fulfill({status:422,json:{error:'Synthetic model not ready'}});
  });
  await page.goto('/ui/#/agents/new');await page.getByRole('textbox',{name:'Task',exact:true}).fill('Keep this task draft');
  await expect(page.locator('#computer-expected-text, .computer-mode-picker')).toHaveCount(0);
  await page.getByRole('button',{name:'Start task',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('Synthetic model not ready');
  expect(submitted).not.toHaveProperty('computer_expected_text');
  expect(submitted).not.toHaveProperty('computer_session');
  await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Keep this task draft');
});

test('missing saved task shows recovery without replaying work or losing a separate draft',async({page})=>{
  await fixture(page);
  await page.addInitScript(()=>{
    localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:','missing-run');
    localStorage.setItem(`offgrid.draft.v2:${encodeURIComponent(JSON.stringify(['alice','legacy']))}:agent-task:`,'Keep my research draft');
  });
  let calls=0;
  await page.route('**/api/v2/jobs/missing-run',r=>{calls++;return r.fulfill({status:404,json:{error:'Saved task not found'}})});
  await page.goto('/ui/#/agents');
  await expect(page.getByRole('alert')).toContainText('Saved task not found');
  await page.getByRole('button',{name:'New task',exact:true}).click();
  await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Keep my research draft');
  const before=calls;await page.reload();
  await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Keep my research draft');
  expect(calls).toBe(before);
});
