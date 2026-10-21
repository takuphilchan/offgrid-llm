import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page, theme = 'light') {
  await page.addInitScript(theme => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.theme', theme); }, theme);
  const writes: string[] = [];
  const session = { name: 'Research notes', model_id: 'local-model', messages: [{ role: 'user', content: 'Explain the findings' }, { role: 'assistant', content: Array.from({ length: 32 }, (_, i) => `### Finding ${i + 1}\n\nEvidence and a useful explanation of the result.`).join('\n\n') }], updated_at: '2026-10-15T00:00:00Z' };
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, r => {
    const path = decodeURIComponent(new URL(r.request().url()).pathname);
    if (r.request().method() !== 'GET') writes.push(path);
    const responses: Record<string, unknown> = {
      '/v1/users/me': { authenticated: false, auth_required: false, guest: true },
      '/v1/system/config': { require_auth: false },
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: 'refinement', capabilities: ['agent-profiles-v1', 'task-first-agents-v2'] },
      '/v1/models': { data: [{ id: 'local-model', type: 'chat', size: 1024 ** 3 }] },
      '/v1/sessions': { sessions: [session] }, '/v1/sessions/Research notes': session,
      '/v1/rag/status': { enabled: true, stats: {} }, '/v1/documents': { documents: [] },
      '/v1/catalog': { models: [] }, '/api/v2/models': { models: [] }, '/api/v2/models/operations': { operations: [] },
      '/api/v2/agent-profiles': { profiles: [] }, '/api/v2/agent-resources': { tools: {}, computer_permitted: false },
      '/api/v2/jobs': [], '/v1/runs': { runs: [] }, '/v1/stats': { server: { uptime: '1m' } },
      '/v1/audio/status': { profiles: [] },
    };
    return r.fulfill({ json: path.endsWith('/turn') ? { turn: null } : responses[path] ?? {} });
  });
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer textarea')).toBeVisible();
  return writes;
}

test('one contextual sidebar preserves draft and selection through drawer transitions', async ({ page }) => {
  const writes = await fixture(page);
  await expect(page.locator('.sidebar')).toHaveCSS('width', '248px');
  await expect(page.locator('.sidebar #conversation-history')).toHaveCount(1);
  await expect(page.locator('.chat-workspace #conversation-history')).toHaveCount(0);
  const editor = page.locator('.composer textarea');
  await editor.fill('Keep café 日本語');
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator('.sidebar')).toBeHidden();
  const trigger = page.getByRole('button', { name: 'Primary navigation', exact: true });
  await trigger.click();
  await expect(page.locator('.sidebar')).toHaveAttribute('aria-modal', 'true');
  await expect(page.locator('main')).toHaveAttribute('inert', '');
  await page.keyboard.press('Escape');
  await expect(trigger).toBeFocused();
  await expect(editor).toHaveValue('Keep café 日本語');
  await trigger.click();
  await page.locator('.navigation-scrim').click({ position: { x: 380, y: 200 }, force: true });
  await expect(page.locator('.sidebar')).toBeHidden();
  await trigger.click();
  await page.getByRole('searchbox', { name: 'Search conversations' }).fill('nothing');
  await page.getByRole('button', { name: 'Clear search' }).click();
  await expect(page.getByRole('searchbox', { name: 'Search conversations' })).toBeFocused();
  await page.locator('.history-open').click();
  await expect(page.locator('.sidebar')).toBeHidden();
  await expect(page.locator('.message.assistant')).toContainText('Finding 32');
  expect(writes).toEqual([]);
});

test('manual reading exposes Latest without a second scrolling page', async ({ page }) => {
  await fixture(page);
  await page.locator('.history-open').click();
  const transcript = page.locator('.conversation');
  await expect(page.locator('.message.assistant')).toBeVisible();
  await transcript.focus(); await page.keyboard.press('Control+Home');
  await transcript.evaluate(el => { el.scrollTop = 0; el.dispatchEvent(new Event('scroll')); });
  await page.getByRole('button', { name: 'Latest', exact: true }).click();
  await expect.poll(() => transcript.evaluate(el => el.scrollHeight - el.clientHeight - el.scrollTop)).toBeLessThan(3);
  expect(await page.evaluate(() => document.documentElement.scrollHeight <= innerHeight + 1)).toBe(true);
});

for (const theme of ['light', 'dark']) test(`six-page visual matrix ${theme}`, async ({ page }, info) => {
  const writes = await fixture(page, theme);
  for (const [width, height] of [[1440, 900], [1280, 720], [390, 844], [320, 480]]) {
    await page.setViewportSize({ width, height });
    for (const route of ['chat', 'agents', 'models', 'knowledge', 'activity', 'settings']) {
      await page.goto(`/ui/#/${route}`);
      await expect(page.locator('.topbar h1')).toBeVisible();
      expect((await page.locator('.topbar h1').boundingBox())!.height).toBeLessThan(40);
      await expect(page.locator('.page-failure')).toHaveCount(0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      if (route === 'chat') {
        await expect(page.locator('.composer-actions > button')).toBeInViewport({ ratio: 1 });
        await expect(page.getByRole('button', { name: 'Voice settings', exact: true })).toBeInViewport({ ratio: 1 });
      }
      await page.screenshot({ path: info.outputPath(`${route}-${width}-${theme}.png`), fullPage: true });
    }
  }
  expect(writes).toEqual([]);
});
