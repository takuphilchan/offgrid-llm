import {test,expect} from '@playwright/test';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {craftFixture} from './helpers/craft-fixture';
const desktop=resolve('../desktop');
const themes=JSON.parse(readFileSync(resolve(desktop,'theme-palettes.json'),'utf8')).themes;
const copy=JSON.parse(readFileSync(resolve('public/desktop-presentation.json'),'utf8'));

for(const [theme,palette] of [['light','slate'],['dark','offgrid']] as const) {
 test('terminal cards both respond to hover and keep rounded canvas '+theme,async({page},info)=>{
  await page.addInitScript(presentation=>{(window as any).terminalDesktop={onEvent:()=>()=>{},request:async()=>({generation:'fixture',available:true,presentation})};},{theme,locale:'en',palette:themes[palette][theme],visible:true});
  await page.setViewportSize({width:1100,height:740});await page.goto(pathToFileURL(resolve(desktop,'terminal.html')).href);
  const cards=page.locator('.terminal-launch-options button');await expect(cards).toHaveCount(2);
  for(const card of await cards.all()) {
   await page.mouse.move(1,300);await expect(card).not.toBeDisabled();
   const before=await card.evaluate(el=>getComputedStyle(el).backgroundColor);
   await card.hover();await expect.poll(()=>card.evaluate(el=>getComputedStyle(el).backgroundColor)).not.toBe(before);
   await expect(card).toHaveCSS('transform','matrix(1, 0, 0, 1, 0, -2)');
  }
  await expect(page.locator('body')).toHaveCSS('border-top-left-radius','20px');
  await page.screenshot({path:info.outputPath('terminal-'+theme+'.png')});
  await page.emulateMedia({reducedMotion:'reduce'});await cards.first().hover();await expect(cards.first()).toHaveCSS('transform','none');
 });
 test('startup follows the chosen palette, with restrained loading and actionable recovery '+theme,async({page},info)=>{
  const presentation={locale:'en',effectiveTheme:theme,palette:themes[palette][theme],appearance:{reduceMotion:false},copy:copy.en};
  await page.addInitScript(presentation=>{(window as any).electron={presentation,getPresentation:async()=>presentation,getBackendInfo:async()=>({state:'starting'}),onStartupState:(render:any)=>{(window as any).startupFixtureState=render;return()=>{};}};},presentation);
  await page.goto(pathToFileURL(resolve(desktop,'loading.html')).href);
  await expect(page.locator('html')).toHaveAttribute('data-startup-state','loading');
  await expect(page.locator('#technical')).toBeHidden();await expect(page.locator('#recovery')).toBeHidden();
  expect(await page.locator('html').evaluate(el=>getComputedStyle(el).getPropertyValue('--bg').trim())).toBe(presentation.palette['surface-0']);
  await expect(page.locator('header svg')).toHaveCSS('animation-name','breathe');
  await page.screenshot({path:info.outputPath('startup-'+theme+'.png')});
  await page.emulateMedia({reducedMotion:'reduce'});await expect(page.locator('header svg')).toHaveCSS('animation-name','none');
  await page.evaluate(()=>(window as any).startupFixtureState({state:'unavailable',reason:'Fixture service is stopped',managedByDesktop:true}));
  await expect(page.locator('#retry')).toBeVisible();await expect(page.locator('#technical')).toBeVisible();await expect(page.locator('#separate')).toBeHidden();
  await page.setViewportSize({width:390,height:700});await page.screenshot({path:info.outputPath('recovery-'+theme+'.png')});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 });
}

test('shell separates its panels with rounded surfaces rather than vertical rules',async({page})=>{
 await craftFixture(page,'en','light');await page.setViewportSize({width:1440,height:900});
 for(const route of ['chat','settings/shortcuts','knowledge']) {
  await page.goto('/ui/#/'+route);
  await expect(page.locator('main.workspace')).toHaveCSS('border-top-left-radius','20px');
  await expect(page.locator('main.workspace')).toHaveCSS('border-left-width','0px');
  await expect(page.locator('.sidebar-rail')).toHaveCSS('border-right-width','0px');
 }
});

for(const [locale,theme,width] of [['en','light',1440],['ar','dark',390]] as const) test('Library action cards and child widgets '+locale,async({page},info)=>{
 const state=await craftFixture(page,locale,theme);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:['knowledge-collections-v1','document-comparison-jobs-v1','document-analysis-history-v1','document-synthesis-candidates-v1']}}));
 await page.route('**/v1/collections*',r=>r.fulfill({json:{collections:[]}}));
 await page.route('**/v1/users',r=>r.fulfill({json:{users:[]}}));
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:[]}}));
 await page.route('**/v1/document-analyses/synthesis?*',r=>r.fulfill({json:{analyses:[],next_offset:null}}));
 await page.setViewportSize({width,height:900});await page.goto('/ui/#/knowledge');
 const cards=page.locator('.knowledge-workflows .action-card');await expect(cards).toHaveCount(2);
 await page.screenshot({path:info.outputPath('library-'+locale+'.png'),fullPage:true});
 const collections=page.locator('.collection-library-controls button.icon-button');await collections.click();const manager=page.locator('.collection-manager');await expect(manager).toBeVisible();
 await manager.locator('.collection-create-choice').click();await expect(manager.locator('.collection-create input')).toBeVisible();
 await page.screenshot({path:info.outputPath('collection-'+locale+'.png')});
 await page.keyboard.press('Escape');await expect(collections).toBeFocused();
 await cards.nth(1).click();await expect(page.locator('.synthesis-modes label')).toHaveCount(2);
 await page.locator('.synthesis-modes label').last().click();await expect(page.locator('.synthesis-modes input').last()).toBeChecked();
 await page.locator('.synthesis-modes input').last().focus();await page.keyboard.press('ArrowLeft');
 await expect(page.locator('.synthesis-modes input').first()).toBeChecked();
 await page.screenshot({path:info.outputPath('analysis-'+locale+'.png')});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.keyboard.press('Escape');await expect(cards.nth(1)).toBeFocused();
 expect(state.writes).toEqual([]);
});
