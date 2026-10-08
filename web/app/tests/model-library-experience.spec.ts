import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page, role = 'admin') {
  const state = { searches: [] as string[], mutations: [] as string[], calls: [] as string[] };
  const pkg = (id: string, category: string) => ({ id, name: id, kind: 'package', category, revision: 'r1', provenance: { kind: 'import' }, readiness: {}, package: { installed: true, integrity: 'checked', runtime_compatible: false, issue: 'Matching runtime is unavailable', manifest: { artifacts: [{ size: 4096 }], runtime: { adapter: 'fixture', revision: 'test' }, license: 'MIT' } } });
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, route => {
    const url = new URL(route.request().url()), path = url.pathname;
    state.calls.push(path);
    if (route.request().method() !== 'GET') state.mutations.push(path);
    if (path === '/v1/search' || (path === '/api/v2/models/catalog' && url.searchParams.has('q'))) state.searches.push(url.searchParams.get('query') ?? url.searchParams.get('q')!);
    const bodies: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: 'library-fixture', capabilities: [] },
      '/v1/system/config': { require_auth: true }, '/v1/users/me': { authenticated: true, auth_required: true, user: { id: 'reader', username: 'reader', role } },
      '/v1/models': { data: [{ id: 'fixture-chat', type: 'llm', size: 2048 }, { id: 'fixture-embed', type: 'embedding' }, { id: 'must-not-be-chat', type: 'asr' }] },
      '/v1/catalog': { models: [] }, '/v1/models/download/progress': {}, '/api/v2/jobs': [], '/v1/sessions': { sessions: [] },
      '/api/v2/models': { models: [pkg('recognition-fixture', 'speech_recognition'), pkg('synthesis-fixture', 'speech_generation')] },
      '/api/v2/models/operations': { operations: [] }, '/api/v2/models/packages': { packages: [] },
      '/api/v2/models/catalog': { models: [], repositories: [{ id: 'publisher/speech', size_bytes: 8192 }] },
      '/v1/search': { results: [{ id: 'publisher/chat', name: 'Chat', author: 'publisher' }] },
    };
    return route.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}

for (const theme of ['light', 'dark']) test(`library selection remains visible in ${theme} after shared controls and hover`, async ({ page }) => {
  const state = await fixture(page);
  await page.addInitScript(theme => localStorage.setItem('offgrid.theme', theme), theme);
  await page.goto('/ui/#/models');
  const installed = page.getByRole('tab', { name: 'Installed', exact: true });
  const discover = page.getByRole('tab', { name: 'Discover models', exact: true });
  const background = (control: typeof installed) => control.evaluate(el => getComputedStyle(el).backgroundColor);
  const border = (control: typeof installed) => control.evaluate(el => getComputedStyle(el).borderColor);
  await expect(installed).toHaveAttribute('aria-selected', 'true');
  await expect.poll(async () => (await background(installed)) !== (await background(discover))).toBe(true);
  await installed.hover();
  await expect.poll(async () => (await background(installed)) !== (await background(discover))).toBe(true);
  await discover.click(); await page.mouse.move(0, 0);
  await expect(discover).toHaveAttribute('aria-selected', 'true');
  await expect.poll(async () => (await background(discover)) !== (await background(installed))).toBe(true);
  const language = page.getByRole('button', { name: 'Language', exact: true });
  const embeddings = page.getByRole('button', { name: 'Embeddings', exact: true });
  await embeddings.click(); await page.mouse.move(0, 0);
  await expect(embeddings).toHaveAttribute('aria-pressed', 'true');
  await expect.poll(async () => (await border(language)) !== (await border(embeddings))).toBe(true);
  await embeddings.hover();
  await expect.poll(async () => (await border(language)) !== (await border(embeddings))).toBe(true);
  expect(state.mutations).toEqual([]);
});

