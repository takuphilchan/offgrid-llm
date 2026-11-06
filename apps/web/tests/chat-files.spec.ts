import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {chatFiles} from '../src/i18n/chat-files';
import { documentRecovery } from '../src/i18n/document-recovery';
import type {LocaleCode} from '../src/i18n';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

const source={id:'file-'+ 'a'.repeat(32),name:'notes — café.txt',media_type:'text/plain',bytes:27,sha256:'b'.repeat(64),revision:'c'.repeat(64),status:'ready',created_at:'2026-11-01T00:00:00Z'};
async function fixture(page:Page,locale:LocaleCode='en',failure=false){
 await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1','session-images-v1']}}));
 let uploads=0;
 await page.route('**/v1/sessions/*/attachments',r=>{
  if(r.request().method()==='GET')return r.fulfill({json:{attachments:[source]}});
  uploads++;expect(r.request().headers()['content-type']).toContain('multipart/form-data');
  return r.fulfill({status:failure?422:200,json:failure?{attachment:{...source,status:'failed',revision:undefined,error_code:'document_ocr_required'},error:{code:'document_ocr_required',message:'This file needs OCR.'}}:{attachment:source}});
 });
 await page.addInitScript(()=>{const prior=window.fetch;window.fetch=async(input,init)=>{if(!String(input).endsWith('/generate'))return prior(input,init);(window as any).filePayload=JSON.parse(String(init?.body));return new Response(JSON.stringify({error:{code:'context_limit',message:'Choose a smaller file.'}}),{status:422,headers:{'Content-Type':'application/json'}});};});
 await page.goto('/ui/#/chat');await expect(page.locator('.composer textarea')).toBeEnabled();
 return {uploads:()=>uploads};
}
const file={name:source.name,mimeType:'text/plain',buffer:Buffer.from('A café in 日本語. Code 12345.')};

test('adding and removing a document preserves image and text drafts across reload',async({page})=>{
 const state=await fixture(page);
 await page.locator('.composer textarea').fill('Compare this picture with the document');
 const dataURL=await page.evaluate(()=>{const canvas=document.createElement('canvas');canvas.width=4;canvas.height=4;return canvas.toDataURL('image/png');});
 await page.locator('.composer input[accept="image/png,image/jpeg"]').setInputFiles({name:'picture.png',mimeType:'image/png',buffer:Buffer.from(dataURL.split(',')[1],'base64')});
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 await expect(page.locator('.composer .file-attachments')).toContainText(source.name);
 await page.reload();
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 await expect(page.locator('.composer .file-attachments')).toContainText(source.name);
 await page.getByRole('button',{name:`${chatFiles('en').remove}: ${source.name}`,exact:true}).click();
 await expect(page.locator('.composer .file-attachments')).toHaveCount(0);
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 await expect(page.locator('.composer textarea')).toHaveValue('Compare this picture with the document');
 expect(state.uploads()).toBe(1);
 expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
});

