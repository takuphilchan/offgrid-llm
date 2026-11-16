import {readerOptions} from './helpers/reader-options';
import {test,expect,type Page} from '@playwright/test';
import {createHash} from 'node:crypto';
import {craftFixture} from './helpers/craft-fixture';
import {documentReview} from '../src/i18n/document-review';
import {documentComparison} from '../src/i18n/document-comparison';
import {documentNavigation} from '../src/i18n/document-navigation';
import {documentReader} from '../src/i18n/document-reader';
import {wordRevisions} from '../src/i18n/word-revisions';
import {languageRegistry,type LanguageCode} from '../src/i18n/language';

const old='a'.repeat(64),next='b'.repeat(64),id='file-'+'c'.repeat(32);
const sha=(text:string)=>createHash('sha256').update(text).digest('hex');
async function setup(page:Page,locale:LanguageCode='en',uncertain=false,form=false,tracked=false){
 await craftFixture(page,locale,locale==='ar'?'dark':'light');
 const state={revision:old,text:'Rural exception: July.',writes:0,previewRevisions:[] as string[]};
 const attachment={id,name:'Policy.txt',sha256:'d'.repeat(64),revision:old,extraction_revision:'extraction-first',media_type:'text/plain',bytes:22,status:'ready',created_at:'2026-11-14T00:00:00Z'};
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1','session-document-corrections-v1']}}));
 await page.route('**/v1/sessions/*/attachments',r=>r.fulfill({json:r.request().method()==='POST'?{attachment}:{attachments:[attachment]}}));
 await page.route('**/attachments/*/revisions',r=>r.fulfill({json:{id,current_revision:state.revision,revisions:[...(state.revision===next?[{revision:next,extraction_revision:'extraction-next',parent_revision:old,author:'Reviewer',created_at:'2026-11-14T01:00:00Z'}]:[]),{revision:old,extraction_revision:'extraction-first'}]}}));
 await page.route('**/attachments/*/preview*',r=>{const rev=new URL(r.request().url()).searchParams.get('revision')!;state.previewRevisions.push(rev);return r.fulfill({json:{name:attachment.name,sha256:attachment.sha256,format:'txt',pages:0,state:'complete',error_code:'',units:[{kind:'section',index:1,text:rev===old?'Rural exception: July.':state.text,state:'ready',source_sha256:attachment.sha256,extractor_revision:rev===old?'extraction-first':'extraction-next'}]}});});
 await page.route('**/attachments/*/structure?**',r=>{const q=new URL(r.request().url()).searchParams,rev=q.get('revision')!,text=form?'Amount: 0017.25':rev===old?'Rural exception: July.':state.text,editable=rev===state.revision&&q.get('corrections')==='true';return r.fulfill({json:{document_id:id,attachment_revision:rev,revision:rev===old?'extraction-first':'extraction-next',format:'text',method:rev===old?'native':'review',units:[{id:'section-1',kind:'section',index:1,span:{start_byte:0,end_byte:text.length}}],blocks:[{id:'note',unit_id:'section-1',kind:form?'form_field':'paragraph',...(tracked?{review_state:'tracked_changes',revisions:[{id:'1',kind:'deletion',author:'Policy reviewer',date:'2026-01-02T00:00:00Z',text:'999.99 superseded'}]}:{}),...(form?{review_state:'needs_review',form_field:{kind:'text',name:'Amount',value:'0017.25'}}:{}),method:'native',span:{start_byte:0,end_byte:text.length},text,text_sha256:sha(text),editable,truncated:false,metadata_omitted:false}],offset:0,total_blocks:1,next_offset:null,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:editable}});});
 await page.route('**/attachments/*/corrections',r=>{state.writes++;const body=r.request().postDataJSON();expect(body.expected_revision).toBe(old);expect(body.extraction_revision).toBe('extraction-first');state.revision=next;state.text=body.edits[0].text;if(uncertain)return r.fulfill({status:503,json:{error:{message:'Lost acknowledgment'}}});return r.fulfill({json:{attachment:{...attachment,revision:next,extraction_revision:'extraction-next',parent_revision:old}}});});
 await page.goto('/ui/#/chat');await expect(page.locator('.composer textarea')).toBeEnabled();
 await page.locator('input[data-attachment=files]').setInputFiles({name:attachment.name,mimeType:'text/plain',buffer:Buffer.from('Rural exception: July.')});
 await page.getByRole('button',{name:documentReader(locale).open,exact:true}).click();
 await readerOptions(page);await page.getByRole('button',{name:documentReview(locale).title,exact:true}).click();
 return state;
}

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`private correction preserves history and pinned preview: ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:844});const state=await setup(page,locale),copy=documentReview(locale),panel=page.locator('.document-review');
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();
 await panel.locator('textarea').fill('Rural exception: August.');
 await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await expect(panel.locator('.document-review-blocks pre')).toHaveText('Rural exception: August.');
 const versions=panel.locator('.document-revision-picker select');await expect(versions).toHaveValue(next);await versions.selectOption(old);
 await expect(panel.locator('.document-review-blocks pre')).toHaveText('Rural exception: July.');await expect(panel.getByRole('button',{name:copy.edit,exact:true})).toHaveCount(0);
 if(locale==='en'||locale==='ar')await page.screenshot({path:test.info().outputPath(`revision-reader-${locale}.png`)});
 await page.locator('.document-reader').getByRole('button',{name:documentReader(locale).document,exact:true}).click();
 await expect(page.locator('.document-reader-surface pre')).toContainText('July');expect(state.previewRevisions.every(r=>r===old)).toBe(true);expect(state.writes).toBe(1);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('private correction uncertain save inspects latest without replay',async({page})=>{
 const state=await setup(page,'en',true),copy=documentReview('en'),panel=page.locator('.document-review');
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();await panel.locator('textarea').fill('Rural exception: August.');await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await expect(panel.getByRole('alert')).toHaveText(copy.outcome);await expect(panel.locator('textarea')).toHaveValue('Rural exception: August.');await panel.getByRole('button',{name:copy.load,exact:true}).click();await expect(panel.getByText(copy.same,{exact:true})).toBeVisible();expect(state.writes).toBe(1);
});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`native form review state is localized: ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:844});const state=await setup(page,locale,false,true),panel=page.locator('.document-review');
 await expect(panel.locator('.document-review-blocks pre')).toHaveText('Amount: 0017.25');
 await expect(panel.locator('.document-review-blocks')).toContainText(documentNavigation(locale).needsReview);
 expect(state.writes).toBe(0);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(locale==='en'||locale==='ar')await page.screenshot({path:test.info().outputPath(`native-form-${locale}.png`)});
});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`tracked alternatives stay distinct from current text: ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:844});const state=await setup(page,locale,false,false,true),panel=page.locator('.document-review'),alternatives=panel.locator('.document-revision-alternatives');
 await expect(panel.locator('.document-review-blocks>li>pre')).toHaveText('Rural exception: July.');
 await expect(alternatives.locator('summary')).toContainText(wordRevisions(locale).title);
 await expect(alternatives.locator('pre')).not.toBeVisible();await alternatives.locator('summary').focus();await page.keyboard.press('Enter');
 await expect(alternatives.locator('pre')).toHaveText('999.99 superseded');await expect(alternatives).toContainText(documentComparison(locale).before);
 expect(state.writes).toBe(0);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await expect(alternatives.locator('time')).toHaveAttribute('dateTime','2026-01-02T00:00:00Z');
 if(locale==='en'||locale==='ar')await page.screenshot({path:test.info().outputPath(`tracked-alternatives-${locale}.png`)});
});
