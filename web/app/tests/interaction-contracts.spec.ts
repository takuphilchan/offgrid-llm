import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const state = { role: 'admin', authRequired: true, failHistory: false, slowHistory: false, failStats: false, knowledge: false, deletes: [] as string[], calls: [] as string[] };
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.onboarding.complete', 'true'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2 } }));
  await page.route('**/v1/**', async r => {
    const path = new URL(r.request().url()).pathname;
    state.calls.push(path);
    if (path === '/v1/agents/tasks' && state.slowHistory) await new Promise(resolve => setTimeout(resolve, 700));
    if ((path === '/v1/agents/tasks' && state.failHistory) || (path === '/v1/stats' && state.failStats)) return r.fulfill({ status: 503, json: { error: 'Fixture unavailable' } });
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
      '/v1/agents/tasks': tasks, '/v1/agents/tools': { tools: [], enabled_count: 0 }, '/v1/agents/mcp': { servers: [] },
      '/v1/computer/status': { available: false }, '/v1/integrations': { integrations: [] },
      '/v1/runs': { runs: [{ id: 'one', status: 'completed', updated_at: '2026-09-19T10:00:00Z', event_count: 0, data: { prompt: 'Saved run' } }] },
      '/v1/runs/one/events': { events: [] }, '/v1/stats': {}, '/v1/system/config': { require_auth: state.authRequired }
    };
    return r.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}

test('history toolbar and search field have deliberate vertical separation', async ({ page }, testInfo) => {
  await fixture(page); await page.goto('/ui/#/agents');
  await expect(page.locator('.task-history article')).toHaveCount(3);
  const heading = await page.locator('.agent-history-panel > .section-heading').boundingBox();
  const search = await page.locator('.agent-history-panel .history-search').boundingBox();
  expect(search!.y - heading!.y - heading!.height).toBeGreaterThanOrEqual(12);
  await page.locator('.agent-history-panel').screenshot({ path: testInfo.outputPath('history-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator('.agent-history-panel .history-search')).toBeVisible();
  expect(await page.locator('.agent-history-panel').evaluate(e => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await page.locator('.agent-history-panel').screenshot({ path: testInfo.outputPath('history-mobile.png') });
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
  for (let i = 0; i < 8; i++) { await page.keyboard.press('Tab'); expect(await page.evaluate(() => !!document.activeElement?.closest('#conversation-history'))).toBe(true); }
  await page.keyboard.press('Escape'); await expect(page.locator('#conversation-history')).not.toBeVisible(); await expect(trigger).toBeFocused();
});

test('member UI does not call administrator endpoints or offer privileged actions', async ({ page }) => {
  const state = await fixture(page); state.role = 'user';
  await page.goto('/ui/#/agents'); await expect(page.getByText('Administrator access required')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Run task', exact: true })).toHaveCount(0);
  await page.goto('/ui/#/settings'); await expect(page.locator('.settings-page')).toContainText('Administrator access required');
  await page.goto('/ui/#/models'); await expect(page.locator('.catalog-card .primary-button')).toBeDisabled();
  await page.goto('/ui/#/knowledge'); await expect(page.getByRole('button', { name: 'Add document' })).toBeDisabled();
  expect(state.calls.filter(path => path.startsWith('/v1/agents/') || path === '/v1/computer/status' || path === '/v1/models/download/progress')).toEqual([]);
});

test('anonymous local workspace remains usable when authentication is explicitly disabled', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/users/me', r => r.fulfill({ json: { authenticated: false, guest: true, auth_required: false, user: null } }));
  await page.goto('/ui/#/agents'); await expect(page.getByRole('button', { name: 'Run task', exact: true })).toBeVisible();
});

test('history loading, failure and successful recovery stay distinct', async ({ page }) => {
  const state = await fixture(page); state.slowHistory = true; state.failHistory = true;
  await page.goto('/ui/#/agents'); await expect(page.locator('.agent-history-panel')).toContainText('Preparing history');
  await expect(page.locator('.agent-history-panel')).not.toContainText('No agent tasks');
  await expect(page.locator('.agent-history-panel')).toContainText('History refresh failed');
  state.failHistory = false; state.slowHistory = false;
  await page.locator('.topbar').getByRole('button', { name: 'Refresh' }).click();
  await expect(page.locator('.agent-history-panel')).toContainText('Task one'); await expect(page.getByRole('alert')).toHaveCount(0);
});

test('statistics failure does not hide available run history', async ({ page }) => {
  const state = await fixture(page); state.failStats = true;
  await page.goto('/ui/#/activity'); await page.getByRole('button', { name: /Saved run/ }).click();
  await expect(page.locator('.event-list')).toContainText('No events recorded');
});

test('connector drafts and selected Activity survive navigation without refetching online searches', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/agents/connections');
  await page.locator('.connector-panel input').nth(0).fill('Private connector'); await page.locator('.connector-panel input').nth(1).fill('http://localhost:3000/mcp');
  await page.locator('.primary-nav a[href="#/models"]').click(); await page.getByLabel('Model name or publisher', { exact: true }).fill('Research model');
  await page.locator('.primary-nav a[href="#/agents"]').click(); await page.getByRole('tab', { name: 'MCP connections' }).click();
  await expect(page.locator('.connector-panel input').nth(0)).toHaveValue('Private connector');
  await page.locator('.primary-nav a[href="#/models"]').click(); await expect(page.getByLabel('Model name or publisher', { exact: true })).toHaveValue('Research model');
  await page.locator('.primary-nav a[href="#/activity"]').click(); await page.getByRole('button', { name: /Saved run/ }).click();
  await page.locator('.primary-nav a[href="#/models"]').click(); await page.locator('.primary-nav a[href="#/activity"]').click();
  await expect(page.locator('.run-row.selected')).toContainText('Saved run');
});

test('bulk deletion can stop after the in-flight item without deleting remaining entries', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/agents');
  await page.getByRole('button', { name: 'Clear removable tasks' }).click();
  const dialog = page.getByRole('dialog'); await dialog.getByRole('button', { name: 'Confirm delete' }).click();
  await expect(dialog.getByRole('status')).toHaveText('0 of 3 processed');
  await dialog.getByRole('button', { name: 'Stop after current item' }).click();
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeEnabled();
  expect(state.deletes).toHaveLength(1); await expect(dialog).not.toContainText('Task one');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.locator('.task-history article')).toHaveCount(2);
});

test('unknown index status is never presented as ready and unavailable knowledge cannot be enabled in chat', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/knowledge');
  await expect(page.locator('.resource-card')).toContainText('Status unavailable');
  await page.locator('.primary-nav a[href="#/chat"]').click();
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
  await page.getByRole('tab', { name: 'Work', exact: true }).click();
  await page.getByRole('combobox', { name: 'Model', exact: true }).selectOption('model-b');
  release();
  await page.getByRole('tab', { name: 'MCP connections' }).click();
  await expect(page.locator('.provider-card')).toContainText('model-b');
  await expect(page.locator('.provider-setup')).toHaveCount(0);
});
