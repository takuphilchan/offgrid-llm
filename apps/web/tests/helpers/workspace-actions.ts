import type { Page } from '@playwright/test';

export async function openSettings(page: Page) {
  await page.locator('.sidebar').waitFor({state:'attached'});
  if (await page.evaluate(()=>matchMedia('(max-width:1023px)').matches) && !await page.locator('.sidebar').isVisible()) await page.locator('.navigation-trigger').click();
  await page.locator('.sidebar-foot a[href="#/settings"]').click();
}

export async function changePreferences(page: Page, values: {locale?: string; theme?: string}) {
  const previous = page.url();
  await openSettings(page);
  if (values.locale) await page.locator('#settings-language').selectOption(values.locale);
  if (values.theme) await page.locator('#setting-theme button').nth(['system','dark','light'].indexOf(values.theme)).click();
  if (!previous.endsWith('#/settings')) await page.goto(previous);
}

// Simulate preference synchronization from another window while recording stays
// mounted. This deliberately does not claim an in-place Settings UI interaction.
export async function syncPreferences(page: Page, values: {locale?: string; theme?: string}) {
  await page.evaluate(values => {
    for (const [name,value] of Object.entries(values)) {
      const key='offgrid.'+name;localStorage.setItem(key,value);
      window.dispatchEvent(new StorageEvent('storage',{key,newValue:value}));
    }
  },values);
}

export async function openActivityArchive(page: Page) {
  const archive = page.locator('.activity-archive');
  await archive.waitFor({state:'visible'});
  if (!await archive.evaluate(el => (el as HTMLDetailsElement).open)) await archive.locator(':scope > summary').click();
}

export async function openWorkDetails(page: Page) {
  const details = page.locator('.conversation-work-details');
  if (!await details.evaluate(el => (el as HTMLDetailsElement).open)) await details.locator(':scope > summary').click();
}

export async function refreshWorkspace(page: Page) {
  const drawer = page.locator('.sidebar');
  await drawer.waitFor({state:'attached'});
  const opened = await page.evaluate(()=>matchMedia('(max-width:1023px)').matches) && !await drawer.isVisible();
  if (opened) await page.locator('.navigation-trigger').click();
  await page.locator('.workspace-options > button').click();
  await page.locator('.workspace-options-panel button[aria-busy]').click();
  await page.keyboard.press('Escape');
  if (opened) await page.keyboard.press('Escape');
}
