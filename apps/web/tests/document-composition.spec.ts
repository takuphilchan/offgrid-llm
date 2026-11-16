import {expect,test,type Page} from '@playwright/test';
import {fixture,open,result,hash,source} from './helpers/synthesis-fixture';
import {compositionCopy} from '../src/i18n/document-composition';
import {documentSynthesis} from '../src/i18n/document-synthesis';
import {documentComparison} from '../src/i18n/document-comparison';
import {documentReader} from '../src/i18n/document-reader';
import {languageRegistry,type LanguageCode} from '../src/i18n/language';
import type {Composition,CompositionReport,CompositionEvidence} from '../src/api/document-composition';

test.describe.configure({mode:'parallel'});
async function setup(page:Page,locale:LanguageCode='en'){
 const parent=await fixture(page,locale);
 const state={created:false,posts:0,unknown:false,forged:false,exports:0,job:{id:'composition-1',analysis_id:parent.record.id,plan_id:'composition-plan',source_plan_id:parent.record.plan.id,model:parent.record.model,state:'awaiting_review',attempt:1,completed_steps:1,total_steps:1,reviews_current:true,created_at:'2026-11-13T13:00:00Z',updated_at:'2026-11-13T13:00:00Z'} as Composition};
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:['document-synthesis-candidates-v1','document-synthesis-export-v1','document-synthesis-review-v1','document-composition-v1']}}));
 const report=():CompositionReport=>({composition:state.job,source_units:1,sections:[{text:'The report is due by 31 July. Approved extensions remain valid.',contributors:1,verification:'source_identity_only'}],gaps:[],extraction_gaps:[],candidate_gaps:[],unrepresented_batches:0,semantic_review_required:true});
 await page.route('**/v1/document-analyses/synthesis/compositions**',r=>{
  if(parent.revoked)return r.fulfill({status:404,json:{error:'unavailable'}});
  const url=new URL(r.request().url()),path=url.pathname;
  if(path.endsWith('/report'))return r.fulfill({json:report()});
  if(path.endsWith('/export')){state.exports++;const content=JSON.stringify({draft:true,report:report()});return r.fulfill({json:{analysis_id:state.job.id,format:url.searchParams.get('format'),content,sha256:hash(content)}});}
  if(path.endsWith('/evidence')){
   const c=result().receipts[0].result.candidates[0],candidate={text:c.text,evidence:c.evidence,conditions:c.conditions,exceptions:c.exceptions,verification:c.verification};if(state.forged)candidate.evidence[0].text='forged';
   const evidence:CompositionEvidence={composition_id:state.job.id,plan_id:state.job.plan_id,section:0,offset:0,total:1,items:[{input:{id:'leaf',batch:0,batch_id:'batch1',candidate_index:0,candidate_sha256:hash(JSON.stringify(candidate)),review_revision:1,decision:'keep',section_checked:true},candidate}],next_offset:null};return r.fulfill({json:evidence});
  }
  if(path.endsWith('/cancel')||path.endsWith('/retry')){state.job.state=path.endsWith('/cancel')?'cancelled':'running';return r.fulfill({json:{accepted:true}});}
  if(r.request().method()==='POST'){state.created=true;state.posts++;return state.unknown?r.abort('failed'):r.fulfill({status:202,json:state.job});}
  if(url.searchParams.has('id'))return r.fulfill({json:state.job});
  return r.fulfill({json:{analysis_id:parent.record.id,items:state.created?[state.job]:[],next_offset:null}});
 });
 const dialog=await open(page,locale);await dialog.locator('form').getByRole('button',{name:documentSynthesis(locale).create}).click();
 await dialog.getByRole('button',{name:compositionCopy(locale).title,exact:true}).click();return {state,parent,dialog};
}

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`composed summary source inspection ${locale}`,async({page},info)=>{
 const {state,parent,dialog}=await setup(page,locale),copy=compositionCopy(locale),reader=documentReader(locale);
 await dialog.getByRole('button',{name:copy.create,exact:true}).click();await expect(dialog.locator('.composition-prose')).toContainText('Approved extensions remain valid.');
 await dialog.locator('.composition-source-link').click();await expect(dialog.locator('.composition-evidence')).toBeFocused();await expect(dialog.locator('.composition-evidence')).toContainText(source);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`composition-${locale}-desktop.png`)});
 await dialog.locator('.composition-evidence').getByRole('button',{name:reader.close,exact:true}).click();await expect(dialog.locator('.composition-source-link')).toBeFocused();
 await page.setViewportSize({width:390,height:844});await dialog.locator('.composition-source-link').click();expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`composition-${locale}-narrow.png`)});
 expect(state.posts).toBe(1);expect(parent.errors).toEqual([]);
});
test('changed reviews stay explicit and forged sources are withheld',async({page})=>{
 const {state,dialog}=await setup(page);await dialog.getByRole('button',{name:compositionCopy('en').create,exact:true}).click();await expect(dialog.locator('.composition-prose')).toBeVisible();
 state.job.reviews_current=false;await expect(dialog.locator('.composition-panel')).toContainText(compositionCopy('en').stale,{timeout:6000});
 state.forged=true;await dialog.locator('.composition-source-link').click();await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.locator('.composition-prose')).toHaveCount(0);
});
test('accepted composition loss recovers through history without replay',async({page})=>{
 const {state,dialog}=await setup(page);state.unknown=true;await dialog.getByRole('button',{name:compositionCopy('en').create,exact:true}).click();await expect(dialog.getByRole('alert')).toContainText(documentComparison('en').unknown);
 await dialog.locator('.composition-toolbar').getByRole('button',{name:'Refresh',exact:true}).click();await expect(dialog.locator('.composition-prose')).toBeVisible();expect(state.posts).toBe(1);
});
test('report export and access revocation discard cached evidence',async({page})=>{
 const {state,parent,dialog}=await setup(page);await dialog.getByRole('button',{name:compositionCopy('en').create,exact:true}).click();await expect(dialog.locator('.composition-prose')).toBeVisible();
 const exports=dialog.locator('.composition-panel .comparison-export');await exports.locator(':scope > button').click();const pending=page.waitForEvent('download');await exports.getByRole('button',{name:'JSON',exact:true}).click();expect((await pending).suggestedFilename()).toBe('offgrid-draft-summary.json');expect(state.exports).toBe(1);
 parent.revoked=true;await expect(dialog.locator('.composition-prose')).toHaveCount(0,{timeout:6000});await expect(dialog.getByRole('alert')).toBeVisible();
});