test('real service privately prepares PDF and Office attachments and retains originals after draft removal',async({page,request})=>{
 test.skip(!process.env.OFFGRID_E2E_URL,'Requires the disposable real service');
 test.setTimeout(60_000);
 const created=await request.post('/v1/sessions',{data:{name:'Demo briefing',model_id:'fixture-model'}});expect(created.ok()).toBeTruthy();
 await fixture(page);
 await page.route('**/v1/sessions/*/attachments',r=>r.continue());
 await page.locator('.composer textarea').fill('What do these documents say?');
 for(const name of ['compressed.pdf','paragraphs.docx']){
  const bytes=readFileSync(resolve('../../internal/rag/testdata/documents/corpus',name));
  const prepared=page.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith('/attachments'),{timeout:35_000});
  await page.locator('input[data-attachment=files]').setInputFiles({name,mimeType:'application/octet-stream',buffer:bytes});
  const response=await prepared;
  expect(response.ok(),await response.text()).toBeTruthy();
  await expect(page.locator('.file-attachments')).toContainText(name);
  await expect(page.locator('.file-attachments li').filter({hasText:name})).toContainText(chatFiles('en').ready);
  const inventory=await request.get('/v1/sessions/Demo%20briefing/attachments');expect(inventory.ok()).toBeTruthy();
  const item=(await inventory.json()).attachments.find((entry:{name:string})=>entry.name===name);expect(item.status).toBe('ready');
  const original=await request.get(`/v1/sessions/Demo%20briefing/attachments/${item.id}/original`);expect(original.ok()).toBeTruthy();expect(await original.body()).toEqual(bytes);
 }
 await page.reload();await expect(page.locator('.file-attachments')).toContainText('compressed.pdf');await expect(page.locator('.file-attachments')).toContainText('paragraphs.docx');
 await page.getByRole('button',{name:`${chatFiles('en').remove}: compressed.pdf`,exact:true}).click();
 await expect(page.locator('.file-attachments')).not.toContainText('compressed.pdf');
 expect((await (await request.get('/v1/sessions/Demo%20briefing/attachments')).json()).attachments).toHaveLength(2);
 await expect(page.locator('.composer textarea')).toHaveValue('What do these documents say?');
});

