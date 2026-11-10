import { expect, test, type Page } from '@playwright/test';
import { settingsNavigation } from '../src/i18n/settings-navigation';

async function fixture(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    if (!localStorage.getItem('offgrid.locale')) localStorage.setItem('offgrid.locale', 'en');
  });
  const writes: string[] = [];
  await page.route(/\/(?:health|v1\/|api\/v2\/)/, route => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() !== 'GET') writes.push(path);
    const bodies: Record<string, unknown> = {
      '/health': { status: 'healthy' },
      '/api/v2/system': { product: 'offgrid', version: 'fixture', api_version: 2, workspace_id: 'settings-fixture' },
      '/v1/users/me': { authenticated: false, user: null },
      '/v1/models': { data: [{ id: 'fixture-model', type: 'chat' }] },
      '/v1/sessions': { sessions: [] },
      '/v1/system/config': { version: 'fixture', inference_slots: 1 },
      '/v1/rag/status': { enabled: false },
      '/api/v2/computer/status': { available: false, active_sessions: 0 },
      '/v1/audio/status': { asr: { available: true }, tts: { available: true }, profiles: [
        { id: 'asr', revision: 'r1', name: 'Test recognition', available: true, capabilities: ['transcription'] },
        { id: 'tts', revision: 'r2', name: 'Test voice', available: true, capabilities: ['speech_synthesis'], voices: [{ id: 'speaker', language: 'en' }] }
      ] }
    };
    return route.fulfill({ json: bodies[path] ?? {} });
  });
  return writes;
}

test('settings share persisted response preferences with Chat without starting inference', async ({ page }) => {
  const writes = await fixture(page);
  await page.goto('/ui/#/settings');
  await page.getByRole('button', { name: 'Models & responses', exact: true }).click();
  await page.getByRole('combobox', { name: 'Context profile', exact: true }).selectOption('extended');
  await page.getByRole('combobox', { name: 'Maximum response tokens', exact: true }).selectOption('256');
  await page.reload();
  await page.getByRole('button', { name: 'Models & responses', exact: true }).click();
  await expect(page.getByRole('combobox', { name: 'Context profile', exact: true })).toHaveValue('extended');
  await expect(page.getByRole('combobox', { name: 'Maximum response tokens', exact: true })).toHaveValue('256');
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await page.locator('.response-advanced > summary').click();
  await expect(page.getByRole('combobox', { name: 'Context profile', exact: true })).toHaveValue('extended');
  await expect(page.getByRole('combobox', { name: 'Maximum response tokens', exact: true })).toHaveValue('256');
  expect(writes).toEqual([]);
});

test('voice choices persist while browsing sections grants no access and starts no audio', async ({ page }) => {
  const writes = await fixture(page);
  await page.goto('/ui/#/settings');
  await page.getByRole('button', { name: 'Voice', exact: true }).click();
  await page.locator('.voice-settings-inline .model-choice').first().getByRole('button').first().click();
  await page.getByRole('radio', { name: /Test recognition/ }).last().click();
  await page.reload();
  await page.getByRole('button', { name: 'Voice', exact: true }).click();
  await expect(page.locator('.voice-settings-inline .model-choice').first()).toContainText('Test recognition');
  await page.getByRole('button', { name: 'Terminal', exact: true }).click();
  await expect(page.getByText('Terminal display settings are available in the OffGrid desktop app.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Open terminal', exact: true })).toHaveCount(0);
  for (const name of ['Access & data', 'About', 'General']) await page.getByRole('button', { name, exact: true }).click();
  expect(writes).toEqual([]);
});

test('About keeps connection details compact and diagnostics optional', async ({ page }) => {
  await fixture(page);
  await page.goto('/ui/#/settings');
  await page.getByRole('button', { name: 'About', exact: true }).click();
  await expect(page.locator('.settings-product')).toContainText('OffGrid');
  await expect(page.locator('.settings-diagnostics')).not.toHaveAttribute('open', '');
  await page.screenshot({ path: test.info().outputPath('settings-about-wide.png') });
  await page.setViewportSize({ width: 560, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: test.info().outputPath('settings-about-narrow.png') });
});

test('desktop terminal opens only on explicit action', async ({ page }) => {
  await fixture(page);
  await page.addInitScript(() => {
    (window as any).opens = 0;
    (window as any).electron = { getPaths: async () => ({ models: 'C:\\models', data: 'C:\\workspace' }), getSystemTheme: async () => 'light', onThemeChange: () => () => {}, openTerminal: async () => { (window as any).opens++; return { visible: true }; } };
  });
  await page.goto('/ui/#/settings');
  await page.getByRole('button', { name: 'Terminal', exact: true }).click();
  expect(await page.evaluate(() => (window as any).opens)).toBe(0);
  await page.getByRole('button', { name: 'Open terminal', exact: true }).click();
  expect(await page.evaluate(() => (window as any).opens)).toBe(1);
});

for (const locale of Object.keys(settingsNavigation) as (keyof typeof settingsNavigation)[]) {
  test(`settings search and narrow keyboard navigation: ${locale}`, async ({ page }) => {
    await fixture(page);
    await page.addInitScript(locale => localStorage.setItem('offgrid.locale', locale), locale);
    await page.setViewportSize({ width: 560, height: 800 });
    await page.goto('/ui/#/settings');
    const copy = settingsNavigation[locale];
    const search = page.getByRole('searchbox', { name: copy.search });
    await search.fill(copy.sections[3]);
    await expect(page.locator('.settings-content').getByRole('heading', { name: copy.sections[3], exact: true })).toBeVisible();
    await search.fill('no-matching-setting-xyz');
    await expect(page.getByText(copy.empty, { exact: true })).toBeVisible();
    await search.fill('');
    const voice = page.locator('.settings-navigation nav button').nth(2);
    await voice.focus(); await page.keyboard.press('Enter');
    await expect(voice).toHaveAttribute('aria-current', 'page');
    await expect(page.locator('.settings-content').getByRole('heading', { name: copy.sections[2], exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    if (locale === 'ar') await expect(page.locator('html')).toHaveAttribute('dir', 'rtl');
    if (locale === 'en') await page.screenshot({ path: test.info().outputPath('settings-narrow.png') });
  });
}