test('composition controls remain usable at enlarged display',async({page},info)=>{
 await page.setViewportSize({width:640,height:450});const cdp=await page.context().newCDPSession(page);await cdp.send('Emulation.setDeviceMetricsOverride',{width:640,height:450,deviceScaleFactor:2,mobile:false,screenWidth:1280,screenHeight:900});
 const {state,dialog}=await setup(page),copy=compositionCopy('en');state.job.state='running';state.job.completed_steps=0;
 const create=dialog.getByRole('button',{name:copy.create,exact:true});await create.focus();await page.keyboard.press('Enter');await expect(dialog.locator('.composition-progress progress')).toBeVisible();
 await dialog.locator('.composition-panel').getByRole('button',{name:'Cancel',exact:true}).click();await expect(dialog.locator('.composition-panel').getByRole('button',{name:'Try again',exact:true})).toBeVisible();
 await dialog.locator('.composition-panel').getByRole('button',{name:'Try again',exact:true}).click();await expect(dialog.locator('.composition-progress progress')).toBeVisible();state.job.state='awaiting_review';state.job.completed_steps=1;
 await expect(dialog.locator('.composition-prose')).toBeVisible({timeout:6000});await expect(dialog.locator('.composition-history option:checked')).toContainText(documentSynthesis('en').review);await dialog.locator('.composition-source-link').focus();await page.keyboard.press('Enter');await expect(dialog.locator('.composition-evidence')).toBeFocused();
 expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);await page.screenshot({path:info.outputPath('composition-zoom-200.png')});expect(state.posts).toBe(1);
});
