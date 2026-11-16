import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const locale of ['en', 'ar']) for (const theme of ['light', 'dark']) test(`Settings remains reachable beside terminal ${locale} ${theme}`, async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await craftFixture(page, locale, theme);
  await page.goto('/ui/#/chat');
  await page.evaluate(() => document.documentElement.setAttribute('data-native-terminal-visible', ''));
  const settings=page.locator('.sidebar-foot a[href="#/settings"]');
  await expect(settings).toBeInViewport();await settings.click();
  await expect(page.locator('#settings-language')).toBeVisible();
  await expect(page.locator('.workspace-options')).toHaveCount(0);
});