test('dropping a document prepares a private attachment and retains the question',async({page})=>{
 const state=await fixture(page);await page.locator('.composer textarea').fill('Read this document');
 const transfer=await page.evaluateHandle(()=>{const value=new DataTransfer();value.items.add(new File(['Draft content'],'dropped.txt',{type:'text/plain'}));return value;});
 await page.locator('.composer').dispatchEvent('drop',{dataTransfer:transfer});await transfer.dispose();
 await expect(page.locator('.file-attachments')).toContainText(source.name);
 expect(state.uploads()).toBe(1);await expect(page.locator('.composer textarea')).toHaveValue('Read this document');
 expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
});
test('a file-only reply saves references and clears only the submitted draft',async({page})=>{
 await fixture(page);await page.locator('input[data-attachment=files]').setInputFiles(file);
 await expect(page.locator('.file-attachments')).toContainText('Ready');
 await page.evaluate(()=>{const prior=window.fetch;window.fetch=async(url,init)=>{
  if(!String(url).endsWith('/generate'))return prior(url,init);
  const body=JSON.parse(String(init?.body)),message={role:'assistant',content:'The source says 12345.'},session={name:'Demo briefing',model_id:body.model_id,messages:[{role:'user',content:body.content,attachments:body.attachments},message]};
  (window as any).savedFilesSession=session;
  return new Response(`data: ${JSON.stringify({type:'done',session,message})}\n\n`,{headers:{'Content-Type':'text/event-stream'}});
 };});
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.locator('.message.assistant').last()).toContainText('12345');
 await expect(page.locator('.composer .file-attachments')).toHaveCount(0);
 await expect(page.locator('.message.user .saved-file-attachments')).toContainText(source.name);
 const saved=await page.evaluate(()=>(window as any).savedFilesSession);
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[saved]}}));await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:saved}));
 await page.reload();await expect(page.locator('.message.user .saved-file-attachments')).toContainText(source.name);
 await expect(page.locator('.composer .file-attachments')).toHaveCount(0);
});
test('a saved file supports a follow-up without reuploading or attaching it twice',async({page})=>{
 const state=await fixture(page);
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await expect(page.locator('.composer .file-attachments')).toContainText('Ready');
 await page.evaluate(()=>{
  const prior=window.fetch;
  const flow={requests:[] as any[],messages:[] as any[]};
  (window as any).sameFileFlow=flow;
  window.fetch=async(url,init)=>{
   if(!String(url).endsWith('/generate'))return prior(url,init);
   const body=JSON.parse(String(init?.body));flow.requests.push(body);
   flow.messages.push({role:'user',content:body.content,...(body.attachments?.length?{attachments:body.attachments}:{})});
   const message={role:'assistant',content:flow.requests.length===1?'First source answer.':'Follow-up source answer.'};
   flow.messages.push(message);
   const session={name:'Demo briefing',model_id:body.model_id,messages:[...flow.messages]};
   return new Response(`data: ${JSON.stringify({type:'done',session,message})}\n\n`,{headers:{'Content-Type':'text/event-stream'}});
  };
 });
 await page.locator('.composer textarea').fill('What is the code in this file?');
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.locator('.message.assistant').last()).toContainText('First source answer.');
 await expect(page.locator('.composer .file-attachments')).toHaveCount(0);
 await page.locator('.composer textarea').fill('Follow up on the same file.');
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect(page.locator('.message.assistant').last()).toContainText('Follow-up source answer.');
 const flow=await page.evaluate(()=>(window as any).sameFileFlow);
 expect(flow.requests).toHaveLength(2);
 expect(flow.requests[0].attachments).toEqual([{id:source.id,revision:source.revision}]);
 expect(flow.requests[1].attachments??[]).toEqual([]);
 expect(flow.messages[0].attachments).toEqual([{id:source.id,revision:source.revision}]);
 expect(flow.messages[2].attachments??[]).toEqual([]);
 expect(state.uploads()).toBe(1);
 await expect(page.locator('.message.user .saved-file-attachments')).toHaveCount(1);
});
test('upload into a fresh conversation preserves the current question',async({page})=>{
 const state=await fixture(page);await page.route('**/v1/sessions',r=>r.request().method()==='POST'?r.fulfill({json:{...r.request().postDataJSON(),messages:[]}}):r.fulfill({json:{sessions:[]}}));
 await page.reload();await page.getByRole('button',{name:'New chat',exact:true}).click();await expect(page.locator('.composer textarea')).toBeEnabled();await page.locator('.composer textarea').fill('Read the shipment number');
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await expect(page.locator('.file-attachments')).toContainText('Ready');
 await expect(page.locator('.composer textarea')).toHaveValue('Read the shipment number');expect(state.uploads()).toBe(1);
});
test('files prepare privately, retain failed sends and survive reload without another upload',async({page})=>{
 const state=await fixture(page);await page.locator('.composer textarea').fill('What is the code?');
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await expect(page.locator('.file-attachments')).toContainText('Ready');
 await expect(page.locator('.composer textarea')).toHaveValue('What is the code?');
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).filePayload)).toMatchObject({attachments:[{id:source.id,revision:source.revision}],content:'What is the code?',durable:true});
 await expect(page.locator('.file-attachments')).toContainText(source.name);
 await page.reload();await expect(page.locator('.file-attachments')).toContainText(source.name);
 expect(state.uploads()).toBe(1);expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
 await page.getByRole('button',{name:'Remove from draft: '+source.name,exact:true}).click();
 await expect(page.locator('.file-attachments')).toHaveCount(0);
});
for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as LocaleCode[])test(`failed file keeps localized guidance after reload: ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:390,height:844});const state=await fixture(page,locale,true);
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await page.reload();
 await expect(page.locator('.file-attachments')).toContainText(documentRecovery(locale,'document_ocr_required')!);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 expect(state.uploads()).toBe(1);expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
 if(locale==='en'||locale==='ar')await page.screenshot({path:info.outputPath(`failed-files-${locale}.png`),fullPage:true});
});
test('saved sources can be reselected without repeating an upload or generation',async({page})=>{
 const state=await fixture(page);
 await page.locator('.composer-attachments-menu > button').click();
 await page.getByRole('button',{name:'Saved files',exact:true}).click();
 await expect(page.locator('.saved-file-list')).toContainText(source.name);
 await page.getByRole('button',{name:'Use in next message',exact:true}).click();
 await expect(page.locator('.composer .file-attachments')).toContainText(source.name);
 expect(state.uploads()).toBe(0);expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
 await page.getByRole('button',{name:'Remove from draft: '+source.name,exact:true}).click();
 await page.locator('.composer-attachments-menu > button').click();
 await expect(page.getByRole('button',{name:'Use in next message',exact:true})).toBeEnabled();
 await page.keyboard.press('Escape');
 await expect(page.locator('.composer textarea')).toBeInViewport();
 await page.screenshot({path:'../../build/conversation-files-recovery-ui.png',fullPage:true});
});

test('retry prepares the retained original without uploading it again',async({page})=>{
 const state=await fixture(page,'en',true);let retries=0;
 await page.route('**/attachments/*/prepare',r=>{retries++;return r.fulfill({json:{attachment:source}});});
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await page.getByRole('button',{name:'Retry preparation'}).click();
 await expect(page.locator('.composer .file-attachments')).toContainText('Ready');
 expect(retries).toBe(1);expect(state.uploads()).toBe(1);expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
});
test('failed extraction is retained with recovery and cannot be silently sent',async({page})=>{
 const state=await fixture(page,'en',true);await page.locator('.composer textarea').fill('Read it');
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 await expect(page.locator('.file-attachments')).toContainText(documentRecovery('en','document_ocr_required')!);
 await expect(page.getByRole('button',{name:'Retry preparation'})).toBeVisible();
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
 await page.reload();await expect(page.locator('.file-attachments')).toContainText(source.name);
 await expect(page.locator('.file-attachments')).toContainText(documentRecovery('en','document_ocr_required')!);
 expect(state.uploads()).toBe(1);expect(await page.evaluate(()=>(window as any).filePayload)).toBeUndefined();
});
for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as LocaleCode[])test(`file chips fit and remove by keyboard: ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:320,height:800});await fixture(page,locale);const copy=chatFiles(locale);
 await page.locator('.composer textarea').fill('Keep this question');
 await page.locator('.composer-attachments-menu > button').click();await page.getByRole('button',{name:copy.add,exact:true}).focus();await page.keyboard.press('Escape');
 await page.locator('input[data-attachment=files]').setInputFiles(file);
 const remove=page.getByRole('button',{name:copy.remove+': '+source.name,exact:true});await expect(remove).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
 if(locale==='en'||locale==='ar')await page.screenshot({path:info.outputPath(`files-${locale}.png`),fullPage:true});
 await remove.focus();await remove.press('Enter');await expect(page.locator('.file-attachments')).toHaveCount(0);
 await expect(page.locator('.composer-attachments-menu > button')).toBeFocused();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this question');
});


