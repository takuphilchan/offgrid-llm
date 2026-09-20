import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const state = { enabled: false, docs: 0, failSettings: false, slowSettings: false, transfer: 'complete', polls: 0 };
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.theme', 'light');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2 } }));
  await page.route(/\/(?:v1|api\/v2\/computer)\//, async r => {
    const path = new URL(r.request().url()).pathname;
    if (state.slowSettings && ['/v1/rag/status', '/api/v2/computer/status'].includes(path)) await new Promise(resolve => setTimeout(resolve, 700));
    if (state.failSettings && ['/v1/rag/status', '/api/v2/computer/status'].includes(path)) return r.fulfill({ status: 503, body: 'Service unavailable' });
    const bge = { id: 'bge-m3', name: 'BGE M3', type: 'embedding', repo: 'fixture/bge', file: 'bge.gguf', quant: 'Q4_K_M', description: 'Multilingual embedding model', parameters: '567M', size_bytes: 437778496, min_ram_gb: 2 };
    const data: Record<string, unknown> = {
      '/v1/users/me': { authenticated: false, user: null },
      '/v1/models': { data: [{ id: 'chat-model', type: 'chat' }, bge] },
      '/v1/catalog': { models: [bge, ...Array.from({ length: 17 }, (_, i) => ({ ...bge, id: `model-${i}`, name: `Model ${i}`, type: 'chat' }))] },
      '/v1/documents': { documents: state.docs ? [{ id: 'doc', name: 'Refreshed document', source_retained: true, size: 10, chunk_count: 1 }] : [], count: state.docs },
      '/v1/rag/status': { enabled: state.enabled, embedding_model: 'bge-m3', stats: {} },
      '/v1/models/download/progress': state.transfer === 'missing' ? {} : { 'bge-m3.gguf': { file_name: 'bge-m3.gguf', model_id: 'bge-m3', enable_knowledge: true, status: state.transfer, percent: 100, bytes_done: 100, bytes_total: 100, speed: 0, started_at: 1 } },
      '/v1/agents/tasks': [],
      '/v1/agents/tools': { tools: [{ name: 'calculator', description: 'Calculate', enabled: true, source: 'builtin' }], enabled_count: 1 },
      '/v1/agents/mcp': { servers: [] }, '/v1/integrations': { integrations: [] },
      '/api/v2/computer/status': { available: false }, '/v1/system/config': { version: 'test', inference_slots: 1 },
      '/v1/sessions': { sessions: [] }
    };
    if (path === '/v1/models/download/progress') state.polls++;
    if (path === '/v1/agents/run' || path === '/v1/agents/tasks/example') return r.fulfill({ json: { run_id: 'example', status: 'completed', output: '## Report\n\n| Item | Value |\n| --- | --- |\n| Test | Passed |\n\n![tracker](https://example.com/track)', steps: [], resumable: false } });
    return r.fulfill({ json: data[path] ?? {} });
  });
  return state;
}

test('completed download is not retrieval readiness and setup stays compact', async ({ page }, info) => {
  await fixture(page);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto('/ui/#/knowledge');
  await expect(page.getByRole('button', { name: 'Enable knowledge', exact: true })).toBeEnabled();
  await expect(page.locator('.knowledge-setup')).not.toContainText('100.0%');
  await expect(page.locator('.knowledge-setup [role=progressbar]')).toHaveCount(0);
  expect((await page.locator('.knowledge-setup').boundingBox())!.height).toBeLessThan(320);
  await page.screenshot({ path: info.outputPath('knowledge-disabled.png') });
  await page.goto('/ui/#/models');
  expect((await page.locator('#model-search-query').boundingBox())!.y).toBeLessThan(500);
  const card = page.locator('.catalog-card').filter({ has: page.getByRole('heading', { name: 'BGE M3', exact: true }) });
  await expect(card.getByRole('status')).toHaveText('Installed');
  await expect(card.getByRole('progressbar')).toHaveCount(0);
  await expect(card.getByRole('button', { name: 'Installed', exact: true })).toHaveCount(0);
  await page.getByLabel('Filter suggested models').fill('BGE');
  await expect(page.locator('.catalog-card')).toHaveCount(1);
  await page.screenshot({ path: info.outputPath('models-installed.png'), fullPage: true });
});

