import {expect,test} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {workspaceHelp} from '../src/i18n/workspace-help';
import type {LocaleCode} from '../src/i18n';

for(const locale of Object.keys(workspaceHelp) as LocaleCode[]) test(`Help preserves drafts and fits localized narrow layout: ${locale}`,async({page},info)=>{
  const state=await craftFixture(page,locale,locale==='ar'?'dark':'light');
  await page.setViewportSize({width:390,height:640});await page.goto('/ui/#/chat');
  const editor=page.locator('.composer textarea');await editor.fill('Unsent — café 日本語');
  const trigger=page.getByRole('button',{name:workspaceHelp[locale].help,exact:true});await trigger.click();
  const help=page.getByRole('dialog',{name:workspaceHelp[locale].help,exact:true});
  await expect(help).toBeVisible();await expect(help).toContainText(workspaceHelp[locale].returning);
  await expect(page.locator('html')).toHaveCSS('overflow','hidden');
  await expect(help.getByRole('button',{name:workspaceHelp[locale].close})).toBeFocused();
  await page.keyboard.press('Control+k');await expect(page.locator('dialog[open]')).toHaveCount(1);
  const box=await help.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(390);expect(box!.y+box!.height).toBeLessThanOrEqual(640);
  expect(await help.locator('section p').first().evaluate(el=>getComputedStyle(el).fontSize)).toBe('16px');
  expect(await help.locator('h3').first().evaluate(el=>getComputedStyle(el).fontSize)).toBe('18px');
  if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`help-${locale}.png`)});
  await page.keyboard.press('Escape');await expect(help).toHaveCount(0);await expect(trigger).toBeFocused();await expect(editor).toHaveValue('Unsent — café 日本語');
  expect(state.writes).toEqual([]);
});

test('Help navigation and Quick actions never submit work or lose a returning draft',async({page})=>{
  const state=await craftFixture(page,'en','light');await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep my draft');
  await page.keyboard.press('Control+k');const commands=page.locator('dialog.command-palette');
  await commands.getByRole('combobox').fill('Help');await page.keyboard.press('Enter');
  const help=page.getByRole('dialog',{name:'Help',exact:true});await expect(help).toBeVisible();
  await page.keyboard.press('Escape');await expect(page.locator('.composer textarea')).toBeFocused();
  await page.locator('.help-trigger').click();
  await help.getByRole('link',{name:'Activity',exact:true}).click();await expect(help).toHaveCount(0);await expect(page).toHaveURL(/#\/activity$/);
  await page.locator('.primary-nav a[href="#/chat"]').click();await expect(page.locator('.composer textarea')).toHaveValue('Keep my draft');
  expect(state.writes).toEqual([]);
});
