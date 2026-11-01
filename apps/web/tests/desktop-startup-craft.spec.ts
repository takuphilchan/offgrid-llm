import { test, expect } from '@playwright/test';
import { readFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

// Chromium rendering of the real startup assets with an inert bridge. This
// does not launch Electron, connect a service or exercise native window controls.
const root=resolve('../..'), desktop=resolve(root,'apps/desktop');
const copy=JSON.parse(readFileSync(resolve(root,'apps/web/public/desktop-presentation.json'),'utf8'));
// Exact startup.css from 7357acae431e40092fd40a5458207d3649d503e8.
// Keep the historical comparison reproducible in shallow checkouts/source archives.
const oldCSS=readFileSync(resolve(root,'apps/web/tests/fixtures/desktop-startup-before.css'),'utf8');
for(const theme of ['light','dark']) for(const locale of ['en','ar']) for(const width of [390,1440]) test(`startup surface ${theme} ${locale} ${width}`,async({page})=>{
  await page.setViewportSize({width,height:900});
  let before=false;
  await page.addInitScript(({theme,locale,copy})=>{
    Object.defineProperty(window,'electron',{value:{
      presentation:{locale,effectiveTheme:theme,copy},
      getBackendInfo:async()=>({state:'starting',desktopVersion:'fixture',url:'http://isolated-fixture.invalid',elapsedMs:1200}),
      onStartupState:()=>()=>{}
    }});
  },{theme,locale,copy:copy[locale]});
  await page.route('**/*',r=>{
    const name=new URL(r.request().url()).pathname.slice(1);
    if(!['loading.html','startup.css','startup.js'].includes(name))return r.abort();
    return r.fulfill({body:name==='startup.css'&&before?oldCSS:readFileSync(resolve(desktop,name),'utf8'),contentType:name.endsWith('.html')?'text/html':name.endsWith('.css')?'text/css':'text/javascript'});
  });
  const dir=resolve(root,'build/ui-craft/startup',`${theme}-${locale}-${width}`);mkdirSync(dir,{recursive:true});
  for(const phase of ['before','after']){
    before=phase==='before';await page.goto('http://desktop-fixture.invalid/loading.html');
    await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
    await expect(page.locator('#status-title')).toHaveText(copy[locale].starting);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
    await page.screenshot({path:resolve(dir,`${phase}.png`),fullPage:true,animations:'disabled'});
    await expect(page.locator('body')).toHaveCSS('background-image','none');
  }
});
