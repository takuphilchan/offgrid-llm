import {expect,test,type Page} from '@playwright/test';
import {createHash} from 'node:crypto';
import {craftFixture} from './helpers/craft-fixture';
import {documentComparison} from '../src/i18n/document-comparison';
import {documentReader} from '../src/i18n/document-reader';
import {documentExport} from '../src/i18n/document-export';
import {readFile} from 'node:fs/promises';
import {documentReview} from '../src/i18n/document-review';
import {workflow} from '../src/i18n/workflow';
import {sourceClose} from '../src/i18n/knowledge-original';
import {languageRegistry,type LanguageCode} from '../src/i18n/language';
import {analysisAPI,validateAnalysis,validateAnalysisResults,type Analysis,type AnalysisResults} from '../src/api/document-analyses';
test.describe.configure({mode:'parallel'});

for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`comparison export localized ${locale}`,async({page},info)=>{
 const state=await fixture(page,locale),copy=documentComparison(locale),exportCopy=documentExport(locale),dialog=await open(page,locale);
 await page.setViewportSize({width:390,height:844});await dialog.getByRole('button',{name:copy.title,exact:true}).click();
 await dialog.locator('.comparison-export > button').click();await expect(dialog.locator('.comparison-export')).toContainText(exportCopy.title);await expect(dialog.locator('.comparison-export')).toContainText(exportCopy.retention);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`export-${locale}.png`)});
 const saving=page.waitForEvent('download');await dialog.getByRole('button',{name:'JSON',exact:true}).click();const saved=await saving;
 expect(saved.suggestedFilename()).toBe('offgrid-comparison.json');const artifact=JSON.parse(await readFile((await saved.path())!,'utf8'));expect(artifact.state).toBe('incomplete');expect(artifact.sources).toEqual(refs);
 expect(state.calls.filter(c=>c.path.includes('/export?'))).toHaveLength(1);expect(state.renderErrors).toEqual([]);
 expect(await dialog.evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBe(true);
});

test('comparison export rejects forged artifact without downloading',async({page})=>{
 const state=await fixture(page),dialog=await open(page);state.exportForge=true;let downloads=0;page.on('download',()=>downloads++);
 await dialog.getByRole('button',{name:documentComparison('en').title,exact:true}).click();await dialog.locator('.comparison-export > button').click();await dialog.getByRole('button',{name:'CSV',exact:true}).click();
 await expect(dialog.locator('.comparison-export [role=alert]')).toBeVisible();expect(downloads).toBe(0);expect(state.calls.filter(c=>c.path.includes('/export?'))).toHaveLength(1);
});

test('comparison export offers markdown and hides on unfinished work',async({page})=>{
 const state=await fixture(page),dialog=await open(page);await dialog.getByRole('button',{name:documentComparison('en').title,exact:true}).click();await dialog.locator('.comparison-export > button').click();
 const saving=page.waitForEvent('download');await dialog.getByRole('button',{name:'Markdown',exact:true}).click();expect((await saving).suggestedFilename()).toBe('offgrid-comparison.md');
 state.record.state='running';state.record.completed_steps=0;state.record.completed_units=0;await expect(dialog.locator('.comparison-export')).toHaveCount(0);
});

