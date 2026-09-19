import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page, locale = 'en', theme = 'dark') {
  await page.addInitScript(({ locale, theme }) => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.locale', locale);
    localStorage.setItem('offgrid.theme', theme);
  }, { locale, theme });
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', route => route.fulfill({ json: { product: 'offgrid', version: 'fixture', api_version: 2, ui_build_id: 'a'.repeat(64), capabilities: [] } }));
  await page.route('**/v1/**', route => {
    const path = new URL(route.request().url()).pathname;
    const data: Record<string, unknown> = {
      '/v1/users/me': { authenticated: false, user: null },
      '/v1/models': { data: [{ id: 'test-model-with-a-long-name.Q4_K_M', type: 'chat' }] },
      '/v1/sessions': { sessions: [] },
      '/v1/catalog': { models: [] },
      '/v1/documents': { count: 1, documents: [{ id: 'source', name: 'A document with a long title for layout checks', size: 10, chunk_count: 1, source_retained: true }] },
      '/v1/rag/status': { enabled: true, embedding_model: 'embedding-model-with-a-long-name', stats: {} },
      '/v1/agents/tasks': [], '/v1/agents/tools': { tools: [], total: 0, enabled_count: 0 },
      '/v1/agents/mcp': { servers: [] }, '/v1/integrations': { integrations: [] }
    };
    return route.fulfill({ json: data[path] ?? {} });
  });
}

test('search, task and document controls share sizing and keyboard focus', async ({ page }) => {
  await fixture(page);
  await page.goto('/ui/#/models');
  const query = page.locator('#model-search-query');
  await expect(query).toBeVisible();
  await expect(query).toHaveCSS('min-height', '40px');
  await expect(query).toHaveCSS('border-radius', '8px');
  const search = page.locator('.model-search-form button');
  await expect(search).toBeDisabled();
  await expect(search).toHaveCSS('cursor', 'not-allowed');
  await query.fill('BGE');
  await expect(search).toBeEnabled();
  await query.press('Tab');
  await expect(search).toBeFocused();
  await expect(search).toHaveCSS('outline-width', '2px');
  const [inputBox, buttonBox] = await Promise.all([query.boundingBox(), search.boundingBox()]);
  expect(inputBox!.height).toBe(buttonBox!.height);
  await page.goto('/ui/#/agents');
  await expect(page.locator('.task-card select').first()).toHaveCSS('min-height', '40px');
  await expect(page.locator('.task-card textarea')).toHaveCSS('border-radius', '8px');
  await page.goto('/ui/#/knowledge');
  const controls = page.locator('.section-actions .action-group button');
  await expect(controls).toHaveCount(3);
  for (const control of await controls.all()) await expect(control).toHaveCSS('min-height', '40px');
  await expect(page.locator('.knowledge-workspace > button')).toHaveCount(0);
});

for (const profile of [
  { width: 1280, locale: 'en', theme: 'dark' },
  { width: 320, locale: 'en', theme: 'light' },
  { width: 390, locale: 'de', theme: 'light' },
  { width: 390, locale: 'ar', theme: 'dark' }
]) test(`controls stay contained at ${profile.width}px ${profile.locale} ${profile.theme}`, async ({ page }, testInfo) => {
  await page.setViewportSize({ width: profile.width, height: 900 });
  await fixture(page, profile.locale, profile.theme);
  for (const route of ['models', 'knowledge', 'agents', 'settings']) {
    await page.goto(`/ui/#/${route}`);
    await expect(page.locator('.page-content')).toBeVisible();
    await expect(page.locator('.page-failure')).toHaveCount(0);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const overflow = await page.locator('.page-content button:visible, .page-content input:visible, .page-content select:visible, .page-content textarea:visible').evaluateAll(elements => elements.filter(element => {
      const box = element.getBoundingClientRect();
      return box.left < -1 || box.right > innerWidth + 1;
    }).map(element => element.className || element.tagName));
    expect(overflow).toEqual([]);
    if (route === 'agents') {
      await page.locator('#agent-connections-tab').click();
      await expect(page.locator('.connector-panel')).toBeVisible();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    await page.screenshot({ path: testInfo.outputPath(`${route}.png`), fullPage: true });
  }
});
