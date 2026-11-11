import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import registry from '../../desktop/language-registry.json' with {type:'json'};
import {inputs,entriesFor} from '../../../scripts/quality/translation-review.mjs';

const inventory=inputs();
test.describe.configure({mode:'parallel'});
// These are the exact content areas reported as mixed-language, including body
// copy and inputs, rather than a navigation-only test of the language picker.
const routes:{route:string;messages:[string,string][]}[]=[
 {route:'agents',messages:[['i18n/agent-management:agentManagement','tools'],['i18n/agent-management:agentManagement','external'],['core','guidance.profiles'],['i18n/agent-profiles:profileSelectionPrompt','']]},
 {route:'knowledge',messages:[['i18n/workspace-management:labels','0'],['core','knowledge.add'],['i18n/knowledge-options:labels',''],['core','knowledge.empty'],['i18n/workspace-audit:libraryScope','']]},
 {route:'models',messages:[['core','models.installed'],['core','models.discover'],['core','guidance.languageModels'],['i18n/model-library:labels','3'],['i18n/model-library:labels','0'],['i18n/model-library:labels','9']]},
 {route:'activity',messages:[['core','guidance.activity'],['core','shell.work'],['i18n/agent-schedules:rows','0'],['i18n/workspace-management:labels','11']]},
 {route:'settings',messages:[['i18n/settings-navigation:settingsNavigation','descriptions.0'],['i18n/personalization:personalization','9']]},
 {route:'settings/responses',messages:[['i18n/settings-navigation:settingsNavigation','descriptions.1'],['core','chatStreaming.profile'],['core','chatStreaming.responseLimit'],['core','chatStreaming.profileHint'],['i18n/response-budget:responseBudgetHint','']]},
 {route:'settings/access',messages:[['i18n/settings-navigation:settingsNavigation','descriptions.4'],['core','settings.computerUse'],['i18n/computer-status:computerStatus','installed'],['core','shell.storage'],['i18n/workspace-management:desktopStorage',''],['core','shell.modelsPath'],['core','shell.dataPath']]}
];

for(const locale of Object.keys(registry))test(`reported management screens use authored copy in ${locale}`,async({page})=>{
 page.on('pageerror',error=>console.error(error));
 const state=await craftFixture(page,locale,'light');state.mode='empty';
 await page.route('**/v1/rag/status',route=>route.fulfill({json:{enabled:true,embedding_model:'fixture',stats:{},document_formats:[{id:'text',extensions:['.txt'],runtime_state:'available'}]}}));
 await page.addInitScript(()=>{
  Object.assign(window,{electron:{getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},getPaths:async()=>({models:'C:/fixture/models',data:'C:/fixture/data'}),getComputerStatus:async()=>({installed:true})}});
 });
 const entries=entriesFor(locale,inventory);
 for(const item of routes){
  await page.goto('/ui/#/'+item.route);
  await expect(page.locator('html')).toHaveAttribute('lang',locale);
  await expect(page.locator('html')).toHaveAttribute('dir',registry[locale as keyof typeof registry].direction);
  for(const [namespace,key] of item.messages){
   const entry=entries.find(e=>e.namespace===namespace&&e.key===key)!;
   expect(entry.coverage,`${locale} ${namespace}.${key} must not fall back`).not.toBe('missing');
   // A translated toolbar label may contain the same word in a hidden span.
   // Assert the visible screen copy rather than whichever substring comes first.
   await expect(page.getByText(entry.currentText,{exact:false}).filter({visible:true}).first()).toBeVisible();
   if(locale!=='en'&&entry.currentText!==entry.source)expect(await page.locator('body').innerText()).not.toContain(entry.source);
  }
 }
 // Profile names and local filesystem paths are user/runtime content, not copy.
 await expect(page.getByText('C:/fixture/models',{exact:true})).toBeVisible();
 await page.goto('/ui/#/agents');await expect(page.getByText('Researcher',{exact:true})).toBeVisible();
 expect(state.writes).toEqual([]);
});
