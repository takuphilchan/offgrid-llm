import {test,expect,type Page} from '@playwright/test';
import {settingsNavigation} from '../src/i18n/settings-navigation';
import {resolveMessages} from '../src/i18n/catalog';
import {changePreferences,syncPreferences} from './helpers/workspace-actions';
import registry from '../../desktop/language-registry.json' with {type:'json'};
import {inputs,entriesFor} from '../../../scripts/quality/translation-review.mjs';

async function fixture(page:Page){
 await page.addInitScript(()=>{localStorage.setItem('offgrid.onboarding.complete','true');localStorage.setItem('offgrid.locale','en');});
 await page.route(/\/(?:health|v1\/|api\/v2\/)/,route=>{
  const path=new URL(route.request().url()).pathname;
  const data:Record<string,unknown>={
   '/health':{status:'healthy'},'/api/v2/system':{product:'offgrid',api_version:2,workspace_id:'locale-fixture',capabilities:['agent-profiles-v1']},
   '/v1/users/me':{authenticated:false,user:null},'/v1/models':{data:[{id:'fixture-model',type:'chat'}]},
   '/v1/sessions':{sessions:[]},'/v1/rag/status':{enabled:false},'/v1/audio/status':{profiles:[]},
   '/api/v2/agent-profiles':{profiles:[]},'/api/v2/agent-profiles/resources':{tools:{},computer_permitted:false}
  };return route.fulfill({json:data[path]??{}});
 });
}

test('feature resolution preserves context, missing siblings and literal user-like text',()=>{
 const original={action:'Stop',nested:{body:'User <input> $&',cancel:'Cancel'},options:['Light','Dark']};
 const next=resolveMessages(original,{'action':'停止','options.1':'深色','unknown':'ignored'});
 expect(next).toEqual({action:'停止',nested:original.nested,options:['Light','深色']});
 expect(original.options).toEqual(['Light','Dark']);
 expect(settingsNavigation['zh-Hans'].search).toBe('搜索设置');
 expect(settingsNavigation['pt-PT'].sections[7]).toBeUndefined();
});

test('global feature language reaches settings, chooser, agent controls and header',async({page})=>{
 await fixture(page);await page.goto('/ui/#/settings');
 await page.locator('#settings-language').selectOption('zh-Hans');
 await expect(page.getByRole('searchbox',{name:'搜索设置'})).toBeVisible();
 await expect(page.getByRole('button',{name:'模型与回复',exact:true})).toBeVisible();
 await page.locator('.theme-summary').nth(1).click();
 await expect(page.getByRole('dialog',{name:'选择主题'})).toBeVisible();
 await expect(page.getByRole('searchbox',{name:'搜索主题'})).toBeVisible();
 await page.keyboard.press('Escape');
 await expect(page.locator('#settings-language')).toHaveValue('zh-Hans');
 await page.goto('/ui/#/agents');
 await expect(page.getByRole('button',{name:'新建智能体',exact:true})).toBeVisible();
 await page.setViewportSize({width:390,height:844});
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:test.info().outputPath('chinese-agent-controls.png')});
});

test('language switches preserve an unsent multilingual draft and update existing-language controls',async({page})=>{
 await fixture(page);await page.goto('/ui/#/chat');
 const draft='Do not translate my text: 日本語, مرحبا, café';
 await page.locator('.composer textarea').fill(draft);
 for(const [code,optionLabel] of [['fr','Options de l’espace de travail'],['ar','خيارات مساحة العمل'],['ja','ワークスペースのオプション']]){
  await changePreferences(page,{locale:code});
  await expect(page.locator('html')).toHaveAttribute('lang',code);
  await expect(page.locator('.workspace-options > button')).toHaveAccessibleName(optionLabel);
  await expect(page.locator('.composer textarea')).toHaveValue(draft);
 }
});

test('every language updates an already open Help dialog and preserves the unsent draft',async({page})=>{
 await fixture(page);await page.goto('/ui/#/chat');
 const draft='Keep my words unchanged: café 日本語 مرحباً বাংলা';
 await page.locator('.composer textarea').fill(draft);
 await page.locator('.help-trigger').click();
 const inventory=inputs();
 for(const language of Object.keys(registry)){
  await syncPreferences(page,{locale:language});
  await expect(page.locator('html')).toHaveAttribute('lang',language);
  await expect(page.locator('html')).toHaveAttribute('dir',registry[language as keyof typeof registry].direction);
  const entries=entriesFor(language,inventory);
  const copy=(key:string)=>entries.find(e=>e.namespace==='i18n/workspace-help:workspaceHelp'&&e.key===key)!;
  for(const key of ['help','example','chat'])expect(copy(key).coverage,language+' Help '+key).not.toBe('missing');
  await expect(page.locator('#workspace-help-title')).toHaveText(copy('help').currentText);
  await expect(page.locator('.workspace-help-example')).toHaveText(copy('example').currentText);
  await expect(page.locator('.workspace-help details').first().locator('p')).toHaveText(copy('chat').currentText);
  await expect(page.locator('.composer textarea')).toHaveValue(draft);
 }
 await page.keyboard.press('Escape');
 await expect(page.locator('.composer textarea')).toHaveValue(draft);
});

test('brand mark inherits each palette instead of the operating-system image colors',async({page})=>{
 await fixture(page);await page.emulateMedia({colorScheme:'light'});await page.goto('/ui/#/chat');
 for(const [mode,palette] of [['dark','warm'],['light','catppuccin'],['dark','nord'],['dark','dracula']]){
  await page.evaluate(({mode,palette})=>{
   localStorage.setItem('offgrid.theme',mode);window.dispatchEvent(new StorageEvent('storage',{key:'offgrid.theme',newValue:mode}));
   const value=JSON.stringify({version:1,light:palette,dark:palette,textSize:100,reduceMotion:false});
   localStorage.setItem('offgrid.appearance.v1',value);window.dispatchEvent(new StorageEvent('storage',{key:'offgrid.appearance.v1',newValue:value}));
  },{mode,palette});
  await expect(page.locator('html')).toHaveAttribute('data-theme',mode);
  await expect(page.locator('.brand img')).toHaveCount(0);
  const colors=await page.locator('.brand').evaluate(el=>({text:getComputedStyle(el).color,stroke:getComputedStyle(el.querySelector('svg g')!).stroke,fill:getComputedStyle(el.querySelector('svg')!).fill}));
  expect(colors.stroke).toBe(colors.text);expect(colors.fill).toBe('none');
 }
 await page.screenshot({path:test.info().outputPath('themed-brand.png')});
 await page.emulateMedia({forcedColors:'active'});
 const colors=await page.locator('.brand').evaluate(el=>[getComputedStyle(el).color,getComputedStyle(el.querySelector('svg g')!).stroke]);
 expect(colors[0]).toBe(colors[1]);
});
