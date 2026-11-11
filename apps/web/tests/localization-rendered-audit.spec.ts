import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import registry from '../../desktop/language-registry.json' with {type:'json'};
import {inputs,entriesFor,digest} from '../../../scripts/quality/translation-review.mjs';
import {renderedText,classifyRendered} from '../../../scripts/quality/rendered-text-audit.mjs';
import {writeFileSync} from 'node:fs';
import {formatBytes} from '../src/features/models/model-format';

const inventory=inputs();
test.describe.configure({mode:'parallel'});
// Collection success is not translation readiness. Normal/CI runs fail for every
// unresolved element; only the explicit report-only command disables that gate.
for(const language of Object.keys(registry))test(`rendered text inventory: ${language}`,async({page},info)=>{
 test.setTimeout(120_000);
 const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
 const state=await craftFixture(page,language,'light');state.mode='empty';
 await page.addInitScript(()=>{Object.assign(window,{electron:{
  getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},openTerminal:async()=>{},
  getPaths:async()=>({models:'C:/fixture/models',data:'C:/fixture/data'}),getComputerStatus:async()=>({installed:true})
 }});});
 await page.route('**/v1/audio/status',route=>route.fulfill({json:{profiles:[
  {id:'fixture-asr',revision:'1',available:true,capabilities:['transcription'],name:'Fixture ASR'},
  {id:'fixture-tts',revision:'1',available:true,capabilities:['speech_synthesis'],name:'Fixture TTS',voices:[{id:'fixture-voice',language:'en'}]}
 ]}}));
 await page.route('**/v1/catalog',route=>route.fulfill({json:{models:[
  {id:'qwen3.5-2b',name:'Qwen3.5 2B',description:'Small multilingual chat model. Bundle download includes its vision projector; image and agent readiness are checked separately.',repo:'unsloth/Qwen3.5-2B-GGUF',file:'Qwen3.5-2B-Q4_K_M.gguf',type:'chat',recommended:true,size_bytes:1280835840},
  {id:'smollm3-3b',name:'SmolLM3 3B',description:'Text-only multilingual model from Hugging Face. A compact alternative for chat and writing; does not accept images.',repo:'ggml-org/SmolLM3-3B-GGUF',file:'SmolLM3-Q4_K_M.gguf',type:'chat',recommended:true,size_bytes:1915305312}
 ]}}));
 await page.route('**/api/v2/models/catalog*',route=>route.fulfill({json:{models:[{id:'model-32c96ae87d3e',name:'Fixture speech package',category:'speech_generation',description:'Complete speech model package. Speech runtime and qualification are not included.',variants:[],source_notices:[]}],repositories:[]}}));
 const entries=entriesFor(language,inventory),screens:{state:string;elements:ReturnType<typeof classifyRendered>}[]=[];
 const catalogEntries=[...entries];
 // These are exact, source-derived compositions with controlled fixture values,
 // not patterns learned from whatever text happens to appear on the screen.
 const composed=(namespace:string,key:string,format:(text:string)=>string)=>{
  const entry=entries.find(e=>e.namespace===namespace&&e.key===key)!;
  catalogEntries.push({...entry,key:entry.key+':rendered',source:format(entry.source),currentText:format(entry.currentText)});
 };
 composed('core','shell.quickActions',text=>`${text} (Ctrl K)`);
 composed('core','models.cancel',text=>`${text} (Esc)`);
 for(const name of ['Demo briefing','Prepare a demo briefing'])composed('i18n/workspace-experience:inline1','manageHistory',text=>`${text}: ${name}`);
 composed('i18n/workspace-order:labels','0',text=>`${text}: Local-Research-7B-Instruct-Q4_K_M-long-model-name`);
 for(const key of ['1','2'])composed('i18n/personalization:personalization',key,text=>`${text}: OffGrid`);
 for(const name of ['Fixture ASR','Fixture TTS'])composed('i18n/voice-settings:copy','3',text=>`${text} · ${name}`);
 for(const count of [0,1])composed('core','models.installed',text=>`${text} · ${count}`);
 const autonyms=['English','Français','Español','العربية','Kiswahili','ChiShona','isiNdebele','isiZulu','Deutsch',...Object.values(inventory.packs).map((pack:any)=>pack.label)];
 const fixtureBody=['Prepare a concise briefing with an example.','A quieter workspace','Keep the work readable and the next action clear. This is a saved answer with','emphasis',', a','source',', and multilingual text: café 日本語 مرحباً.','Next steps','Review the evidence.','Prepare a short briefing.','typescript','const summary = { ready: true, title: "OffGrid" }; console.log(summary); // A deliberately long line to exercise horizontal code scrolling without widening the conversation.','Item','Status','Notes','Ready','Sources','Reviewed'];
 const copy=(namespace:string,key:string)=>entries.find(e=>e.namespace===namespace&&e.key===key)!.currentText;
 // ICU coverage can differ between Node and the browser (notably sn/nd/zu).
 // Derive the known fixture date with the browser's formatter, never from UI text.
 const fixtureDate=await page.evaluate(language=>new Date('2026-10-30T00:00:00Z').toLocaleDateString(language),language);
 const capture=async(name:string)=>{
  await page.waitForLoadState('networkidle');
  const route=new URL(page.url()).hash.slice(2);
  const settingsHeadings:Record<string,string>={'settings':'preferences-title','settings/responses':'model-settings-title','settings/voice':'voice-settings-title','settings/terminal':'terminal-settings-title','settings/access':'access-settings-title','settings/service':'workspace-title'};
  if(settingsHeadings[route])await expect(page.locator('#'+settingsHeadings[route])).toBeVisible();
  if(route==='settings/voice')await expect(page.locator('.voice-settings-inline .model-choice-trigger')).toHaveCount(2);
  if(route==='agents')await expect(page.locator('.agent-profiles')).toBeVisible();
  if(route==='knowledge')await expect(page.locator('.knowledge-workspace')).toBeVisible();
  if(route==='chat')await expect(page.locator('.composer textarea')).toBeVisible();
  if(route.startsWith('models'))await expect(page.locator('.models-workspace')).toBeVisible();
  const elements=classifyRendered(await page.evaluate(renderedText),catalogEntries,language,[
   ...['OffGrid','OffGrid CLI','Hugging Face','API','UI SHA-256'].map(text=>({text,reason:'Brand or protocol identifier'})),
   ...['Researcher','Local Research','C:/fixture/models','C:/fixture/data','visual-fixture','fixture-voice · en','Fixture ASR','Fixture TTS','fixture-asr@1','fixture-tts@1','Local Research 7B Instruct Q4 K M long model name','Local-Research-7B-Instruct-Q4_K_M-long-model-name','Qwen3.5 2B','SmolLM3 3B','qwen3.5-2b','smollm3-3b','unsloth/Qwen3.5-2B-GGUF','ggml-org/SmolLM3-3B-GGUF','Fixture speech package','model-32c96ae87d3e',new URL(page.url()).origin].map(text=>({text,reason:'Isolated fixture data or stable model identifier; never translate user content'})),
   ...fixtureBody.map(text=>({text,element:'message-body',reason:'Exact saved conversation fixture text; not authored UI copy'})),
   {text:'Demo briefing',element:'history-open',reason:'Saved conversation title'},
   {text:`Demo briefing · ${fixtureDate}`,element:'history-open',reason:'Saved title and browser locale-formatted fixture date'},
   {text:fixtureDate,element:'time',reason:'Known fixture timestamp formatted by browser Intl; may contain locale-owned direction marks'},
   {text:'Prepare a demo briefing',element:'task-list-item',reason:'Saved task prompt'},
   ...autonyms.map(text=>({text,element:'#settings-language',reason:'Language autonym, intentionally written in its own language'})),
   ...(inventory.packs[language]?[{text:inventory.packs[language].notice,reason:'Authored language-preview metadata; linguistic review remains pending'}]:[]),
   ...[1280835840,1915305312,4.4*1024**3].map(bytes=>({text:formatBytes(bytes,language),element:'model-size',reason:'Locale-formatted quantity with stable unit abbreviation'}))
  ]);
  expect(elements.length,name).toBeGreaterThan(15);screens.push({state:name,elements});
  if(['settings/voice/expanded','help/all-topics','models/discover/0','narrow/settings'].includes(name))await page.screenshot({path:info.outputPath(name.replaceAll('/','-')+'.png'),fullPage:true});
 };
 const expand=async()=>{
  // Re-resolve after each click: opening a disclosure changes this locator's
  // indexes and may reveal nested disclosures that also need inspection.
  for(let count=0;count<30;count++){
   // A modal leaves background details CSS-visible but not interactive.
   // Audit the open dialog instead of attempting to click through it.
   const dialog=page.locator('dialog[open]');
   const scope=await dialog.count()?dialog:page;
   if(scope===dialog)await expect(dialog).toHaveCount(1);
   const closed=scope.locator('details:visible:not([open]) > summary');if(!await closed.count())return;
   await closed.first().click();
  }
  throw Error('Disclosure audit exceeded its bounded traversal');
 };
 const routes=['chat','agents','knowledge','activity/attention','activity/running','activity/history','activity/all','activity/archived','settings','settings/responses','settings/voice','settings/terminal','settings/access','settings/service'];
 for(const route of routes){
  await page.goto('/ui/#/'+route);await expect(page.locator('html')).toHaveAttribute('lang',language);
  await capture(route);await expand();await capture(route+'/expanded');
  if(route==='settings/voice'){
   await expect(page.locator('.voice-settings-inline .model-choice-trigger')).toHaveCount(2);
   for(const trigger of await page.locator('.voice-settings-inline .model-choice-trigger').all())await trigger.click();
   await expect(page.locator('.voice-settings-inline .model-choice-panel')).toHaveCount(2);
   await capture('settings/voice/choices');
   for(const search of await page.locator('.voice-settings-inline input[type="search"]').all())await search.fill('no-fixture-matches');
   await capture('settings/voice/no-matches');
   await expect(page.locator('.voice-settings-inline .model-choice-panel [role="status"]')).toHaveCount(2);
   for(const status of await page.locator('.voice-settings-inline .model-choice-panel [role="status"]').all())await expect(status).toHaveText(copy('i18n/model-choice:modelChoiceText','empty'));
  }
 }
 for(const view of ['installed','discover']){
  await page.goto('/ui/#/models'+(view==='discover'?'/discover':''));await capture('models/'+view);
  for(let category=0;category<4;category++){
   await page.locator('.model-category-picker button').nth(category).click();await expand();await capture(`models/${view}/${category}`);
  }
 }
 await page.goto('/ui/#/settings');await page.keyboard.press('Control+k');await capture('quick-actions');await page.keyboard.press('Escape');await page.locator('.help-trigger').click();await expand();await capture('help/all-topics');await page.keyboard.press('Escape');
 await page.getByRole('button',{name:copy('core','settings.showGuide'),exact:true}).click();await expand();await capture('setup/expanded');
 await page.keyboard.press('Escape');await page.setViewportSize({width:390,height:844});
 for(const route of ['settings','settings/voice','models/discover']){
  await page.goto('/ui/#/'+route);await expand();await capture('narrow/'+route);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+2),'Horizontal overflow in '+language+' '+route).toBe(true);
 }
 await page.locator('.help-trigger').click();await expand();await capture('narrow/help');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+2),'Horizontal overflow in '+language+' Help').toBe(true);
 expect(errors).toEqual([]);expect(state.writes).toEqual([]);
 const unresolved=screens.flatMap(screen=>screen.elements.filter(row=>['untranslated','unclassified'].includes(row.status)).map(row=>({state:screen.state,...row})));
 const report={language,translationRevision:digest({messages:entries.map(e=>[e.namespace,e.key,e.currentText]),notice:inventory.packs[language]?.notice??null}),states:screens.length,elements:screens.reduce((n,s)=>n+s.elements.length,0),unresolved:unresolved.length,screens};
 const path=info.outputPath('rendered-text.json');writeFileSync(path,JSON.stringify(report,null,2));await info.attach('rendered-text',{path,contentType:'application/json'});
 if(process.env.OFFGRID_I18N_STRICT!=='0')expect(unresolved,'Untranslated or unclassified rendered text; inspect the attached report').toEqual([]);
});
