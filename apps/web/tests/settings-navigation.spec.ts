import { expect, test, type Page } from '@playwright/test';
import { settingsNavigation } from '../src/i18n/settings-navigation';
import catalog from '../../desktop/theme-palettes.json' with { type: 'json' };
import languagePacks from '../src/i18n/language-packs.json' with { type: 'json' };
import {resolveLanguage} from '../src/i18n/language';
import {featureTranslations} from '../src/i18n/catalog';

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

async function openTheme(page:Page,mode:'light'|'dark') {
 await page.locator('.theme-summary').nth(mode==='light'?0:1).click();
 await expect(page.getByRole('dialog',{name:'Choose a theme'})).toBeVisible();
}

for(const [id,modes] of Object.entries(catalog.themes).filter(([id])=>Object.hasOwn(catalog.sources,id)))for(const mode of Object.keys(modes) as ('light'|'dark')[]) {
 test(`established theme ${id} ${mode} persists and follows system mode`,async({page})=>{
  await fixture(page);await page.emulateMedia({colorScheme:mode});await page.goto('/ui/#/settings');
  await openTheme(page,mode);
  const label=(catalog.labels as Record<string,Partial<Record<'light'|'dark',string>>>)[id][mode]!;
  await page.getByRole('group',{name:mode==='dark'?'Dark theme':'Light theme',exact:true}).getByRole('button',{name:label,exact:true}).click();
  await expect(page.locator('html')).toHaveAttribute('data-palette',id);
  await page.getByRole('button',{name:'Apply',exact:true}).click();await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-palette',id);
  const background=await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--surface-0').trim());
  expect(background).toBe((modes as Record<string,{'surface-0':string}>)[mode]['surface-0']);
  await page.emulateMedia({colorScheme:mode==='dark'?'light':'dark'});
  await expect(page.locator('html')).toHaveAttribute('data-palette','offgrid');
 });
}
test('theme chooser supports search, keyboard, narrow layout and forced colors',async({page})=>{
 await fixture(page);await page.setViewportSize({width:390,height:844});await page.goto('/ui/#/settings');
 await page.getByLabel('Text size',{exact:true}).selectOption('150');
 await openTheme(page,'dark');
 const dialog=page.getByRole('dialog',{name:'Choose a theme'}),search=dialog.getByRole('searchbox',{name:'Search themes'});
 await search.fill('Dracula');await expect(dialog.locator('.theme-option')).toHaveCount(1);
 const dracula=dialog.getByRole('button',{name:'Dracula',exact:true});await dracula.focus();await page.keyboard.press('Enter');
 await expect(dracula).toHaveAttribute('aria-pressed','true');await search.fill('');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await expect(dialog.getByRole('button',{name:'Apply',exact:true})).toBeInViewport();
 await page.screenshot({path:test.info().outputPath('theme-chooser-narrow.png')});
 await page.emulateMedia({forcedColors:'active'});
 await expect(page.locator('html')).not.toHaveAttribute('style',/--surface-0:/);
 await dialog.getByRole('button',{name:'Cancel',exact:true}).click();
 await expect(page.locator('.theme-summary').nth(1)).toBeFocused();
});

test('theme chooser keeps the settings page compact and cancels previews with Escape',async({page})=>{
 await fixture(page);await page.emulateMedia({colorScheme:'dark'});await page.setViewportSize({width:1280,height:900});
 await page.goto('/ui/#/settings');
 await expect(page.locator('.theme-summary')).toHaveCount(2);await expect(page.locator('.theme-gallery')).toHaveCount(0);
 expect((await page.locator('#appearance-text-size').boundingBox())!.width).toBeLessThanOrEqual(180);
 await page.screenshot({path:test.info().outputPath('appearance-summary-wide.png')});
 await openTheme(page,'dark');const dialog=page.getByRole('dialog',{name:'Choose a theme'});
 await dialog.getByRole('button',{name:'Catppuccin Mocha',exact:true}).click();
 await expect(page.locator('html')).toHaveAttribute('data-palette','catppuccin');
 await page.screenshot({path:test.info().outputPath('theme-chooser-wide.png')});
 await dialog.getByRole('tab',{name:'Dark theme',exact:true}).focus();await page.keyboard.press('Home');
 await expect(dialog.getByRole('tab',{name:'Light theme',exact:true})).toBeFocused();
 await expect(dialog.getByRole('button',{name:'Nord',exact:true})).toHaveCount(0);
 await dialog.getByRole('searchbox').fill('no-such-theme');await expect(dialog.getByText('No matching themes.',{exact:true})).toBeVisible();
 await dialog.getByRole('tab',{name:'Light theme',exact:true}).focus();await page.keyboard.press('Escape');
 await expect(dialog).toHaveCount(0);await expect(page.locator('html')).toHaveAttribute('data-palette','offgrid');
 await expect(page.locator('.theme-summary').nth(1)).toBeFocused();
});

