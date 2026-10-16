import type { Page } from '@playwright/test';

export async function openSettings(page: Page) {
  if (!await page.locator('.sidebar').isVisible()) await page.locator('.navigation-trigger').click();
  await page.locator('.workspace-options > button').click();
  await page.locator('.workspace-options-panel a[href="#/settings"]').click();
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
  const opened = !await drawer.isVisible();
  if (opened) await page.locator('.navigation-trigger').click();
  await page.locator('.workspace-options > button').click();
  await page.locator('.workspace-options-panel button[aria-busy]').click();
  await page.keyboard.press('Escape');
  if (opened) await page.keyboard.press('Escape');
}
