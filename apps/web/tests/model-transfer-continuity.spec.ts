import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page, role = 'admin') {
  const state = { legacy: 'downloading', speech: 'interrupted', failRead: false, loseAck: false, discarded: false, reads: 0, mutations: [] as { path: string; body: unknown }[] };
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, async route => {
    const request = route.request(), path = new URL(request.url()).pathname;
    if (request.method() !== 'GET') state.mutations.push({ path, body: request.postDataJSON() });
    if (path === '/v1/models/download/cancel') state.legacy = 'cancelled';
    if (path === '/v1/models/download') state.legacy = 'downloading';
    if (path === '/api/v2/models/operations/speech-transfer/resume') {
      state.speech = 'downloading';
      if (state.loseAck) return route.fulfill({ status: 503, json: { error: 'Acknowledgment unavailable' } });
    }
    if (path === '/api/v2/models/operations/speech-transfer/cancel') state.speech = 'cancelled';
    if (path === '/api/v2/models/operations/speech-transfer/discard') state.discarded = true;
    if (path === '/api/v2/models/operations') {
      state.reads++;
      if (state.failRead) return route.fulfill({ status: 503, json: { error: 'Transfer status unavailable' } });
    }
    const data: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: 'transfer-continuity', capabilities: [] },
      '/v1/system/config': { require_auth: true }, '/v1/users/me': { authenticated: true, user: { id: 'fixture', role } },
      '/v1/models': { data: [{ id: 'local-chat', type: 'llm' }] }, '/v1/catalog': { models: [] },
      '/v1/sessions': { sessions: [] }, '/api/v2/jobs': [], '/api/v2/models': { models: [] },
      '/api/v2/models/catalog': { models: [], repositories: [] }, '/v1/search': { results: [] },
      '/v1/models/download/progress': { 'retained-chat.gguf': { model_id: 'retained-chat', file_name: 'retained-chat.gguf', status: state.legacy, bytes_done: 2048, bytes_total: 8192, percent: 25, speed: 0, repository: 'publisher/exact-source', source_file: 'weights/chat.Q4.gguf', quantization: 'Q4', enable_knowledge: false } },
      '/api/v2/models/operations': { operations: state.discarded ? [] : [{ id: 'speech-transfer', request_id: 'once', state: state.speech, bytes_done: 2048, bytes_total: 8192, retained_bytes: 2048, artifacts: [], provenance: { repository: 'publisher/speech', revision: 'pinned' }, target: { kind: 'package', package: { id: 'speech-exact', revision: 'r1', name: 'Speech transfer with a deliberately long readable name', capabilities: ['transcription'] } } }] },
    };
    return route.fulfill({ json: data[path] ?? {} });
  });
  return state;
}

