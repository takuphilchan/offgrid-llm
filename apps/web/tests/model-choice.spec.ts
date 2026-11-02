import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { chooseModel } from './helpers/model-choice';
import { voiceText } from '../src/i18n/voice';
import type { LocaleCode } from '../src/i18n';

const profiles = [
  {id:'whisper',revision:'r1',name:'Whisper multilingual',architecture:'whisper',available:true,smoke_tested:false,capabilities:['transcription']},
  {id:'whisper',revision:'r2',name:'Whisper multilingual',architecture:'whisper',available:true,smoke_tested:true,capabilities:['transcription']},
  {id:'whisper',revision:'r3',name:'Whisper unavailable',architecture:'whisper',available:false,issue:'Matching runtime missing',capabilities:['transcription']},
  {id:'speech-only',revision:'r1',name:'Speech only',available:true,capabilities:['speech_synthesis']},
];
async function fixture(page: Page, locale='en') {
  const state=await craftFixture(page,locale,'light');
  const audio={profiles:[...profiles]};
  await page.route('**/v1/audio/status',r=>r.fulfill({json:audio}));
  await page.goto('/ui/#/chat');
  return {state,audio};
}

test('speech choices distinguish revisions, filter locally and never capture or submit',async({page})=>{
  const {state,audio}=await fixture(page);
  await page.locator('.composer textarea').fill('Keep this unsent draft');
  await page.getByRole('button',{name:'Response settings',exact:true}).click();
  await page.getByRole('button',{name:'Recognition model',exact:true}).click();
  const group=page.getByRole('radiogroup',{name:'Recognition model'});
  await expect(group.getByRole('radio')).toHaveCount(4);
  await expect(group.getByRole('radio',{name:/Matching runtime missing/})).toBeDisabled();
  await expect(group).not.toContainText('Speech only');
  await page.getByRole('searchbox',{name:'Filter model choices'}).fill('@r2');
  await expect(group.getByRole('radio')).toHaveCount(1);
  await expect(group).toContainText('Quality and latency');
  await page.getByRole('searchbox',{name:'Filter model choices'}).press('Enter');
  await expect(group.getByRole('radio')).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button',{name:'Recognition model',exact:true})).toBeFocused();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this unsent draft');
  expect(state.writes).toEqual([]);
  await page.keyboard.press('Escape');audio.profiles=profiles.filter(p=>p.revision!=='r2');
  await page.getByRole('button',{name:'Response settings',exact:true}).click();
  await expect(page.locator('.model-choice-missing')).toContainText('whisper@r2');
  await expect(page.getByRole('button',{name:'Recognition model',exact:true})).toContainText('unavailable');
  expect(state.writes).toEqual([]);
});

test('embedding selection distinguishes installed and catalog data without starting a download',async({page})=>{
  const state=await craftFixture(page,'en','light');
  await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:false,embedding_model:'embed-installed',stats:{}}}));
  await page.route('**/v1/models',r=>r.fulfill({json:{data:[{id:'chat',type:'chat'},{id:'embed-installed',type:'embedding',loaded:true}]}}));
  await page.route('**/v1/catalog',r=>r.fulfill({json:{models:[{id:'embed-catalog',name:'Embedding catalog choice',type:'embedding',size_bytes:2048,license:'MIT'}]}}));
  await page.goto('/ui/#/knowledge');
  await page.getByText('Change search model', { exact: true }).click();
  await page.getByRole('button',{name:'Search model',exact:true}).click();
  const group=page.getByRole('radiogroup',{name:'Search model'});
  await expect(group.getByRole('radio')).toHaveCount(2);
  await expect(group).toContainText('Installed');await expect(group).toContainText('Loaded');
  await expect(group).toContainText('Curated catalog');await expect(group).toContainText('MIT');
  await chooseModel(page,'Search model','embed-catalog');
  await expect(page.getByRole('button',{name:'Install and enable',exact:true})).toBeEnabled();
  expect(state.writes).toEqual([]);
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu']) test(`speech choice works in a short narrow ${locale} viewport`,async({page},info)=>{
 const {state}=await fixture(page,locale);await page.setViewportSize({width:320,height:480});
 const copy=voiceText(locale as LocaleCode);
 await expect(page.getByRole('button',{name:copy.record,exact:true})).toBeVisible();
 const voice=page.locator('.composer-context-menu > button');await voice.click();
 await expect(page.getByRole('checkbox',{name:copy.speakResponses,exact:true})).not.toBeChecked();
 const panel=page.locator('.composer-context-menu > .utility-popover-panel'),trigger=panel.locator('.voice-settings-panel .model-choice-trigger').first();
 await expect(trigger).toBeEnabled();await trigger.click();
 await panel.getByRole('searchbox').fill('r2');
 await panel.getByRole('searchbox').dispatchEvent('keydown',{key:'Enter',isComposing:true});
 await expect(trigger).toHaveAttribute('aria-expanded','true');expect(state.writes).toEqual([]);
 const box=await panel.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(320);expect(box!.height).toBeLessThanOrEqual(480);
 const overflow=await panel.evaluate(node=>[node,...Array.from(node.querySelectorAll('*'))].filter(child=>child.scrollWidth>child.clientWidth+1).map(child=>({tag:child.tagName,className:child.className,width:child.getBoundingClientRect().width,client:child.clientWidth,scroll:child.scrollWidth,text:child.textContent?.slice(0,70)})));
 expect(await panel.evaluate(node=>node.scrollWidth<=node.clientWidth+1),JSON.stringify(overflow)).toBe(true);
 if(locale==='en'||locale==='ar') await page.screenshot({path:info.outputPath(`speech-choice-${locale}.png`)});
 await page.keyboard.press('Escape');await expect(panel).toBeVisible();await expect(trigger).toBeFocused();
 await page.keyboard.press('Escape');await expect(panel).toBeHidden();await expect(voice).toBeFocused();
});

test('voice controls and recovery messages have a complete locale-specific copy set',()=>{
 const english=voiceText('en');
 for(const locale of ['fr','de','es','ar','sw','sn','nd','zu'] as const) {
  const copy=voiceText(locale);
  expect(Object.keys(copy).sort()).toEqual(Object.keys(english).sort());
  for(const key of Object.keys(english) as (keyof typeof english)[]) {
   expect(copy[key].trim(),`${locale}.${key}`).not.toBe('');
   expect(copy[key],`${locale}.${key} must not fall back to English`).not.toBe(english[key]);
  }
 }
});

test('unavailable automatic speech is explained without expanding a model chooser',async({page})=>{
 const {state,audio}=await fixture(page);audio.profiles=[];
 await page.getByRole('button',{name:'Response settings',exact:true}).click();
 await expect(page.locator('.model-choice-summary')).toHaveCount(2);
 await expect(page.locator('.model-choice-summary').first()).toContainText('No compatible speech model is ready');
 await expect(page.getByRole('radiogroup')).toHaveCount(0);
 expect(state.writes).toEqual([]);
});
