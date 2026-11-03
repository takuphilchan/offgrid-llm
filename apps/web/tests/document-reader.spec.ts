import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { documentReader } from '../src/i18n/document-reader';
import type { LocaleCode } from '../src/i18n';
import { readFileSync,writeFileSync } from 'node:fs';
import { resolve,basename } from 'node:path';
import { createHash } from 'node:crypto';

const file = {id:'file-'+'a'.repeat(32),name:'Research paper.pdf',sha256:'b'.repeat(64),revision:'c'.repeat(64),media_type:'application/pdf',bytes:100,status:'failed',error_code:'document_ocr_language_missing',created_at:'2026-11-03T00:00:00Z'};
async function setup(page:Page,locale:LocaleCode='en',theme:'light'|'dark'='light', attachment=file) {
 const file=attachment;
 await craftFixture(page,locale,theme);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1']}}));
 await page.route('**/v1/sessions/*/attachments',r=>r.fulfill({status:r.request().method()==='POST'&&file.status!=='ready'?422:200,json:r.request().method()==='POST'?{attachment:file,...(file.status!=='ready'?{error:{code:file.error_code,message:'OCR unavailable'}}:{})}:{attachments:[file]}}));
 await page.route('**/attachments/*/preview*',async r=>{
  const u=new URL(r.request().url());
  if(u.searchParams.has('page'))return r.fulfill({contentType:'image/png',body:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j5ZkAAAAASUVORK5CYII=','base64')});
  return r.fulfill({json:{name:file.name,sha256:file.sha256,format:'pdf',pages:2,state:'partial',error_code:'document_ocr_required',units:[{kind:'page',index:1,text:'Native café 日本語 abstract',state:'ready',source_sha256:file.sha256,extractor_revision:'fixture'},{kind:'page',index:2,text:'The result is 42. <script>untrusted</script>',state:'ocr_required',source_sha256:file.sha256,extractor_revision:'fixture'}]}});
 });
 await page.goto('/ui/#/chat');await expect(page.locator('.composer textarea')).toBeEnabled();
 await page.locator('.composer textarea').fill('Explain this paper');
 await page.locator('input[data-attachment=files]').setInputFiles({name:file.name,mimeType:'application/pdf',buffer:Buffer.from('%PDF-fixture')});
 await expect(page.locator('.file-attachments')).toContainText(file.name);
}

for(const width of [390,1280]) test(`ready attachment never collapses into a vertical strip: ${width}`,async({page})=>{
 await page.setViewportSize({width,height:1000});
 await setup(page,'en','dark',{...file,status:'ready',error_code:'',coverage:'native_text_only'} as typeof file);
 if(width>1000)await page.getByRole('button',{name:'Open document',exact:true}).click();
 const card=page.locator('.file-attachments li'), summary=card.locator('.file-summary');
 await expect(card).toContainText('Images and figures have not been interpreted');
 const bounds=await summary.boundingBox();expect(bounds!.width).toBeGreaterThan(200);expect(bounds!.height).toBeLessThan(75);
 expect((await card.boundingBox())!.height).toBeLessThan(240);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.screenshot({path:test.info().outputPath(`attachment-${width}.png`)});
});

test('active attachment announcement stays accessible without floating beside the file',async({page})=>{
 await page.setViewportSize({width:700,height:1000});
 await setup(page,'en','dark',{...file,status:'ready',error_code:'',coverage:'native_text_only'} as typeof file);
 await page.locator('.composer-actions').getByRole('button',{name:'Send',exact:true}).click();
 const attachment=page.locator('.saved-file-attachments');
 const announcement=attachment.getByRole('status');
 await expect(announcement).toHaveText(documentReader('en').attached);
 await expect(announcement).toHaveCSS('clip-path','inset(50%)');
 const bounds=await announcement.boundingBox();
 expect(bounds!.width).toBeLessThanOrEqual(1);expect(bounds!.height).toBeLessThanOrEqual(1);
 await expect(attachment.getByRole('button',{name:file.name})).toBeVisible();
 await expect(attachment).toContainText('Images and figures have not been interpreted');
 await page.screenshot({path:test.info().outputPath('active-document-attachment.png')});
});

test('context recovery opens response settings without another submission',async({page})=>{
 await setup(page,'en','dark',{...file,status:'ready',error_code:''});
 await page.evaluate(()=>{
  const original=window.fetch.bind(window);
  window.fetch=async(input,init)=>String(input).endsWith('/generate')
   ?new Response(JSON.stringify({error:{code:'attachment_context_limit',message:'Not enough room for source excerpts. Your files and draft are retained.'}}),{status:422,headers:{'Content-Type':'application/json'}})
   :original(input,init);
 });
 await page.locator('.composer-actions').getByRole('button',{name:'Send',exact:true}).click();
 const notice=page.locator('.chat-recovery');await expect(notice).toContainText('Not enough room');
 await expect(page.getByRole('heading',{name:'What would you like to work on?'})).toHaveCount(0);
 await notice.getByRole('button',{name:'Response settings',exact:true}).click();
 await expect(page.getByRole('region',{name:'Response settings',exact:true})).toBeVisible();
 await page.locator('.response-advanced summary').click();
 await expect(page.locator('.response-advanced summary')).toHaveText(/Advanced.*Interactive/);
 await expect(page.locator('.response-advanced summary')).not.toContainText('1,024');
 await expect(page.locator('.response-advanced')).toContainText('This limits one answer, not the conversation.');
 await page.getByRole('combobox',{name:'Maximum response tokens',exact:true}).selectOption('4096');
 await expect(page.getByRole('combobox',{name:'Maximum response tokens',exact:true})).toHaveValue('4096');
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
});

test('saved document and excerpt coverage remain separate readable rows after reload',async({page})=>{
 await page.setViewportSize({width:700,height:1000});
 await setup(page,'en','dark',{...file,status:'ready',error_code:'',coverage:'native_text_only'} as typeof file);
 const savedSession={name:'Demo briefing',model_id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',messages:[
  {role:'user',content:'What is this?',attachments:[{id:file.id,revision:file.revision}]},
  {role:'assistant',content:'A research paper.',attachment_selections:[{id:file.id,revision:file.revision,name:file.name,mode:'excerpts',total_bytes:170000,ranges:[[0,1000]]}]}
 ]};
 await page.route('**/v1/sessions/Demo%20briefing',route=>route.fulfill({json:savedSession}));
 await page.route('**/v1/sessions',route=>route.fulfill({json:{sessions:[savedSession]}}));
 await page.reload();
 await expect(page.locator('.document-answer-coverage')).toContainText('not a complete review');
 const saved=page.locator('.saved-file-attachments');await expect(saved).toContainText('Images and figures have not been interpreted');
 const button=await saved.locator('button').boundingBox(),note=await saved.locator('small').boundingBox();
 expect(note!.y).toBeGreaterThanOrEqual(button!.y+button!.height);
 expect(note!.width).toBeGreaterThan(200);expect(note!.height).toBeLessThan(100);
});

test('partial PDF stays readable beside chat; search, zoom and closing preserve the draft',async({page})=>{
 await page.setViewportSize({width:1600,height:1000});await setup(page);
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.getByRole('complementary',{name:'Document',exact:true});
 await expect(reader).toBeVisible();await expect(reader.locator('img')).toBeVisible();
 await expect(reader).toContainText(documentReader('en').partial);
 await expect(page.locator('.composer textarea')).toBeVisible();
 await page.screenshot({path:test.info().outputPath('reader-wide.png')});
 await reader.getByRole('searchbox').fill('42');await reader.getByRole('button',{name:'2',exact:true}).click();
 await expect(reader.locator('mark')).toHaveText('42');await expect(reader.locator('pre')).toContainText('<script>untrusted</script>');
 await reader.getByLabel('Zoom',{exact:true}).selectOption('150');
 await expect(reader.locator('pre')).toHaveCSS('font-size','24px');
 await reader.getByRole('button',{name:'Close document'}).click();
 await expect(reader).toHaveCount(0);await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await expect(page.getByRole('button',{name:'Open document',exact:true})).toBeFocused();
});

test('text-only acceptance is explicit and keeps unread coverage',async({page})=>{
 await page.setViewportSize({width:390,height:844});await setup(page);let prepares=0;
 await page.route('**/attachments/*/prepare*',r=>{prepares++;expect(new URL(r.request().url()).searchParams.get('text_only')).toBe('true');return r.fulfill({json:{attachment:{...file,status:'ready',error_code:undefined,coverage:'native_text_only'}}});});
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(page.locator('.document-reader')).toBeVisible();expect(prepares).toBe(0);
 await page.getByRole('tab',{name:'Chat',exact:true}).click();
 await page.getByRole('button',{name:'Use available text only',exact:true}).click();
 await expect(page.locator('.file-attachments')).toContainText('Images and figures have not been interpreted');
 expect(prepares).toBe(1);await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
});

test('page arrows and page keys navigate without intercepting text input',async({page})=>{
 await page.setViewportSize({width:1600,height:1000});await setup(page);
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader'), copy=documentReader('en');
 const number=reader.locator('input[type=number]');
 await expect(reader.locator('img')).toBeVisible();
 await expect(reader.getByRole('button',{name:copy.previous,exact:true})).toBeDisabled();
 await reader.locator('.document-reader-surface').focus();await page.keyboard.press('PageDown');
 await expect(number).toHaveValue('2');
 await expect(reader.getByRole('button',{name:copy.next,exact:true})).toBeDisabled();
 await reader.getByRole('searchbox').focus();await page.keyboard.press('PageUp');
 await expect(number).toHaveValue('2');
 await reader.getByRole('button',{name:copy.previous,exact:true}).click();await expect(number).toHaveValue('1');
 await reader.getByRole('button',{name:copy.next,exact:true}).click();await expect(number).toHaveValue('2');
 await reader.locator('.document-reader-surface').focus();await page.keyboard.press('PageUp');await expect(number).toHaveValue('1');
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
});

test('failed page rendering can retry the same page without preparing or sending the document',async({page})=>{
 await setup(page);let attempts=0;
 await page.route('**/attachments/*/preview*',async route=>{
  if(!new URL(route.request().url()).searchParams.has('page'))return route.fallback();
  attempts++;
  if(attempts===1)return route.fulfill({status:503,json:{error:{message:'Page rendering unavailable'}}});
  return route.fallback();
 });
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader');
 await expect(reader.getByRole('alert')).toBeVisible();
 await reader.locator('.document-page-error button').click();
 await expect(reader.locator('img')).toBeVisible();await expect(reader.getByRole('alert')).toHaveCount(0);
 expect(attempts).toBe(2);await expect(reader.locator('input[type=number]')).toHaveValue('1');
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as const) test(`reader controls and keyboard work: ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:844});await setup(page,locale,'dark');
 const copy=documentReader(locale);await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(page.getByRole('button',{name:copy.close,exact:true})).toBeFocused();
 await expect(page.getByRole('searchbox',{name:copy.search})).toBeVisible();
 await expect(page.locator('.document-reader')).toContainText(copy.partial);
 await page.keyboard.press('Escape');await expect(page.locator('.document-reader')).toHaveCount(0);
 await expect(page.getByRole('button',{name:copy.open,exact:true})).toBeFocused();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
});

test('real service previews mixed PDF and only accepts partial text explicitly',async({request,page})=>{
 test.skip(!process.env.OFFGRID_E2E_URL,'Requires disposable service and packaged worker');test.setTimeout(90_000);
 const name='reader-source-fixture';expect((await request.post('/v1/sessions',{data:{name,model_id:'fixture-model'}})).ok()).toBeTruthy();
 const data=readFileSync(resolve('../../internal/rag/testdata/documents/corpus/mixed.pdf'));
 const response=await request.post(`/v1/sessions/${name}/attachments`,{multipart:{file:{name:'mixed.pdf',mimeType:'application/pdf',buffer:data}}});
 expect(response.status()).toBe(422);const item=(await response.json()).attachment;expect(item.status).toBe('failed');
 const base=`/v1/sessions/${name}/attachments/${item.id}`;
 const preview=await request.get(`${base}/preview?sha256=${item.sha256}`);expect(preview.ok(),await preview.text()).toBeTruthy();
 const source=await preview.json();expect(source.state).toBe('partial');expect(source.units.some((unit:{text:string})=>unit.text.length>0)).toBeTruthy();
 await page.setViewportSize({width:1600,height:1000});await setup(page,'en','light',item);
 await page.route('**/attachments/*/preview*',async route=>{
  const query=new URL(route.request().url()).search;
  const response=await request.get(`${base}/preview${query}`);
  await route.fulfill({response});
 });
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(page.locator('.document-reader img')).toBeVisible();
 await expect(page.locator('.document-reader img')).toHaveJSProperty('naturalWidth',1700);
 await page.screenshot({path:test.info().outputPath('real-pdf-reader.png')});
 const raster=await request.get(`${base}/preview?sha256=${item.sha256}&page=1`);expect(raster.ok(),await raster.text()).toBeTruthy();expect(raster.headers()['content-type']).toBe('image/png');expect((await raster.body()).length).toBeGreaterThan(100);
 expect((await request.get(`${base}/preview?sha256=${'0'.repeat(64)}`)).status()).toBe(409);
 expect((await request.get(`/v1/sessions/${name}/attachments`)).ok()).toBeTruthy();
 const accepted=await request.post(`${base}/prepare?text_only=true`);expect(accepted.ok(),await accepted.text()).toBeTruthy();const ready=(await accepted.json()).attachment;expect(ready.coverage).toBe('native_text_only');expect(ready.status).toBe('ready');
 const session=await request.get(`/v1/sessions/${name}`);expect((await session.json()).messages).toHaveLength(0);
});

test('narrow reader switches views and reduced motion removes scanning animation',async({page})=>{
 await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});await setup(page);
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(page.locator('.document-reader')).toBeVisible();await expect(page.locator('.composer textarea')).toBeHidden();
 await page.getByRole('tab',{name:'Chat',exact:true}).click();await expect(page.locator('.composer textarea')).toBeVisible();
 await page.getByRole('tab',{name:'Document',exact:true}).click();await expect(page.locator('.document-reader')).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.screenshot({path:test.info().outputPath('reader-narrow.png')});
});

for(const reduced of [false,true]) test(`preparation animation follows pending work and closes safely: reduced=${reduced}`,async({page})=>{
 await page.emulateMedia({reducedMotion:reduced?'reduce':'no-preference'});await setup(page);
 let finish!:()=>void;const pending=new Promise<void>(resolve=>{finish=resolve;});
 await page.route('**/attachments/*/preview*',async route=>{await pending;await route.fulfill({status:404,json:{error:{message:'Source removed'}}}).catch(()=>{});});
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const scan=page.locator('.document-reader .document-scan');await expect(scan).toBeVisible();
 expect(await scan.evaluate(element=>getComputedStyle(element,'::after').animationName)).toBe(reduced?'none':'document-border-orbit');
 if(!reduced){const angle=await scan.evaluate(element=>getComputedStyle(element,'::after').getPropertyValue('--document-border-angle'));await expect.poll(()=>scan.evaluate(element=>getComputedStyle(element,'::after').getPropertyValue('--document-border-angle'))).not.toBe(angle);}
 await page.getByRole('button',{name:'Close document',exact:true}).click();finish();
 await expect(page.locator('.document-reader')).toHaveCount(0);await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await expect(page.getByRole('alert')).toHaveCount(0);
});

test('structured previews label layout limits and never request PDF raster',async({page})=>{
 await page.setViewportSize({width:900,height:700});await setup(page);
 await page.route('**/attachments/*/preview*',route=>{
  expect(new URL(route.request().url()).searchParams.has('page')).toBeFalsy();
  return route.fulfill({json:{name:file.name,sha256:file.sha256,format:'xlsx',pages:0,state:'complete',units:[{kind:'sheet',index:1,name:'Budget',text:'A1: café 日本語\nB2: 42',state:'ready',source_sha256:file.sha256}]}});
 });
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(page.locator('.document-reader')).toContainText(documentReader('en').structured);
 await expect(page.locator('.document-reader pre')).toContainText('café 日本語');
 await expect(page.locator('.document-reader img')).toHaveCount(0);
});

test('stale preview identity is rejected without discarding the draft',async({page})=>{
 await setup(page);await page.route('**/attachments/*/preview*',route=>route.fulfill({json:{name:file.name,sha256:'0'.repeat(64),units:[],state:'complete',format:'pdf',pages:1}}));
 await page.getByRole('button',{name:'Open document',exact:true}).click();await expect(page.locator('.document-reader [role=alert]')).toBeVisible();
 await expect(page.locator('.document-reader img')).toHaveCount(0);await page.getByRole('button',{name:'Close document',exact:true}).click();
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
});

test('changing conversations closes the previous source instead of carrying it over',async({page})=>{
 await setup(page);await page.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(page.locator('.document-reader img')).toBeVisible();
 await page.getByRole('button',{name:'New chat',exact:true}).click();
 await expect(page.locator('.document-reader')).toHaveCount(0);
 await expect(page.locator('.composer textarea')).toHaveValue('');
});

test('200 percent layout retains view controls, reading position and draft',async({page})=>{
 // Equivalent CSS viewport reduction to browser zoom, plus enlarged type.
 await page.setViewportSize({width:800,height:500});await setup(page,'ar','dark');
 await page.addStyleTag({content:'html {font-size:200%}'});
 const copy=documentReader('ar');await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(page.locator('.document-reader img')).toBeVisible();
 await page.getByRole('button',{name:copy.next,exact:true}).click();
 const tabs=page.locator('.document-view-tabs [role=tab]');await tabs.nth(0).click();await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await tabs.nth(1).click();await expect(page.locator('.document-page-tools input')).toHaveValue('2');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.screenshot({path:test.info().outputPath('reader-rtl-200.png')});
});

test('private supplied PDF is text-ready without OCR while retaining visual limits',async({request,page})=>{
 test.skip(!process.env.OFFGRID_E2E_URL||!process.env.OFFGRID_TEST_DOCUMENT_SAMPLE,'Explicit local sample and disposable service required');test.setTimeout(180_000);
 const bytes=readFileSync(process.env.OFFGRID_TEST_DOCUMENT_SAMPLE!),sha=createHash('sha256').update(bytes).digest('hex'),name='private-paper-check';
 const language=process.env.OFFGRID_TEST_OCR_LANGUAGE;
 if(language){const imported=await request.post('/v1/documents/ocr-languages?language=eng',{headers:{'Content-Type':'application/octet-stream'},data:readFileSync(language)});expect(imported.ok()).toBeTruthy();}
 expect((await request.post('/v1/sessions',{data:{name,model_id:'fixture-model'}})).ok()).toBeTruthy();
 const uploaded=await request.post(`/v1/sessions/${name}/attachments`,{multipart:{file:{name:basename(process.env.OFFGRID_TEST_DOCUMENT_SAMPLE!),mimeType:'application/pdf',buffer:bytes}},timeout:120_000});
 expect(uploaded.status()).toBe(200);const attachment=(await uploaded.json()).attachment;
 expect(attachment.status).toBe('ready');expect(attachment.coverage).toBe('native_text_only');expect(attachment.error_code).toBeFalsy();
 const base=`/v1/sessions/${name}/attachments/${attachment.id}`;
 const response=await request.get(`${base}/preview?sha256=${sha}`,{timeout:120_000});expect(response.ok()).toBeTruthy();
 const result=await response.json();expect(result.sha256).toBe(sha);expect(result.state).toBe('partial');expect(result.units.some((unit:{text:string})=>unit.text.length>0)).toBeTruthy();
 const accepted=await request.post(`${base}/prepare`,{timeout:120_000});expect(accepted.ok()).toBeTruthy();expect((await accepted.json()).attachment.revision).toBe(attachment.revision);
 await page.setViewportSize({width:1600,height:1000});await setup(page,'en','light',attachment);
 await page.route('**/attachments/*/preview*',async route=>{const response=await request.get(`${base}/preview${new URL(route.request().url()).search}`,{timeout:120_000});await route.fulfill({response});});
 await page.getByRole('button',{name:'Open document',exact:true}).click();await expect(page.locator('.document-reader img')).toBeVisible({timeout:60_000});
 await expect(page.locator('.file-attachments')).toContainText('Images and figures have not been interpreted');
 await expect(page.locator('.file-attachments').getByRole('button',{name:'Retry preparation',exact:true})).toHaveCount(0);
 await expect(page.locator('.document-coverage-pages button')).toHaveCount(6);
 await page.locator('.document-coverage-pages button').last().click();await expect(page.locator('.document-reader input[type=number]')).toHaveValue('37');
 // Page 13 in the reported paper contains native glyphs whose OCR boxes are
 // invalid. Original-page viewing must not depend on those OCR coordinates.
 if(result.pages>=13){
  await page.locator('.document-reader input[type=number]').fill('13');
  await expect(page.locator('.document-reader img')).toHaveAttribute('alt',/13$/,{timeout:60_000});
  await expect(page.locator('.document-reader img')).toHaveJSProperty('naturalWidth',1700);
  await expect(page.locator('.document-reader [role=alert]')).toHaveCount(0);
 }
 const destination=process.env.OFFGRID_TEST_DOCUMENT_SAMPLE_OUTPUT;
 if(destination){await page.screenshot({path:resolve(destination,'reader.png')});writeFileSync(resolve(destination,'receipt.json'),JSON.stringify({sha256:sha,pages:result.pages,characters:result.units.reduce((n:number,u:{text:string})=>n+[...u.text].length,0),review_pages:result.units.filter((u:{state:string})=>u.state==='native_text_with_visuals').map((u:{index:number})=>u.index),coverage:'native_text_only'},null,2));}
 expect((await (await request.get(`/v1/sessions/${name}`)).json()).messages).toHaveLength(0);
});
