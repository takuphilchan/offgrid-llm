import { expect, test, type Page } from '@playwright/test';

const id = 'run-' + 'e'.repeat(32);
async function fixture(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.theme', 'light');
  });
  const state = { status: 'running', writes: [] as string[] };
  const session = { name: 'Tourism research', model_id: 'local-model', updated_at: '2026-10-15T00:00:00Z', messages: [
    { role: 'user', content: 'Summarize the research and recommend next steps.' },
    { role: 'assistant', content: '## Research summary\n\nThe strongest opportunities combine local culture, accessible transport and responsible tourism.\n\n### Next steps\n\n1. Compare three destinations.\n2. Verify seasonal availability.\n3. Draft an itinerary with cited sources.' },
  ] };
  const run = () => ({ run_id: id, id, prompt: 'Open Notepad', model: 'local-model', status: state.status, created_at: '2026-10-15T00:00:00Z', steps: [], resumable: false,
    ...(state.status === 'waiting_for_input' ? { requested_computer_scope: 'desktop', pending_input: { id: 'access-1', mode: 'desktop', app: 'Notepad', prompt: 'Open Notepad' } } : {}) });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, r => {
    const path = decodeURIComponent(new URL(r.request().url()).pathname);
    if (r.request().method() !== 'GET') state.writes.push(path);
    if (path === `/api/v2/jobs/${id}/events`) return r.fulfill({ contentType: 'text/event-stream', body: `data: ${JSON.stringify({ ...run(), type: 'snapshot' })}\n\n` });
    const data: Record<string, unknown> = {
      '/v1/users/me': { authenticated: false, auth_required: false },
      '/api/v2/system': { product: 'offgrid', workspace_id: 'composition', api_version: 2, capabilities: ['agent-profiles-v1', 'task-first-agents-v2'] },
      '/v1/models': { data: [{ id: 'local-model', type: 'chat' }] },
      '/v1/sessions': { sessions: [session] }, '/v1/sessions/Tourism research': session,
      '/api/v2/jobs': [run()], [`/api/v2/jobs/${id}`]: run(),
      '/v1/runs': { runs: [] }, '/v1/stats': { server: {} },
      '/api/v2/computer/sessions': { sessions: [] }, '/api/v2/agent-profiles': { profiles: [] },
    };
    return r.fulfill({ json: path.endsWith('/turn') ? { turn: null } : data[path] ?? {} });
  });
  return state;
}

test('representative composition captures without dispatch', async ({ page }, info) => {
  const state = await fixture(page);
  for (const width of [1440, 1120]) {
    await page.setViewportSize({ width, height: 820 });
    await page.goto('/ui/#/chat');
    await page.locator('.history-open').first().click();
    await expect(page.locator('.message.assistant')).toContainText('Research summary');
    await page.screenshot({ path: info.outputPath(`chat-${width}.png`), fullPage: true });
    for (const status of ['running', 'waiting_for_input']) {
      state.status = status;
      await page.goto(`/ui/#/agents/task/${id}`); await page.reload();
      await expect(page.getByRole('heading', { name: 'Open Notepad', exact: true })).toBeVisible();
      if (status === 'waiting_for_input') await expect(page.locator('.task-access')).toBeVisible();
      await page.screenshot({ path: info.outputPath(`${status}-${width}.png`), fullPage: true });
    }
  }
  expect(state.writes).toEqual([]);
});

test('sidebar collapse preserves the live composer and exposes labelled navigation', async ({ page }) => {
  const state = await fixture(page);
  await page.setViewportSize({ width: 1440, height: 820 });
  await page.goto('/ui/#/chat');
  const editor = page.locator('.composer textarea');
  await editor.fill('Keep this unsent café 日本語 draft');
  await page.locator('.sidebar-collapse').click();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '64px');
  await expect(editor).toHaveValue('Keep this unsent café 日本語 draft');
  await expect(page.locator('.primary-nav').getByRole('link', { name: 'Agents', exact: true })).toBeVisible();
  await page.locator('.sidebar-collapse').click();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '248px');
  await expect(page.locator('.history-open')).toBeVisible();
  expect(state.writes).toEqual([]);
});

test('legacy task links inspect the same job in Activity without an extra history pane', async ({ page }) => {
  const state = await fixture(page);
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.topbar h1')).toHaveText('Activity');
  await expect(page.locator('.work-inspector')).toHaveCount(1);
  await expect(page.locator('.task-list')).toHaveCount(0);
  await page.locator('.task-inspector-toolbar').getByRole('link', { name: 'Activity', exact: true }).click();
  await page.locator('.task-list-item').click();
  await expect(page).toHaveURL(new RegExp(`#/activity/task/${id}$`));
  await expect(page.getByRole('heading', { name: 'Open Notepad', exact: true })).toBeVisible();
  expect(state.writes).toEqual([]);
});

test('non-admin inspection does not enumerate administrative history or expose administrative steering', async ({ page }) => {
  const state = await fixture(page);
  let historyReads = 0;
  await page.route('**/v1/users/me', r => r.fulfill({ json: { authenticated: true, auth_required: true, user: { id: 'reader', username: 'reader', role: 'user' } } }));
  await page.route('**/api/v2/jobs', r => { historyReads++; return r.fulfill({ status: 403, json: {} }); });
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.work-inspector')).toBeVisible();
  await expect(page.locator('.task-steering, .task-list, .task-inspector-toolbar .history-item-actions')).toHaveCount(0);
  expect(historyReads).toBe(0);
  expect(state.writes).toEqual([]);
});
