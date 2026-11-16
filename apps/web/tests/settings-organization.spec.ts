import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
for(const locale of ['en','ar','zh-Hans','de']) test('desktop Settings separates real capabilities: '+locale,async({page})=>{
 const fixture=await craftFixture(page,locale,'light');
 await page.addInitScript(()=>{
  const state={mode:'full',shortcut:'CommandOrControl+Shift+Space',registered:true};
  (window as any).shortcutWrites=[];
  (window as any).electron={platform:'win32',getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},getPaths:async()=>({models:'C:/Workspace/models',data:'C:/Workspace/data',config:'C:/Workspace/config'}),
   getCommandWindow:async()=>state,onCommandWindow:()=>()=>{},setCommandShortcut:async(shortcut:string)=>{(window as any).shortcutWrites.push(shortcut);return {...state,shortcut};},openCommandWindow:async()=>state};
 });
 await page.setViewportSize({width:1440,height:900});await page.goto('/ui/#/settings');
 await expect(page.locator('.settings-navigation nav button')).toHaveCount(9);
 await expect(page.locator('#settings-language')).toBeVisible();await expect(page.locator('#desktop-command-shortcut')).toHaveCount(0);
 await page.locator('[data-section=shortcuts]').click();
 const shortcut=page.locator('#desktop-command-shortcut');await expect(shortcut).toBeEnabled();await shortcut.selectOption('Alt+Space');
 expect(await page.evaluate(()=>(window as any).shortcutWrites)).toEqual(['Alt+Space']);
 await expect(page.locator('#settings-language')).toBeHidden();await expect(page.locator('.desktop-local-paths')).toHaveCount(0);
 await page.locator('[data-section=storage]').click();await expect(page.locator('.desktop-local-paths')).toContainText('C:/Workspace/models');
 await expect(shortcut).toHaveCount(0);await page.reload();await expect(page.locator('.desktop-local-paths')).toBeVisible();
 const out=resolve('../../build/ui-craft/quick-access-repair');mkdirSync(out,{recursive:true});await page.screenshot({path:resolve(out,'settings-storage-'+locale+'.png')});
 await page.locator('[data-section=access]').click();await expect(page.locator('.desktop-local-paths')).toHaveCount(0);
 await page.locator('[data-section=shortcuts]').click();await page.setViewportSize({width:390,height:780});await expect(shortcut).toBeInViewport();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:resolve(out,'settings-shortcuts-'+locale+'.png')});expect(fixture.writes).toEqual([]);
});

for(const locale of ['en','ar']) test('grouped settings remain usable at 200 percent zoom '+locale,async({page})=>{
 await craftFixture(page,locale,'dark');
 await page.addInitScript(()=>localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'offgrid',dark:'dracula',textSize:100,reduceMotion:false})));
 await page.setViewportSize({width:1440,height:1100});await page.goto('/ui/#/settings/models');
 await expect(page.locator('html')).toHaveAttribute('data-palette','dracula');
 await page.evaluate(()=>document.documentElement.style.zoom='2');
 const actions=page.locator('.settings-content .action-card:visible');await expect(actions).toHaveCount(2);const first=await actions.first().boundingBox(),second=await actions.nth(1).boundingBox();
 expect(second!.y).toBeGreaterThanOrEqual(first!.y+first!.height);
 const fields=page.locator('.response-settings select');await fields.nth(1).selectOption('extended');await expect(fields.nth(1)).toHaveValue('extended');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 for(const item of await page.locator('.settings-content .action-card:visible,.response-settings select').all()) {
  await item.scrollIntoViewIfNeeded();const box=await item.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(1441);
 }
 const out=resolve('../../build/ui-craft/control-refinement');mkdirSync(out,{recursive:true});
 await page.screenshot({path:resolve(out,'response-dracula-zoom-'+locale+'.png'),fullPage:true});
});

test('specific Settings links focus after the destination is rendered',async({page})=>{
 await craftFixture(page,'en','light');await page.goto('/ui/#/settings/text-size');await expect(page.locator('#appearance-text-size')).toBeFocused();
 await page.getByRole('searchbox',{name:'Search settings'}).fill('language');await page.locator('.settings-results a').click();
 await expect(page.locator('#settings-language')).toBeFocused();
});

for(const [locale,theme,width] of [['en','light',1440],['en','dark',1440],['de','light',720],['ar','dark',390],['zh-Hans','light',320]] as const) test('model and voice settings hierarchy '+locale+' '+theme,async({page})=>{
 const fixture=await craftFixture(page,locale,theme);
 await page.route('**/v1/audio/status',route=>route.fulfill({json:{asr:{available:true},tts:{available:true},profiles:[
  {id:'whisper-exact',revision:'r1',name:'ggerganov/whisper.cpp · ggml-tiny.en.bin',architecture:'whisper',capabilities:['transcription'],available:true},
  {id:'piper-exact',revision:'r2',name:'rhasspy/piper-voices · en_US-lessac-medium.onnx',architecture:'piper',capabilities:['speech_synthesis'],available:true,voices:[{id:'lessac',language:'en-US'}]}
 ]}}));
 await page.setViewportSize({width,height:960});await page.goto('/ui/#/settings/models');
 const cards=page.locator('.settings-content .action-card-grid');await expect(cards.getByRole('link')).toHaveCount(2);
 await expect(page.locator('.settings-content details.response-advanced')).toHaveCount(0);
 const fields=page.locator('.response-settings select');await expect(fields).toHaveCount(3);
 await fields.nth(1).selectOption('extended');await fields.last().selectOption('1024');await page.reload();
 await expect(fields.nth(1)).toHaveValue('extended');await expect(fields.last()).toHaveValue('1024');
 const out=resolve('../../build/ui-craft/control-refinement');mkdirSync(out,{recursive:true});
 await page.screenshot({path:resolve(out,'responses-'+locale+'-'+theme+'.png'),fullPage:true});
 await page.goto('/ui/#/settings/voice');const choices=page.locator('.voice-model-settings .model-choice-trigger');
 await expect(choices.first()).toContainText('Whisper · tiny.en');await expect(choices.last()).toContainText('Piper · en US lessac medium');
 await choices.first().click();await expect(page.locator('.model-choice-options code')).toHaveCount(0);
 await page.locator('.model-choice-details').first().click();await expect(page.locator('.model-choice-options code').last()).toHaveText('whisper-exact@r1');
 await page.keyboard.press('Escape');await expect(choices.first()).toBeFocused();
 await page.locator('#voice-settings-title').click();
 if(width>=1000) {
  const choice=page.locator('.voice-model-settings .model-choice').first();
  const label=await choice.locator('.model-choice-label').boundingBox(),button=await choices.first().boundingBox();
  expect(Math.abs(label!.y+label!.height/2-button!.y-button!.height/2)).toBeLessThan(3);
 }
 await page.screenshot({path:resolve(out,'voice-settings-'+locale+'-'+theme+'.png'),fullPage:true});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 expect(fixture.writes).toEqual([]);
 await page.locator('.voice-settings-panel .action-card').click();await expect(page.locator('.model-category-filter')).toHaveValue('speech_recognition');
});
