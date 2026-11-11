import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {settingsNavigation} from '../src/i18n/settings-navigation';

test('empty libraries and management sections do not leave orphaned rules',async({page})=>{
  const state=await craftFixture(page,'en','light');state.mode='empty';
  await page.addInitScript(()=>localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'dracula',dark:'dracula',textSize:100,reduceMotion:false})));
  await page.route('**/v1/models',r=>r.fulfill({json:{data:[]}}));
  await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[]}}));
  await page.route('**/api/v2/agent-profiles',r=>r.fulfill({json:{profiles:[]}}));
  await page.route('**/api/v2/jobs',r=>r.fulfill({json:[]}));
  for(const route of ['models','knowledge','agents','activity','agents/tools','agents/connections','agents/external']){
    await page.goto('/ui/#/'+route);
    await expect(page.locator('main h1')).toBeVisible();
    if(route==='models') {
      await expect(page.locator('.model-section[aria-busy="true"]')).toHaveCount(0);
      // An empty language-package section used to inherit a top border.
      expect(await page.locator('.model-section').evaluateAll(items=>items.filter(el=>!el.textContent?.trim()).length)).toBe(0);
    }
    expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
    await page.screenshot({path:test.info().outputPath(route.replaceAll('/','-')+'.png'),fullPage:true});
  }
  await page.goto('/ui/#/settings');
  for(const name of settingsNavigation.en.sections){
    const button=page.locator('.settings-navigation').getByRole('button',{name,exact:true});
    await button.click();
    await expect(button).toHaveAttribute('aria-current','page');
    expect(await page.locator('.settings-content').evaluate(el=>el.scrollWidth<=el.clientWidth)).toBe(true);
    await page.screenshot({path:test.info().outputPath('settings-'+name.replaceAll(/[^a-z]/gi,'-')+'.png'),fullPage:true});
  }
  expect(state.writes).toEqual([]);
});
