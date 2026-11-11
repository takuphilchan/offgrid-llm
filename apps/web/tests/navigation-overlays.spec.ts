import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const locale of ['en', 'ar']) test(`navigation keeps keyboard focus while a utility is open: ${locale}`, async ({ page }) => {
  const state = await craftFixture(page, locale, 'light');
  await page.setViewportSize({ width: 390, height: 700 });
  await page.goto('/ui/#/settings');
  await page.locator('.navigation-trigger').click();
  const drawer = page.locator('.sidebar');
  await drawer.locator('.workspace-options > button').click();
  const panel = page.locator('.workspace-options-panel');
  await expect(panel).toBeVisible();
  await expect(panel).toHaveCSS('opacity', '1');
  const last = panel.getByRole('button').last();
  await last.focus();
  await last.press('Tab');
  await expect(drawer.locator('.sidebar-foot a[href="#/settings"]')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(panel).not.toBeVisible();
  await expect(drawer).toBeVisible();
  await expect(drawer.locator('.workspace-options > button')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(drawer).not.toBeVisible();
  await expect(page.locator('.navigation-trigger')).toBeFocused();
  expect(state.writes).toEqual([]);
});

test('Markdown tables use the same reading typeface as the answer', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/chat');
  await page.locator('.history-open').first().click();
  const table = page.locator('.message-body table');
  await expect(table).toBeVisible();
  const font = await page.locator('.message-body p').first().evaluate(el => getComputedStyle(el).fontFamily);
  await expect(table.locator('th').first()).toHaveCSS('font-family', font);
  await expect(table.locator('td').first()).toHaveCSS('font-family', font);
});
