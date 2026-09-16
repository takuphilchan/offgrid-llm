import { expect, test, type Page } from '@playwright/test';

const now = '2026-09-15T12:00:00Z';

async function mockWorkspace(page: Page, hasChatModel: boolean) {
  await page.addInitScript(() => {
    if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale', 'en');
  });
  await page.route('**/health', route => route.fulfill({ contentType: 'application/json', body: '{"status":"healthy"}' }));
  await page.route('**/v1/**', route => {
    const path = new URL(route.request().url()).pathname;
    const model = { id: 'workspace-test-model', type: 'chat', context_window: 8192 };
    const user = { role: 'user', content: 'Say hello', timestamp: now };
    const assistant = { role: 'assistant', content: ['## Hello from OffGrid', '', '- Private', '- Local', '', '| Mode | State |', '| --- | --- |', '| Inference | Ready |', '', '```go', 'fmt.Println("offgrid")', '```', '', '![Remote diagram](https://example.com/tracker.png)'].join('\n'), timestamp: now };
    let body: unknown = {};
    let status = 200;
    if (path === '/v1/models') body = { data: hasChatModel ? [model] : [] };
    else if (path === '/v1/users/me') body = { user: null, authenticated: false, guest: true };
    else if (path === '/v1/sessions' && route.request().method() === 'GET') body = { sessions: [] };
    else if (path === '/v1/sessions' && route.request().method() === 'POST') {
      const request = route.request().postDataJSON() as { name: string; model_id: string };
      body = { name: request.name, model_id: request.model_id, messages: [], created_at: now, updated_at: now };
      status = 201;
    } else if (/^\/v1\/sessions\/[^/]+\/generate$/.test(path)) {
      const sessionName = decodeURIComponent(path.split('/')[3]);
      body = { session: { name: sessionName, model_id: model.id, messages: [user, assistant], created_at: now, updated_at: now }, message: assistant };
    } else if (path === '/v1/catalog') body = { models: [] };
    else if (path === '/v1/models/download/progress') body = {};
    else if (path === '/v1/system/config') body = { version: 'test', inference_slots: 1 };
    else if (path === '/v1/rag/status') body = { enabled: false };
    else if (path === '/v1/computer/status') body = { available: false, emergency_stop: false, active_sessions: 0 };
    return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
  });
}

test('browser chrome uses the monochrome workspace identity', async ({ page, request }) => {
  await mockWorkspace(page, false);
  await page.goto('/ui/');
  await expect(page.locator('link[rel="icon"]')).toHaveAttribute('href', '/ui/app-mark.svg');
  await expect(page.locator('meta[name="theme-color"]')).toHaveAttribute('content', /#(?:101011|f7f7f6)/);

  const response = await request.get('/ui/app-mark.svg');
  expect(response.ok()).toBeTruthy();
  const favicon = await response.text();
  expect(favicon).toContain('#111112');
  expect(favicon).toContain('#f5f5f4');
  expect(favicon).not.toMatch(/#(?:9aafff|526fc9|22d3ee|0d1220)/i);
});

test('first-run stays pending while a chat model is not available', async ({ page }) => {
  await mockWorkspace(page, false);
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('dialog', { name: 'Your private AI workspace' })).toBeVisible();
  await page.getByRole('dialog', { name: 'Your private AI workspace' }).getByRole('button', { name: 'Choose a chat model' }).click();
  await expect(page).toHaveURL(/#\/models$/);
  expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).toBeNull();
  await expect(page.getByText('Download a chat model to begin working.').first()).toBeVisible();
});

test('first-run completes only after a returned assistant message', async ({ page }) => {
  await mockWorkspace(page, true);
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Start your first chat' }).click();
  expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).toBeNull();
  const composer = page.locator('.composer textarea');
  await composer.fill('First line\nSecond line\nThird line');
  expect((await composer.boundingBox())?.height ?? 0).toBeGreaterThan(50);
  await composer.fill('Say hello');
  await page.locator('.composer button').click();
  await expect(page.locator('.message.assistant').getByRole('heading', { name: 'Hello from OffGrid' })).toBeVisible();
  await expect(page.locator('.message.assistant table')).toBeVisible();
  await expect(page.locator('.message.assistant pre code')).toContainText('fmt.Println("offgrid")');
  await expect(page.locator('.message.assistant img')).toHaveCount(0);
  await expect(page.locator('.message.assistant .blocked-image')).toContainText('Remote diagram');
  await page.getByRole('button', { name: 'Copy response' }).click();
  await expect(page.getByRole('button', { name: 'Copied' }).first()).toBeVisible();
  await expect.poll(() => page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).toBe('true');
  if (process.env.OFFGRID_VISUAL_CAPTURE) await page.screenshot({ path: test.info().outputPath('chat.png') });
});

