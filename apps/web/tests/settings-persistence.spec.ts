import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { personalization } from '../src/i18n/personalization';
import { languageRegistry, type LanguageCode } from '../src/i18n/language';

const failure=personalization.en[10];
async function nativeFixture(page:Page, failInitially=false) {
 const state=await craftFixture(page,'en','light');
 await page.addInitScript(fail=>{
  const fixture=window as any;
  fixture.holdPresentation=false;fixture.presentationSaves=[];
  fixture.electron={getPaths:async()=>({models:'fixture-models',data:'fixture-data'}),getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},setPresentation:(value:unknown)=>{
   if(fixture.holdPresentation)return new Promise((resolve,reject)=>fixture.presentationSaves.push({value,resolve,reject}));
   return fail?Promise.reject(Error('Fixture disk failure')):Promise.resolve();
  }};
 },failInitially);
 return state;
}
async function changeSize(page:Page,size:string) {
 await page.locator('#appearance-text-size').selectOption(size);
 await expect.poll(()=>page.evaluate(()=>(window as any).presentationSaves.length)).toBe(size==='110'?1:2);
}

test('an unsaved appearance survives leaving Settings without losing the draft',async({page})=>{
 const state=await craftFixture(page,'en','light');
 await page.addInitScript(()=>{
  const write=Storage.prototype.setItem;
  Storage.prototype.setItem=function(key,value){if(key==='offgrid.appearance.v1'&&(window as any).failAppearance)throw new DOMException('Full','QuotaExceededError');return write.call(this,key,value);};
 });
 await page.goto('/ui/#/chat/new');await page.locator('.composer textarea').fill('Keep this request while changing appearance');
 await page.evaluate(()=>{(window as any).failAppearance=true;location.hash='#/settings/appearance';});
 await page.locator('#appearance-text-size').selectOption('125');
 await expect(page.locator('.appearance-footer [role=status]')).toHaveText(failure);
 await page.evaluate(()=>location.hash='#/chat');await expect(page.locator('.composer textarea')).toHaveValue('Keep this request while changing appearance');
 await page.evaluate(()=>location.hash='#/settings/appearance');
 await expect(page.locator('#appearance-text-size')).toHaveValue('125');
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toHaveCount(1);
 await page.evaluate(()=>(window as any).failAppearance=false);
 await page.locator('#appearance-text-size').selectOption('110');
 await expect(page.locator('.appearance-footer [role=status]')).toHaveText('Saved');
 await expect(page.getByText(failure,{exact:true})).toHaveCount(0);
 expect(state.writes).toEqual([]);
});

test('appearance waits for native persistence and retains its failed outcome across navigation',async({page})=>{
 const state=await nativeFixture(page);await page.goto('/ui/#/settings/appearance');
 await page.evaluate(()=>(window as any).holdPresentation=true);await changeSize(page,'110');
 await expect(page.locator('.appearance-footer [role=status]')).not.toHaveText('Saved');
 await page.evaluate(()=>(window as any).presentationSaves[0].reject(Error('Fixture write failure')));
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toHaveCount(1);
 await expect(page.locator('.appearance-footer [role=status]')).not.toHaveText('Saved');
 await page.evaluate(()=>location.hash='#/chat');await expect(page.locator('.composer textarea')).toBeVisible();
 await page.evaluate(()=>location.hash='#/settings/general');
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toHaveCount(1);
 expect(state.writes).toEqual([]);
});

test('a native save failure before Settings opens is still disclosed',async({page})=>{
 const state=await nativeFixture(page,true);await page.goto('/ui/#/chat/new');
 await expect(page.locator('.composer textarea')).toBeVisible();
 await page.evaluate(()=>location.hash='#/settings/general');
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toBeVisible();
 expect(state.writes).toEqual([]);
});

for(const latestFails of [false,true])test(`only the latest native presentation save owns feedback: failure=${latestFails}`,async({page})=>{
 const state=await nativeFixture(page);await page.goto('/ui/#/settings/appearance');
 await page.evaluate(()=>(window as any).holdPresentation=true);
 await changeSize(page,'110');await changeSize(page,'125');
 await page.evaluate(fail=>{const saves=(window as any).presentationSaves;fail?saves[1].reject(Error('Newest failed')):saves[1].resolve();},latestFails);
 await expect(page.locator('.appearance-footer [role=status]')).toHaveText(latestFails?failure:'Saved');
 await page.evaluate(fail=>{const saves=(window as any).presentationSaves;fail?saves[0].resolve():saves[0].reject(Error('Older failed'));},latestFails);
 await expect(page.locator('.appearance-footer [role=status]')).toHaveText(latestFails?failure:'Saved');
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toHaveCount(latestFails?1:0);
 await expect(page.locator('#appearance-text-size')).toHaveValue('125');
 expect(state.writes).toEqual([]);
});

