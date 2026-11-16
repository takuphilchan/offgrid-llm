import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {chatImages} from '../src/i18n/chat-images';
import type {LocaleCode} from '../src/i18n';
import {languageRegistry} from '../src/i18n/language';
import {imageMetadataNotice} from '../src/i18n/image-metadata';

async function fixture(page:Page,{locale='en',vision=true,capable=true,orientation=true}:{locale?:LocaleCode;vision?:boolean;capable?:boolean;orientation?:boolean}={}){
 const state=await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:capable?['session-images-v1','agent-profiles-v1',...(orientation?['chat-image-orientation-v1']:[])]:[]}}));
 await page.route('**/v1/models',r=>r.fulfill({json:{data:[{id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',type:'chat',capability_status:vision?{vision:'unknown'}:{}}]}}));
 await page.goto('/ui/#/chat');await expect(page.locator('.composer textarea')).toBeEnabled();
 const dataURL=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=4;c.height=4;const ctx=c.getContext('2d')!;ctx.fillStyle='red';ctx.fillRect(0,0,4,4);return c.toDataURL('image/png');});
 const file={name:'red — café.png',mimeType:'image/png',buffer:Buffer.from(dataURL.split(',')[1],'base64')};
 return {state,file,dataURL};
}

test('oversized pixel headers are rejected before browser bitmap allocation',async({page})=>{
 await fixture(page);
 await page.evaluate(()=>{(window as any).bitmapAllocations=0;window.createImageBitmap=(()=>{(window as any).bitmapAllocations++;return Promise.reject(new Error('Allocation intercepted by regression fixture'));}) as typeof createImageBitmap;});
 const png=Buffer.alloc(33);Buffer.from([137,80,78,71,13,10,26,10]).copy(png);png.writeUInt32BE(13,8);png.write('IHDR',12);png.writeUInt32BE(100000,16);png.writeUInt32BE(100000,20);png[24]=8;png[25]=6;
 const jpeg=Buffer.from([0xff,0xd8,0xff,0xc0,0,17,8,0x7f,0xff,0x7f,0xff,3,1,0x11,0,2,0x11,0,3,0x11,0]);
 for(const [name,mimeType,buffer] of [['bomb.png','image/png',png],['bomb.jpg','image/jpeg',jpeg]] as const){
  await page.locator('.composer input[type=file]').setInputFiles({name,mimeType,buffer});
  await expect(page.locator('.image-input-notice[role=alert]')).toContainText(chatImages('en').limits);
  expect(await page.evaluate(()=>(window as any).bitmapAllocations)).toBe(0);
 }
});

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

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`images fit the narrow composer with keyboard removal: ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:320,height:800});const {file}=await fixture(page,{locale});const copy=chatImages(locale);
 await page.locator('.composer-attachments-menu > button').click();await expect(page.getByRole('button',{name:copy.add,exact:true})).toBeVisible();await page.keyboard.press('Escape');await page.locator('.composer input[type=file]').setInputFiles(file);
 const remove=page.getByRole('button',{name:copy.remove+': '+file.name,exact:true});await expect(remove).toBeVisible();await remove.focus();await expect(remove).toBeFocused();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
 const original=Buffer.from(file.buffer),animation=pngMetadata(original,'acTL',Buffer.alloc(8));
 await page.locator('.composer input[type=file]').setInputFiles({name:'animated.png',mimeType:'image/png',buffer:animation});
 await expect(page.locator('.image-input-notice[role=alert]')).toHaveText(imageMetadataNotice(locale));
 await expect(page.locator('.composer .image-attachments img')).toHaveCount(1);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`images-${locale}.png`),fullPage:true});
 await remove.press('Enter');await expect(page.locator('.composer .image-attachments')).toHaveCount(0);await expect(page.locator('.composer-attachments-menu > button')).toBeFocused();
});

function pngMetadata(original:Buffer,kind:string,payload:Buffer,afterPixels=false){
 const chunk=Buffer.alloc(payload.length+12);chunk.writeUInt32BE(payload.length);chunk.write(kind,4,'ascii');payload.copy(chunk,8);
 let crc=0xffffffff;for(const byte of chunk.subarray(4,-4)){crc^=byte;for(let j=0;j<8;j++)crc=(crc>>>1)^((crc&1)?0xedb88320:0);}
 chunk.writeUInt32BE((crc^0xffffffff)>>>0,chunk.length-4);
 const position=afterPixels?original.length-12:33;
 return Buffer.concat([original.subarray(0,position),chunk,original.subarray(position)]);
}
function exifOrientation(orientation:number){
 const exif=Buffer.alloc(26);exif.write('II');exif.writeUInt16LE(42,2);exif.writeUInt32LE(8,4);exif.writeUInt16LE(1,8);
 exif.writeUInt16LE(0x112,10);exif.writeUInt16LE(3,12);exif.writeUInt32LE(1,14);exif.writeUInt16LE(orientation,18);return exif;
}

test('animation and malformed orientation are rejected before decoding without losing the draft',async({page})=>{
 const {file}=await fixture(page);const input=page.locator('.composer input[type=file]');await input.setInputFiles(file);
 await expect(page.locator('.composer .image-attachments img')).toHaveCount(1);
 await page.locator('.composer textarea').fill('Keep this question');
 await page.evaluate(()=>{(window as any).imageMetadataDecodes=0;const original=window.createImageBitmap;window.createImageBitmap=((...args:any[])=>{(window as any).imageMetadataDecodes++;return (original as any)(...args);})as typeof createImageBitmap;});
 const original=Buffer.from(file.buffer);
 for(const buffer of [pngMetadata(original,'acTL',Buffer.alloc(8)),pngMetadata(original,'eXIf',exifOrientation(9)),pngMetadata(pngMetadata(original,'eXIf',exifOrientation(6)),'eXIf',exifOrientation(3))]){
  await input.setInputFiles({name:'unsupported.png',mimeType:'image/png',buffer});
  await expect(page.locator('.image-input-notice[role=alert]')).toHaveText(imageMetadataNotice('en'));
  await expect(page.locator('.composer .image-attachments img')).toHaveCount(1);
 }
 expect(await page.evaluate(()=>(window as any).imageMetadataDecodes)).toBe(0);
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this question');
});

test('an older service retains oriented images but cannot silently send their raw pixels',async({page})=>{
 const {file,state}=await fixture(page,{orientation:false});
 const buffer=pngMetadata(Buffer.from(file.buffer),'eXIf',exifOrientation(6));
 await page.locator('.composer textarea').fill('Keep this question');
 await page.locator('.composer input[type=file]').setInputFiles({...file,buffer});
 await expect(page.locator('.composer .image-attachments img')).toHaveCount(1);
 await expect(page.locator('.composer')).toContainText(chatImages('en').service);
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
 expect(state.writes).toEqual([]);
 await page.locator('.composer .image-attachment-remove').click();
 await page.locator('.composer input[type=file]').setInputFiles(file);
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeEnabled();
 await expect(page.locator('.composer textarea')).toHaveValue('Keep this question');
});

test('oriented saved history also requires service support before a follow-up',async({page})=>{
 const {file}=await fixture(page,{orientation:false});
 const buffer=pngMetadata(Buffer.from(file.buffer),'eXIf',exifOrientation(6));
 const saved={name:'Demo briefing',model_id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',messages:[{role:'user',content:'Read this',images:[{name:'saved.png',data_url:'data:image/png;base64,'+buffer.toString('base64')}]},{role:'assistant',content:'Saved reply'}]};
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[saved]}}));
 await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:saved}));
 await page.reload();await expect(page.locator('.message.user .image-attachments img')).toBeVisible();
 await page.locator('.composer textarea').fill('A follow-up question');
 await expect(page.locator('.composer')).toContainText(chatImages('en').service);
 await expect(page.getByRole('button',{name:'Send',exact:true})).toBeDisabled();
});

for(const variant of ['png','png-late','jpeg'])test(`all eight ${variant} orientations preview upright while retaining original bytes`,async({page})=>{
 const type=variant==='jpeg'?'jpeg':'png';
 await fixture(page);
 const dataURL=await page.evaluate(type=>{const c=document.createElement('canvas');c.width=48;c.height=32;const ctx=c.getContext('2d')!;const colors=['#ff0000','#00ff00','#0000ff','#ffff00','#ff00ff','#00ffff'];colors.forEach((color,i)=>{ctx.fillStyle=color;ctx.fillRect(i%3*16,Math.floor(i/3)*16,16,16);});return c.toDataURL('image/'+type,1);},type);
 const original=Buffer.from(dataURL.split(',')[1],'base64'),expected=['123456','321654','654321','456123','142536','415263','635241','362514'];
 for(let orientation=1;orientation<=8;orientation++){
  const exif=exifOrientation(orientation);
  const segment=Buffer.concat([Buffer.from([255,225,0,exif.length+8]),Buffer.from('Exif\0\0'),exif]);
  const buffer=type==='png'?pngMetadata(original,'eXIf',exif,variant==='png-late'):Buffer.concat([original.subarray(0,2),segment,original.subarray(2)]);
  const url='data:image/'+type+';base64,'+buffer.toString('base64');
  await page.locator('.composer input[type=file]').setInputFiles({name:`rotation.${type}`,mimeType:'image/'+type,buffer});
  const preview=page.locator('.composer .image-attachments img');await expect(preview).toBeVisible();
  if(variant!=='png-late'||orientation===1)await expect(preview).toHaveAttribute('src',url);
  expect(await page.evaluate(url=>Object.values(localStorage).some(value=>value.includes(url)),url)).toBe(true);
  const values=await preview.evaluate((node:HTMLImageElement)=>{const c=document.createElement('canvas');c.width=node.naturalWidth;c.height=node.naturalHeight;const ctx=c.getContext('2d')!;ctx.drawImage(node,0,0);const palette=[[255,0,0],[0,255,0],[0,0,255],[255,255,0],[255,0,255],[0,255,255]];let result='';for(let y=8;y<c.height;y+=16)for(let x=8;x<c.width;x+=16){const color=ctx.getImageData(x,y,1,1).data;const distances=palette.map(p=>p.reduce((sum,v,i)=>sum+Math.abs(v-color[i]),0));result+=String(distances.indexOf(Math.min(...distances))+1);}return {width:c.width,height:c.height,result};});
  expect(values).toEqual({width:orientation>=5?32:48,height:orientation>=5?48:32,result:expected[orientation-1]});
  if(variant==='png-late'){
   const display=await preview.getAttribute('src');
   await page.locator('.composer .image-attachment-open').click();
   await expect(page.locator('.image-preview img')).toHaveAttribute('src',display!);
   await page.keyboard.press('Escape');
  }
  await page.locator('.composer .image-attachment-remove').click();await expect(preview).toHaveCount(0);
 }
});

test('multiple image files decode sequentially and keep their original bytes',async({page})=>{
 const {file,dataURL}=await fixture(page);
 await page.evaluate(()=>{
  const original=window.createImageBitmap.bind(window);let active=0;(window as any).peakImageDecodes=0;
  window.createImageBitmap=(async(...args:any[])=>{active++;(window as any).peakImageDecodes=Math.max((window as any).peakImageDecodes,active);try{await new Promise(resolve=>setTimeout(resolve,40));return await (original as any)(...args);}finally{active--;}}) as typeof createImageBitmap;
 });
 await page.locator('.composer input[type=file]').setInputFiles([file,{...file,name:'second.png'},{...file,name:'third.png'}]);
 await expect(page.locator('.composer .image-attachments img')).toHaveCount(3);
 expect(await page.evaluate(()=>(window as any).peakImageDecodes)).toBe(1);
 for(const image of await page.locator('.composer .image-attachments img').all())await expect(image).toHaveAttribute('src',dataURL);
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
