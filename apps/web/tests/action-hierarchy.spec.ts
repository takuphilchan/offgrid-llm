import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { workspaceManagement } from '../src/i18n/workspace-management';
import { knowledgeGuidance } from '../src/i18n/knowledge-options';

for(const theme of ['light','dark']) test(`shared action weights remove redundant borders while preserving focus: ${theme}`,async({page})=>{
 const state=await craftFixture(page,'en',theme);
 for(const route of ['chat','agents','agents/tools','agents/connections','knowledge','models','activity','settings']) {
  await page.goto(`/ui/#/${route}`);await expect(page.locator('.page-content')).toBeVisible();
  await page.mouse.move(0,0);
  const controls=page.locator('.page-content :is(.secondary-button,.text-button,.utility-button):visible');
  for(const control of await controls.all()) {
   const border = await control.evaluate(el => { const style=getComputedStyle(el); return style.borderTopWidth==='0px'||style.borderTopColor==='rgba(0, 0, 0, 0)'; });
   expect(border).toBe(true);
   if(await control.isEnabled()){await control.focus();await expect(control).toHaveCSS('outline-width','2px');}
  }
 }
 expect(state.writes).toEqual([]);
});

test('Knowledge primary, secondary and management roles remain distinct and reachable',async({page})=>{
 const state=await craftFixture(page,'en','light');await page.goto('/ui/#/knowledge');
 const add=page.getByRole('button',{name:'Add document',exact:true}),ask=page.getByRole('button',{name:'Ask using Knowledge',exact:true}),manage=page.getByRole('button',{name:'Manage Knowledge',exact:true});
 await expect(page.locator(".workspace-section-heading").getByRole("button",{name:"Add document",exact:true})).toBeVisible();
 await expect(page.locator(".knowledge-library-toolbar").getByRole("button",{name:"Ask using Knowledge",exact:true})).toBeVisible();
 await expect(manage).toHaveClass(/icon-button/);
 await manage.focus();await manage.press('Enter');await expect(manage).toHaveAttribute('aria-expanded','true');await manage.press('Escape');await expect(manage).toBeFocused();
 expect(state.writes).toEqual([]);
});

for(const locale of ['en','ar'] as const) for(const width of [320,1440]) test(`Knowledge separates library management from asking: ${locale} ${width}`,async({page},info)=>{
 const state=await craftFixture(page,locale,'light');await page.setViewportSize({width,height:900});await page.goto('/ui/#/knowledge');
 const copy=workspaceManagement(locale);
 const manage=page.getByRole('button',{name:copy.manageKnowledge,exact:true});
 await expect(page.locator('.workspace-section-heading').getByRole('button',{name:copy.manageKnowledge,exact:true})).toBeVisible();
 const ask=page.getByRole('button',{name:copy.askKnowledge,exact:true});
 await expect(ask).toHaveAccessibleDescription(knowledgeGuidance(locale).scope);
 const button=await ask.boundingBox(),description=await page.locator('#knowledge-retrieval-scope').boundingBox();
 expect(description!.y).toBeGreaterThanOrEqual(button!.y+button!.height);
 expect(description!.width).toBeLessThanOrEqual(720);
 const controls=await page.locator('.knowledge-workspace > header button:visible').evaluateAll(nodes=>nodes.map(node=>{const r=node.getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,bottom:r.bottom};}));
 for(let i=0;i<controls.length;i++)for(const other of controls.slice(i+1))expect(Math.min(controls[i].right,other.right)-Math.max(controls[i].left,other.left)>1&&Math.min(controls[i].bottom,other.bottom)-Math.max(controls[i].top,other.top)>1).toBe(false);
 await manage.focus();await manage.press('Enter');await expect(manage).toHaveAttribute('aria-expanded','true');await page.keyboard.press('Escape');await expect(manage).toBeFocused();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);expect(state.writes).toEqual([]);
 await page.screenshot({path:info.outputPath(`knowledge-placement-${locale}-${width}.png`),fullPage:true});
});

test('composer keeps one outer boundary and usable inner selectors',async({page})=>{
 const state=await craftFixture(page,'en','light');await page.goto('/ui/#/chat');
 await expect(page.locator('.composer')).toHaveCSS('border-top-width','1px');
 const settings=page.getByRole('button',{name:'Response settings',exact:true});
 await settings.click();
 for(const name of ['Agent','Model']){
  const control=page.getByRole('button',{name,exact:true});await expect(control).toHaveCSS('border-top-color','rgba(0, 0, 0, 0)');
  await control.focus();await control.press('Enter');await expect(control).toHaveAttribute('aria-expanded','true');
  await page.keyboard.press('Escape');await expect(control).toBeFocused();
 }
 await page.keyboard.press('Escape');await expect(settings).toBeFocused();
 await expect(page.locator('.composer-actions .voice-button')).toHaveCSS('border-top-color','rgba(0, 0, 0, 0)');
 expect(state.writes).toEqual([]);
});

test('forced colors retains explicit secondary and selected-tab boundaries',async({page})=>{
 await craftFixture(page,'en','light');await page.emulateMedia({forcedColors:'active'});await page.goto('/ui/#/models');
 const selected=page.getByRole('tab',{name:'Installed',exact:true});
 await expect(selected).toHaveAttribute('aria-selected','true');await expect(selected).not.toHaveCSS('border-top-color','rgba(0, 0, 0, 0)');
 await selected.focus();await expect(selected).toHaveCSS('outline-width','2px');
});

test.describe('touch controls',()=>{
 test.use({hasTouch:true,viewport:{width:390,height:844}});
 test('settings, composer and Knowledge controls retain 44px targets',async({page})=>{
  const state=await craftFixture(page,'en','light');
  for(const route of ['settings','chat','knowledge']){
   await page.goto(`/ui/#/${route}`);expect(await page.evaluate(()=>matchMedia('(pointer: coarse)').matches)).toBe(true);
   const controls=route==='settings'?page.locator('.settings-group :is(select,button):visible'):route==='chat'?page.locator('.composer button:visible'):page.locator('.knowledge-library-toolbar button:visible');
   await expect(controls.first()).toBeVisible();
   for(const control of await controls.all()){const box=await control.boundingBox();expect(box!.height,await control.getAttribute('class')??'control').toBeGreaterThanOrEqual(44);}
  }
  expect(state.writes).toEqual([]);
 });
});
