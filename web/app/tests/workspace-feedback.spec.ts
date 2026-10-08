import { expect, test, type Page } from '@playwright/test';
import { locales } from '../src/i18n';
import { operationFeedback } from '../src/i18n/operation-feedback';

// UI contract fixtures only: no Hub downloads, microphone or native input.
async function fixture(page: Page) {
  await page.addInitScript(() => {
    if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  const state = { inventory: 0, searches: [] as string[], writes: [] as string[] };
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, route => {
    const url = new URL(route.request().url()), path = url.pathname;
    if (route.request().method() === 'POST') state.writes.push(path);
    if (path === '/api/v2/models') state.inventory++;
    if (path === '/api/v2/models/catalog' && url.searchParams.has('q')) state.searches.push(url.searchParams.get('q')!);
    const bodies: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', version: 'test', api_version: 2, workspace_id: 'feedback' },
      '/v1/system/config': { version: 'test', require_auth: false },
      '/v1/users/me': { authenticated: false, user: null },
      '/v1/models': { data: [{ id: 'chat-fixture', type: 'chat' }] },
      '/v1/catalog': { models: [] }, '/v1/models/download/progress': {},
      '/v1/sessions': { sessions: [] }, '/v1/rag/status': { enabled: false },
      '/api/v2/models': { models: [] }, '/api/v2/models/operations': { operations: [] },
      '/api/v2/models/catalog': { models: [], repositories: [{ id: 'fixture/whisper', size_bytes: 1000 }] },
      '/api/v2/models/packages': { packages: [] }, '/api/v2/computer/status': { available: false },
    };
    return route.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}

async function models(page: Page) {
  await page.goto('/ui/#/models');
  await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Installed', exact: true })).toHaveAttribute('aria-busy', 'false');
  await page.getByRole('tab', { name: 'Discover models', exact: true }).click();
}

test('search retry preserves the failed query and survives unrelated inventory polling', async ({ page }) => {
  const state = await fixture(page);
  let failed = false;
  await page.route('**/api/v2/models/catalog?*', route => {
    const query = new URL(route.request().url()).searchParams.get('q');
    if (query === null) return route.fallback();
    state.searches.push(query);
    if (!failed) { failed = true; return route.fulfill({ status: 503, json: { error: 'Search temporarily offline' } }); }
    return route.fulfill({ json: { models: [], repositories: [{ id: 'fixture/recovered' }] } });
  });
  await models(page);
  await page.getByRole('searchbox').fill('qwe');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Search temporarily offline');
  const before = state.inventory;
  await expect.poll(() => state.inventory).toBeGreaterThan(before);
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Retry search' })).toBeVisible();
  await page.getByRole('button', { name: 'Retry search' }).click();
  await expect(page.getByRole('heading', { name: 'fixture/recovered' })).toBeVisible();
  expect(state.searches).toEqual(['qwe', 'qwe']);
  expect(state.writes).toEqual([]);
});

test('catalog, inventory and metadata recover independently without unrelated errors disappearing', async ({ page }) => {
  await fixture(page);
  let catalogFailed = true, inventoryFailed = true;
  await page.route('**/api/v2/models?*', route => inventoryFailed
    ? route.fulfill({ status: 503, json: { error: 'Inventory offline' } }) : route.fallback());
  await page.route('**/api/v2/models/catalog?*', route => {
    if (new URL(route.request().url()).searchParams.has('q')) return route.fulfill({ status: 503, json: { error: 'Search offline' } });
    return catalogFailed ? route.fulfill({ status: 503, json: { error: 'Catalog offline' } }) : route.fallback();
  });
  await models(page);
  await expect(page.getByRole('alert').filter({ hasText: 'Inventory offline' })).toBeVisible();
  await expect(page.getByRole('alert').filter({ hasText: 'Catalog offline' })).toBeVisible();
  await page.getByRole('searchbox').fill('test');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Search offline' })).toBeVisible();
  inventoryFailed = false;
  await page.getByRole('alert').filter({ hasText: 'Inventory offline' }).getByRole('button').click();
  await expect(page.getByRole('alert').filter({ hasText: 'Inventory offline' })).toHaveCount(0);
  catalogFailed = false;
  await page.getByRole('button', { name: 'Refresh catalog' }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Catalog offline' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Retry search' })).toBeVisible();
});