test('Settings retains preview language and discloses its status',async({page})=>{
 await fixture(page);await page.goto('/ui/#/settings');
 await page.locator('#settings-language').selectOption('ja');
 const picker=page.locator('#settings-language');await expect(picker).toHaveValue('ja');
 await expect(page.getByText(languagePacks.ja.notice,{exact:true})).toBeVisible();
 await picker.selectOption('zh-Hant');await expect(page.locator('#settings-language')).toHaveValue('zh-Hant');
});

for (const language of ['en','ar']) test(`theme chooser remains usable at 320 by 480: ${language}`,async({page})=>{
 await fixture(page);await page.addInitScript(code=>localStorage.setItem('offgrid.locale',code),language);
 await page.setViewportSize({width:320,height:480});await page.goto('/ui/#/settings');
 await page.locator('.theme-summary').nth(1).click();
 const dialog=page.locator('dialog.theme-chooser'),body=dialog.locator('.theme-chooser-body');
 await expect(dialog).toBeVisible();
 const bounds=await dialog.boundingBox();expect(bounds!.x).toBeGreaterThanOrEqual(0);expect(bounds!.y).toBeGreaterThanOrEqual(0);
 expect(bounds!.x+bounds!.width).toBeLessThanOrEqual(321);expect(bounds!.y+bounds!.height).toBeLessThanOrEqual(481);
 expect((await body.boundingBox())!.height).toBeGreaterThan(60);
 await dialog.getByRole('searchbox').fill('Dracula');
 await dialog.getByRole('button',{name:'Dracula',exact:true}).click();
 await expect(dialog.getByRole('button',{name:'Dracula',exact:true})).toHaveAttribute('aria-pressed','true');
 await expect(dialog.locator('.theme-chooser-footer button').last()).toBeInViewport();
 expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth)).toBe(true);
 if(language==='ar')await expect(page.locator('html')).toHaveAttribute('dir','rtl');
 await page.screenshot({path:test.info().outputPath(`theme-chooser-small-${language}.png`)});
 await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);
 await expect(page.locator('.theme-summary').nth(1)).toBeFocused();
});

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

for (const palette of ['slate','warm','forest','contrast']) {
 test('appearance preview persistence and reset: '+palette, async ({page})=>{
  await fixture(page); await page.emulateMedia({colorScheme:'dark'}); await page.goto('/ui/#/settings');
  const family=palette==='contrast'?'High contrast':palette[0].toUpperCase()+palette.slice(1);
  await openTheme(page,'dark');
  const dark=page.getByRole('group',{name:'Dark theme',exact:true});
  await dark.getByRole('button',{name:family,exact:true}).click();
  await expect(page.locator('html')).toHaveAttribute('data-palette',palette);
  expect(await page.evaluate(()=>localStorage.getItem('offgrid.appearance.v1'))).toBeNull();
  await page.getByRole('button',{name:'Cancel',exact:true}).click();
  await expect(page.locator('html')).toHaveAttribute('data-palette','offgrid');
  await openTheme(page,'dark');
  await dark.getByRole('button',{name:family,exact:true}).click();
  await page.getByRole('button',{name:'Apply',exact:true}).click();
  await page.reload();await expect(page.locator('html')).toHaveAttribute('data-palette',palette);
  await page.screenshot({path:test.info().outputPath(palette+'.png')});
  await page.getByRole('button',{name:'Reset appearance',exact:true}).click();
  await expect(page.locator('html')).toHaveAttribute('data-palette','offgrid');
 });
}
test('translation repair has independent expected labels',()=>{
 expect(settingsNavigation.de.terminal).toBe('Terminal öffnen');
 expect(settingsNavigation.ar.terminal).toBe('فتح الطرفية');
});
test('appearance save failure remains visible and does not claim persistence',async({page})=>{
 await fixture(page);await page.goto('/ui/#/settings');
 await page.evaluate(()=>{Storage.prototype.setItem=()=>{throw new Error('full');};});
 await openTheme(page,'dark');
 await page.getByRole('group',{name:'Dark theme',exact:true}).getByRole('button',{name:'Forest',exact:true}).click();
 await page.getByRole('button',{name:'Apply',exact:true}).click();
 await expect(page.getByText('Applied for this session; could not save.',{exact:true})).toBeVisible();
});