const texts=['政策: 17.25. Exception July.','政策: 18.25. Exception August.'];
const hash=(s:string)=>createHash('sha256').update(s).digest('hex');
const refs=[{document_id:'before',revision:'old'},{document_id:'after',revision:'new'}];
const pair={before_unit:'section-1',after_unit:'section-1',alignment:'position' as const};
const citations=texts.map((text,i)=>({source:i,revision:refs[i].revision,unit_id:'section-1',span:{start_byte:0,end_byte:Buffer.byteLength(text)},text_sha256:hash(text)}));
function job():Analysis{return {id:'comparison-1',state:'incomplete',attempt:1,completed_steps:1,completed_units:2,review_required:false,created_at:'2026-11-13T01:00:00Z',updated_at:'2026-11-13T01:00:00Z',pairs:[{...pair}],plan:{id:'plan',workflow:'comparison',version:'document-analysis-v1',sources:refs.map(s=>({...s})),units:citations.map((c,i)=>({ordinal:i,source:i,unit_id:c.unit_id,span:{...c.span},blocks:1})),gaps:[{source:0,code:'extraction_incomplete'}]}};}
function result():AnalysisResults{return {analysis_id:'comparison-1',step:0,offset:0,next_offset:null,total_changes:2,previews:[{before:{text:texts[0],span:citations[0].span,text_sha256:hash(texts[0]),truncated:false}},{after:{text:texts[1],span:citations[1].span,text_sha256:hash(texts[1]),truncated:false}}],result:{pair,unchanged_lines:0,before_lines:1,after_lines:1,compared_cells:0,ambiguous_cells:0,changes:[{kind:'text_removed',before:citations[0]},{kind:'text_added',after:citations[1]}]}};}
async function fixture(page:Page,locale:LanguageCode='en'){
 await craftFixture(page,locale,['ar','zh-Hans'].includes(locale)?'dark':'light');
 const state={created:false,unknown:false,revoked:false,forge:false,old:false,cell:false,paginate:false,exportForge:false,calls:[] as {path:string;method:string;body:any}[],record:job(),reply:result(),structureReads:[] as URL[],renderErrors:[] as string[]};
 page.on('console',message=>{if(message.type()==='error'&&message.text().includes('same key'))state.renderErrors.push(message.text());});page.on('pageerror',error=>state.renderErrors.push(error.message));
 const docs=refs.map((r,i)=>({id:r.document_id,name:i?'Updated policy.txt':'Original policy.txt',size:200,index_status:'ready',source_retained:true,extraction_revision:r.revision}));
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:state.old?[]:['document-comparison-jobs-v1','document-analysis-history-v1','document-analysis-export-v1']}}));
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:state.revoked?[]:docs}}));
 await page.route('**/v1/documents/structure?*',r=>{const url=new URL(r.request().url());state.structureReads.push(url);const id=url.searchParams.get('id')!,i=id==='before'?0:1;return r.fulfill({json:{document_id:id,revision:refs[i].revision,format:'txt',method:'native',units:[],blocks:[],offset:0,total_blocks:0,next_offset:null,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:true}});});
 await page.route('**/v1/document-analyses**',r=>{
  const request=r.request(),url=new URL(request.url()),path=url.pathname,method=request.method(),q=url.searchParams;
  state.calls.push({path:request.url(),method,body:method==='POST'?request.postDataJSON():null});
  if(state.revoked)return r.fulfill({status:404,json:{error:'private source unavailable'}});
  if(path.endsWith('/export')){const content=JSON.stringify({state:'incomplete',sources:refs,changes:texts});return r.fulfill({json:{analysis_id:'comparison-1',format:q.get('format'),content,sha256:state.exportForge?'0'.repeat(64):hash(content)}});}
  if(path.endsWith('/source')){const side=q.get('side')==='before'?0:1;return r.fulfill({json:{analysis_id:'comparison-1',document_id:refs[side].document_id,citation:citations[side],text:state.forge?'Forged evidence':texts[side],truncated:false,...(state.cell?{cell:{reference:'B2',formula:side?'SUM(A1:A3)':'SUM(A1:A2)',value:'17.25',cached_value:'17.25'}}:{})}});}
  if(path.endsWith('/results')){if(state.paginate){const offset=Number(q.get('offset')??0);return r.fulfill({json:{...state.reply,offset,total_changes:21,next_offset:offset===0?20:null,previews:Array.from({length:offset===0?20:1},()=>state.reply.previews![0]),result:{...state.reply.result,changes:Array.from({length:offset===0?20:1},()=>state.reply.result.changes[0])}}});}return r.fulfill({json:state.reply});}
  if(path.endsWith('/cancel')||path.endsWith('/retry')){state.record.state=path.endsWith('/cancel')?'cancelled':'running';return r.fulfill({json:{accepted:true}});}
  if(method==='POST'){state.created=true;return state.unknown?r.abort('failed'):r.fulfill({status:202,json:state.record});}
  if(q.has('id'))return r.fulfill({json:state.record});
  return r.fulfill({json:{analyses:state.created?[{id:state.record.id,state:state.record.state,documents:refs.map(r=>r.document_id),created_at:state.record.created_at,updated_at:state.record.updated_at}]:[],next_offset:null}});
 });
 await page.goto('/ui/#/knowledge');return state;
}
async function open(page:Page,locale:LanguageCode='en'){
 const copy=documentComparison(locale);await page.getByRole('button',{name:copy.title,exact:true}).click();
 const dialog=page.getByRole('region',{name:copy.title});await dialog.locator('.comparison-start').getByLabel(copy.before,{exact:true}).selectOption('before');await dialog.locator('.comparison-start').getByLabel(copy.after,{exact:true}).selectOption('after');return dialog;
}
for(const locale of Object.keys(languageRegistry) as LanguageCode[])test(`comparison and evidence localized ${locale}`,async({page},info)=>{
 const state=await fixture(page,locale),copy=documentComparison(locale),dialog=await open(page,locale);
 await page.setViewportSize({width:390,height:844});await dialog.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(dialog.locator('.comparison-job')).toContainText(copy.states.incomplete);await expect(dialog.locator('.comparison-gaps')).toContainText(copy.gaps);
 await expect(dialog.locator('.comparison-excerpts pre').first()).toHaveText(texts[0]);
 await dialog.getByRole('button',{name:workflow[locale].viewSource+' · '+copy.before,exact:true}).click();await expect(dialog.locator('.comparison-source pre').first()).toHaveText(texts[0]);await expect(dialog.locator('.comparison-source')).toBeFocused();
 await dialog.getByRole('button',{name:workflow[locale].viewSource+' · '+copy.after,exact:true}).click();await expect(dialog.locator('.comparison-source pre').first()).toHaveText(texts[1]);
 expect(state.calls.filter(c=>c.method==='POST')).toHaveLength(1);expect(state.calls.find(c=>c.method==='POST')!.body.sources).toEqual(refs);
 expect(state.renderErrors).toEqual([]);
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 if(['en','ar','zh-Hans'].includes(locale))await page.screenshot({path:info.outputPath(`comparison-${locale}.png`),fullPage:true});
 await dialog.getByRole('button',{name:sourceClose[locale],exact:true}).click();await page.getByRole('button',{name:copy.title,exact:true}).click();await page.locator('.comparison-tabs').getByRole('button',{name:copy.history}).click();const history=page.locator('.comparison-history');await history.getByRole('button',{name:/Original policy/}).click();await expect(page.locator('.comparison-changes li')).toHaveCount(2);expect(state.calls.filter(c=>c.method==='POST')).toHaveLength(1);
});
test('unknown submission is not replayed; history recovers the accepted job',async({page})=>{
 const state=await fixture(page),copy=documentComparison('en'),dialog=await open(page);state.unknown=true;await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog.getByRole('alert')).toHaveText(copy.unknown);await expect(dialog.getByRole('button',{name:copy.title,exact:true})).toBeDisabled();
 await dialog.getByRole('button',{name:'Refresh',exact:true}).click();await dialog.locator('.comparison-history').getByRole('button',{name:/Original policy/}).click();await expect(dialog.locator('.comparison-changes li')).toHaveCount(2);expect(state.calls.filter(c=>c.method==='POST')).toHaveLength(1);
});
test('revocation clears evidence, source titles and history while open',async({page})=>{
 const state=await fixture(page),copy=documentComparison('en'),dialog=await open(page);await dialog.getByRole('button',{name:copy.title,exact:true}).click();await dialog.getByRole('button',{name:workflow.en.viewSource+' · '+copy.before,exact:true}).click();await expect(dialog.locator('.comparison-source')).toContainText(texts[0]);state.revoked=true;await expect(dialog.getByRole('alert')).toHaveText(copy.unavailable,{timeout:7000});await expect(dialog.locator('.comparison-source')).toHaveCount(0);await expect(dialog).not.toContainText('Original policy.txt');
});
test('a forged source body is rejected instead of displayed',async({page})=>{
 const state=await fixture(page),copy=documentComparison('en'),dialog=await open(page);state.forge=true;await dialog.getByRole('button',{name:copy.title,exact:true}).click();await dialog.getByRole('button',{name:workflow.en.viewSource+' · '+copy.before,exact:true}).click();await expect(dialog.getByRole('alert')).toHaveText(copy.unavailable);await expect(dialog).not.toContainText('Forged evidence');
});
test('keyboard close restores focus with an enlarged root font',async({page})=>{
 await fixture(page);const dialog=await open(page);await page.evaluate(()=>{document.documentElement.style.fontSize='200%';});await page.setViewportSize({width:780,height:844});expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);await page.keyboard.press('Escape');await expect(page.getByRole('button',{name:documentComparison('en').title,exact:true})).toBeFocused();
});
test('older services do not advertise a comparison action',async({page})=>{
 const state=await fixture(page);state.old=true;await page.reload();await expect(page.locator('.knowledge-documents')).toBeVisible();await expect(page.getByRole('button',{name:documentComparison('en').title,exact:true})).toHaveCount(0);
});
test('client rejects false coverage, duplicate units, wrong revisions and broken pagination',()=>{
 for(const mutate of [(r:Analysis)=>{r.state='complete';},(r:Analysis)=>{r.completed_units=1;},(r:Analysis)=>{r.plan.units[1].source=0;},(r:Analysis)=>{r.plan.sources[1]=r.plan.sources[0];}]){const r=job();mutate(r);expect(()=>validateAnalysis(r)).toThrow();}
 for(const mutate of [(r:AnalysisResults)=>{r.result.changes[0].before!.revision='unrelated';},(r:AnalysisResults)=>{r.next_offset=1;},(r:AnalysisResults)=>{r.result.changes[0].kind='text_added';}]){const r=structuredClone(result());mutate(r);expect(()=>validateAnalysisResults(r,job(),0,0,20)).toThrow();}
});
test('history validation rejects invalid cursors and duplicate identities',async()=>{
 for(const p of [{analyses:[],next_offset:0},{analyses:[{id:'a',state:'bogus',documents:['a','b'],created_at:'2026-11-13'}],next_offset:null}])await expect(analysisAPI((async()=>p) as any).history()).rejects.toThrow();
});
test('cancel and retry are explicit and keep completed checkpoints',async({page})=>{
 const state=await fixture(page);state.record.state='queued';state.record.completed_steps=0;state.record.completed_units=0;
 const dialog=await open(page),copy=documentComparison('en');await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog.locator('.comparison-job')).toContainText(copy.states.queued);await dialog.locator('.comparison-job').getByRole('button',{name:'Cancel',exact:true}).click();await expect(dialog.locator('.comparison-job')).toContainText(copy.states.cancelled);await dialog.locator('.comparison-job').getByRole('button',{name:'Try again',exact:true}).click();await expect(dialog.locator('.comparison-job')).toContainText(copy.states.running);expect(state.calls.filter(c=>c.method==='POST').map(c=>new URL(c.path).pathname)).toEqual(['/v1/document-analyses','/v1/document-analyses/cancel','/v1/document-analyses/retry']);
});
test('bounded change pages preserve global source ordinals and native formula metadata',async({page})=>{
 const state=await fixture(page);state.paginate=true;state.cell=true;const dialog=await open(page),copy=documentComparison('en'),reader=documentReader('en');await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog.locator('.comparison-changes li')).toHaveCount(20);await dialog.getByRole('navigation',{name:copy.changes,exact:true}).getByRole('button',{name:reader.next,exact:true}).click();await expect(dialog.locator('.comparison-changes li')).toHaveCount(1);await dialog.getByRole('button',{name:workflow.en.viewSource+' · '+copy.before,exact:true}).click();await expect(dialog.locator('.comparison-source dl')).toContainText('SUM(A1:A2)');await expect(dialog.locator('.comparison-source dl')).toContainText(copy.cached);expect(state.calls.some(c=>new URL(c.path).searchParams.get('change')==='20')).toBe(true);
 await dialog.locator('.comparison-tabs').getByRole('button',{name:copy.history}).click();await dialog.locator('.comparison-history').getByRole('button',{name:/Original policy/}).click();await expect(dialog.locator('.comparison-changes li')).toHaveCount(1);expect(state.calls.filter(c=>new URL(c.path).pathname.endsWith('/results')).at(-1)!.path).toContain('offset=20');
});
test('zero changes remain qualified and source inspection uses the immutable revision',async({page})=>{
 const state=await fixture(page);state.reply.total_changes=0;state.reply.result.changes=[];state.reply.previews=[];
 const dialog=await open(page),copy=documentComparison('en'),review=documentReview('en');await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog).toContainText(copy.empty);await expect(dialog).toContainText(copy.gaps);await dialog.locator('.comparison-coverage>button').click();await dialog.getByRole('button',{name:review.title+' · '+copy.before,exact:true}).click();await expect(dialog.locator('.document-review')).toContainText(review.readOnly);expect(state.structureReads.at(-1)!.searchParams.get('revision')).toBe('old');await expect(dialog.locator('.document-review').getByRole('button',{name:review.edit,exact:true})).toHaveCount(0);
});
test('corrupted preview is withheld before any source text is shown',async({page})=>{
 const state=await fixture(page);state.reply.previews![0].before!.text='Altered excerpt';const dialog=await open(page),copy=documentComparison('en');await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog.getByRole('alert')).toHaveText(copy.unavailable);await expect(dialog).not.toContainText('Altered excerpt');
});
for(const locale of ['en','ar'] as LanguageCode[])test(`comparison desktop visual hierarchy ${locale}`,async({page},info)=>{
 await fixture(page,locale);await page.setViewportSize({width:1280,height:900});const dialog=await open(page,locale),copy=documentComparison(locale);await page.screenshot({path:info.outputPath(`comparison-desktop-${locale}-setup.png`),fullPage:true});await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog.locator('.comparison-excerpts pre')).toHaveCount(2);const columns=await dialog.locator('.comparison-side').evaluateAll(nodes=>nodes.map(n=>({x:n.getBoundingClientRect().x,y:n.getBoundingClientRect().y})));expect(columns[0].y).toBe(columns[1].y);expect(Math.abs(columns[0].x-columns[1].x)).toBeGreaterThan(300);await expect(dialog.locator('.comparison-start')).toBeVisible();await expect(dialog.locator('.comparison-start').getByLabel(copy.before,{exact:true})).toHaveValue('before');await expect(dialog.locator('.comparison-start').getByLabel(copy.after,{exact:true})).toHaveValue('after');await page.screenshot({path:info.outputPath(`comparison-desktop-${locale}-results.png`),fullPage:true});if(locale==='en'){await page.evaluate(()=>{document.documentElement.dataset.theme='dark';});await page.screenshot({path:info.outputPath('comparison-desktop-dark-results.png'),fullPage:true});}
});
test('comparison fits a 200 percent browser zoom equivalent viewport',async({page},info)=>{
 await fixture(page);await page.setViewportSize({width:640,height:450});
 // Browser zoom reduces the CSS viewport and increases devicePixelRatio.
 // CSS zoom on the root instead magnifies viewport units and is not equivalent.
 const cdp=await page.context().newCDPSession(page);await cdp.send('Emulation.setDeviceMetricsOverride',{width:640,height:450,deviceScaleFactor:2,mobile:false,screenWidth:1280,screenHeight:900});
 const dialog=await open(page),copy=documentComparison('en');await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog.locator('.comparison-excerpts pre')).toHaveCount(2);const size=await dialog.evaluate(node=>({width:node.scrollWidth,client:node.clientWidth,right:node.getBoundingClientRect().right,left:node.getBoundingClientRect().left,viewport:innerWidth}));expect(size.width).toBeLessThanOrEqual(size.client+1);expect(size.left).toBeGreaterThanOrEqual(0);expect(size.right).toBeLessThanOrEqual(size.viewport);expect(await page.evaluate(()=>devicePixelRatio)).toBe(2);await page.screenshot({path:info.outputPath('comparison-zoom-200.png'),fullPage:true});await dialog.getByRole('button',{name:sourceClose.en,exact:true}).click();
});
test('source close restores keyboard focus and comparison works with retrieval disabled',async({page})=>{
 const state=await fixture(page);await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:false,stats:{}}}));await page.reload();const dialog=await open(page),copy=documentComparison('en');await dialog.getByRole('button',{name:copy.title,exact:true}).click();const source=dialog.getByRole('button',{name:workflow.en.viewSource+' · '+copy.before,exact:true});await source.click();await expect(dialog.locator('.comparison-source')).toBeFocused();await dialog.locator('.comparison-source').getByRole('button',{name:documentReader('en').close,exact:true}).click();await expect(source).toBeFocused();expect(state.calls.filter(c=>c.method==='POST')).toHaveLength(1);
});

