import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const collapsed of [false, true]) test(`one Chat destination on management pages, collapsed preference ${collapsed}`, async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto('/ui/#/chat/new');
  if (collapsed) await page.locator('.sidebar-collapse').click();
  for (const route of ['models', 'knowledge', 'agents', 'activity', 'settings']) {
    await page.goto(`/ui/#/${route}`);
    const sidebar = page.locator('#workspace-sidebar');
    await expect(sidebar.getByRole('link', { name: 'Chat', exact: true })).toHaveCount(1);
    await expect(sidebar.getByRole('button', { name: 'Chat', exact: true })).toHaveCount(0);
    await expect(sidebar.locator('.brand svg.brand-symbol')).toBeVisible();
    await sidebar.getByRole('link', { name: 'Chat', exact: true }).click();
    await expect(page.locator('.composer textarea')).toBeVisible();
    await expect(sidebar.locator('.sidebar-collapse')).toBeVisible();
  }
});
