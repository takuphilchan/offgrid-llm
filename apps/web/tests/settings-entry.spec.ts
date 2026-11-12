import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {locales} from '../src/i18n';

for(const [locale,width] of [['en',1280],['sw',1280],['de',640],['ar',390]] as const){
  test(`gear opens Settings directly and overflow stays separate ${locale} ${width}`,async({page})=>{
    const state=await craftFixture(page,locale,'light');
    await page.setViewportSize({width,height:800});
    await page.goto('/ui/#/chat');
    await page.locator('.composer textarea').fill('Keep this draft: café 日本語 مرحبا');
    if(width<1024)await page.locator('.navigation-trigger').click();
    const gear=page.locator('.sidebar-foot a[href="#/settings"]');
    await expect(gear).toHaveAccessibleName(locales[locale].messages.nav.settings);
    await gear.click();
    await expect(page).toHaveURL(/#\/settings$/);
    await expect(page.locator('#settings-language')).toBeVisible();
    await expect(page.locator('.workspace-options-panel')).toBeHidden();
    await expect(gear).toHaveAttribute('aria-current','page');
    if(width<1024){await expect(page.locator('.sidebar')).toBeHidden();await page.locator('.navigation-trigger').click();}
    const more=page.locator('.workspace-options > button');await more.click();
    const panel=page.locator('.workspace-options-panel');await expect(panel).toBeVisible();
    await expect(panel.locator('select,a[href="#/settings"]')).toHaveCount(0);
    await expect(panel.getByRole('status')).toBeVisible();
    await expect(panel.locator('button[aria-busy]')).toBeVisible();
    await page.screenshot({path:test.info().outputPath('separate-workspace-menu.png')});
    await page.keyboard.press('Escape');await expect(more).toBeFocused();
    if(width<1024)await page.keyboard.press('Escape');
    await page.goto('/ui/#/chat');
    await expect(page.locator('.composer textarea')).toHaveValue('Keep this draft: café 日本語 مرحبا');
    expect(state.writes).toEqual([]);
  });
}