test('all four categories keep Installed local and Discover explicit, with retained independent filters', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/models');
  for (const [category, name] of [['Language', 'fixture-chat'], ['Embeddings', 'fixture-embed'], ['Speech recognition', 'recognition-fixture'], ['Speech generation', 'synthesis-fixture']]) {
    await page.getByRole('button', { name: category, exact: true }).click();
    await page.getByRole('tab', { name: 'Installed', exact: true }).click();
    const inventory = page.getByRole('region', { name: 'Installed', exact: true });
    await expect(inventory).toContainText(name); await expect(inventory).not.toContainText('must-not-be-chat');
    await expect(page.getByRole('searchbox')).toHaveCount(1);
    await page.getByRole('searchbox').fill('absent');
    await expect(inventory).toContainText('No models match this filter');
    await expect(inventory).not.toContainText('No local models');
    await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
    await expect(page.getByRole('searchbox')).toHaveCount(1);
    await page.getByRole('searchbox').fill(category);
    expect(state.searches).toEqual([]);
    await expect(page.getByText('No catalog models in this category.', { exact: false })).toBeVisible();
    await page.getByRole('tab', { name: 'Installed', exact: true }).click();
    await expect(page.getByRole('searchbox')).toHaveValue('absent');
    await page.getByRole('searchbox').fill('');
  }
  await expect(page.locator('.package-card')).toContainText('Matching runtime is unavailable');
  expect(state.mutations).toEqual([]);
});

test('navigation keeps the reviewed intent without online re-search and tabs support keyboard', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/models');
  await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
  const installed = page.getByRole('tab', { name: 'Installed', exact: true });
  await installed.focus(); await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('tab', { name: 'Discover models', exact: true })).toBeFocused();
  await page.getByRole('searchbox').fill('speech'); await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'publisher/speech', exact: true })).toBeVisible();
  await page.locator('.primary-nav a[href="#/chat"]').click(); await page.locator('.primary-nav a[href="#/models"]').click();
  await expect(page.getByRole('searchbox')).toHaveValue('speech');
  await expect(page.getByRole('heading', { name: 'publisher/speech', exact: true })).toBeVisible();
  expect(state.searches).toEqual(['speech']); expect(state.mutations).toEqual([]);
});

