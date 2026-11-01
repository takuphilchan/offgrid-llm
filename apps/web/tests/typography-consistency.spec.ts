import {expect,test} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

for(const route of ['chat','agents/profiles','models','knowledge','activity','settings']) test(`visible interface text respects the minimum type scale: ${route}`,async({page})=>{
  await craftFixture(page,'en','light');await page.goto(`/ui/#/${route}`);
  await expect(page.locator('.topbar h1')).toBeVisible();
  const small=await page.locator('.app-shell').evaluate(root=>Array.from(root.querySelectorAll<HTMLElement>('*')).filter(el=>{
    const box=el.getBoundingClientRect(),style=getComputedStyle(el);
    return box.width>0&&box.height>0&&style.visibility!=='hidden'&&parseFloat(style.fontSize)<12&&Array.from(el.childNodes).some(n=>n.nodeType===Node.TEXT_NODE&&n.textContent?.trim());
  }).map(el=>({tag:el.tagName,class:el.className,size:getComputedStyle(el).fontSize,text:el.textContent?.slice(0,65)})));
  expect(small).toEqual([]);
});
