import {test,expect} from '@playwright/test';

test('login mark follows explicit warm theme, RTL and forced colors',async({page})=>{
  await page.emulateMedia({colorScheme:'light'});
  await page.addInitScript(()=>{
    localStorage.setItem('offgrid.locale','ar');localStorage.setItem('offgrid.theme','dark');
    localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'warm',dark:'warm',textSize:100,reduceMotion:false}));
  });
  await page.route(/\/(?:v1|api\/v2)\//,r=>r.fulfill({status:401,json:{error:'Authentication required'}}));
  await page.goto('/ui/#/chat');
  const mark=page.locator('.login-card .brand-symbol');await expect(mark).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('dir','rtl');
  await expect(page.locator('html')).toHaveAttribute('data-palette','warm');
  for(const forcedColors of ['none','active'] as const){
    await page.emulateMedia({forcedColors});
    const colors=await mark.evaluate(el=>({stroke:getComputedStyle(el.querySelector('g')!).stroke,text:getComputedStyle(el).color,fill:getComputedStyle(el).fill}));
    expect(colors.stroke).toBe(colors.text);expect(colors.fill).toBe('none');
    await expect(page.locator('.login-card img')).toHaveCount(0);
  }
  await page.screenshot({path:test.info().outputPath('login-brand.png')});
});