test('non-administrators cannot initiate discovery transfers or package administration', async ({ page }) => {
  const state = await fixture(page, 'user'); await page.goto('/ui/#/models');
  await expect(page.getByRole('button', { name: 'Delete', exact: true })).toBeDisabled();
  await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
  await expect(page.getByText('Administrator access required', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Speech generation', exact: true }).click();
  await page.getByRole('searchbox').fill('voice');
  await expect(page.getByRole('button', { name: 'Search', exact: true })).toBeDisabled();
  expect(state.calls).not.toContain('/api/v2/models/operations');
  expect(state.mutations).toEqual([]);
});

test('curated legacy review keeps focus and downloads only after the explicit decision', async ({ page }) => {
  const state = await fixture(page);
  await page.route('**/v1/catalog', route => route.fulfill({ json: { models: [{ id: 'review-chat', name: 'Review chat', type: 'llm', repo: 'publisher/chat', file: 'model.Q4.gguf', size_bytes: 4096, license: 'MIT' }] } }));
  await page.goto('/ui/#/models'); await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
  const card = page.locator('article').filter({ has: page.getByRole('heading', { name: 'Review chat', exact: true }) });
  const review = card.getByRole('button', { name: 'Review download', exact: true });
  await review.focus(); await page.keyboard.press('Enter');
  await expect(card.getByRole('button', { name: 'Download', exact: true })).toBeFocused();
  await expect(card).toContainText('disk-space preflight'); expect(state.mutations).toEqual([]);
  await expect(card.getByRole('link', { name: 'Model card and license' })).toHaveAttribute('href', 'https://huggingface.co/publisher/chat');
  await card.getByRole('button', { name: 'Download', exact: true }).dblclick();
  expect(state.mutations.filter(path => path === '/v1/models/download')).toHaveLength(1);
});

test('curated package alternatives resolve the selected identity in place, never transfer on review', async ({ page }) => {
  const state = await fixture(page);
  const manifest = (id: string) => ({ id, name: id, revision: 'r1', license: 'MIT', artifacts: [] });
  await page.route('**/api/v2/models/catalog?*', route => route.fulfill({ json: { models: [{ id: 'voice-collection', name: 'Voice collection', category: 'speech_generation', description: 'Two reviewed profiles', license: 'MIT', variants: ['small', 'large'].map((id, i) => ({ id, name: id, size_bytes: (i + 1) * 4096, provenance: { repository: 'publisher/voice' }, target: { kind: 'package', package: manifest(id) } })) }], repositories: [] } }));
  const resolutions: string[] = [];
  await page.route('**/api/v2/models/resolve', route => {
    const id = route.request().postDataJSON().catalog_id; resolutions.push(id);
    return route.fulfill({ json: { id: `resolution-${id}`, preflight: { transfer_bytes: id === 'small' ? 4096 : 8192, required_free_bytes: 16384, warnings: ['Fixture runtime unavailable'] }, resolution: { manifest: manifest(id), provenance: { repository: 'publisher/voice', revision: 'r1' } } } });
  });
  await page.goto('/ui/#/models'); await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
  await page.getByRole('button', { name: 'Speech generation', exact: true }).click();
  const card = page.locator('article').filter({ has: page.getByRole('heading', { name: 'Voice collection', exact: true }) });
  await card.getByRole('button', { name: 'Review download' }).click();
  expect(resolutions).toEqual([]); await expect(card.getByRole('button', { name: 'Download', exact: true })).toHaveCount(0);
  await card.getByRole('combobox').selectOption('large');
  await expect(card.locator('.package-download-details')).toContainText('8 KB');
  await expect(card).toContainText('Required free space'); expect(resolutions).toEqual(['large']); expect(state.mutations).toEqual([]);
  await card.getByRole('combobox').selectOption('small');
  await expect(card.locator('.package-download-details')).toContainText('4 KB');
  expect(resolutions).toEqual(['large', 'small']); expect(state.mutations).toEqual([]);
  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 600 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
});

test('failed operations retain the exact model and safe recovery across library and route changes', async ({ page }) => {
  const state = await fixture(page);
  let failRead = false;
  const operation = { id: 'operation-fixture', request_id: 'once', state: 'interrupted', bytes_done: 2048, bytes_total: 8192, retained_bytes: 2048, artifacts: [], message: 'Fixture: transfer was interrupted', provenance: { repository: 'publisher/speech', revision: 'pinned' }, target: { kind: 'package', package: { id: 'recognition-fixture', revision: 'r1', name: 'Recognition download', capabilities: ['transcription'] } } };
  await page.route('**/api/v2/models/operations', route => failRead ? route.fulfill({ status: 503, json: { error: 'Operation status offline' } }) : route.fulfill({ json: { operations: [operation] } }));
  await page.goto('/ui/#/models'); await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
  const card = page.locator('[data-operation-id="operation-fixture"]');
  await expect(card).toContainText('recognition-fixture · r1');
  await expect(card.getByRole('status')).toContainText('Interrupted');
  await expect(card.getByRole('button', { name: 'Resume', exact: true })).toBeEnabled();
  await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
  await expect(card).toBeVisible();
  await page.locator('.primary-nav a[href="#/chat"]').click(); await page.locator('.primary-nav a[href="#/models"]').click();
  await expect(card).toContainText('Retained data: 2 KB'); expect(state.mutations).toEqual([]);
  failRead = true;
  await page.getByRole('tab', { name: 'Installed', exact: true }).click();
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Operation status offline');
  await expect(page.getByRole('region', { name: 'Installed', exact: true })).toContainText('Matching runtime is unavailable');
  await expect(card).toContainText('Fixture: transfer was interrupted');
  expect(state.mutations).toEqual([]);
});

test('legacy verification remains attached to its model after navigating away and back', async ({ page }) => {
  await fixture(page); let checks = 0;
  await page.route('**/v1/models/verify?*', route => {
    checks++;
    return route.fulfill({ json: { verified: true, file_name: 'fixture-chat', message: 'Fixture integrity evidence', sha256: 'a'.repeat(64) } });
  });
  await page.goto('/ui/#/models'); await page.getByRole('button', { name: 'Verify', exact: true }).click();
  await expect(page.locator('.installed-model')).toContainText('Fixture integrity evidence');
  await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
  await expect(page.getByText('Fixture integrity evidence')).toHaveCount(0);
  await page.locator('.primary-nav a[href="#/chat"]').click(); await page.locator('.primary-nav a[href="#/models"]').click();
  await page.getByRole('tab', { name: 'Installed', exact: true }).click();
  await expect(page.locator('.installed-model')).toContainText('Fixture integrity evidence'); expect(checks).toBe(1);
});