test('GGUF progress and explicit recovery survive every model category and route return', async ({ page }, info) => {
  const state = await fixture(page);
  await page.goto('/ui/#/models');
  const downloads = page.locator('.model-transfers');
  const transfers = downloads.locator('article').filter({ has: page.getByRole('heading', { name: 'retained-chat.gguf', exact: true }) });
  await expect(downloads).toHaveCount(1);
  await expect(downloads.locator(':scope > summary')).toHaveText('Downloads · 2');
  for (const category of ['language', 'embeddings', 'speech_recognition', 'speech_generation']) {
    await page.locator('.model-category-filter').selectOption(category);
    await expect(transfers).toContainText('retained-chat.gguf');
    await expect(transfers.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Discover models', exact: true }).click();
    await expect(transfers.getByRole('progressbar')).toBeVisible();
    await page.getByRole('button', { name: 'Search Hugging Face', exact: true }).click();
    await expect(transfers.getByRole('progressbar')).toBeVisible();
  }
  expect(state.mutations).toEqual([]);
  await transfers.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect.poll(() => state.legacy).toBe('cancelled');
  await expect(downloads).toHaveAttribute('open', '');
  await expect(transfers).toContainText('Downloaded data is kept');
  await transfers.getByRole('button', { name: 'Resume', exact: true }).dblclick();
  await expect.poll(() => state.mutations.length).toBe(2);
  expect(state.mutations[1]).toEqual({ path: '/v1/models/download', body: { model_id: 'retained-chat', repository: 'publisher/exact-source', file_name: 'weights/chat.Q4.gguf', quantization: 'Q4', enable_knowledge: false } });
  await page.locator('.sidebar a[href="#/chat"]').click();
  await page.locator('.sidebar a[href="#/models"]').click();
  await expect(transfers.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
  expect(state.mutations).toHaveLength(2);
  await page.setViewportSize({ width: 390, height: 650 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('transfers-narrow.png'), fullPage: true });
});

test('package progress stays visible without duplicate controls through filters, read failure and discard', async ({ page }, info) => {
  const state = await fixture(page);
  await page.goto('/ui/#/models');
  const card = page.locator('[data-operation-id="speech-transfer"]');
  for (const category of ['language', 'speech_recognition', 'embeddings', 'speech_generation']) {
    await page.locator('.model-category-filter').selectOption(category);
    await expect(card).toHaveCount(1);
    await expect(card).toContainText('speech-exact · r1');
    await expect(card.getByRole('button', { name: 'Resume', exact: true })).toBeVisible();
  }
  state.failRead = true;
  await expect(page.getByRole('alert').filter({ hasText: 'Transfer status unavailable' })).toBeVisible();
  await expect(card).toContainText('Retained data: 2 KB');
  expect(state.mutations).toEqual([]);
  state.failRead = false;
  await page.getByRole('alert').getByRole('button', { name: 'Check saved state', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Transfer status unavailable' })).toHaveCount(0);
  await card.getByRole('button', { name: 'Discard partial data', exact: true }).click();
  expect(state.mutations).toEqual([]);
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(card).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.screenshot({ path: info.outputPath('transfers-desktop.png'), fullPage: true });
  await card.getByRole('button', { name: 'Discard partial data', exact: true }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Discard partial data', exact: true }).click();
  await expect(card).toHaveCount(0);
  expect(state.mutations.map(item => item.path)).toEqual(['/api/v2/models/operations/speech-transfer/discard']);
});

test('lost package-control acknowledgment reconciles by reading without replay', async ({ page }) => {
  const state = await fixture(page); state.loseAck = true;
  await page.goto('/ui/#/models');
  const card = page.locator('[data-operation-id="speech-transfer"]');
  await card.getByRole('button', { name: 'Resume', exact: true }).dblclick();
  await expect(page.getByRole('alert').filter({ hasText: 'Acknowledgment unavailable' })).toBeVisible();
  await page.getByRole('alert').getByRole('button', { name: 'Check saved state', exact: true }).click();
  await expect(card.getByRole('button', { name: 'Cancel', exact: true })).toBeVisible();
  await page.locator('.model-category-filter').selectOption('speech_recognition');
  await expect(card).toHaveCount(1);
  expect(state.mutations.map(item => item.path)).toEqual(['/api/v2/models/operations/speech-transfer/resume']);
});

test('ordinary members do not read administrative transfer history or gain controls', async ({ page }) => {
  const state = await fixture(page, 'user');
  await page.goto('/ui/#/models');
  for (const category of ['language', 'speech_recognition', 'speech_generation', 'embeddings']) {
    await page.locator('.model-category-filter').selectOption(category);
    await expect(page.getByRole('region', { name: 'Installed', exact: true })).toBeVisible();
  }
  await expect(page.locator('[data-operation-id]')).toHaveCount(0);
  expect(state.reads).toBe(0); expect(state.mutations).toEqual([]);
});

for (const locale of ['en', 'ar']) for (const theme of ['light', 'dark']) test(`one transfer surface reflows with long names: ${locale} ${theme}`, async ({ page }, info) => {
  const state = await fixture(page);
  await page.addInitScript(({ locale, theme }) => { localStorage.setItem('offgrid.locale', locale); localStorage.setItem('offgrid.theme', theme); }, { locale, theme });
  await page.goto('/ui/#/models');
  await page.locator('.model-category-filter').selectOption('speech_generation');
  const downloads = page.locator('.model-transfers');
  await expect(downloads).toHaveCount(1);
  await expect(downloads.locator('article')).toHaveCount(2);
  for (const [width, height] of [[1280, 650], [390, 650], [320, 500]]) {
    await page.setViewportSize({ width, height });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await downloads.scrollIntoViewIfNeeded();
    await page.screenshot({ path: info.outputPath(`transfers-${width}.png`), fullPage: true });
  }
  expect(state.mutations).toEqual([]);
});

test('an expanded transfer stays open as the observed operation becomes cancelled', async ({ page }) => {
  const state = await fixture(page); state.legacy = 'complete'; state.speech = 'downloading';
  await page.goto('/ui/#/models');
  const downloads = page.locator('.model-transfers');
  const card = page.locator('[data-operation-id="speech-transfer"]');
  await expect(downloads).toHaveAttribute('open', '');
  await card.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(card.getByRole('button', { name: 'Resume', exact: true })).toBeVisible();
  await expect(downloads).toHaveAttribute('open', '');
  await downloads.locator(':scope > summary').click();
  await page.locator('.model-category-filter').selectOption('embeddings');
  await expect(downloads).not.toHaveAttribute('open', '');
  expect(state.mutations.map(item => item.path)).toEqual(['/api/v2/models/operations/speech-transfer/cancel']);
});
