import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { modelImport } from '../src/i18n/model-import';
import type { LocaleCode } from '../src/i18n';

async function firstUse(page: Page, locale: LocaleCode = 'en', capability = true, admin = true) {
  const fixture = await craftFixture(page, locale, 'dark');
  await page.addInitScript(() => {
    localStorage.removeItem('offgrid.onboarding.complete');
    if (!sessionStorage.getItem('first-use-seeded')) {
      localStorage.removeItem('offgrid.onboarding.stage');
      localStorage.setItem('offgrid.model', 'saved-unavailable');
      sessionStorage.setItem('first-use-seeded', 'true');
    }
  });
  await page.route('**/v1/models', r => r.fulfill({ json: { data: [] } }));
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [] } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: {
    product: 'offgrid', version: 'fixture', workspace_id: 'craft', api_version: 2,
    capabilities: capability ? ['gguf-offline-import-v1'] : [],
  } }));
  if (!admin) await page.route('**/v1/users/me', r => r.fulfill({ json: {
    authenticated: true, auth_required: true, user: { id: 'reader', role: 'user', username: 'Reader' },
  } }));
  return fixture;
}

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
  test(`first-use offline path is explicit and does not select or download: ${locale}`, async ({ page }) => {
    const fixture = await firstUse(page, locale);
    await page.setViewportSize({ width: 390, height: 620 });
    await page.goto('/ui/');
    const guide = page.getByRole('dialog');
    const action = guide.getByRole('button', { name: modelImport(locale).title, exact: true });
    await expect(action).toBeVisible();
    if (locale === 'en' || locale === 'ar') await page.screenshot({ path: test.info().outputPath(`offline-setup-${locale}.png`), fullPage: true });
    await action.focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/#\/models\/import$/);
    const panel = page.locator('.offline-model-import');
    await expect(panel).toHaveAttribute('open', '');
    await expect(panel.locator('summary')).toBeFocused();
    await expect(panel.getByLabel(modelImport(locale).choose, { exact: true })).toBeAttached();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('saved-unavailable');
    await page.reload();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(panel).toHaveAttribute('open', '');
    expect(fixture.writes).toEqual([]);
  });
}

for (const mode of ['older-service', 'not-admin', 'offline'] as const) {
  test(`first-use import is not offered when unavailable: ${mode}`, async ({ page }) => {
    const fixture = await firstUse(page, 'en', mode !== 'older-service', mode !== 'not-admin');
    if (mode === 'offline') await page.route('**/health', r => r.abort());
    await page.goto('/ui/');
    const guide = page.getByRole('dialog');
    await expect(guide).toBeVisible();
    await expect(guide.getByRole('button', { name: modelImport('en').title })).toHaveCount(0);
    expect(fixture.writes).toEqual([]);
  });
}

test('import destination replaces a retained speech category without reopening setup', async ({ page }) => {
  const fixture = await firstUse(page);
  await page.goto('/ui/');
  await page.getByRole('dialog').getByRole('button', { name: modelImport('en').title }).click();
  await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
  await expect(page.locator('.offline-model-import')).toHaveCount(0);
  await page.evaluate(() => { location.hash = '#/models'; });
  await page.evaluate(() => { location.hash = '#/models/import'; });
  await expect(page.locator('.offline-model-import')).toHaveAttribute('open', '');
  await expect(page.locator('.offline-model-import summary')).toBeFocused();
  expect(fixture.writes).toEqual([]);
});

test('first-use import ends with explicit model choice and returns to the retained draft', async ({ page }) => {
  const fixture = await firstUse(page);
  let imported = false;
  await page.route('**/v1/models', r => r.fulfill({ json: { data: imported ? [{ id: 'offline', type: 'chat', runtime_ready: true }] : [] } }));
  await page.route('**/api/v2/models/import/gguf?*', r => {
    imported = true;
    return r.fulfill({ status: 201, json: { model_id: 'offline', filename: 'offline.gguf', size: 1024 ** 2, sha256: 'a'.repeat(64), inventory_refreshed: true } });
  });
  await page.goto('/ui/#/chat/new');
  await page.getByRole('dialog').getByRole('button', { name: 'Close guide', exact: true }).click();
  await page.locator('.composer textarea').fill('Keep my first question');
  await page.evaluate(() => { location.hash = '#/models/import'; });
  const panel = page.locator('.offline-model-import');
  await panel.getByLabel('Choose GGUF file', { exact: true }).setInputFiles({ name: 'offline.gguf', mimeType: 'application/octet-stream', buffer: Buffer.alloc(1024 ** 2) });
  await panel.getByRole('button', { name: 'Import file', exact: true }).click();
  const select = page.locator('.installed-model').getByRole('button', { name: 'Select model', exact: true });
  await expect(select).toBeEnabled();
  await expect(page.getByRole('link', { name: 'Start your first chat', exact: true })).toHaveCount(0);
  await select.click();
  await page.screenshot({ path: test.info().outputPath('first-chat-handoff.png'), fullPage: true });
  await page.getByRole('link', { name: 'Start your first chat', exact: true }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep my first question');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('offline');
  expect(fixture.writes).toEqual([]);
});
