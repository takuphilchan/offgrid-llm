import {readerOptions} from './helpers/reader-options';
import {expect,test,type Page} from '@playwright/test';
import {createHash} from 'node:crypto';
import {craftFixture} from './helpers/craft-fixture';
import {documentNavigation} from '../src/i18n/document-navigation';
import {documentReview} from '../src/i18n/document-review';
import {documentReader} from '../src/i18n/document-reader';
import {workflow} from '../src/i18n/workflow';
import {languageRegistry,type LanguageCode} from '../src/i18n/language';
import {validateStructure,validatePreview} from '../src/api/document-structure';
import type {DocumentStructurePage,DocumentPreview} from '../src/api/client';

const hash=(s:string)=>createHash('sha256').update(s).digest('hex');
async function setup(page:Page,locale:LanguageCode='en',theme='light'){
 await craftFixture(page,locale,theme);
 const state={reads:[] as URL[],revision:'extraction-first',invalid:false,legacy:false,fail:false};
 const text='Policy clause 17.25. Exception applies to island applicants.';
 await page.route('**/v1/documents/source?*',r=>{
  const q=new URL(r.request().url()).searchParams;
  if(q.get('format')==='preview'){
   if(q.has('page'))return r.fulfill({contentType:'image/png',body:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j5ZkAAAAASUVORK5CYII=','base64')});
   return r.fulfill({json:{name:'Policy.pdf',format:'pdf',sha256:'a'.repeat(64),state:'partial',units:[{kind:'page',index:1,text,state:'native_text_with_visuals'},{kind:'page',index:2,text:'Unread scan',state:'ocr_required'}]}});
  }
  return r.fulfill({json:{document:{id:'notes',name:'Policy.pdf',source_retained:true,original_retained:true,extraction_revision:'extraction-first'},content:text,truncated:false}});
 });
 await page.route('**/v1/documents/structure?*',r=>{
  const q=new URL(r.request().url()).searchParams;state.reads.push(new URL(r.request().url()));if(state.fail)return r.fulfill({status:404,json:{error:'revoked'}});
  const offset=Number(q.get('offset')??0),nav=Number(q.get('navigation_offset')??0),unit=q.get('unit')||'page-1',value=offset===104?'Late exception 17.25':text;
  const units=[{id:'page-1',kind:'page',index:1,state:'native_text_with_visuals',span:{start_byte:0,end_byte:500},parser_revision:'fixture',block_count:150,review_blocks:1,figure_blocks:1},{id:'page-2',kind:'page',index:2,state:'ocr_required',span:{start_byte:500,end_byte:500},parser_revision:'fixture',block_count:0,review_blocks:0,figure_blocks:0,hidden:true}];
  const entries=Array.from({length:nav?2:100},(_,i)=>({block_id:`block-${nav+i}`,unit_id:'page-1',block_offset:nav+i,kind:'heading',title:`Section ${nav+i}`,needs_review:false}));
  if(nav)entries[1]={block_id:'block-104',unit_id:state.invalid?'not-a-source-unit':'page-1',block_offset:104,kind:'heading',title:'Late exception',needs_review:true};
  return r.fulfill({json:{document_id:'notes',revision:q.get('revision')||state.revision,format:'pdf',method:'native',units,blocks:unit==='page-2'?[]:[{id:`block-${offset}`,unit_id:unit,kind:'paragraph',span:{start_byte:0,end_byte:value.length},method:'native',text:value,text_sha256:hash(value),editable:true,truncated:false,metadata_omitted:false}],offset,total_blocks:unit==='page-2'?0:150,next_offset:unit==='page-2'?null:offset+1,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:true,...(state.legacy?{}:{navigation:entries,navigation_offset:nav,total_navigation:102,next_navigation_offset:nav?null:100})}});
 });
 await page.goto('/ui/#/knowledge');await page.locator('.resource-card').getByRole('button',{name:workflow[locale].viewSource,exact:true}).click();
 await expect(page.locator('.document-reader-surface img')).toBeVisible();return state;
}
for(const [i,locale]of(Object.keys(languageRegistry) as LanguageCode[]).entries())test(`shared reader coverage and outline stay localized: ${locale}`,async({page},info)=>{
 const state=await setup(page,locale,i%2?'dark':'light'),copy=documentNavigation(locale),review=documentReview(locale),reader=documentReader(locale);
 await page.setViewportSize({width:390,height:844});const panel=page.locator('.document-reader');
 await readerOptions(page);await panel.locator('.document-view-select').selectOption('compare');await expect(panel.locator('.document-reader-surface img')).toBeVisible();await expect(panel.locator('.document-reader-surface pre')).toContainText('17.25');
 await readerOptions(page);await panel.getByRole('button',{name:review.title,exact:true}).click();const reviewPanel=panel.locator('.document-review');
 await reviewPanel.locator('summary').filter({hasText:copy.coverage}).click();await expect(reviewPanel.locator('.document-unit-coverage')).toContainText(copy.figures);await expect(reviewPanel.locator('.document-unit-coverage')).toContainText(copy.hidden);
 await reviewPanel.locator('summary').filter({hasText:copy.outline}).click();await expect(reviewPanel.locator('.document-outline li')).toHaveCount(100);
 state.revision='extraction-concurrent';await reviewPanel.getByRole('navigation',{name:copy.outline,exact:true}).getByRole('button',{name:reader.next,exact:true}).click();await expect(reviewPanel.locator('.document-outline li')).toHaveCount(2);
 await reviewPanel.getByRole('button',{name:'Late exception',exact:true}).click();await expect(reviewPanel.locator('[data-block-id="block-104"]')).toBeFocused();await expect(reviewPanel.locator('.document-review-blocks pre')).toHaveText('Late exception 17.25');
 expect(state.reads.at(-1)!.searchParams.get('revision')).toBe('extraction-first');expect(state.reads.at(-1)!.searchParams.get('offset')).toBe('104');expect(state.reads.at(-1)!.searchParams.get('unit')).toBe('page-1');
 await reviewPanel.getByRole('button',{name:review.edit,exact:true}).click();await reviewPanel.getByRole('textbox',{name:review.draft,exact:true}).fill('Draft retained');
 await expect(reviewPanel.locator('.document-unit-coverage button').first()).toBeDisabled();await panel.getByRole('button',{name:reader.document,exact:true}).click();await readerOptions(page);await panel.getByRole('button',{name:review.title,exact:true}).click();await expect(reviewPanel.getByRole('textbox',{name:review.draft,exact:true})).toHaveValue('Draft retained');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(locale==='en'||locale==='ar')await page.screenshot({path:info.outputPath(`navigation-${locale}.png`),fullPage:true});
});
test('malformed navigation never opens a different unit',async({page})=>{
 const state=await setup(page);state.invalid=true;await readerOptions(page);await page.getByRole('button',{name:documentReview('en').title,exact:true}).click();const panel=page.locator('.document-review');await panel.locator('summary').filter({hasText:documentNavigation('en').outline}).click();await panel.getByRole('navigation',{name:documentNavigation('en').outline}).getByRole('button',{name:documentReader('en').next}).click();await expect(panel.getByRole('alert')).toHaveText(documentReview('en').unavailable);expect(state.reads.some(url=>url.searchParams.get('unit')==='not-a-source-unit')).toBe(false);
});
test('legacy navigation is explicitly unavailable while retained text remains readable',async({page})=>{
 const state=await setup(page);state.legacy=true;await readerOptions(page);await page.getByRole('button',{name:documentReview('en').title,exact:true}).click();const panel=page.locator('.document-review');await panel.locator('summary').filter({hasText:documentNavigation('en').outline}).click();await expect(panel).toContainText(documentNavigation('en').unavailable);await expect(panel.locator('.document-review-blocks')).toContainText('17.25');
});
test('preview and navigation validators reject unrelated or out-of-range responses',()=>{
 for(const bad of [{},{sha256:'a',format:'pdf',units:[null]},{sha256:'a',format:'pdf',units:[{kind:'page',index:0,text:'text',state:'ready'}]}])expect(()=>validatePreview(bad as DocumentPreview)).toThrow();
 const page={document_id:'d',revision:'r',offset:0,total_blocks:0,blocks:[],units:[],can_correct:false,next_offset:null,navigation:[],navigation_offset:0,total_navigation:0,next_navigation_offset:100};expect(()=>validateStructure(page as unknown as DocumentStructurePage,'d','r',0)).toThrow();
});