test('edited queries and category changes discard late search results without failure banners', async ({ page }) => {
  await fixture(page);
  const pending: (() => Promise<void>)[] = [];
  await page.route('**/api/v2/models/catalog?*', async route => {
    const q = new URL(route.request().url()).searchParams.get('q');
    if (!q) return route.fallback();
    if (q === 'old') {
      await new Promise<void>(resolve => pending.push(async () => {
        await route.fulfill({ json: { models: [], repositories: [{ id: 'fixture/stale' }] } }); resolve();
      }));
    } else await route.fulfill({ json: { models: [], repositories: [{ id: 'fixture/new' }] } });
  });
  await models(page);
  await page.getByRole('searchbox').fill('old');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect.poll(() => pending.length).toBe(1);
  await page.getByRole('searchbox').fill('new');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'fixture/new' })).toBeVisible();
  await pending[0]();
  await expect(page.getByRole('heading', { name: 'fixture/stale' })).toHaveCount(0);
  await page.getByRole('searchbox').fill('old');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect.poll(() => pending.length).toBe(2);
  await page.getByRole('button', { name: 'Speech generation', exact: true }).click();
  await pending[1]();
  await expect(page.getByRole('heading', { name: 'fixture/stale' })).toHaveCount(0);
  await expect(page.getByRole('alert')).toHaveCount(0);
});

