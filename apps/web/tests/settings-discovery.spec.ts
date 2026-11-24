import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { responseDisplay } from '../src/i18n/response-display';
import { voiceSettingsText } from '../src/i18n/voice-settings';
import { desktopCommand } from '../src/i18n/desktop-command';
import { personalization } from '../src/i18n/personalization';
import { languageRegistry, type LanguageCode } from '../src/i18n/language';

test('search presents complete result rows and returns to the chosen setting',async({page},info)=>{
 const state=await craftFixture(page,'en','light');await page.setViewportSize({width:754,height:560});await page.goto('/ui/#/settings');
 const search=page.locator('.settings-search input');await search.fill('he');
 const results=page.locator('.settings-results');await expect(results.getByRole('listitem')).toHaveCount(5);
 await expect(page.locator('#settings-language')).toBeHidden();
 const theme=results.locator('a[href="#/settings/theme"]');await expect(theme).toContainText('Appearance');await expect(theme).toHaveCSS('text-decoration-line','none');
 await page.screenshot({path:info.outputPath('settings-search-results.png')});
 await theme.focus();await page.keyboard.press('Enter');await expect(page.locator('#setting-theme button[aria-pressed="true"]')).toBeFocused();await expect(search).toHaveValue('');
 await search.fill('Dracula');await expect(search).toBeInViewport();await expect(results.locator('a[href="#/settings/appearance"]')).toBeVisible();await results.locator('a').click();await expect(page.locator('#setting-appearance')).toBeFocused();
 await search.fill('unmatched-xyz');await expect(page.getByText('No matching settings.',{exact:true})).toBeVisible();await expect(page.locator('.settings-groups')).toBeHidden();
 await search.fill('');await expect(page.locator('#setting-appearance')).toBeVisible();
 await search.fill('Local storage');await results.locator('a[href="#/settings/storage"]').click();await expect(page.locator('#storage-settings-title')).toBeFocused();expect(state.writes).toEqual([]);
});

test('response search focuses the requested field without changing its value or losing the draft', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/chat/new');
  await page.locator('.composer textarea').fill('Keep this request while finding a preference');
  await page.evaluate(() => location.hash = '#/settings');
  await page.locator('.settings-search input').fill(responseDisplay('en').label);
  const result = page.locator('.settings-results a[href="#/settings/response-display"]');
  await expect(result).toBeVisible();
  await result.focus(); await page.keyboard.press('Enter');
  await expect(page.getByLabel(responseDisplay('en').label, { exact: true })).toBeFocused();
  await expect(page.getByLabel(responseDisplay('en').label, { exact: true })).toHaveValue('live');
  await page.evaluate(() => location.hash = '#/chat');
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this request while finding a preference');
  expect(state.writes).toEqual([]);
});

test('recognition model is searchable and its link leaves the picker closed', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'dark');
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { profiles: [], asr: { available: true }, tts: { available: false } } }));
  await page.goto('/ui/#/settings');
  await page.locator('.settings-search input').fill(voiceSettingsText('en').recognition);
  const result = page.locator('.settings-results a[href="#/settings/recognition-model"]');
  await expect(result).toBeVisible();
  await expect(page.locator('.voice-settings-inline .model-choice').first().locator('button')).toBeEnabled();
  await result.click();
  await expect(page.locator('.voice-settings-inline .model-choice').first().getByRole('button')).toBeFocused();
  await expect(page.getByRole('radio')).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('browser search does not offer an unavailable desktop shortcut destination', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/settings');
  await page.locator('.settings-search input').fill(desktopCommand('en').title);
  await expect(page.locator('.settings-results a[href="#/settings/quick-window"]')).toHaveCount(0);
  await expect(page.locator('.settings-results a[href="#/settings/shortcut"]')).toHaveCount(0);
  await expect(page.locator('.desktop-command-settings')).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('cold voice links keep a stable focus target while metadata loads and never steal later focus', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  let release: (() => void) | undefined;
  await page.route('**/v1/audio/status', async route => {
    await new Promise<void>(resolve => { release = resolve; });
    await route.fulfill({ json: { profiles: [], asr: { available: false }, tts: { available: false } } });
  });
  await page.goto('/ui/#/settings/recognition-model');
  await expect.poll(() => !!release).toBe(true);
  await expect(page.locator('[data-setting="recognition-model"]')).toBeFocused();
  await page.locator('.settings-search input').focus();
  release!();
  await expect(page.locator('.voice-settings-inline .model-choice')).toHaveCount(0);
  await expect(page.locator('.settings-search input')).toBeFocused();
  expect(state.writes).toEqual([]);
});

