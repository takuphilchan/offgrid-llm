import { expect, test, type Page } from '@playwright/test';
import { workspaceShell } from '../src/i18n/workspace-shell';
import { locales, type LocaleCode } from '../src/i18n';

async function fixture(page: Page, desktop = false) {
  await page.addInitScript(desktop => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale', 'en');
    if (desktop) {
      (window as any).presentations = [];
      (window as any).electron = {
        getPresentation: async () => ({ locale: 'en', theme: 'dark' }),
        getSystemTheme: async () => 'dark', onThemeChange: () => () => {},
        setPresentation: async (value: unknown) => { (window as any).presentations.push(value); },
      };
    }
  }, desktop);
  const state = { reads: 0, writes: [] as string[] };
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, route => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() !== 'GET') state.writes.push(path);
    state.reads++;
    const bodies: Record<string, unknown> = {
      '/v1/users/me': { authenticated: true, auth_required: true, user: { id: 'fixture-user', username: 'Workspace tester', role: 'user' } },
      '/v1/models': { data: [{ id: 'local-chat-model', type: 'chat' }] },
      '/v1/sessions': { sessions: [] }, '/v1/rag/status': { enabled: false },
      '/api/v2/system': { product: 'offgrid', version: 'test', api_version: 2, workspace_id: 'shell-fixture' },
      '/v1/audio/status': { profiles: [] }, '/v1/system/config': { version: 'test', require_auth: true },
    };
    return route.fulfill({ json: bodies[path] ?? {} });
  });
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer textarea')).toBeVisible();
  return state;
}

test('compact shell preserves a draft across navigation layout, preferences, and refresh', async ({ page }, info) => {
  const state = await fixture(page);
  const editor = page.locator('.composer textarea');
  await editor.fill('Keep this draft — لا ترسل');
  await expect(page.locator('.topbar h1')).toHaveText('Chat');
  await expect(page.locator('.workspace-kicker')).toHaveCount(0);
  expect((await page.locator('.topbar').boundingBox())!.height).toBeLessThanOrEqual(74);
  await page.getByRole('button', { name: 'Collapse navigation' }).click();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '72px');
  for (const name of ['Chat', 'Agents', 'Knowledge', 'Models', 'Activity', 'Settings']) {
    await expect(page.locator('.primary-nav').getByRole('link', { name, exact: true })).toBeVisible();
  }
  await expect(page.locator('.primary-nav').getByRole('link', { name: 'Chat', exact: true })).toHaveAttribute('aria-current', 'page');
  const options = page.getByRole('button', { name: 'Workspace options', exact: true });
  await options.click();
  await expect.poll(() => page.locator('.workspace-options-panel select').evaluateAll(fields =>
    fields[0].getBoundingClientRect().width - fields[1].getBoundingClientRect().width)).toBe(0);
  await expect(page.getByLabel('Language', { exact: true })).toHaveCSS('border-width', '1px');
  await page.getByLabel('Appearance', { exact: true }).selectOption('light');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  await page.getByLabel('Language', { exact: true }).selectOption('de');
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  await page.keyboard.press('Escape');
  const before = state.reads;
  await page.locator('.topbar button[aria-busy]').click();
  await expect.poll(() => state.reads).toBeGreaterThan(before);
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  expect(state.writes).toEqual([]);
  await page.screenshot({ path: info.outputPath('shell-collapsed-light.png') });
  await page.reload();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '72px');
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  await page.locator('.sidebar-toggle').click();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '228px');
});

test('workspace and voice utilities share keyboard and outside dismissal without competing panels', async ({ page }) => {
  await fixture(page);
  const options = page.getByRole('button', { name: 'Workspace options', exact: true });
  const panel = page.getByRole('region', { name: 'Workspace options', exact: true });
  await options.focus(); await options.press('Enter');
  await expect(options).toHaveAttribute('aria-expanded', 'true');
  await expect(panel.getByText('Workspace tester', { exact: true })).toBeVisible();
  await expect(panel.getByRole('button', { name: 'Sign out' })).toBeVisible();
  await page.getByLabel('Language', { exact: true }).focus();
  await page.keyboard.press('Escape');
  await expect(panel).toBeHidden(); await expect(options).toBeFocused();
  await options.click(); await page.locator('.composer textarea').click();
  await expect(panel).toBeHidden(); await expect(page.locator('.composer textarea')).toBeFocused();
  await page.getByRole('button', { name: 'Voice settings', exact: true }).click();
  await options.click();
  await expect(page.getByRole('region', { name: 'Voice settings', exact: true })).toBeHidden();
  await expect(panel).toBeVisible();
  await expect(page.getByRole('link', { name: 'Service details', exact: true })).toHaveCount(0);
  await page.locator('.primary-nav').getByRole('link', { name: 'Settings', exact: true }).click();
  await expect(page).toHaveURL(/#\/settings$/); await expect(panel).toBeHidden();
});

test('workspace preferences still synchronize through the desktop presentation bridge', async ({ page }) => {
  await fixture(page, true);
  await page.getByRole('button', { name: 'Workspace options', exact: true }).click();
  await page.getByLabel('Appearance', { exact: true }).selectOption('light');
  await page.getByLabel('Language', { exact: true }).selectOption('ar');
  await expect.poll(() => page.evaluate(() => (window as any).presentations.at(-1))).toEqual({ locale: 'ar', theme: 'light' });
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
});

test('every locale fits the compact mobile shell and options in light and dark', async ({ page }, info) => {
  await fixture(page);
  await page.getByRole('button', { name: 'Collapse navigation' }).click();
  await page.setViewportSize({ width: 320, height: 480 });
  for (const theme of ['light', 'dark']) {
    for (const locale of Object.keys(locales) as LocaleCode[]) {
      const trigger = page.locator('.workspace-options > button');
      await trigger.click();
      await page.locator('.locale-picker select').selectOption(locale);
      const panel = page.getByRole('region', { name: workspaceShell[locale].options, exact: true });
      await panel.getByLabel(locales[locale].messages.shell.appearance, { exact: true }).selectOption(theme);
      await expect(panel).toBeVisible();
      await expect.poll(async () => {
        const box = await panel.boundingBox();
        return !!box && box.x >= 0 && box.x + box.width <= 320 && box.y >= 0 && box.y + box.height <= 480;
      }).toBe(true);
      await expect(page.locator('.mobile-nav a')).toHaveCount(6);
      for (const link of await page.locator('.mobile-nav a').all()) await expect(link).toBeInViewport();
      await expect(page.locator('.sidebar')).toBeHidden();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      if (locale === 'ar') await page.screenshot({ path: info.outputPath(`shell-mobile-rtl-${theme}.png`) });
      await page.keyboard.press('Escape');
      await expect(trigger).toBeFocused();
    }
  }
});

test('expanded navigation remains usable in a short desktop viewport', async ({ page }, info) => {
  await fixture(page);
  await page.setViewportSize({ width: 1100, height: 480 });
  await page.locator('.primary-nav').getByRole('link', { name: 'Settings', exact: true }).click();
  await expect(page).toHaveURL(/#\/settings$/);
  await page.locator('.sidebar-toggle').scrollIntoViewIfNeeded();
  await expect(page.locator('.sidebar-toggle')).toBeInViewport();
  await page.screenshot({ path: info.outputPath('shell-expanded-desktop.png') });
});
