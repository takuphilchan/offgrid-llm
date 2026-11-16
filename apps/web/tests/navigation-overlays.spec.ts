import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const locale of ['en','ar']) test('navigation restores keyboard focus: '+locale,async({page})=>{
 const state=await craftFixture(page,locale,'light');await page.setViewportSize({width:390,height:700});await page.goto('/ui/#/settings');
 const trigger=page.locator('.navigation-trigger');await trigger.click();const drawer=page.locator('.sidebar');
 await expect(drawer).toBeVisible();await drawer.locator('.sidebar-foot a').focus();await page.keyboard.press('Tab');
 expect(await drawer.evaluate(el=>el.contains(document.activeElement))).toBe(true);
 await page.keyboard.press('Escape');await expect(drawer).toBeHidden();await expect(trigger).toBeFocused();expect(state.writes).toEqual([]);
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
