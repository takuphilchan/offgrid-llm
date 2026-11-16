import { openSettings, changePreferences, refreshWorkspace } from './helpers/workspace-actions';
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

test('navigation labels stay readable without covering adjacent controls', async ({page}) => {
  await fixture(page);
  const chat = page.locator('.primary-nav').getByRole('link', {name:'Chat',exact:true});
  const label = chat.locator('span');
  await chat.hover();
  await expect(label).toBeVisible();
  await expect(chat).not.toHaveAttribute('title');
  await chat.click();
  await expect(chat).toBeFocused();
  await expect(label).toBeHidden();
  await page.mouse.move(700,400);
  await expect(label).toBeHidden();
  await page.keyboard.press('Tab');
  const next = page.locator('.primary-nav .nav-link:focus');
  await expect(next.locator('span')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(next.locator('span')).toBeHidden();
  await expect(next).toBeFocused();
});

test('compact shell preserves a draft across navigation layout, preferences, and refresh', async ({ page }, info) => {
  const state = await fixture(page);
  const editor = page.locator('.composer textarea');
  await editor.fill('Keep this draft — لا ترسل');
  await expect(page.locator('.topbar h1')).toHaveText('Chat');
  await expect(page.locator('.workspace-kicker')).toHaveCount(0);
  expect((await page.locator('.topbar').boundingBox())!.height).toBeLessThanOrEqual(74);
  await expect(page.locator('.sidebar')).toHaveCSS('width', '340px');
  for (const name of ['Chat', 'Library', 'Activity', 'Settings']) {
    await expect(page.locator('.sidebar').getByRole('link', { name, exact: true })).toBeVisible();
  }
  await expect(page.locator('.primary-nav').getByRole('group', { name: 'Work' }).locator('a').first()).toHaveAttribute('href', '#/chat');
  await expect(page.locator('.primary-nav').getByRole('group', { name: 'Work' }).locator('.nav-link')).toHaveCount(4);
  await expect(page.locator('.primary-nav').getByRole('group', { name: 'Library' })).toHaveCount(0);
  await expect(page.locator('.sidebar').getByRole('link', { name: 'Chat', exact: true })).toHaveAttribute('aria-current', 'page');
  await expect(page.locator('.workspace-options')).toHaveCount(0);
  await changePreferences(page,{theme:'light',locale:'de'});
  await expect(page.locator('html')).toHaveAttribute('data-theme','light');
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  await page.keyboard.press('Escape');
  const selectionPreference=await page.evaluate(()=>localStorage.getItem('offgrid.model.autoselect'));
  const before = state.reads;
  await refreshWorkspace(page);
  await expect.poll(() => state.reads).toBeGreaterThan(before);
  expect(await page.evaluate(()=>localStorage.getItem('offgrid.model.autoselect'))).toBe(selectionPreference);
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  expect(state.writes).toEqual([]);
  await page.getByRole('button',{name:workspaceShell.de.collapse,exact:true}).click();
  await expect(page.locator('.sidebar')).toHaveCSS('width','60px');
  await expect(page.locator('.sidebar-create .new-chat-button')).toBeInViewport();
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  await page.getByRole('button',{name:workspaceShell.de.expand,exact:true}).click();
  await page.screenshot({ path: info.outputPath('shell-collapsed-light.png') });
  await page.reload();
  await expect(page.locator('.sidebar')).toHaveCSS('width', '340px');
  await expect(editor).toHaveValue('Keep this draft — لا ترسل');
  await expect(page.locator('.navigation-trigger')).toBeHidden();
});

test('account actions live in General settings, not a bottom overflow',async({page})=>{
 const state=await fixture(page);await expect(page.getByRole('button',{name:'Workspace options',exact:true})).toHaveCount(0);
 await openSettings(page);await expect(page.locator('.settings-account')).toContainText('Workspace tester');
 await expect(page.locator('.settings-account').getByRole('button',{name:'Sign out'})).toBeVisible();
 await page.locator('[data-section=appearance]').click();await expect(page.locator('.settings-account')).toBeHidden();expect(state.writes).toEqual([]);
});

test('workspace preferences still synchronize through the desktop presentation bridge', async ({ page }) => {
  await fixture(page, true);
  await changePreferences(page,{theme:'light',locale:'ar'});
  await expect.poll(() => page.evaluate(() => (window as any).presentations.at(-1))).toEqual({
    locale: 'ar', theme: 'light',
    appearance: { version: 1, light: 'offgrid', dark: 'offgrid', textSize: 100, reduceMotion: false },
  });
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
});

test('every locale fits the compact mobile navigation in light and dark', async ({ page }, info) => {
  test.setTimeout(180000); // 58 locale/theme combinations.
  await fixture(page);
  await page.setViewportSize({ width: 320, height: 480 });
  for (const theme of ['light', 'dark']) {
    for (const locale of Object.keys(locales) as LocaleCode[]) {
      await changePreferences(page,{locale,theme});
      await page.locator('.navigation-trigger').click();
      await expect(page.locator('.mobile-nav')).toHaveCount(0);
      await expect(page.locator('.sidebar')).toBeVisible();
      for (const link of await page.locator('.sidebar .nav-link').all()) await expect(link).toBeInViewport();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      if (locale === 'ar') await page.screenshot({ path: info.outputPath(`shell-mobile-rtl-${theme}.png`) });
      await page.keyboard.press('Escape');
      await expect(page.locator('.navigation-trigger')).toBeFocused();
    }
  }
});

test('expanded navigation remains usable in a short desktop viewport', async ({ page }, info) => {
  await fixture(page);
  await page.setViewportSize({ width: 1100, height: 480 });
  await openSettings(page);
  await expect(page).toHaveURL(/#\/settings$/);
  await expect(page.locator('.sidebar-foot')).toBeInViewport();
  const positions = await page.locator('.primary-nav .nav-link').evaluateAll(links => links.map(link => link.getBoundingClientRect().x));
  expect(new Set(positions).size).toBe(1);
  const knowledge = page.locator('.primary-nav a[href="#/knowledge"]');
  await knowledge.focus();
  await page.keyboard.press('Tab');
  await page.keyboard.press('Shift+Tab');
  await expect(knowledge.locator('span')).toHaveCSS('opacity','1');
  await expect(knowledge.locator('span')).toBeInViewport();
  await page.screenshot({ path: info.outputPath('shell-expanded-desktop.png') });
});

test('new chat centers its welcome above a bottom editor while recent work scrolls independently', async ({ page }, info) => {
  const state = await fixture(page);
  await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: Array.from({ length: 80 }, (_, index) => ({
    name: `Research notes ${index + 1}`, model_id: 'local-chat-model', messages: [], updated_at: '2026-11-06T00:00:00Z',
  })) } }));
  for (const theme of ['light', 'dark']) {
    await page.evaluate(theme => localStorage.setItem('offgrid.theme', theme), theme);
    await page.reload();
    for (const height of [900, 480]) {
      await page.setViewportSize({ width: 1440, height });
      await expect(page.locator('.history-row').first()).toBeVisible();
      const heading = await page.locator('.work-home h2').boundingBox();
      const composer = await page.locator('.composer').boundingBox();
      expect(composer!.y - heading!.y - heading!.height).toBeGreaterThanOrEqual(16);
      expect(height - composer!.y - composer!.height).toBeGreaterThanOrEqual(8);
      expect(height - composer!.y - composer!.height).toBeLessThanOrEqual(32);
      await expect(page.locator('.composer textarea')).toBeInViewport();
      const history = page.locator('.sidebar-context .history-list');
      const rail = await page.locator('.primary-nav').boundingBox();
      const list = await history.boundingBox();
      expect(list!.x).toBeGreaterThan(rail!.x + rail!.width);
      expect(list!.width).toBeLessThanOrEqual(280);
      expect(list!.height).toBeGreaterThan(0);
      expect(list!.y + list!.height).toBeLessThanOrEqual(height);
      expect(await history.evaluate(el => el.scrollHeight > el.clientHeight)).toBe(true);
      await history.evaluate(el => { el.scrollTop = el.scrollHeight; });
      await expect(page.locator('.sidebar-foot')).toBeInViewport();
      for (const link of await page.locator('.nav-section-library a').all()) await expect(link).toBeInViewport();
      await page.screenshot({ path: info.outputPath(`new-chat-history-${theme}-${height}.png`) });
    }
  }
  expect(state.writes).toEqual([]);
});