test('comparison keeps editable sources, reruns with a fresh request, and restores history pair',async({page})=>{
 const state=await fixture(page),copy=documentComparison('en'),view=await open(page);
 await view.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(view.locator('.comparison-excerpts pre')).toHaveCount(2);
 await expect(view.locator('.comparison-start').getByLabel(copy.before,{exact:true})).toHaveValue('before');
 await view.getByRole('button',{name:copy.swap,exact:true}).click();
 await expect(view.locator('.comparison-start').getByLabel(copy.before,{exact:true})).toHaveValue('after');
 await expect(view.locator('.comparison-selection-notice')).toBeVisible();
 await expect(view.locator('.comparison-excerpts pre')).toHaveCount(0);
 await view.getByRole('button',{name:copy.swap,exact:true}).click();
 await view.getByRole('button',{name:copy.title,exact:true}).click();
 await expect.poll(()=>state.calls.filter(c=>c.method==='POST').length).toBe(2);
 const posts=state.calls.filter(c=>c.method==='POST');expect(posts[0].body.request_key).not.toBe(posts[1].body.request_key);
 expect(posts[1].body.sources).toEqual(refs);expect(state.structureReads.length).toBeGreaterThanOrEqual(4);
 await expect(view.locator('.comparison-excerpts pre')).toHaveCount(2);
 await view.getByRole('button',{name:copy.newComparison,exact:true}).click();
 await expect(view.locator('.comparison-start').getByLabel(copy.before,{exact:true})).toHaveValue('');
 await expect(view.getByRole('button',{name:copy.title,exact:true})).toBeDisabled();
 await view.locator('.comparison-tabs').getByRole('button',{name:new RegExp(copy.history)}).click();
 await view.locator('.comparison-history li button').click();
 await expect(view.locator('.comparison-start').getByLabel(copy.before,{exact:true})).toHaveValue('before');
 await expect(view.locator('.comparison-start').getByLabel(copy.after,{exact:true})).toHaveValue('after');
 await expect(view.locator('.comparison-excerpts pre')).toHaveCount(2);
});


