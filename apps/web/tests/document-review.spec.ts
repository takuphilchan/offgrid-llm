import {readerOptions} from './helpers/reader-options';
import { expect, test, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { craftFixture } from './helpers/craft-fixture';
import { documentReview } from '../src/i18n/document-review';
import { workflow } from '../src/i18n/workflow';
import { documentReader } from '../src/i18n/document-reader';
import { languageRegistry, type LanguageCode } from '../src/i18n/language';
import { documentFormatLimits } from '../src/i18n/document-format-limits';

const original = 'Applications close 31 July. <script>untrusted()</script> 中文';
const hash = (text:string) => createHash('sha256').update(text).digest('hex');
async function setup(page:Page, locale:LanguageCode='en', format='txt') {
 await craftFixture(page,locale,'light');
 const state = { revision:'extraction-first', text:original, writes:[] as any[], reads:[] as URL[], fail:false, staleReceipt:false, readonly:false, truncated:false, invalid:false, pagination:false, failRead:false };
 const document = {id:'notes',name:'Policy.txt',content_type:'text/plain',size:200,chunk_count:1,source_retained:true,original_retained:false,extraction_revision:'extraction-first'};
 await page.route('**/v1/documents/source?**',r=>r.fulfill({json:{document,content:original,truncated:false}}));
 await page.route('**/v1/documents/structure?**',r=>{
  const url=new URL(r.request().url()); state.reads.push(url);
  if(state.failRead)return r.fulfill({status:404,json:{error:{code:'document_not_found'}}});
  const pinned=url.searchParams.get('revision') || state.revision;
  const text=pinned==='extraction-first'?original:state.text;
  const offset=Number(url.searchParams.get('offset') || 0), editable=!state.readonly && pinned===state.revision && !state.truncated;
  return r.fulfill({json:{document_id:state.invalid?'other':'notes',revision:pinned,format,coverage:format==='txt'?undefined:{format,status:'text_only',total_units:1,readable_units:1,review_units:[1]},method:'native',units:[{id:'section-1',kind:'section',index:1,name:'Policy',span:{start:0,end:500},state:'ready'}],blocks:[{id:offset?'later':'clause',unit_id:'section-1',kind:'paragraph',span:{start:0,end:Buffer.byteLength(text)},method:'native',text:offset?'Later evidence':text,text_sha256:hash(text),editable,truncated:state.truncated,metadata_omitted:false}],offset,total_blocks:state.pagination?2:1,next_offset:state.pagination&&!offset?1:null,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:!state.readonly && pinned===state.revision}});
 });
 await page.route('**/v1/documents/corrections',r=>{
  const body=r.request().postDataJSON(); state.writes.push(body);
  if(state.fail)return r.fulfill({status:503,json:{error:{code:'document_review_unavailable'}}});
  state.text=body.edits[0].text; state.revision='extraction-second';
  return r.fulfill({json:{success:true,document:{...document,extraction_revision:state.staleReceipt?body.expected_revision:state.revision}}});
 });
 await page.goto('/ui/#/knowledge');
 await page.locator('.resource-card').getByRole('button',{name:workflow[locale].viewSource,exact:true}).click();
 await page.getByRole('button',{name:documentReview(locale).title,exact:true}).click();
 return state;
}

for(const locale of Object.keys(languageRegistry) as LanguageCode[]) for(const format of ['docx','xlsx','pptx']) test(`Office extraction limits in ${locale}: ${format}`,async({page})=>{
 await page.setViewportSize({width:390,height:844});
 await setup(page,locale,format);
 const panel=page.locator('.document-review'),copy=documentFormatLimits(locale,format);
 await expect(panel.getByText(copy,{exact:true})).toBeVisible();
 await expect(panel.locator('[data-extraction-coverage="text_only"]')).toHaveCount(1);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(locale!=='en')expect(copy).not.toEqual(documentFormatLimits('en',format));
});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`review controls are localized and bounded: ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:844});
 const state=await setup(page,locale), copy=documentReview(locale), panel=page.locator('.document-review');
 await expect(panel.getByText(copy.note,{exact:true})).toBeVisible();
 await expect(panel.locator('.document-review-blocks pre')).toHaveText(original);
 await expect(panel.locator('script')).toHaveCount(0);
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();
 await panel.getByRole('textbox',{name:copy.draft,exact:true}).fill('Corrected 中文 العربية');
 await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeEnabled();
 await expect(panel.getByRole('combobox')).toBeDisabled();
 await page.getByRole('button',{name:workflow[locale].sourceTitle,exact:true}).click();
 await page.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(panel.getByRole('textbox')).toHaveValue('Corrected 中文 العربية');
 expect(state.writes).toHaveLength(0);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(locale!=='en') for(const key of ['title','note','outcome','save'] as const) expect(copy[key]).not.toEqual(documentReview('en')[key]);
});

test('correction is revision-checked and displayed only after a matching publication receipt',async({page})=>{
 const state=await setup(page),copy=documentReview('en'), panel=page.locator('.document-review');
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();
 const replacement='Applications close 31 August. 中文';
 await panel.getByRole('textbox').fill(replacement);
 await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await expect(panel.getByRole('status')).toHaveText(copy.saved);
 await expect(panel.locator('.document-review-blocks pre')).toHaveText(replacement);
 expect(state.writes).toEqual([{document_id:'notes',expected_revision:'extraction-first',edits:[{block_id:'clause',expected_text_sha256:hash(original),text:replacement}]}]);
 expect(state.reads.at(-1)!.searchParams.get('revision')).toBe('extraction-second');
});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`correction draft survives navigation and waits for fresh source authority: ${locale}`,async({page})=>{
 const state=await setup(page,locale),copy=documentReview(locale),panel=page.locator('.document-review');
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();await panel.getByRole('textbox').fill('Unsaved correction 中文 العربية');
 expect(await page.evaluate(()=>Object.values(localStorage).some(value=>String(value).includes('Unsaved correction')))).toBe(false);
 await page.keyboard.press('Escape');await page.evaluate(()=>{location.hash='#/chat';});await expect(panel).toHaveCount(0);
 await page.evaluate(()=>{location.hash='#/knowledge';});
 const reopen=async()=>{await page.locator('.resource-card').getByRole('button',{name:workflow[locale].viewSource,exact:true}).click();await page.getByRole('button',{name:copy.title,exact:true}).click();};
 await reopen();await expect(panel.getByRole('textbox')).toHaveValue('Unsaved correction 中文 العربية');expect(state.writes).toHaveLength(0);
 await page.keyboard.press('Escape');state.failRead=true;await reopen();
 await expect(panel.getByRole('alert')).toHaveText(copy.unavailable);await expect(panel.getByRole('textbox')).toHaveCount(0);
 await page.keyboard.press('Escape');state.failRead=false;state.readonly=true;await reopen();
 await expect(panel.getByRole('textbox')).toHaveValue('Unsaved correction 中文 العربية');await expect(panel.getByRole('textbox')).toHaveAttribute('readonly','');await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();
 await panel.getByRole('button',{name:copy.cancel,exact:true}).click();await page.keyboard.press('Escape');state.readonly=false;await reopen();await expect(panel.getByRole('textbox')).toHaveCount(0);expect(state.writes).toHaveLength(0);
});

test('a restored correction requires explicit rebase when the current extraction changed',async({page})=>{
 const state=await setup(page),copy=documentReview('en'),panel=page.locator('.document-review');
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();await panel.getByRole('textbox').fill('Keep my draft');await page.keyboard.press('Escape');
 state.revision='extraction-concurrent';state.text='Another editor changed the clause';
 await page.route('**/v1/documents/source?**',r=>r.fulfill({json:{document:{id:'notes',name:'Policy.txt',source_retained:true,original_retained:false,extraction_revision:state.revision},content:state.text,truncated:false}}));
 await page.locator('.resource-card').getByRole('button',{name:workflow.en.viewSource,exact:true}).click();await page.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(panel.getByRole('textbox')).toHaveValue('Keep my draft');await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();
 await panel.getByRole('button',{name:copy.load,exact:true}).click();await expect(panel.locator('.document-correction pre')).toHaveText(state.text);
 await panel.getByRole('button',{name:copy.rebase,exact:true}).click();await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeEnabled();
 await panel.getByRole('button',{name:copy.save,exact:true}).click();await expect(panel.getByRole('status')).toHaveText(copy.saved);expect(state.writes[0].expected_revision).toBe('extraction-concurrent');
});

test('navigating away during a correction keeps uncertainty instead of replaying the write',async({page})=>{
 const state=await setup(page),copy=documentReview('en'),panel=page.locator('.document-review');
 let finish!:()=>void;const pending=new Promise<void>(resolve=>{finish=resolve;});
 await page.route('**/v1/documents/corrections',async route=>{await pending;await route.fallback();});
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();await panel.getByRole('textbox').fill('Pending correction');await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await page.keyboard.press('Escape');await page.locator('.resource-card').getByRole('button',{name:workflow.en.viewSource,exact:true}).click();await page.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(panel.getByRole('alert')).toHaveText(copy.outcome);await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();finish();await expect.poll(()=>state.writes.length).toBe(1);
 await panel.getByRole('button',{name:copy.load,exact:true}).click();await expect(panel.getByRole('status')).toHaveText(copy.same);expect(state.writes).toHaveLength(1);
});

test('unknown save outcome retains draft and requires explicit latest-base review before another write',async({page})=>{
 const state=await setup(page),copy=documentReview('en'),panel=page.locator('.document-review'); state.fail=true;
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();
 await panel.getByRole('textbox').fill('My draft');
 await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await expect(panel.getByRole('alert')).toHaveText(copy.outcome);
 await expect(panel.getByRole('textbox')).toHaveValue('My draft');
 await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();
 state.revision='extraction-concurrent';state.text='Another editor changed this';state.fail=false;
 await panel.getByRole('button',{name:copy.load,exact:true}).click();
 await expect(panel.locator('.document-correction pre')).toHaveText(state.text);
 expect(state.writes).toHaveLength(1);
 await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();
 await panel.getByRole('button',{name:copy.rebase,exact:true}).click();
 await expect(panel.getByRole('textbox')).toHaveValue('My draft');
 await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await expect(panel.getByRole('status')).toHaveText(copy.saved);
 expect(state.writes[1].expected_revision).toBe('extraction-concurrent');
 expect(state.writes[1].edits[0].expected_text_sha256).toBe(hash('Another editor changed this'));
});

test('a stale receipt never announces success or replays an already-applied correction',async({page})=>{
 const state=await setup(page),copy=documentReview('en'),panel=page.locator('.document-review');state.staleReceipt=true;
 await panel.getByRole('button',{name:copy.edit,exact:true}).click();
 await panel.getByRole('textbox').fill('Already saved draft');
 await panel.getByRole('button',{name:copy.save,exact:true}).click();
 await expect(panel.getByRole('alert')).toHaveText(copy.outcome);
 await panel.getByRole('button',{name:copy.load,exact:true}).click();
 await expect(panel.getByRole('status')).toHaveText(copy.same);
 await expect(panel.getByRole('button',{name:copy.rebase,exact:true})).toHaveCount(0);
 await expect(panel.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();
 expect(state.writes).toHaveLength(1);
});

for(const mode of ['readonly','truncated','invalid','failRead'] as const)test(`review refuses unsafe editing: ${mode}`,async({page})=>{
 const state=await setup(page);state[mode]=true;
 // Reopen to perform a new authorized, revision-pinned read.
 await page.keyboard.press('Escape');
 await page.locator('.resource-card').getByRole('button',{name:workflow.en.viewSource,exact:true}).click();
 await page.getByRole('button',{name:documentReview('en').title,exact:true}).click();
 const panel=page.locator('.document-review'),copy=documentReview('en');
 await expect(panel.getByText(mode==='readonly'?copy.readOnly:mode==='truncated'?copy.incomplete:copy.unavailable,{exact:true})).toBeVisible();
 await expect(panel.getByRole('button',{name:copy.edit,exact:true})).toHaveCount(0);
 expect(state.writes).toHaveLength(0);
});

test('paging stays pinned when the active revision changes',async({page})=>{
 const state=await setup(page);state.pagination=true;
 await page.keyboard.press('Escape');
 await page.locator('.resource-card').getByRole('button',{name:workflow.en.viewSource,exact:true}).click();
 await page.getByRole('button',{name:documentReview('en').title,exact:true}).click();
 const panel=page.locator('.document-review');
 await expect(panel.locator('.document-review-blocks pre')).toHaveText(original);
 state.revision='extraction-concurrent';
 await panel.getByRole('button',{name:documentReader('en').next,exact:true}).click();
 await expect(panel.locator('.document-review-blocks pre')).toHaveText('Later evidence');
 expect(state.reads.at(-1)!.searchParams.get('revision')).toBe('extraction-first');
 await expect(panel.getByRole('button',{name:documentReview('en').edit,exact:true})).toHaveCount(0);
});

test('Chat citations pass their historical revision to the shared review panel',async({page})=>{
 const state=await setup(page);await page.keyboard.press('Escape');state.readonly=true;
 const citation={kind:'section',index:1,start_byte:0,end_byte:Buffer.byteLength(original),source_sha256:'b'.repeat(64),extractor_revision:'parser-old'};
 const session={name:'Demo briefing',messages:[{role:'assistant',content:'Answer [1]',knowledge_sources:[{document_id:'notes',chunk_id:'old-chunk',name:'Policy.txt',content:original,content_hash:'a'.repeat(32),source:citation}]}]};
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
 await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:session}));
 await page.route('**/v1/documents/source?**',r=>{
  const query=new URL(r.request().url()).searchParams;
  if(query.get('format')==='preview')return r.fulfill({json:{name:'Policy.txt',format:'txt',sha256:citation.source_sha256,state:'ready',units:[{...citation,text:original,state:'ready'}]}});
  return r.fulfill({json:{document:{id:'notes',name:'Policy.txt',original_retained:true,extraction_revision:'extraction-first'},content:original,citation_verified:true,truncated:false}});
 });
 await page.goto('/ui/#/chat');
 await page.getByRole('button',{name:'Demo briefing',exact:true}).click();
 await page.locator('.knowledge-passages > summary').click();
 await page.locator('.knowledge-passages').getByRole('button',{name:workflow.en.viewSource,exact:true}).click();
 const reader=page.locator('.document-reader');
 await readerOptions(page);await reader.getByRole('button',{name:documentReview('en').title,exact:true}).click();
 await expect(reader.locator('.document-review mark')).toHaveText(original);
 await expect(reader.getByText(documentReview('en').readOnly,{exact:true})).toBeVisible();
 expect(state.reads.at(-1)!.searchParams.get('revision')).toBe('extraction-first');
 await reader.getByRole('button',{name:documentReader('en').document,exact:true}).click();
 await expect(reader.locator('.document-reader-surface pre')).toHaveText(original);
});

for(const locale of ['en','ar'] as const)test(`review remains readable with keyboard and enlarged layout: ${locale}`,async({page})=>{
 await page.setViewportSize({width:1440,height:1100});await setup(page,locale);
 await page.evaluate(()=>{document.documentElement.style.zoom='2';});
 const panel=page.locator('.document-review'),copy=documentReview(locale);
 const edit=panel.getByRole('button',{name:copy.edit,exact:true});
 await edit.focus();await page.keyboard.press('Enter');
 await expect(panel.getByRole('textbox')).toBeFocused();
 await panel.getByRole('textbox').fill('Applications close 31 August.\n中文 العربية <b>literal</b>');
 await panel.getByRole('button',{name:copy.save,exact:true}).scrollIntoViewIfNeeded();
 expect(await page.getByRole('dialog').evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
 await page.screenshot({path:test.info().outputPath(`review-${locale}-enlarged.png`)});
});
