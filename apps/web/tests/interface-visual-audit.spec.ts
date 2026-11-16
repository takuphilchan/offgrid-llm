import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {createHash} from 'node:crypto';
import {workflow} from '../src/i18n/workflow';
import {documentReview} from '../src/i18n/document-review';

// Opt-in captures of the real renderer, with isolated, explicitly synthetic data.
for(const [theme,locale,width] of [['light','en',1440],['dark','en',1440],['light','de',390],['dark','ar',390],['light','zh-Hans',1280],['dark','en',1280]] as const){
 test(`interface audit ${theme} ${locale} ${width}`,async({page})=>{
  test.skip(!process.env.OFFGRID_UI_AUDIT,'Opt-in visual evidence');test.setTimeout(120000);
  await page.setViewportSize({width,height:900});const state=await craftFixture(page,locale,theme);
  if(width===1280)await page.addInitScript(()=>localStorage.setItem('offgrid.appearance.v1',JSON.stringify({version:1,light:'dracula',dark:'warm',textSize:100,reduceMotion:false})));
  const sourceText='DEPARTMENTAL RECORD · SYNTHETIC SAMPLE\n\n1. Scope\nThis policy applies to all regional offices. Retain original records and identify the evidence used in each review.\n\n2. Procurement threshold\nPurchases above 17,250 require approval before a commitment is made.\n\nException\nEmergency expenditure must be documented and reviewed within seven days.\n\nSource note\nThis is synthetic interface-test content, not an actual departmental policy.';
  await page.route('**/v1/documents/source?**',r=>new URL(r.request().url()).searchParams.get('format')==='preview'
   ?r.fulfill({json:{name:'Policy.txt',sha256:'b'.repeat(64),format:'txt',pages:1,state:'complete',units:[{kind:'section',index:1,text:sourceText,state:'ready'}]}})
   :r.fulfill({json:{document:{id:'notes',name:'Policy.txt',original_retained:true,extraction_revision:'extraction-audit'},content:sourceText,truncated:false}}));
  await page.route('**/v1/documents/structure?**',r=>r.fulfill({json:{document_id:'notes',revision:'extraction-audit',original_sha256:'b'.repeat(64),format:'txt',method:'native',units:[{id:'section-1',kind:'section',index:1,state:'ready',block_count:1}],blocks:[{id:'block-1',unit_id:'section-1',kind:'paragraph',text:sourceText,text_sha256:createHash('sha256').update(sourceText).digest('hex'),editable:false,truncated:false,metadata_omitted:false}],offset:0,total_blocks:1,next_offset:null,relationships:[],navigation:[],navigation_offset:0,next_navigation_offset:null,total_navigation:0,can_correct:false}}));
  const dir=resolve('../../build/interface-audit',process.env.OFFGRID_UI_AUDIT!,`${theme}-${locale}-${width}`);mkdirSync(dir,{recursive:true});
  for(const route of ['chat','agents','activity','knowledge','models','settings']){
   await page.goto(`/ui/#/${route}`);await expect(page.locator('.page-content')).toBeVisible();
   const ready:Record<string,string>={chat:'.composer',agents:'.profile-list-item',activity:'.task-list-item',knowledge:'.knowledge-documents article',models:'.installed-model',settings:'.settings-workspace'};
   await expect(page.locator(ready[route]).first()).toBeVisible();await page.evaluate(()=>document.fonts.ready);
   await expect(page.locator('.page-failure')).toHaveCount(0);
   await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
   await page.screenshot({path:resolve(dir,`${route}.png`),fullPage:true,animations:'disabled'});
   if(route==='knowledge'){
    await page.getByRole('button',{name:workflow[locale].viewSource,exact:true}).click();
    const reader=page.locator('.document-reader');await expect(reader.locator('pre')).toHaveText(sourceText);
    await page.screenshot({path:resolve(dir,'knowledge-preview.png'),animations:'disabled'});
    await reader.getByRole('button',{name:documentReview(locale).title,exact:true}).click();
    await expect(reader.locator('.document-review-blocks')).toContainText(sourceText);
    await page.screenshot({path:resolve(dir,'knowledge-review.png'),animations:'disabled'});
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button',{name:workflow[locale].viewSource,exact:true})).toBeFocused();
   }
  }
  expect(state.writes).toEqual([]);
 });
}