test('appearance choice survives reload and changes the whole shell', async ({ page }) => {
  await mockWorkspace(page, false);
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
  await page.goto('/ui/#/settings');
  await page.getByRole('button', { name: 'Light', exact: true }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await expect(page.locator('.sidebar')).toHaveCSS('background-color', 'rgb(255, 255, 255)');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await page.getByRole('button', { name: 'Dark', exact: true }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  if (process.env.OFFGRID_VISUAL_CAPTURE) await page.screenshot({ path: test.info().outputPath('settings.png') });
});

test('southern African languages are complete, selectable, and persistent', async ({ page }) => {
  await mockWorkspace(page, false);
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
  await page.goto('/ui/#/settings');

  const language = page.getByLabel('Language');
  await expect(language.locator('option')).toHaveCount(9);

  await language.selectOption('sn');
  await expect(page.locator('html')).toHaveAttribute('lang', 'sn');
  await expect(page.getByRole('heading', { name: 'Zvirongwa' })).toBeVisible();

  await page.getByLabel('Mutauro').selectOption('nd');
  await expect(page.locator('html')).toHaveAttribute('lang', 'nd');
  await expect(page.getByRole('heading', { name: 'Izilungiselelo' })).toBeVisible();

  await page.getByLabel('Ulimi').selectOption('zu');
  await expect(page.locator('html')).toHaveAttribute('lang', 'zu');
  await page.getByLabel('Ulimi').selectOption('de');
  await expect(page.locator('html')).toHaveAttribute('lang', 'de');
  await expect(page.getByRole('heading', { name: 'Einstellungen' })).toBeVisible();
  await expect.poll(() => page.evaluate(() => localStorage.getItem('offgrid.locale'))).toBe('de');
  await page.reload();
  await expect(page.getByLabel('Sprache')).toHaveValue('de');
  await expect(page.locator('html')).toHaveAttribute('lang', 'de');
});

test('conversation history remains accessible at narrow desktop width', async ({ page }) => {
  await mockWorkspace(page, true);
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
  await page.setViewportSize({ width: 800, height: 700 });
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Show conversations' }).click();
  await expect(page.locator('.conversation-history')).toBeVisible();
  await page.getByRole('button', { name: 'Close conversations' }).last().click();
  await expect(page.locator('.conversation-history')).toBeHidden();
});

test('command palette navigates and applies appearance with the keyboard', async ({ page }) => {
  await mockWorkspace(page, true);
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
  await page.goto('/ui/#/chat');
  await page.keyboard.press('Control+K');
  const palette = page.getByRole('dialog', { name: 'Quick actions' });
  await expect(palette).toBeVisible();
  const search = palette.getByRole('textbox', { name: 'Search pages and actions…' });
  await search.fill('models');
  if (process.env.OFFGRID_VISUAL_CAPTURE) await page.screenshot({ path: test.info().outputPath('command-palette.png') });
  await search.press('Enter');
  await expect(page).toHaveURL(/#\/models$/);

  await page.keyboard.press('Control+K');
  await search.fill('dark');
  await search.press('Enter');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
});