test('a new comparison resolves current extraction revisions after an external document update',async({page})=>{
 const state=await fixture(page),view=await open(page),copy=documentComparison('en');
 await view.getByRole('button',{name:copy.title,exact:true}).click();await expect(view.locator('.comparison-excerpts pre')).toHaveCount(2);
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:refs.map((ref,i)=>({id:ref.document_id,name:i?'Updated policy.txt':'Original policy.txt',extraction_revision:i?'newer':'old'}))}}));
 await page.route('**/v1/documents/structure?*',r=>{const q=new URL(r.request().url()).searchParams;return r.fulfill({json:{document_id:q.get('id'),revision:q.get('revision'),format:'txt',method:'native',units:[],blocks:[],offset:0,total_blocks:0,next_offset:null,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:true}});});
 state.record.plan.sources[1]={...state.record.plan.sources[1],revision:'newer'};state.reply.result.changes[1].after={...state.reply.result.changes[1].after!,revision:'newer'};
 await view.getByRole('button',{name:copy.title,exact:true}).click();await expect.poll(()=>state.calls.filter(c=>c.method==='POST').length).toBe(2);
 expect(state.calls.filter(c=>c.method==='POST')[1].body.sources).toEqual([refs[0],{document_id:'after',revision:'newer'}]);
 await expect(view.locator('.comparison-excerpts pre')).toHaveCount(2);
});