test('attachment actions form a compact aligned menu in light, dark and RTL layouts', async ({page}, info) => {
  await fixture(page);
  for (const [locale,theme,width] of [['en','light',1280],['ar','dark',390]] as const) {
    await page.setViewportSize({width:1280,height:800});
    await page.locator('.workspace-options > button').click();
    await page.locator('.workspace-options-panel select').nth(0).selectOption(locale);
    await page.locator('.workspace-options-panel select').nth(1).selectOption(theme);
    await page.keyboard.press('Escape');
    await expect(page.locator('html')).toHaveAttribute('lang',locale);
    await expect(page.locator('html')).toHaveAttribute('data-theme',theme);
    await page.setViewportSize({width,height:800});
    const trigger=page.locator('.composer-attachments-menu > button');
    await trigger.click();
    const panel=page.locator('.composer-attachments-menu > .utility-popover-panel');
    await expect(panel).toBeInViewport();
    const bounds=await panel.boundingBox();
    expect(bounds!.width).toBeLessThanOrEqual(240);
    const actions=panel.locator('.utility-popover-content > button, .saved-file-picker > button');
    await expect(actions).toHaveCount(3);
    for (const button of await actions.all()) {
      await expect(button.locator('svg')).toHaveCount(1);
      const box=await button.boundingBox();
      expect(box!.width).toBeGreaterThan(bounds!.width-25);
    }
    await page.screenshot({path:info.outputPath('attachment-menu-'+locale+'-'+theme+'.png')});
    await page.keyboard.press('Escape');
    await expect(panel).not.toBeVisible();
    await expect(trigger).toBeFocused();
  }
});
