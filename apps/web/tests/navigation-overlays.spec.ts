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
  await expect(drawer.locator('.sidebar-toggle')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(panel).not.toBeVisible();
  await expect(drawer).toBeVisible();
  await expect(drawer.locator('.workspace-options > button')).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(drawer).not.toBeVisible();
  await expect(page.locator('.navigation-trigger')).toBeFocused();
  expect(state.writes).toEqual([]);
});
