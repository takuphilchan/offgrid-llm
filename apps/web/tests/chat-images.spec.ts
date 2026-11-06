import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {chatImages} from '../src/i18n/chat-images';
import type {LocaleCode} from '../src/i18n';

async function fixture(page:Page,{locale='en',vision=true,capable=true}:{locale?:LocaleCode;vision?:boolean;capable?:boolean}={}){
 const state=await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:capable?['session-images-v1','agent-profiles-v1']:[]}}));
 await page.route('**/v1/models',r=>r.fulfill({json:{data:[{id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',type:'chat',capability_status:vision?{vision:'unknown'}:{}}]}}));
 await page.goto('/ui/#/chat');await expect(page.locator('.composer textarea')).toBeEnabled();
 const dataURL=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=4;c.height=4;const ctx=c.getContext('2d')!;ctx.fillStyle='red';ctx.fillRect(0,0,4,4);return c.toDataURL('image/png');});
 const file={name:'red — café.png',mimeType:'image/png',buffer:Buffer.from(dataURL.split(',')[1],'base64')};
 return {state,file,dataURL};
}

for(const mode of ['upload','paste','drop'] as const)test(`image ${mode} preserves text, previews exact bytes and removes without submitting`,async({page})=>{
 const {state,file,dataURL}=await fixture(page);await page.locator('.composer textarea').fill('Keep this text');
 if(mode==='upload')await page.locator('.composer input[type=file]').setInputFiles(file);
 else await page.locator('.composer').evaluate((node,{mode,dataURL,name})=>{const bytes=Uint8Array.from(atob(dataURL.split(',')[1]),c=>c.charCodeAt(0));const data=new DataTransfer();data.items.add(new File([bytes],name,{type:'image/png'}));node.dispatchEvent(mode==='paste'?new ClipboardEvent('paste',{clipboardData:data,bubbles:true,cancelable:true}):new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true}));},{mode,dataURL,name:file.name});
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this text');
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled();
 await page.getByRole('button',{name:'Remove image: '+file.name,exact:true}).click();
 await expect(page.locator('.composer .image-attachments')).toHaveCount(0);expect(state.writes).toEqual([]);
});

test('image draft survives reload and never crosses a workspace',async({page})=>{
 const {file,dataURL}=await fixture(page);await page.locator('.composer input[type=file]').setInputFiles(file);
 await expect(page.locator('.composer .image-attachments img')).toBeVisible();await page.reload();
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'other',capabilities:['session-images-v1']}}));await page.reload();
 await expect(page.locator('.composer textarea')).toBeEnabled();await expect(page.locator('.composer .image-attachments')).toHaveCount(0);
});

test('JPEG preview preserves its original bytes and text drops remain editable',async({page})=>{
 await fixture(page);
 const dataURL=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=4;c.height=4;return c.toDataURL('image/jpeg');});
 await page.locator('.composer input[type=file]').setInputFiles({name:'photo.jpg',mimeType:'image/jpeg',buffer:Buffer.from(dataURL.split(',')[1],'base64')});
 await expect(page.locator('.composer .image-attachments img')).toHaveAttribute('src',dataURL);
 const prevented=await page.locator('.composer').evaluate(node=>{const data=new DataTransfer();data.setData('text/plain','Keep text editing');const event=new DragEvent('drop',{dataTransfer:data,bubbles:true,cancelable:true});node.dispatchEvent(event);return event.defaultPrevented;});
 expect(prevented).toBe(false);
});

for(const restriction of ['model','service'] as const)test(`unsupported ${restriction} retains images and blocks submission`,async({page})=>{
 const {file,state}=await fixture(page,{vision:restriction!=='model',capable:restriction!=='service'});
 await page.locator('.composer input[type=file]').setInputFiles(file);await expect(page.locator('.composer .image-attachments img')).toBeVisible();
 await expect(page.locator('.composer')).toContainText(chatImages('en')[restriction]);await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();expect(state.writes).toEqual([]);
});

test('invalid, oversized and excess files preserve existing attachments',async({page})=>{
 const {file}=await fixture(page);const input=page.locator('.composer input[type=file]');await input.setInputFiles(file);
 await expect(page.locator('.composer .image-attachments img')).toHaveCount(1);
 for(const files of [[{name:'bad.png',mimeType:'image/png',buffer:Buffer.from('<svg/>')}],[{...file,buffer:Buffer.alloc(2*1024*1024+1)}],[file,file,file,file]]){
  await input.setInputFiles(files);await expect(page.locator('.image-input-notice[role=alert]')).toContainText(chatImages('en').limits);await expect(page.locator('.composer .image-attachments img')).toHaveCount(1);
 }
});

