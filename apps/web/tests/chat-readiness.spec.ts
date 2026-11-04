import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { chatReadiness } from '../src/i18n/chat-readiness';
import type { LocaleCode } from '../src/i18n';

async function setup(page: Page, selected = '', first = false, locale: LocaleCode = 'en') {
  const fixture = await craftFixture(page, locale, 'light');
  let models = [{ id: 'working-model', type: 'chat', runtime_ready: true }];
  let failed = false;
  await page.addInitScript(({ selected, first }) => {
    if (selected) localStorage.setItem('offgrid.model', selected);
    if (first) localStorage.removeItem('offgrid.onboarding.complete');
  }, { selected, first });
  await page.route('**/v1/models', route => route.fulfill(failed
    ? { status: 503, json: { error: { message: 'Inventory unavailable' } } }
    : { json: { data: models } }));
  await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [] } }));
  return { fixture, models: (value: typeof models) => { models = value; }, fail: () => { failed = true; } };
}

async function refresh(page: Page) {
  await page.getByRole('button', { name: 'Workspace options' }).click();
  await page.locator('.workspace-options-panel').getByRole('button', { name: 'Refresh', exact: true }).click();
}

test('first-use excludes failed runtimes and offers a setup action without downloads', async ({ page }) => {
  const state = await setup(page, '', true);
  state.models([{ id: 'broken-runtime', type: 'chat', runtime_ready: false }]);
  await page.goto('/ui/');
  const guide = page.getByRole('dialog');
  await expect(guide.getByRole('button', { name: 'Choose a chat model' })).toBeVisible();
  await expect(guide.getByRole('button', { name: 'Start your first chat' })).toHaveCount(0);
  await expect(guide).toContainText(chatReadiness.en.setup);
  expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBeNull();
  expect(state.fixture.writes).toEqual([]);
});

test('first-use with no usable model opens reviewed discovery without starting a download', async ({ page }) => {
  const state = await setup(page, '', true);
  state.models([{ id: 'broken-runtime', type: 'chat', runtime_ready: false }]);
  await page.goto('/ui/');
  await page.getByRole('dialog').getByRole('button', { name: 'Choose a chat model' }).click();
  await expect(page).toHaveURL(/#\/models\/discover$/);
  await expect(page.getByRole('tab', { name: 'Discover models' })).toHaveAttribute('aria-selected', 'true');
  expect(state.fixture.writes).toEqual([]);
});

test('first-use preserves an unavailable saved choice even when an alternative exists', async ({ page }) => {
  const state = await setup(page, 'saved-model', true);
  await page.goto('/ui/');
  await expect(page.getByRole('dialog').getByRole('button', { name: 'Choose a chat model' })).toBeVisible();
  expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('saved-model');
  expect(state.fixture.writes).toEqual([]);
});

test('first-use offers an installed alternative only through explicit selection', async ({ page }) => {
  const state = await setup(page, 'saved-model', true);
  await page.goto('/ui/');
  const guide = page.getByRole('dialog');
  await expect(guide.getByRole('button', { name: 'Select model: working model' })).toBeVisible();
  expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('saved-model');
  await guide.getByRole('button', { name: 'Select model: working model' }).click();
  await expect(page).toHaveURL(/#\/chat$/);
  await expect(guide).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('working-model');
  expect(state.fixture.writes).toEqual([]);
});

test('refresh preserves a removed selection and draft until an explicit replacement', async ({ page }) => {
  const state = await setup(page, 'working-model');
  await page.goto('/ui/#/chat/new');
  const input = page.locator('.composer textarea');
  await input.fill('Keep my question');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  state.models([{ id: 'replacement-model', type: 'chat', runtime_ready: true }]);
  await refresh(page);
  await expect(page.locator('.setup-inline')).toContainText(chatReadiness.en.unavailable);
  await expect(input).toHaveValue('Keep my question');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeDisabled();
  await input.press('Enter');
  expect(state.fixture.writes).toEqual([]);
  expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('working-model');
  await page.locator('.composer-context-menu > button').click();
  await page.locator('.composer .model-select-trigger').click();
  await page.getByRole('option', { name: /replacement model/ }).click();
  await expect(page.locator('.setup-inline')).toHaveCount(0);
  await expect(input).toHaveValue('Keep my question');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  expect(state.fixture.writes).toEqual([]);
});

test('inventory failure does not replace a working model or expose setup again', async ({ page }) => {
  const state = await setup(page, 'working-model');
  await page.goto('/ui/#/chat/new');
  await page.locator('.composer textarea').fill('Retain this draft');
  state.fail(); await refresh(page);
  await expect(page.locator('.error-banner')).toContainText('Inventory unavailable');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('Retain this draft');
  expect(await page.evaluate(() => localStorage.getItem('offgrid.model'))).toBe('working-model');
});

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
  test(`unavailable model recovery is localized and retains the draft: ${locale}`, async ({ page }) => {
    const state = await setup(page, 'saved-model', false, locale);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/ui/#/chat/new');
    await expect(page.locator('.setup-inline')).toContainText(chatReadiness[locale].unavailable);
    await page.locator('.composer textarea').fill('draft café 日本語');
    await page.locator('.composer textarea').press('Enter');
    await expect(page.locator('.composer textarea')).toHaveValue('draft café 日本語');
    expect(state.fixture.writes).toEqual([]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  });
}