test('global refresh updates documents without reloading the workspace', async ({ page }) => {
  const state = await fixture(page);
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-setup')).toBeVisible();
  state.docs = 1;
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Refreshed document' })).toBeVisible();
  await page.goto('/ui/#/agents');
  await page.getByLabel('Task', { exact: true }).fill('Keep this draft');
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByLabel('Task', { exact: true })).toHaveValue('Keep this draft');
});

test('missing progress unlocks recovery instead of preparing forever', async ({ page }) => {
  const state = await fixture(page); state.transfer = 'finalizing';
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-setup .primary-button')).toBeDisabled();
  state.transfer = 'missing';
  await expect(page.locator('.knowledge-setup .primary-button')).toBeEnabled();
  await expect(page.getByRole('alert')).toBeVisible();
});

test('settings distinguishes loading and failed requests from disabled features', async ({ page }) => {
  const state = await fixture(page); state.slowSettings = true; state.failSettings = true;
  await page.goto('/ui/#/settings');
  await expect(page.locator('.settings-page')).toContainText('Loading…');
  await expect(page.locator('.settings-page')).not.toContainText('No platform driver installed');
  await expect(page.locator('.settings-page')).toContainText('Status unavailable');
  await expect(page.getByRole('alert')).toBeVisible();
});

test('agent tabs support keyboard navigation and tool switches have names', async ({ page }) => {
  await fixture(page);
  await page.goto('/ui/#/agents');
  await page.locator('#agent-workspace-tab').focus();
  await page.keyboard.press('ArrowRight');
  await expect(page.locator('#agent-tools-tab')).toBeFocused();
  await expect(page.locator('#agent-tools-tab')).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('checkbox', { name: 'calculator', exact: true })).toBeChecked();
  await page.keyboard.press('End');
  await expect(page.locator('#agent-connections-tab')).toBeFocused();
  await page.setViewportSize({ width: 390, height: 844 });
  for (const link of await page.locator('.mobile-nav a').all()) await expect(link).toBeInViewport();
  expect(await page.locator('.mobile-nav').evaluate(e => e.scrollWidth <= e.clientWidth)).toBe(true);
});

test('agent results render safe headings and tables rather than raw Markdown', async ({ page }) => {
  await fixture(page);
  const remote: string[] = [];
  page.on('request', r => { if (r.url().includes('example.com')) remote.push(r.url()); });
  await page.goto('/ui/#/agents');
  await page.getByLabel('Task', { exact: true }).fill('Make a report');
  await page.getByRole('button', { name: 'Run task', exact: true }).click();
  await expect(page.locator('.agent-answer h2')).toHaveText('Report');
  await expect(page.locator('.agent-answer table')).toContainText('Passed');
  expect(remote).toEqual([]);
});

test('knowledge does not borrow progress from a different embedding model', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/models/download/progress', r => r.fulfill({ json: {
    'other.gguf': { file_name: 'other.gguf', model_id: 'other', enable_knowledge: true, status: 'finalizing', percent: 100, started_at: 2 }
  } }));
  await page.goto('/ui/#/knowledge');
  await expect(page.getByRole('button', { name: 'Enable knowledge', exact: true })).toBeEnabled();
  await expect(page.locator('.knowledge-setup .download-state')).toHaveCount(0);
});

test('Activity presents results with optional technical details', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/runs', r => r.fulfill({ json: { runs: [{ id: 'report', status: 'completed', updated_at: '2026-09-19T10:00:00Z', event_count: 1, data: { prompt: 'Prepare report' } }] } }));
  await page.route('**/v1/runs/report/events', r => r.fulfill({ json: { events: [{ id: 'e1', type: 'agent.finished', sequence: 1, time: '2026-09-19T10:00:00Z', data: { output: '## Findings\n\nUseful result', diagnostic: 'raw payload' } }] } }));
  await page.goto('/ui/#/activity');
  await page.getByRole('button', { name: /Prepare report/ }).click();
  await expect(page.locator('.event-label')).toHaveText('Completed');
  await expect(page.locator('.event-list h2')).toHaveText('Findings');
  await expect(page.locator('.event-list pre')).toBeHidden();
  await page.getByText('Technical details', { exact: true }).click();
  await expect(page.locator('.event-list pre')).toContainText('raw payload');
});
