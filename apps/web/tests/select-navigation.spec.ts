import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {workspaceShell} from '../src/i18n/workspace-shell';

for(const locale of ['de','ar','en'] as const) test(`dropdown geometry and compact navigation ${locale}`,async({page})=>{
  const state=await craftFixture(page,locale,'light');
  await page.addInitScript(()=>{if(!localStorage.getItem('offgrid.appearance.v1'))localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'dracula',dark:'solarized',textSize:150,reduceMotion:false}));});
  await page.setViewportSize({width:locale==='de'?640:390,height:900});
  await page.goto('/ui/#/settings');
  const nav=page.locator('.navigation-trigger');
  await expect(nav).toHaveAccessibleName(workspaceShell[locale].navigation);
  expect((await nav.boundingBox())!.width).toBeLessThanOrEqual(44);
  await expect(nav).toHaveText('');
  await nav.click();await expect(page.locator('.sidebar')).toBeVisible();
  await page.keyboard.press('Escape');await expect(nav).toBeFocused();
  const select=page.locator('#appearance-text-size');
  await expect(select).toHaveCSS('font-size','21px');
  await expect(select).toHaveCSS('appearance','none');
  const geometry=await select.evaluate(el=>{
    const s=getComputedStyle(el);return {height:el.getBoundingClientRect().height,end:parseFloat(s.paddingInlineEnd),start:parseFloat(s.paddingInlineStart),image:s.backgroundImage};
  });
  expect(geometry.height).toBeGreaterThanOrEqual(40);expect(geometry.end).toBeGreaterThanOrEqual(32);expect(geometry.start).toBeGreaterThanOrEqual(10);expect(geometry.image).toContain('gradient');
  await page.screenshot({path:test.info().outputPath('settings.png'),fullPage:true});
  await select.focus();await page.keyboard.press('Home');await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');
  await expect(select).toHaveValue('110');
  await page.reload();await expect(select).toHaveValue('110');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.emulateMedia({forcedColors:'active'});
  await expect(select).toHaveCSS('appearance','auto');await expect(select).toHaveCSS('background-image','none');
  await select.focus();await expect(select).toHaveCSS('outline-style','solid');
  expect(state.writes).toEqual([]);
});
