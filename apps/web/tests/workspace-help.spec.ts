import {expect,test} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {workspaceHelp} from '../src/i18n/workspace-help';
import {helpCenter} from '../src/i18n/help-center';
import type {LocaleCode} from '../src/i18n';
for(const locale of Object.keys(workspaceHelp) as LocaleCode[]) test('Searchable Help preserves drafts: '+locale,async({page},info)=>{
 const state=await craftFixture(page,locale,locale==='ar'?'dark':'light');
 await page.setViewportSize({width:390,height:760});await page.goto('/ui/#/chat');
 const editor=page.locator('.composer textarea');await editor.fill('Unsent — café 日本語');
 const trigger=page.locator('.help-trigger');await trigger.click();const help=page.locator('.workspace-help');
 await expect(help).toBeVisible();await expect(page.locator('dialog[open]')).toHaveCount(0);await expect(editor).toBeHidden();
 const search=help.getByRole('searchbox',{name:helpCenter[locale].search});await expect(search).toBeFocused();
 await search.fill(workspaceHelp[locale].returning);await expect(help.locator('.help-article')).toContainText(workspaceHelp[locale].returning);
 await search.fill('zzzunmatched');await expect(help.locator('.help-empty')).toContainText(helpCenter[locale].empty);
 await help.locator('.help-empty button').click();await expect(help.locator('.help-topics button')).toHaveCount(9);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath('help-'+locale+'.png'),fullPage:true});
 await help.getByRole('button',{name:workspaceHelp[locale].close,exact:true}).focus();await page.keyboard.press('Escape');
 await expect(help).toHaveCount(0);await expect(trigger).toBeFocused();await expect(editor).toHaveValue('Unsent — café 日本語');expect(state.writes).toEqual([]);
});
test('Help search, keyboard topics and destinations preserve a returning draft',async({page},info)=>{
 const state=await craftFixture(page,'en','dark');await page.setViewportSize({width:1440,height:1000});await page.goto('/ui/#/chat');
 await page.locator('.composer textarea').fill('Keep my draft');await page.keyboard.press('Control+k');
 await page.locator('dialog.command-palette').getByRole('combobox').fill('Help');await page.keyboard.press('Enter');
 const help=page.locator('.workspace-help');await expect(help).toBeVisible();await expect(page.locator('dialog[open]')).toHaveCount(0);
 const search=help.getByRole('searchbox');await search.fill('MEMORY larger');await expect(help.locator('.help-article')).toContainText('memory');
 await page.screenshot({path:info.outputPath('help-search-dark.png')});await search.fill('');
 await help.locator('.help-topics').getByRole('button',{name:'Activity',exact:true}).focus();await page.keyboard.press('Enter');
 await expect(help.locator('.help-article')).toBeFocused();await expect(help.locator('.help-article')).toContainText(workspaceHelp.en.returning);
 await help.getByRole('link',{name:'Activity',exact:true}).click();await expect(help).toHaveCount(0);await expect(page).toHaveURL(/#\/activity$/);
 await page.locator('.primary-nav a[href="#/chat"]').click();await expect(page.locator('.composer textarea')).toHaveValue('Keep my draft');expect(state.writes).toEqual([]);
});
test('Help stays within a short app viewport',async({page})=>{
 await craftFixture(page,'en','dark');await page.setViewportSize({width:640,height:450});await page.goto('/ui/#/chat');
 await page.locator('.help-trigger').click();const help=page.getByRole('region',{name:'Help',exact:true});await expect(help).toBeVisible();
 const bounds=()=>help.boundingBox();let box=await bounds(),viewport=await page.evaluate(()=>({width:innerWidth,height:innerHeight}));
 expect(box).not.toBeNull();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.y).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(viewport.width+1);expect(box!.y+box!.height).toBeLessThanOrEqual(viewport.height+1);
 await help.locator('.help-scroll').evaluate(node=>{node.scrollTop=node.scrollHeight;});await expect(help.getByRole('button',{name:workspaceHelp.en.close,exact:true})).toBeInViewport();
 await page.keyboard.press('Escape');await expect(help).toHaveCount(0);
});

for(const [locale,theme,width,height]of [['en','light',754,560],['ar','dark',390,460]]as const)test(`Help content scrolls below its heading: ${locale}`,async({page},info)=>{
 const state=await craftFixture(page,locale,theme);await page.setViewportSize({width,height});await page.goto('/ui/#/chat');
 await page.locator('.composer textarea').fill('Keep this draft');await page.locator('.help-trigger').click();
 const help=page.locator('.workspace-help'),heading=help.locator('.help-heading');const initial=await heading.boundingBox();
 const geometry=await help.evaluate(root=>{
  const topic=root.querySelector('.help-topics')!;
  const scroll=[root,...root.querySelectorAll('*')].find(el=>el.contains(topic)&&['auto','scroll'].includes(getComputedStyle(el).overflowY))!;
  scroll.scrollTop=180;
  return {scrollTop:scroll.scrollTop,top:scroll.getBoundingClientRect().top,headingBottom:root.querySelector('.help-heading')!.getBoundingClientRect().bottom};
 });
 expect(geometry.scrollTop).toBeGreaterThan(0);expect(geometry.top).toBeGreaterThanOrEqual(geometry.headingBottom-1);
 expect((await heading.boundingBox())!.y).toBe(initial!.y);
 await expect(heading.getByRole('button')).toBeInViewport();
 await page.screenshot({path:info.outputPath(`help-scrolled-${locale}.png`)});
 await help.locator('.help-topics button').first().click();await expect(help.locator('.help-article')).toBeFocused();
 const article=await help.locator('.help-article h3').boundingBox();expect(article!.y).toBeGreaterThanOrEqual((await heading.boundingBox())!.y+initial!.height-1);
 expect(await page.evaluate(()=>scrollY)).toBe(0);expect(state.writes).toEqual([]);
 await page.keyboard.press('Escape');await expect(page.locator('.composer textarea')).toHaveValue('Keep this draft');
});
