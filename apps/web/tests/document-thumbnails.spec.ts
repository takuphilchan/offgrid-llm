import {test,expect,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';
import {documentReader} from '../src/i18n/document-reader';
import {documentThumbnails} from '../src/i18n/document-thumbnails';
import {readDocumentThumbnail} from '../src/api/document-thumbnail';

const file={id:'file-'+'a'.repeat(32),name:'Report.pdf',sha256:'b'.repeat(64),revision:'c'.repeat(64),bytes:100,status:'ready',media_type:'application/pdf',created_at:'2026-11-14T00:00:00Z'};
async function setup(page:Page,locale:LocaleCode,mode='valid'){
 await craftFixture(page,locale,locale==='ar'?'dark':'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1']}}));
 await page.route('**/v1/sessions/*/attachments',r=>r.fulfill({json:r.request().method()==='POST'?{attachment:file}:{attachments:[file]}}));
 // Authored small page illustration. The fixture is a preview transport check,
 // not a claim about the packaged PDF renderer or OCR accuracy.
 const small=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=96;c.height=128;const x=c.getContext('2d')!;x.fillStyle='#fff';x.fillRect(0,0,96,128);x.fillStyle='#222';x.fillRect(12,15,42,4);x.fillStyle='#bbb';for(let i=0;i<7;i++)x.fillRect(12,30+i*9,72,2);return c.toDataURL().split(',')[1];});
 const requests:number[]=[];let active=0,peak=0;
 await page.route('**/attachments/*/preview*',async r=>{
  const url=new URL(r.request().url());
  if(url.searchParams.has('page')){
   if(url.searchParams.get('thumbnail')==='true'){
    requests.push(Number(url.searchParams.get('page')));active++;peak=Math.max(peak,active);
    const headers={'X-OffGrid-Source-SHA256':mode==='wrong-source'?'0'.repeat(64):file.sha256,'X-OffGrid-Preview':'thumbnail-v1'};
    await r.fulfill({status:mode==='denied'?404:200,contentType:'image/png',headers,body:Buffer.from(small,'base64')});active--;return;
   }
   return r.fulfill({contentType:'image/png',headers:{'X-OffGrid-Source-SHA256':file.sha256},body:Buffer.from(small,'base64')});
  }
  return r.fulfill({json:{name:file.name,sha256:file.sha256,format:'pdf',pages:9,state:'complete',thumbnails_available:mode!=='old',units:Array.from({length:9},(_,i)=>({kind:'page',index:i+1,text:`Source page ${i+1}`,state:i===2?'ocr_partial':'ready',source_sha256:file.sha256,extractor_revision:'fixture'}))}});
 });
 await page.goto('/ui/#/chat');
 await page.locator('input[data-attachment=files]').setInputFiles({name:file.name,mimeType:'application/pdf',buffer:Buffer.from('%PDF-fixture')});
 await page.getByRole('button',{name:documentReader(locale).open,exact:true}).click();
 await expect(page.locator('.document-reader-surface img')).toBeVisible();
 return{requests,peak:()=>peak};
}

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`bounded page thumbnails navigate using shared translations: ${locale}`,async({page},info)=>{
 await page.setViewportSize({width:locale==='ar'?390:1200,height:900});
 const state=await setup(page,locale),copy=documentReader(locale),strip=page.locator('.document-thumbnails');
 expect(state.requests).toEqual([]);
 await expect(strip.locator('.document-pages-trigger')).toHaveAccessibleName(documentThumbnails(locale).pages);
 await strip.locator('.document-pages-trigger').focus();await page.keyboard.press('Enter');
 await expect(strip.locator('img')).toHaveCount(4);expect(state.requests).toEqual([1,2,3,4]);expect(state.peak()).toBe(1);
 await strip.locator('[data-reader-page="3"]').click();
 await expect(strip.locator('[data-reader-page="3"]')).toHaveAttribute('aria-current','page');
 await expect(page.locator('.document-reader-surface img')).toHaveAttribute('alt','Report.pdf · 3');
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`thumbnails-${locale}.png`)});
 expect(await strip.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 await strip.locator('[aria-current="page"]').focus();await page.keyboard.press('End');
 await expect(strip.locator('[data-reader-page="9"]')).toHaveAttribute('aria-current','page');
 await expect(strip.locator('[data-reader-page="9"]')).toBeFocused();
 await expect(strip.locator('img')).toHaveCount(4);
 expect(state.requests).toContain(9);expect(state.peak()).toBe(1);
 await page.keyboard.press('Home');
 await expect(strip.locator('[data-reader-page="1"]')).toHaveAttribute('aria-current','page');
 await expect(strip.locator('[data-reader-page="1"]')).toBeFocused();
 await strip.locator('.document-pages-trigger').click();await expect(strip.locator('img')).toHaveCount(0);
});

test('thumbnail response bounds reject large streams and oversized decoded dimensions',async({page})=>{
 await craftFixture(page,'en','light');await page.goto('/ui/#/chat');
 const bytes=await page.evaluate(()=>{
  const c=document.createElement('canvas');c.width=193;c.height=256;
  return Array.from(Uint8Array.from(atob(c.toDataURL().split(',')[1]),v=>v.charCodeAt(0)));
 });
 const sha='b'.repeat(64),headers={'Content-Type':'image/png','X-OffGrid-Preview':'thumbnail-v1','X-OffGrid-Source-SHA256':sha};
  let bounds=false,stream=false,cancelled=false;
  try{await readDocumentThumbnail(new Response(new Uint8Array(bytes),{headers}),sha);}catch{bounds=true;}
  const body=new ReadableStream({pull(controller){controller.enqueue(new Uint8Array(128*1024));},cancel(){cancelled=true;}});
  try{await readDocumentThumbnail(new Response(body,{headers}),sha);}catch{stream=true;}
 expect({bounds,stream,cancelled}).toEqual({bounds:true,stream:true,cancelled:true});
});

for(const mode of ['wrong-source','denied','old'])test(`thumbnail recovery does not display unverified source pixels: ${mode}`,async({page})=>{
 const state=await setup(page,'en',mode),strip=page.locator('.document-thumbnails');
 if(mode==='old'){
  await strip.locator('.document-pages-trigger').click();
  await expect(strip.locator('.document-thumbnail-excerpt').first()).toContainText('Source page 1');
  await expect(strip.locator('img')).toHaveCount(0);expect(state.requests).toEqual([]);return;
 }
 await strip.locator('.document-pages-trigger').click();
 await expect.poll(()=>state.requests.length).toBe(4);
 await expect(strip.locator('img')).toHaveCount(0);
 await expect(strip.getByRole('button',{name:'Try again',exact:true})).toBeVisible();
 await strip.locator('.document-thumbnail').nth(1).click();
 await expect(page.locator('.document-reader-surface img')).toHaveAttribute('alt','Report.pdf · 2');
});