test('unsaved response preferences stay visible after leaving and returning',async({page})=>{
 const state=await craftFixture(page,'en','light');
 await page.addInitScript(()=>{
  const write=Storage.prototype.setItem;
  Storage.prototype.setItem=function(key,value){if(key.startsWith('offgrid.chat.preferences:'))throw new DOMException('Full','QuotaExceededError');return write.call(this,key,value);};
 });
 await page.goto('/ui/#/settings/models');
 await page.getByRole('combobox',{name:'Maximum response tokens',exact:true}).selectOption('256');
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toHaveCount(1);
 await page.evaluate(()=>location.hash='#/chat');await expect(page.locator('.composer textarea')).toBeVisible();
 await page.evaluate(()=>location.hash='#/settings/models');
 await expect(page.getByRole('combobox',{name:'Maximum response tokens',exact:true})).toHaveValue('256');
 await expect(page.locator('.settings-content').getByText(failure,{exact:true})).toHaveCount(1);
 expect(state.writes).toEqual([]);
});

test('voice selection discloses unsaved state without starting audio and recovers on an explicit change',async({page})=>{
 const state=await craftFixture(page,'en','light');
 await page.route('**/v1/audio/status',route=>route.fulfill({json:{asr:{available:true},tts:{available:false},profiles:[{id:'asr',revision:'r1',name:'Test recognition',available:true,capabilities:['transcription']}]}}));
 await page.addInitScript(()=>{
  (window as any).failVoice=true;const write=Storage.prototype.setItem;
  Storage.prototype.setItem=function(key,value){if(key.includes('voice-preferences')&&(window as any).failVoice)throw new DOMException('Full','QuotaExceededError');return write.call(this,key,value);};
 });
 await page.goto('/ui/#/settings/voice');
 const voice=page.locator('.voice-settings-inline .model-choice').first();
 await voice.getByRole('button').first().click();await page.getByRole('radio',{name:/Test recognition/}).last().click();
 await expect(page.locator('.voice-settings-inline').getByText(failure,{exact:true})).toBeVisible();
 await page.evaluate(()=>location.hash='#/chat');await expect(page.locator('.composer textarea')).toBeVisible();
 await page.evaluate(()=>location.hash='#/settings/voice');await expect(voice).toContainText('Test recognition');
 await expect(page.locator('.voice-settings-inline').getByText(failure,{exact:true})).toBeVisible();
 await page.evaluate(()=>(window as any).failVoice=false);
 await voice.getByRole('button').first().click();await page.getByRole('radio',{name:/Automatic/}).first().click();
 await expect(page.getByText(failure,{exact:true})).toHaveCount(0);
 expect(state.writes).toEqual([]);
});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`unsaved preferences stay localized and bounded: ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:390,height:760});const state=await craftFixture(page,locale,'dark');
 await page.addInitScript(()=>{
  const write=Storage.prototype.setItem;
  Storage.prototype.setItem=function(key,value){if(key==='offgrid.appearance.v1')throw new DOMException('Full','QuotaExceededError');return write.call(this,key,value);};
 });
 await page.goto('/ui/#/settings/text-size');await expect(page.locator('#appearance-text-size')).toBeFocused();
 await page.locator('#appearance-text-size').selectOption('110');
 await page.evaluate(()=>location.hash='#/chat');await expect(page.locator('.composer textarea')).toBeVisible();
 await page.evaluate(()=>location.hash='#/settings/text-size');await expect(page.locator('#appearance-text-size')).toHaveValue('110');
 const notice=page.locator('.appearance-footer [role=status]');await expect(notice).toHaveText(personalization[locale][10]);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 expect(state.writes).toEqual([]);
 if(['en','ar'].includes(locale)){await notice.scrollIntoViewIfNeeded();await page.screenshot({path:info.outputPath(`unsaved-${locale}.png`)});}
});