test('locale recovery ignores inherited keys and accepts regional language preferences', async ({ page }) => {
  await fixture(page);
  await page.addInitScript(() => Object.defineProperty(navigator, 'languages', { value: ['fr-CA', 'en-US'], configurable: true }));
  for (const [saved, expected, direction] of [['constructor','fr','ltr'], ['__proto__','fr','ltr'], ['ar-SA','ar','rtl']]) {
    await page.evaluate(saved => localStorage.setItem('offgrid.locale', saved), saved);
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('lang', expected);
    await expect(page.locator('html')).toHaveAttribute('dir', direction);
    await expect(page.locator('.composer textarea')).toBeVisible();
  }
});

test('Help topics remain keyboard reachable and localized across all UI languages', async ({ page }) => {
  test.setTimeout(180000); // Each of the 29 languages opens, operates and closes Help.
  const state = await fixture(page);
  for (const locale of Object.keys(locales) as LocaleCode[]) {
    await page.evaluate(locale => localStorage.setItem('offgrid.locale', locale), locale);
    await page.reload();
    await page.locator('.help-trigger').click();
    const dialog = page.locator('.workspace-help');
    await expect(dialog).toBeVisible();
    const summaries = dialog.locator('.help-topics button:not(.help-setup)');
    await expect(summaries).toHaveCount(8);
    await summaries.nth(1).focus();
    await page.keyboard.press('Enter');
    await expect(dialog.locator('.help-article p').first()).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(page.locator('.help-trigger')).toBeFocused();
  }
  expect(state.writes).toEqual([]);
});