test('storage failure is visible and leaves the preview in memory',async({page})=>{
 const {file}=await fixture(page);await page.evaluate(()=>{const original=Storage.prototype.setItem;Storage.prototype.setItem=function(key,value){if(key.includes(':chat-images:'))throw new DOMException('Full','QuotaExceededError');original.call(this,key,value);};});
 await page.locator('.composer input[type=file]').setInputFiles(file);await expect(page.locator('.composer .image-attachments img')).toBeVisible();await expect(page.locator('.composer')).toContainText(chatImages('en').unsaved);
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as LocaleCode[])test(`images fit the narrow composer with keyboard removal: ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:320,height:800});const {file}=await fixture(page,{locale});const copy=chatImages(locale);
 await page.locator('.composer-attachments-menu > button').click();await expect(page.getByRole('button',{name:copy.add,exact:true})).toBeVisible();await page.keyboard.press('Escape');await page.locator('.composer input[type=file]').setInputFiles(file);
 const remove=page.getByRole('button',{name:copy.remove+': '+file.name,exact:true});await expect(remove).toBeVisible();await remove.focus();await expect(remove).toBeFocused();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`images-${locale}.png`),fullPage:true});
 await remove.press('Enter');await expect(page.locator('.composer .image-attachments')).toHaveCount(0);await expect(page.locator('.composer-attachments-menu > button')).toBeFocused();
});

for(const fail of [false,true])test(`image-only send ${fail?'retains failed input':'saves exact bytes and reloads without replay'}`,async({page})=>{
 const {file,dataURL}=await fixture(page);await page.locator('.composer input[type=file]').setInputFiles(file);await expect(page.locator('.composer .image-attachments img')).toBeVisible();
 await page.evaluate(fail=>{const prior=window.fetch;window.fetch=async(url,init)=>{if(!String(url).endsWith('/generate'))return prior(url,init);const body=JSON.parse(String(init?.body));(window as any).imagePayload=body;
  if(fail)return new Response(JSON.stringify({error:{message:'Projector is unavailable',code:'vision_projector_unavailable'}}),{status:422,headers:{'Content-Type':'application/json'}});
  const message={role:'assistant',content:'A red square.'},session={name:'Demo briefing',model_id:body.model_id,messages:[{role:'user',content:body.content,images:body.images},message]};(window as any).imageSession=session;
  return new Response(`data: ${JSON.stringify({type:'done',session,message})}\n\n`,{headers:{'Content-Type':'text/event-stream'}});
 };},fail);
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).imagePayload)).toMatchObject({content:'',images:[{name:file.name,data_url:dataURL}]});
 if(fail){await expect(page.locator('.composer .image-attachments img')).toBeVisible();await expect(page.getByRole('alert')).toContainText('Projector is unavailable');await expect(page.getByRole('alert').getByRole('button',{name:'Models',exact:true})).toBeVisible();await expect(page.getByRole('alert').getByRole('button',{name:'Check request status'})).toHaveCount(0);}
 else{await expect(page.locator('.message.assistant')).toContainText('A red square.');await expect(page.locator('.composer .image-attachments')).toHaveCount(0);const saved=await page.evaluate(()=>(window as any).imageSession);await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[saved]}}));await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:saved}));await page.reload();await expect(page.locator('.message.user .image-attachments img')).toHaveAttribute('src',dataURL);}
});

test('image inspection fits large originals, traps focus and restores its trigger without sending',async({page},info)=>{
 const {state}=await fixture(page);
 const dataURL=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=1600;c.height=1000;const ctx=c.getContext('2d')!;ctx.fillStyle='#d9e4db';ctx.fillRect(0,0,1600,1000);ctx.fillStyle='#364e42';ctx.font='70px sans-serif';ctx.fillText('Field notes — attachment preview',90,180);return c.toDataURL('image/png');});
 const name='Field notes — café 日本語.png';
 await page.locator('.composer textarea').fill('Keep my question');
 await page.locator('.composer input[type=file]').setInputFiles({name,mimeType:'image/png',buffer:Buffer.from(dataURL.split(',')[1],'base64')});
 const trigger=page.getByRole('button',{name:'Preview image: '+name,exact:true});
 await trigger.click();
 const preview=page.getByRole('dialog',{name:'Preview image: '+name});
 await expect(preview).toBeVisible();await expect(preview.locator('img')).toHaveAttribute('src',dataURL);
 const bounds=await preview.locator('img').boundingBox(),surface=await preview.locator('.image-preview-surface').boundingBox();
 expect(bounds!.width).toBeLessThanOrEqual(surface!.width);expect(bounds!.height).toBeLessThanOrEqual(surface!.height);
 await page.screenshot({path:info.outputPath('image-preview-wide.png')});
 await preview.getByRole('button',{name:'Actual size',exact:true}).click();
 await expect(preview.locator('img')).toHaveCSS('width','1600px');
 await preview.getByRole('button',{name:'Actual size',exact:true}).click();
 await page.setViewportSize({width:320,height:480});
 await expect(preview.getByRole('button',{name:'Close image',exact:true})).toBeInViewport();
 await preview.getByRole('button',{name:'Close image',exact:true}).focus();await page.keyboard.press('Tab');
 expect(await preview.evaluate(node=>node.contains(document.activeElement))).toBe(true);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:info.outputPath('image-preview-narrow.png')});
 await page.keyboard.press('Escape');await expect(preview).toHaveCount(0);await expect(trigger).toBeFocused();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep my question');expect(state.writes).toEqual([]);
});
