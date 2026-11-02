import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const locale of ['en', 'ar']) for (const theme of ['light', 'dark']) test(`terminal sidebar menu fits ${locale} ${theme}`, async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await craftFixture(page, locale, theme);
  await page.goto('/ui/#/chat');
  await page.evaluate(() => document.documentElement.setAttribute('data-native-terminal-visible', ''));
  const trigger = page.locator('.workspace-options > button'), panel = page.locator('.workspace-options-panel');
  await trigger.click();
  await expect(panel).toBeVisible();
  const sidebar = await page.locator('.sidebar').boundingBox();
  const box = await panel.boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(sidebar!.x);
  expect(box!.x + box!.width).toBeLessThanOrEqual(sidebar!.x + sidebar!.width);
  for (const select of await panel.locator('select').all()) {
    await select.focus(); await expect(select).toBeFocused();
    const control = await select.boundingBox();
    expect(control!.width).toBeGreaterThan(130);
    expect(control!.x + control!.width).toBeLessThanOrEqual(box!.x + box!.width);
  }
  await page.keyboard.press('Escape'); await expect(panel).not.toBeVisible(); await expect(trigger).toBeFocused();
});
