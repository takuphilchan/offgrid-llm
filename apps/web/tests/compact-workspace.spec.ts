import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

for(const width of [390,1440]) for(const locale of ['en','ar']) test(`direct editor and options ${width} ${locale}`,async({page},info)=>{
 await page.setViewportSize({width,height:900});
 const state=await craftFixture(page,locale,locale==='ar'?'dark':'light');
 await page.goto('/ui/#/agents/profiles');
 await page.locator('.profile-library-header > button').click();
 const basics=page.locator('.profile-basics'),resources=page.locator('.profile-fields > .profile-resources');
 await expect(basics).toBeVisible(); await expect(resources).toBeVisible();
 await expect(page.locator('.profile-capability > summary')).toHaveCount(3);
 await expect(page.locator('.profile-capability[open]')).toHaveCount(0);
 await expect(page.locator('.profile-advanced .profile-context-resources')).toHaveCount(0);
 const [a,b]=await Promise.all([basics.boundingBox(),resources.boundingBox()]);
 if(width===1440)expect(Math.abs(a!.y-b!.y)).toBeLessThan(25);
 else expect(b!.y).toBeGreaterThan(a!.y+a!.height);
 await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 const fields=await page.locator('.profile-fields').boundingBox(),actions=await page.locator('.profile-editor > .profile-actions').boundingBox();
 if(width<960) expect(actions!.y).toBeGreaterThanOrEqual(fields!.y+fields!.height);
 else {
  expect(actions!.y+actions!.height).toBeLessThanOrEqual(900);
  for(const field of await basics.locator('input:visible, textarea:visible').all()) {
   await field.focus();
   await expect.poll(()=>field.evaluate(el=>{const r=el.getBoundingClientRect();const hit=document.elementFromPoint(r.x+r.width/2,r.y+r.height/2);return hit===el||el.contains(hit) ? 'visible' : `${el.closest('label')?.textContent}: ${r.y}/${r.height}, covered by ${hit?.tagName}.${hit?.className}`;})).toBe('visible');
  }
 }
 expect(state.writes).toEqual([]);
 await page.screenshot({path:info.outputPath('editor.png'),fullPage:true});
 if(width<1024)await page.locator('.navigation-trigger').click();
 const trigger=page.locator('.workspace-options > button');
 await trigger.click();
 const panel=page.locator('.workspace-options-panel');
 await expect(panel).toBeVisible();
 await expect(panel).toHaveCSS('opacity','1');
 const box=await panel.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(width);
 await page.screenshot({path:info.outputPath('options.png')});
 await page.keyboard.press('Escape');await expect(trigger).toBeFocused();await expect(panel).not.toBeVisible();
});

test('history thumb keeps its lane and scrolls without moving footer',async({page},info)=>{
 await page.setViewportSize({width:1440,height:900});await craftFixture(page,'en','dark');
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:Array.from({length:80},(_,i)=>({name:`Conversation ${i}`,model_id:'fixture',updated_at:'2026-10-30T00:00:00Z',messages:[]}))}}));
 await page.goto('/ui/#/chat');
 const list=page.locator('.sidebar-context .history-list'),footer=page.locator('.sidebar-foot');
 await expect(list.locator('.history-row').first()).toBeVisible();
 await expect(list).toHaveCSS('scrollbar-gutter','stable');await expect(list).toHaveCSS('overflow-x','hidden');
 const before=await footer.boundingBox();await list.hover();await page.mouse.wheel(0,500);
 await expect.poll(()=>list.evaluate(el=>el.scrollTop)).toBeGreaterThan(0);
 expect(await footer.boundingBox()).toEqual(before);
 await page.screenshot({path:info.outputPath('history.png')});
 await page.emulateMedia({forcedColors:'active'});await expect(list).toHaveCSS('scrollbar-color','auto');
});

test('coarse-pointer targets remain at least 44px',async({browser})=>{
 const context=await browser.newContext({hasTouch:true,viewport:{width:390,height:800}});const page=await context.newPage();
 await craftFixture(page,'en','light');await page.goto('/ui/#/agents/profiles');
 for(const selector of ['.profile-library-header > button','.navigation-trigger'])expect((await page.locator(selector).boundingBox())!.height).toBeGreaterThanOrEqual(44);
 await context.close();
});