test('individual routes preserve response values through browser history and focus disclosures without opening them', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/settings/response-limit');
  const limit = page.locator('.response-settings select[id$="-limit"]');
  await expect(limit).toBeFocused(); await limit.selectOption('1024');
  await page.evaluate(() => location.hash = '#/settings/response-profile');
  await expect(page.locator('.response-settings select[id$="-profile"]')).toBeFocused();
  await page.goBack(); await expect(limit).toBeFocused(); await expect(limit).toHaveValue('1024');
  await page.goForward(); await expect(page.locator('.response-settings select[id$="-profile"]')).toBeFocused();
  await page.evaluate(() => location.hash = '#/settings/dark-theme');
  await expect(page.locator('[data-setting="dark-theme"]')).toBeFocused();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.evaluate(() => location.hash = '#/settings/diagnostics');
  await expect(page.locator('.settings-diagnostics > summary')).toBeFocused();
  await expect(page.locator('.settings-diagnostics')).not.toHaveAttribute('open');
  await page.evaluate(() => location.hash = '#/settings/setup-guide');
  await expect(page.locator('section[aria-labelledby="preferences-title"] > .action-card')).toBeFocused();
  await expect(page.locator('.workspace-setup')).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('desktop shortcut search focuses its existing control without changing or opening Quick window', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.addInitScript(() => {
    const state = { mode: 'full', layout: 'text', shortcut: 'Alt+Space', registered: true };
    Object.assign(window, { shortcutWrites: 0, quickOpens: 0, electron: {
      getSystemTheme: async () => 'light', onThemeChange: () => () => {},
      getCommandWindow: async () => state, onCommandWindow: () => () => {},
      setCommandShortcut: async () => { (window as any).shortcutWrites++; return state; },
      openCommandWindow: async () => { (window as any).quickOpens++; return state; }
    } });
  });
  await page.goto('/ui/#/settings');
  await page.locator('.settings-search input').fill(desktopCommand('en').shortcut);
  await expect(page.locator('#desktop-command-shortcut')).toBeEnabled();
  await page.locator('.settings-results a[href="#/settings/shortcut"]').click();
  await expect(page.locator('#desktop-command-shortcut')).toBeFocused();
  await expect(page.locator('#desktop-command-shortcut')).toHaveValue('Alt+Space');
  expect(await page.evaluate(() => [(window as any).shortcutWrites, (window as any).quickOpens])).toEqual([0, 0]);
  expect(state.writes).toEqual([]);
});

for (const locale of Object.keys(languageRegistry) as LanguageCode[]) test(`localized setting discovery and keyboard return: ${locale}`, async ({ page }, info) => {
  const state = await craftFixture(page, locale, 'dark');
  await page.setViewportSize({ width: 390, height: 760 });
  await page.emulateMedia({ reducedMotion: 'reduce', ...(locale === 'en' ? { forcedColors: 'active' as const } : {}) });
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { asr: { available: true }, tts: { available: true }, profiles: [
    { id: 'recognition', revision: 'r1', name: 'Recognition fixture', available: true, capabilities: ['transcription'] },
    { id: 'speech', revision: 'r2', name: 'Speech fixture', available: true, capabilities: ['speech_synthesis'], voices: [{ id: 'fixture-voice', language: 'en' }] }
  ] } }));
  await page.goto('/ui/#/chat/new');
  await page.locator('.composer textarea').fill('Retain this draft: café 日本語 مرحبا');
  await page.evaluate(() => location.hash = '#/settings');
  const search = page.locator('.settings-search input');
  await search.fill(voiceSettingsText(locale).synthesis);
  await expect(page.locator('[data-setting="speech-model"] button')).toBeEnabled();
  const result = page.locator('.settings-results a[href="#/settings/speech-model"]');
  await result.focus(); await page.keyboard.press('Enter');
  await expect(page.locator('[data-setting="speech-model"] button')).toBeFocused();
  await expect(page.getByRole('radio')).toHaveCount(0);
  await search.fill(responseDisplay(locale).label);
  await page.locator('.settings-results a[href="#/settings/response-display"]').click();
  await expect(page.locator('.response-settings select[id$="-display"]')).toBeFocused();
  await search.fill(personalization[locale][2]);
  await page.locator('.settings-results a[href="#/settings/dark-theme"]').click();
  await expect(page.locator('[data-setting="dark-theme"]')).toBeFocused();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
  if (['en', 'ar'].includes(locale)) await page.screenshot({ path: info.outputPath(`discovery-${locale}.png`) });
  await page.evaluate(() => location.hash = '#/chat');
  await expect(page.locator('.composer textarea')).toHaveValue('Retain this draft: café 日本語 مرحبا');
  expect(state.writes).toEqual([]);
});