test('settings deep links focus the requested control',async({page})=>{
 await fixture(page); await page.goto('/ui/#/settings/text-size');
 await expect(page.getByLabel('Text size',{exact:true})).toBeFocused();
 await page.getByRole('searchbox',{name:'Search settings'}).fill('language');
 await page.locator('.settings-results').getByRole('link').click();
 await expect(page.locator('#settings-language')).toBeFocused();
 await expect(page).toHaveURL(/#\/settings\/language$/);
});
test('appearance restores corrupt storage safely and cross-tab changes update without reload',async({page})=>{
 await fixture(page); await page.addInitScript(()=>localStorage.setItem('offgrid.appearance.v1','{bad'));
 await page.emulateMedia({colorScheme:'dark'});await page.goto('/ui/#/settings');
 await expect(page.locator('html')).toHaveAttribute('data-palette','offgrid');
 await page.evaluate(()=>{const key='offgrid.appearance.v1',newValue=JSON.stringify({version:1,light:'warm',dark:'forest',textSize:125,reduceMotion:true});localStorage.setItem(key,newValue);window.dispatchEvent(new StorageEvent('storage',{key,newValue}));});
 await expect(page.locator('html')).toHaveAttribute('data-palette','forest');
 await expect(page.getByLabel('Text size',{exact:true})).toHaveValue('125');
 await expect(page.getByLabel('Reduce motion',{exact:true})).toBeChecked();
});

test('regional language resolution preserves scripts, regions and rejects malformed tags',()=>{
 for(const [tag,expected] of [['zh-TW','zh-Hant'],['zh-HK','zh-Hant'],['zh-CN','zh-Hans'],['zh-Hans-TW','zh-Hans'],['pt_BR','pt-BR'],['pt-AO','pt-PT'],['fr-CA','fr'],['iw-IL','he'],['../../bad',undefined]])expect(resolveLanguage(tag)).toBe(expected);
});
for(const [code,pack] of Object.entries(languagePacks))test(`preview language ${code} persists with truthful coverage and safe feature fallback`,async({page})=>{
 await fixture(page);await page.setViewportSize({width:390,height:844});
 const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
 await page.goto('/ui/#/settings');
 await page.locator('#settings-language').selectOption(code);
 await expect(page.locator('html')).toHaveAttribute('lang',code);
 await expect(page.locator('html')).toHaveAttribute('dir',pack.direction);
 await expect(page.locator('#translation-notice')).toHaveText(pack.notice);
 await expect(page.locator('#settings-language')).toHaveAccessibleName(pack.messages.common.language);
 await page.reload();await expect(page.locator('#settings-language')).toHaveValue(code);
 // Selected-language feature copy now participates before per-message fallback.
 const navigation=settingsNavigation[code as keyof typeof settingsNavigation];
 await page.getByRole('button',{name:navigation.sections[2],exact:true}).click();
 await expect(page.getByRole('heading',{name:navigation.sections[2],exact:true})).toBeVisible();
 await page.getByRole('button',{name:navigation.sections[4],exact:true}).click();
 await expect(page.getByRole('heading',{name:featureTranslations(code as keyof typeof languagePacks,'core')!['settings.computerUse'],exact:true})).toBeVisible();
 await page.getByRole('button',{name:navigation.sections[0],exact:true}).click();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 expect(errors).toEqual([]);
 if(['zh-Hant','ur','hi','pt-BR'].includes(code))await page.screenshot({path:test.info().outputPath('language-'+code+'.png')});
});
test('selected language is sent intact to desktop and does not change voice preferences',async({page})=>{
 await fixture(page);await page.addInitScript(()=>{
  (window as any).presentations=[];
  (window as any).electron={getPaths:async()=>({models:'fixture-models',data:'fixture-data'}),getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},setPresentation:async(value:unknown)=>(window as any).presentations.push(value)};
 });
 await page.goto('/ui/#/settings');
 const before=await page.evaluate(()=>Object.fromEntries(Object.entries(localStorage).filter(([k])=>/voice|speech|audio/i.test(k))));
 await page.locator('#settings-language').selectOption('ur');
 await expect.poll(()=>page.evaluate(()=>(window as any).presentations.at(-1)?.locale)).toBe('ur');
 expect(await page.evaluate(()=>Object.fromEntries(Object.entries(localStorage).filter(([k])=>/voice|speech|audio/i.test(k))))).toEqual(before);
});
test('preview packs are not silently selected from the browser language',async({page})=>{
 await fixture(page);await page.addInitScript(()=>{localStorage.removeItem('offgrid.locale');Object.defineProperty(navigator,'languages',{value:['zh-TW']});Object.defineProperty(navigator,'language',{value:'zh-TW'});});
 await page.goto('/ui/#/settings');await expect(page.locator('#settings-language')).toHaveValue('en');
});
test('independent translation anchors distinguish Chinese scripts and Portuguese regions',()=>{
 expect(languagePacks['zh-Hans'].messages.nav.settings).toBe('设置');
 expect(languagePacks['zh-Hant'].messages.nav.settings).toBe('設定');
 expect(languagePacks['pt-BR'].messages.nav.settings).toBe('Configurações');
 expect(languagePacks['pt-PT'].messages.nav.settings).toBe('Definições');
 expect(languagePacks.ja.messages.common.language).toBe('言語');
 expect(languagePacks.ur.direction).toBe('rtl');
});