test('repository retry repeats inspection only and mutation recovery checks saved state', async ({ page }) => {
  const state = await fixture(page);
  let inspections = 0, resolutions = 0, installs = 0;
  const manifest = { id: 'fixture-whisper', name: 'Fixture', artifacts: [], license: 'MIT' };
  await page.route('**/api/v2/models/discover?*', route => {
    expect(new URL(route.request().url()).searchParams.get('repository')).toBe('fixture/whisper');
    inspections++;
    if (inspections === 1) return route.fulfill({ status: 503, json: { error: 'Inspection offline' } });
    return route.fulfill({ json: { repository: 'fixture/whisper', revision: 'r1', choices: [{ id: 'whisper', name: 'Whisper', architecture: 'whisper', supported: true }] } });
  });
  await page.route('**/api/v2/models/resolve', route => {
    resolutions++;
    return route.fulfill({ json: { id: 'resolution', preflight: { transfer_bytes: 1000, required_free_bytes: 2000 }, resolution: { manifest, provenance: { repository: 'fixture/whisper' } } } });
  });
  await page.route('**/api/v2/models/operations', route => {
    if (route.request().method() !== 'POST') return route.fallback();
    installs++;
    return route.fulfill({ status: 503, json: { error: 'Acceptance unknown' } });
  });
  await models(page);
  await page.getByRole('searchbox').fill('whisper');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await page.getByRole('button', { name: 'Review download', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Inspection offline');
  await page.getByRole('button', { name: 'Retry model review' }).click();
  await expect(page.getByRole('button', { name: 'Download', exact: true })).toBeEnabled();
  expect(inspections).toBe(2); expect(resolutions).toBe(1); expect(installs).toBe(0);
  await page.getByRole('button', { name: 'Download', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Acceptance unknown');
  const before = state.inventory;
  await page.getByRole('button', { name: 'Check saved state' }).click();
  await expect.poll(() => state.inventory).toBeGreaterThan(before);
  expect(installs).toBe(1);
  await expect(page.getByRole('button', { name: 'Retry model review' })).toHaveCount(0);
});

test('Knowledge checking, failed check and setup are distinct; unused Knowledge does not block chat', async ({ page }) => {
  await fixture(page);
  let release!: () => void, fail = true;
  const held = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/v1/rag/status', async route => {
    if (fail) { await held; await route.fulfill({ status: 503, json: { error: 'Offline' } }); }
    else await route.fulfill({ json: { enabled: false } });
  });
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByText('Checking Knowledge…', { exact: true })).toBeVisible();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).toBeDisabled();
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill('Ordinary question');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  release();
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByText('Could not check Knowledge.', { exact: true })).toBeVisible();
  fail = false;
  await page.getByRole('button', { name: 'Check Knowledge again' }).click();
  await expect(page.getByRole('link', { name: 'Set up Knowledge' })).toHaveAttribute('href', '#/knowledge');
  await expect(page.locator('.chat-workspace [role="alert"]')).toHaveCount(0);
  await expect(page.getByText('Could not check Knowledge.', { exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
});

test('superseded Knowledge reads cannot replace readiness; selected Knowledge is not silently cleared', async ({ page }) => {
  await fixture(page);
  const pending: (() => Promise<void>)[] = [];
  let mode: 'hold' | 'ready' | 'unavailable' = 'hold';
  await page.route('**/v1/rag/status', async route => {
    if (mode === 'hold') await new Promise<void>(resolve => { pending.push(async () => { await route.fulfill({ json: { enabled: false } }); resolve(); }); });
    else await route.fulfill({ json: { enabled: mode === 'ready' } });
  });
  await page.goto('/ui/#/chat');
  await expect.poll(() => pending.length).toBeGreaterThan(0);
  mode = 'ready';
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  const knowledge = page.getByRole('checkbox', { name: 'Use knowledge base' });
  await expect(knowledge).toBeEnabled(); await knowledge.focus(); await page.keyboard.press('Space');
  for (const finish of pending) await finish();
  await expect(knowledge).toBeChecked();
  await expect(page.getByRole('link', { name: 'Set up Knowledge' })).toHaveCount(0);
  mode = 'unavailable';
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.locator('.composer > [role="status"]')).toBeVisible();
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Set up Knowledge' })).toBeVisible();
  await expect(knowledge).toBeChecked();
  await expect(knowledge).toBeEnabled(); // User can turn it off; never silently downgrade.
});

test('connected-service copy and new feedback exist in all nine locales', () => {
  for (const locale of Object.keys(locales) as (keyof typeof locales)[]) {
    const text = locales[locale].messages;
    expect(text.common.serviceDetails.trim()).not.toBe('');
    expect(text.chat.emptyBody.trim()).not.toBe('');
    expect(Object.values(operationFeedback(locale)).every(value => value.trim().length > 0)).toBe(true);
    if (locale !== 'en') expect(operationFeedback(locale).checkingVoice).not.toBe(operationFeedback('en').checkingVoice);
  }
});

test('Settings groups preferences and service identity without claiming desktop paths belong to the container', async ({ page }) => {
  const state = await fixture(page);
  await page.addInitScript(() => {
    (window as any).electron = {
      onThemeChange: () => () => {}, getSystemTheme: async () => 'light',
      getPaths: async () => ({ models: 'C:\\desktop-only\\models', data: 'C:\\desktop-only\\data' }),
      getBackendInfo: async () => ({ state: 'ready', url: 'http://127.0.0.1:11611', managedByDesktop: false, desktopVersion: 'desktop-test', reason: 'Compatibility fixture' }),
    };
  });
  await page.goto('/ui/#/settings');
  await expect(page.getByRole('heading', { name: 'Preferences', exact: true })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Workspace', exact: true }).getByText('test', { exact: true })).toHaveCount(1);
  await expect(page.getByText('http://127.0.0.1:11611', { exact: true })).toBeVisible();
  await expect(page.getByText('Compatibility fixture', { exact: true })).toBeVisible();
  await expect(page.getByText('C:\\desktop-only\\models', { exact: true })).toBeHidden();
  await page.getByText('Diagnostics', { exact: true }).click();
  await expect(page.getByText('C:\\desktop-only\\models', { exact: true })).toBeVisible();
  await expect(page.locator('.desktop-local-paths')).toContainText('not its workspace paths');
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  expect(state.writes).toEqual([]);
});

for (const locale of ['en', 'ar'] as const) for (const theme of ['light', 'dark']) {
  test(`service context is discoverable without client-local claims: ${locale}, ${theme}`, async ({ page }, info) => {
    await fixture(page);
    await page.addInitScript(({ locale, theme }) => { localStorage.setItem('offgrid.locale', locale); localStorage.setItem('offgrid.theme', theme); }, { locale, theme });
    await page.goto('/ui/#/chat');
    const copy = locales[locale].messages;
    await expect(page.getByText(copy.chat.emptyBody, { exact: true })).toBeVisible();
    await expect(page.locator('.sidebar-foot')).not.toContainText('100%');
    await expect(page.getByRole('link', { name: copy.common.serviceDetails, exact: true })).toHaveCount(0);
    await page.locator('.primary-nav').getByRole('link', { name: copy.nav.settings, exact: true }).click();
    await expect(page.locator('.settings-page').getByText(new URL(page.url()).origin, { exact: true })).toBeVisible();
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`service-${locale}-${theme}.png`), fullPage: true });
  });
}
