import {readerOptions} from './helpers/reader-options';
import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { documentReader } from '../src/i18n/document-reader';
import type { LocaleCode } from '../src/i18n';
import { readFileSync,writeFileSync } from 'node:fs';
import { resolve,basename } from 'node:path';
import { createHash } from 'node:crypto';
import { languageRegistry } from '../src/i18n/language';
import { documentEvidenceNotice } from '../src/i18n/document-evidence';
import { documentFormatLimits } from '../src/i18n/document-format-limits';
import { documentReview } from '../src/i18n/document-review';
import { documentNavigation } from '../src/i18n/document-navigation';

const file = {id:'file-'+'a'.repeat(32),name:'Research paper.pdf',sha256:'b'.repeat(64),revision:'c'.repeat(64),media_type:'application/pdf',bytes:100,status:'failed',error_code:'document_ocr_language_missing',created_at:'2026-11-03T00:00:00Z'};

test('document tabs add unsent files and preserve independent reading positions',async({page})=>{
 await page.setViewportSize({width:1440,height:960});
 const first={...file,status:'ready',error_code:''},second={...first,id:'file-'+'d'.repeat(32),name:'Appendix.pdf',sha256:'e'.repeat(64),revision:'f'.repeat(64)};
 await setup(page,'en','light',first);
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader');
 await reader.getByRole('button',{name:'Next page',exact:true}).click();
 await reader.getByRole('combobox',{name:'Zoom',exact:true}).selectOption('125');
 let uploads=0;
 await page.route('**/v1/sessions/*/attachments',r=>{
  if(r.request().method()==='POST')uploads++;
  return r.fulfill({json:r.request().method()==='POST'?{attachment:second}:{attachments:[first,second]}});
 });
 await page.route(`**/attachments/${second.id}/preview*`,r=>new URL(r.request().url()).searchParams.has('page')?r.fallback():r.fulfill({json:{name:second.name,sha256:second.sha256,format:'pdf',pages:1,state:'complete',units:[{kind:'page',index:1,text:'Appendix evidence',state:'ready',source_sha256:second.sha256,extractor_revision:'fixture'}]}}));
 const chooser=page.waitForEvent('filechooser');await reader.locator('.document-add-file').click();
 await (await chooser).setFiles({name:second.name,mimeType:'application/pdf',buffer:Buffer.from('%PDF-second-fixture')});
 const secondTile=page.locator('.file-attachments li').filter({hasText:second.name});
 await secondTile.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(reader.getByRole('tab')).toHaveCount(2);
 await expect(reader.getByRole('spinbutton')).toHaveValue('1');
 await expect(reader.getByRole('combobox',{name:'Zoom',exact:true})).toHaveValue('100');
 await reader.getByRole('tab',{name:second.name,exact:true}).press('ArrowLeft');
 await expect(reader.getByRole('tab',{name:first.name,exact:true})).toBeFocused();
 await expect(reader.getByRole('spinbutton')).toHaveValue('2');
 await expect(reader.getByRole('combobox',{name:'Zoom',exact:true})).toHaveValue('125');
 await page.screenshot({path:test.info().outputPath('document-tabs.png')});
 await reader.getByRole('tab',{name:first.name,exact:true}).press('End');
 await expect(reader.getByRole('tab',{name:second.name,exact:true})).toBeFocused();
 await reader.getByRole('button',{name:`Close document: ${first.name}`,exact:true}).click();
 await expect(reader.getByRole('tab')).toHaveCount(1);
 await expect(reader.getByRole('tab',{name:second.name,exact:true})).toBeFocused();
 await reader.getByRole('button',{name:'Close document',exact:true}).click();
 await expect(reader).toHaveCount(0);
 await expect(secondTile.getByRole('button',{name:'Open document',exact:true})).toBeFocused();
 await expect(page.locator('.file-attachments li')).toHaveCount(2);
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 expect(uploads).toBe(1);
});

for (const chromeHeight of [0, 32, 32 / 1.5, 16]) test(`expanded reader clears desktop captions at ${chromeHeight}px`,async({page})=>{
 await page.setViewportSize({width:1440,height:960});
 await setup(page,'en','light',{...file,status:'ready',error_code:''});
 await page.evaluate(height=>document.documentElement.style.setProperty('--native-chrome-height',`${height}px`),chromeHeight);
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader');
 await reader.getByRole('button',{name:'Expand document',exact:true}).click();
 const bounds=(await reader.boundingBox())!,header=(await reader.locator(':scope > header').boundingBox())!;
 expect(bounds.y).toBeCloseTo(chromeHeight,1);
 expect(header.y).toBeGreaterThanOrEqual(chromeHeight-.1);
 expect(bounds.y+bounds.height).toBeCloseTo(960,0);
 await reader.getByRole('button',{name:'Restore document view',exact:true}).click();
 await expect(reader).not.toHaveAttribute('aria-modal','true');
});

test('expanded reader restores split view and gives utilities first Escape',async({page})=>{
 await page.setViewportSize({width:1440,height:960});
 await setup(page,'en','dark',{...file,status:'ready',error_code:''});
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader');
 await reader.getByRole('button',{name:'Next page',exact:true}).click();
 await reader.getByRole('combobox',{name:'Zoom',exact:true}).selectOption('125');
 const width=(await reader.boundingBox())!.width;
 await reader.getByRole('button',{name:'Expand document',exact:true}).click();
 await expect(reader).toHaveAttribute('aria-modal','true');
 expect((await reader.boundingBox())!.width).toBeGreaterThan(width+200);
 await reader.locator('.document-pages-trigger').click();
 await expect(reader.locator('.document-thumbnail-panel')).toBeVisible();
 await page.keyboard.press('Escape');
 await expect(reader.locator('.document-thumbnail-panel')).toBeHidden();
 await expect(reader).toHaveAttribute('aria-modal','true');
 await page.keyboard.press('Escape');
 await expect(reader).not.toHaveAttribute('aria-modal','true');
 await expect(reader.getByRole('button',{name:'Expand document',exact:true})).toBeFocused();
 expect((await reader.boundingBox())!.width).toBe(width);
 await expect(reader.getByRole('spinbutton')).toHaveValue('2');
 await expect(reader.getByRole('combobox',{name:'Zoom',exact:true})).toHaveValue('125');
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
});

for(const [locale,theme,width] of [['en','dark',1440],['en','light',1920],['ar','dark',390]] as const)test(`reader chrome stays compact with 37 pages: ${locale} ${theme}`,async({page})=>{
 await page.setViewportSize({width,height:960});
 await setup(page,locale,theme,{...file,status:'ready',error_code:''});
 const raster=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=800;c.height=1100;const x=c.getContext('2d')!;x.fillStyle='#fff';x.fillRect(0,0,800,1100);x.fillStyle='#222';x.font='14px sans-serif';x.fillText('SYNTHETIC DOCUMENT / UI TEST',70,60);x.font='bold 27px sans-serif';x.fillText('Research findings',70,115);for(let i=0;i<30;i++){x.fillStyle='#777';x.fillRect(70,160+i*27,290,3);x.fillRect(410,160+i*27,310,3);}const thumb=document.createElement('canvas');thumb.width=180;thumb.height=248;thumb.getContext('2d')!.drawImage(c,0,0,180,248);return {page:c.toDataURL().split(',')[1],thumb:thumb.toDataURL().split(',')[1]};});
 await page.route('**/attachments/*/preview*',r=>{
  if(new URL(r.request().url()).searchParams.has('page'))return r.fulfill({contentType:'image/png',headers:{'X-OffGrid-Source-SHA256':file.sha256,'X-OffGrid-Preview':'thumbnail-v1'},body:Buffer.from(new URL(r.request().url()).searchParams.has('thumbnail')?raster.thumb:raster.page,'base64')});
  return r.fulfill({json:{name:file.name,sha256:file.sha256,format:'pdf',pages:37,state:'partial',thumbnails_available:true,units:Array.from({length:37},(_,i)=>({kind:'page',index:i+1,text:`Page ${i+1} evidence`,state:[1,6,11,12,36,37].includes(i+1)?'ocr_required':'ready',source_sha256:file.sha256,extractor_revision:'fixture'}))}});
 });
 const reader=page.locator('.document-reader'),copy=documentReader(locale);
 await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(reader.locator('.document-reader-surface img')).toBeVisible();
 await page.screenshot({path:test.info().outputPath(`reader-canvas-${locale}-${theme}.png`)});
 const coverage=reader.locator('.document-coverage-disclosure');
 await expect(coverage.locator('.document-coverage-trigger')).toContainText(documentNavigation(locale).needsReview);
 await expect(coverage.locator('nav')).not.toBeVisible();
 await coverage.locator('.document-coverage-trigger').click();
 await coverage.locator('nav button').last().click();
 await expect(reader.getByRole('spinbutton')).toHaveValue('37');
 await coverage.locator('.document-coverage-trigger').click();
 await reader.locator('.document-pages-trigger').click();
 await expect(reader.locator('.document-thumbnail-panel')).toHaveCSS('background-color',await reader.evaluate(element=>getComputedStyle(element).getPropertyValue('--surface-1').trim()).then(color=>page.evaluate(color=>{const element=document.createElement('div');element.style.color=color;document.body.append(element);const value=getComputedStyle(element).color;element.remove();return value;},color)));
 await expect(reader.locator('.document-thumbnail')).toHaveCount(4);await expect(reader.locator('.document-thumbnail[aria-current=page] img')).toBeVisible();
 await reader.locator('.document-thumbnail-panel').evaluate(element=>{element.scrollTop=0;});
 await expect(reader.locator('.document-thumbnail-number').first()).toContainText('1');
 await expect(reader.getByRole('spinbutton')).toHaveValue('37');
 await reader.locator('.document-thumbnail').first().click();
 await expect(reader.getByRole('spinbutton')).toHaveValue('1');
 const box=(await reader.boundingBox())!,canvas=(await reader.locator('.document-reader-surface').boundingBox())!;
 expect(canvas.height/box.height).toBeGreaterThan(.58);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:test.info().outputPath(`reader-pages-${locale}-${theme}.png`)});
 // Virtualized rows keep keyboard selection and focus together, including a
 // Home key when scrolling moved the selected row out of the rendered window.
 await reader.locator('.document-thumbnail[aria-current=page]').press('End');
 await expect(reader.getByRole('spinbutton')).toHaveValue('37');
 await expect(reader.locator('.document-thumbnail[aria-current=page]')).toBeFocused();
 await reader.locator('.document-thumbnail[aria-current=page]').press('Home');
 await expect(reader.getByRole('spinbutton')).toHaveValue('1');
 await reader.locator('.document-thumbnail-panel').evaluate(element=>{element.scrollTop=20*240;});
 await expect(reader.locator('.document-thumbnail-number').first()).toContainText('21');
 await reader.locator('.document-thumbnail').first().press('Home');
 await expect(reader.locator('.document-thumbnail[aria-current=page]')).toBeFocused();
 await expect(reader.locator('.document-thumbnail-number').first()).toContainText('1');
 await reader.getByRole('button',{name:copy.close,exact:true}).click();
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await page.locator('.conversation').focus();
 await expect(page.locator('.conversation')).toHaveCSS('box-shadow','none');
});

test('model load failure offers model selection and keeps document draft',async({page})=>{
 await setup(page,'en','dark',{...file,status:'ready',error_code:''});
 await page.evaluate(()=>{const fetch=window.fetch.bind(window);window.fetch=async(input,init)=>String(input).endsWith('/generate')?new Response('data: '+JSON.stringify({type:'error',error:'Model could not be loaded; check runtime availability and logs',error_code:'model_load_failed'})+'\n\n',{headers:{'Content-Type':'text/event-stream'}}):fetch(input,init);});
 await page.getByRole('button',{name:'Send',exact:true}).click();
 const recovery=page.locator('.chat-recovery');
 await expect(recovery).toContainText('Model');
 await expect(recovery.getByRole('button',{name:'Refresh history'})).toHaveCount(0);
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await expect(page.locator('.file-attachments')).toContainText(file.name);
 await recovery.getByRole('button',{name:'Choose a chat model',exact:true}).click();
 await expect(page).toHaveURL(/models/);
});

for(const locale of ['en','ar'] as LocaleCode[])test(`image evidence regions revalidate and preserve navigation: ${locale}`,async({page})=>{
 test.setTimeout(60_000);
 await page.setViewportSize({width:locale==='ar'?390:1200,height:900});
 await setup(page,locale,locale==='ar'?'dark':'light',{...file,name:'Receipt.png',status:'ready',error_code:'',extraction_revision:'extraction-image'} as typeof file);
 const text='税额 42',bytes=new TextEncoder().encode(text).length;
 const raster=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=480;c.height=320;const x=c.getContext('2d')!;x.fillStyle='#fff';x.fillRect(0,0,480,320);x.fillStyle='#222';x.font='22px sans-serif';x.fillText('Receipt · 42',35,110);return c.toDataURL().split(',')[1];});
 let permitted=true,identity=file.sha256,reads=0,rasterIdentity:string|null=file.sha256,frameWidth=480;
 await page.route('**/attachments/*/preview*',r=>{
  reads++;if(!permitted)return r.fulfill({status:404,json:{error:{message:'Source unavailable'}}});
  if(new URL(r.request().url()).searchParams.has('page'))return r.fulfill({contentType:'image/png',headers:rasterIdentity?{'X-OffGrid-Source-SHA256':rasterIdentity}:{},body:Buffer.from(raster,'base64')});
  return r.fulfill({json:{name:'Receipt.png',sha256:file.sha256,format:'png',pages:1,state:'complete',units:[{kind:'page',index:1,text,state:'ready',source_sha256:file.sha256,extractor_revision:'fixture',image_geometry:{width:frameWidth,height:320,orientation:1}}]}});
 });
 await page.route('**/attachments/*/structure?**',r=>{
  const query=new URL(r.request().url()).searchParams;expect(query.get('revision')).toBe(file.revision);
  return r.fulfill({json:{document_id:file.id,revision:'extraction-image',original_sha256:identity,format:'image_document',method:'ocr',units:[{id:'page-1',kind:'page',index:1,span:{start_byte:0,end_byte:bytes}}],blocks:[{id:'line-1',unit_id:'page-1',kind:'paragraph',method:'ocr',source_part:'original_encoded_pixels',span:{start_byte:0,end_byte:bytes},text,text_sha256:'d'.repeat(64),bounds:{x:30,y:85,width:145,height:32,unit:'pixels'},editable:false,truncated:false,metadata_omitted:false}],offset:0,total_blocks:1,next_offset:null,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:false}});
 });
 const copy=documentReader(locale);await page.getByRole('button',{name:copy.open,exact:true}).click();
 const reader=page.locator('.document-reader');
 await readerOptions(page);await reader.getByRole('button',{name:copy.search,exact:true}).click();await reader.getByRole('searchbox',{name:copy.search,exact:true}).fill('42');
 await expect(reader.locator('.document-source-regions rect')).toHaveCount(1);
 await readerOptions(page);await reader.getByRole('combobox',{name:copy.document,exact:true}).selectOption('compare');
 await expect(reader.locator('mark')).toHaveText('42');
 await expect(reader.locator('.document-source-regions rect')).toHaveCount(1);
 await reader.getByRole('combobox',{name:copy.zoom,exact:true}).selectOption('150');
 await page.screenshot({path:test.info().outputPath(`source-regions-${locale}.png`)});
 await reader.getByRole('button',{name:copy.close,exact:true}).click();const earlier=reads;
 await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(reader.locator('.document-source-regions rect')).toHaveCount(1);expect(reads).toBeGreaterThan(earlier);
 await expect(reader.getByRole('searchbox',{name:copy.search,exact:true})).toHaveValue('42');
 const reopen=async()=>{await reader.getByRole('button',{name:copy.close,exact:true}).click();await page.getByRole('button',{name:copy.open,exact:true}).click();};
 // Legacy responses can be read, but cannot assert pixel/source correspondence.
 rasterIdentity=null;await reopen();await expect(reader.locator('img')).toBeVisible();await expect(reader.locator('.document-source-regions')).toHaveCount(0);
 rasterIdentity=file.sha256;frameWidth=481;await reopen();await expect(reader.locator('img')).toBeVisible();await expect(reader.locator('.document-source-regions')).toHaveCount(0);
 frameWidth=480;rasterIdentity='e'.repeat(64);await reopen();await expect(reader.getByRole('alert')).toBeVisible();await expect(reader.locator('img,.document-source-regions')).toHaveCount(0);
 rasterIdentity=file.sha256;
 // A different extraction cannot lend geometry to the retained original.
 identity='f'.repeat(64);await reader.getByRole('button',{name:copy.close,exact:true}).click();await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(reader.locator('pre')).toHaveText(text);await expect(reader.locator('.document-source-regions')).toHaveCount(0);
 permitted=false;await reader.getByRole('button',{name:copy.close,exact:true}).click();await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(reader.getByRole('alert')).toBeVisible();await expect(reader.locator('pre,img,.document-source-regions')).toHaveCount(0);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('reader position survives app navigation but never crosses a changed source digest',async({page})=>{
 await setup(page,'en','light',{...file,status:'ready',error_code:''});
 const copy=documentReader('en'),open=()=>page.getByRole('button',{name:copy.open,exact:true}).click(),reader=page.locator('.document-reader');
 await open();await reader.getByRole('button',{name:copy.next,exact:true}).click();
 await reader.getByRole('combobox',{name:copy.zoom,exact:true}).selectOption('200');
 await page.evaluate(()=>{location.hash='#/models';});await expect(reader).toHaveCount(0);
 await page.evaluate(()=>{location.hash='#/chat';});await open();
 await expect(reader.getByRole('spinbutton')).toHaveValue('2');await expect(reader.getByRole('combobox',{name:copy.zoom,exact:true})).toHaveValue('200');
 await reader.getByRole('button',{name:copy.close,exact:true}).click();
 await page.route('**/attachments/*/preview*',r=>r.fulfill({json:{name:file.name,sha256:'f'.repeat(64),format:'pdf',pages:1,state:'complete',units:[{kind:'page',index:1,text:'Different original',state:'ready'}]}}));
 await open();await expect(reader.getByRole('alert')).toBeVisible();await expect(reader.locator('pre,img')).toHaveCount(0);
});

test('reopening restores the reading offset and changing pages starts at the top',async({page})=>{
 await page.setViewportSize({width:1000,height:800});await setup(page,'en','light',{...file,status:'ready',error_code:''});
 await page.route('**/attachments/*/preview*',r=>r.fulfill({json:{name:file.name,sha256:file.sha256,format:'xlsx',pages:0,state:'complete',units:[1,2].map(index=>({kind:'sheet',index,name:`Sheet ${index}`,text:Array.from({length:100},(_,i)=>`Row ${i+1}: retained evidence`).join('\n'),state:'ready',source_sha256:file.sha256,extractor_revision:'fixture'}))}}));
 const reader=page.locator('.document-reader'),copy=documentReader('en');
 await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(reader.locator('pre')).toContainText('Row 100');
 await reader.locator('.document-reader-surface').evaluate(element=>element.scrollTo(0,600));
 await expect.poll(()=>reader.locator('.document-reader-surface').evaluate(element=>element.scrollTop)).toBe(600);
 await reader.getByRole('button',{name:copy.close,exact:true}).click();await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect.poll(()=>reader.locator('.document-reader-surface').evaluate(element=>element.scrollTop)).toBe(600);
 await reader.getByRole('button',{name:copy.next,exact:true}).click();
 await expect.poll(()=>reader.locator('.document-reader-surface').evaluate(element=>element.scrollTop)).toBe(0);
});

test('page navigation hides the previous raster while the selected page loads',async({page})=>{
 await setup(page);const copy=documentReader('en'),reader=page.locator('.document-reader');
 let finish!:()=>void;const wait=new Promise<void>(resolve=>{finish=resolve;});
 await page.route('**/attachments/*/preview*',async route=>{
  if(new URL(route.request().url()).searchParams.get('page')==='2')await wait;
  return route.fallback();
 });
 await page.getByRole('button',{name:copy.open,exact:true}).click();await expect(reader.locator('img')).toBeVisible();
 await reader.getByRole('button',{name:copy.next,exact:true}).click();
 await expect(reader.locator('.document-reader-surface')).toHaveAttribute('aria-busy','true');
 await expect(reader.locator('img')).toHaveCount(0);finish();
 await expect(reader.locator('img')).toHaveAttribute('alt',`${file.name} · 2`);
});

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`TIFF reader preserves partial pages and localized navigation: ${locale}`,async({page})=>{
 const evidence={format:'image_document',status:'partial',total_units:2,readable_units:1,review_units:[2]};
 const attachment={...file,name:'Printed records.tiff',media_type:'image/tiff',error_code:'document_ocr_required',evidence};
 await page.setViewportSize({width:locale==='ar'?390:680,height:900});
 await setup(page,locale,locale==='ar'?'dark':'light',attachment);
 const pageImages=await page.evaluate(()=>['Amount 42','Exception 17'].map((text,i)=>{const canvas=document.createElement('canvas');canvas.width=480;canvas.height=320;const ctx=canvas.getContext('2d')!;ctx.fillStyle='#fff';ctx.fillRect(0,0,480,320);ctx.fillStyle='#222';ctx.font='22px sans-serif';ctx.fillText(`Printed record · ${i+1}`,35,55);ctx.font='18px sans-serif';ctx.fillText(text,35,115);return canvas.toDataURL('image/png').split(',')[1];}));
 const requested:number[]=[];
 await page.route('**/attachments/*/preview*',r=>{
  const url=new URL(r.request().url());
  if(url.searchParams.has('page')){expect(url.searchParams.get('sha256')).toBe(file.sha256);const n=Number(url.searchParams.get('page'));requested.push(n);return r.fulfill({contentType:'image/png',body:Buffer.from(pageImages[n-1],'base64')});}
  return r.fulfill({json:{name:attachment.name,sha256:file.sha256,format:'tiff',pages:2,state:'partial',evidence,units:[{kind:'page',index:1,text:'Amount 42',state:'ready',source_sha256:file.sha256,extractor_revision:'tiff-fixture'},{kind:'page',index:2,text:'Exception 17',state:'ocr_partial',source_sha256:file.sha256,extractor_revision:'tiff-fixture'}]}});
 });
 const copy=documentReader(locale);await page.getByRole('button',{name:copy.open,exact:true}).click();const reader=page.locator('.document-reader');
 await expect(reader.locator('.document-reader-surface img')).toBeVisible();
 await reader.getByRole('button',{name:copy.next,exact:true}).click();
 await expect.poll(()=>requested.at(-1)).toBe(2);
 await readerOptions(page);await reader.getByRole('combobox',{name:copy.document,exact:true}).selectOption('compare');
 await expect(reader.locator('.document-reader-surface pre')).toHaveText('Exception 17');
 await reader.locator('.document-coverage-trigger').click();await expect(reader.locator('.document-coverage-disclosure')).toContainText(copy.partial);await page.keyboard.press('Escape');
 await reader.locator('.document-reader-surface').focus();await page.keyboard.press('PageUp');
 await expect(reader.locator('.document-reader-surface pre')).toHaveText('Amount 42');
 await expect.poll(()=>requested.at(-1)).toBe(1);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:test.info().outputPath(`tiff-reader-${locale}.png`)});
 await reader.getByRole('button',{name:copy.next,exact:true}).click();
 await reader.getByRole('combobox',{name:copy.zoom,exact:true}).selectOption('150');
 await readerOptions(page);await reader.getByRole('button',{name:copy.search,exact:true}).click();await reader.getByRole('searchbox',{name:copy.search,exact:true}).fill('Exception');
 await reader.getByRole('button',{name:copy.close,exact:true}).click();
 await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(reader.getByRole('spinbutton')).toHaveValue('2');
 await expect(reader.getByRole('combobox',{name:copy.zoom,exact:true})).toHaveValue('150');
 await expect(reader.getByRole('searchbox',{name:copy.search,exact:true})).toHaveValue('Exception');
 await readerOptions(page);
 await expect(reader.getByRole('combobox',{name:copy.document,exact:true})).toHaveValue('compare');
});

for(const locale of ['en','ar','zh-Hans'] as LocaleCode[])test(`uncertain image OCR requires explicit acceptance and keeps its warning: ${locale}`,async({page})=>{
 const evidence={format:'image_document',status:'partial',total_units:1,readable_units:0,review_units:[1]};
 const attachment={...file,name:'Uncertain scan.png',error_code:'document_ocr_required',evidence};
 await setup(page,locale,'light',attachment);
 let acknowledged=0;
 await page.route('**/attachments/*/prepare*',r=>{
  expect(new URL(r.request().url()).searchParams.get('text_only')).toBe('true');acknowledged++;
  return r.fulfill({json:{attachment:{...attachment,status:'ready',error_code:'',coverage:'partial_text_only'}}});
 });
 expect(acknowledged).toBe(0);
 await page.locator('.file-attachments').getByRole('button',{name:documentReader(locale).textOnly,exact:true}).click();
 await expect(page.locator('.file-attachments li')).toHaveAttribute('data-state','ready');
 await expect(page.locator('.file-attachments [data-extraction-coverage="partial"]')).toHaveCount(1);
 expect(acknowledged).toBe(1);
});

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`image document retains OCR text and original controls: ${locale}`,async({page})=>{
 const evidence={format:'image_document',status:'text_only',total_units:1,readable_units:1,review_units:[1],warnings:[{code:'document_visual_content',message:'Text only'}]};
 const attachment={...file,name:'Printed record.png',status:'ready',error_code:'',evidence};
 await page.setViewportSize({width:620,height:1000});
 await setup(page,locale,'light',attachment);
 let rasterRequests=0;
 await page.route('**/attachments/*/preview*',r=>{
  const url=new URL(r.request().url());
  if(url.searchParams.has('page')){rasterRequests++;expect(url.searchParams.get('page')).toBe('1');return r.fulfill({contentType:'image/png',body:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j5ZkAAAAASUVORK5CYII=','base64')});}
  return r.fulfill({json:{name:attachment.name,sha256:file.sha256,format:'png',pages:1,state:'complete',evidence,units:[{kind:'page',index:1,text:'Scanned receipt total 42',state:'ready',source_sha256:file.sha256,extractor_revision:'image-fixture'}]}});
 });
 const copy=documentReader(locale);
 await page.getByRole('button',{name:copy.open,exact:true}).click();
 const reader=page.locator('.document-reader');
 await expect(reader.locator('.document-reader-surface img')).toBeVisible();
 await readerOptions(page);await reader.getByRole('combobox',{name:copy.document,exact:true}).selectOption('text');
 await expect(reader.locator('.document-reader-surface pre')).toHaveText('Scanned receipt total 42');
 await expect(reader.locator('[data-extraction-coverage="text_only"]')).toHaveCount(1);
 await readerOptions(page);await reader.getByRole('combobox',{name:copy.document,exact:true}).selectOption('original');
 await expect(reader.locator('.document-reader-surface img')).toBeVisible();
 expect(rasterRequests).toBe(2);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

for(const locale of ['en','ar','zh-Hans'] as LocaleCode[])test(`private structured reader pins source and stays read-only: ${locale}`,async({page})=>{
 const reads:URL[]=[];let invalid=false;
 await page.setViewportSize({width:390,height:844});
 await setup(page,locale,'dark',{...file,status:'ready',error_code:'',extraction_revision:'extraction-private'} as typeof file);
 await page.route('**/attachments/*/structure?**',r=>{
  const url=new URL(r.request().url());reads.push(url);const offset=Number(url.searchParams.get('offset'));
  return r.fulfill({json:{document_id:file.id,revision:'extraction-private',format:'pdf',method:'native',units:[{id:'page-1',kind:'page',index:1,span:{start_byte:0,end_byte:25}}],blocks:[{id:offset?'note':'clause',unit_id:'page-1',kind:'paragraph',method:'native',span:{start_byte:0,end_byte:25},text:offset?'Exception: 31 July.':'Applications: 30 June.',text_sha256:'d'.repeat(64),editable:false,truncated:false,metadata_omitted:false}],offset,total_blocks:2,next_offset:offset?null:1,relationships:[],relationship_offset:0,total_relationships:0,next_relationship_offset:null,can_correct:invalid}});
 });
 await page.getByRole('button',{name:documentReader(locale).open,exact:true}).click();
 await readerOptions(page);await page.getByRole('button',{name:documentReview(locale).title,exact:true}).click();
 const panel=page.locator('.document-review');
 await expect(panel.locator('.document-review-blocks pre')).toHaveText('Applications: 30 June.');
 await expect(panel.getByText(documentReview(locale).readOnly,{exact:true})).toBeVisible();
 await expect(panel.getByRole('button',{name:documentReview(locale).edit,exact:true})).toHaveCount(0);
 await panel.getByRole('button',{name:documentReader(locale).next,exact:true}).click();
 await expect(panel.locator('.document-review-blocks pre')).toHaveText('Exception: 31 July.');
 expect(reads.every(url=>url.searchParams.get('revision')===file.revision)).toBe(true);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 invalid=true;
 await panel.getByRole('button',{name:documentReader(locale).previous,exact:true}).click();
 await expect(panel.getByRole('alert')).toHaveText(documentReview(locale).unavailable);
 await expect(panel.locator('.document-review-blocks')).toHaveCount(0);
});

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`extraction limits survive ready status: ${locale}`,async({page})=>{
 const evidence={format:'docx',status:'text_only',total_units:1,readable_units:1,warnings:[{code:'document_text_only',message:'Body text only'}]};
 await page.setViewportSize({width:620,height:1000});
 await setup(page,locale,'light',{...file,name:'Policy.docx',status:'ready',error_code:'',evidence} as typeof file);
 const notice=page.locator('.file-attachments [data-extraction-coverage="text_only"]');
 await expect(notice).toHaveText(documentEvidenceNotice(locale));
 await expect(page.locator('.file-attachments')).toContainText(documentFormatLimits(locale,'docx'));
 if(locale!=='en')expect(await notice.textContent()).not.toEqual(documentEvidenceNotice('en'));
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
});
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

test('document search uses a single subtle focus boundary and high-contrast fallback', async ({page}) => {
 await setup(page);
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 await readerOptions(page);await page.getByRole('button',{name:'Search extracted text',exact:true}).click();
 const search=page.locator('.document-search input'), field=page.locator('.document-search');
 await search.fill('result');
 await expect(search).toBeFocused();
 await expect(search).toHaveCSS('outline-style','none');
 await expect(field).toHaveCSS('border-top-width','1px');
 await expect(field).toHaveCSS('outline-style','none');
 await page.emulateMedia({forcedColors:'active'});
 await expect(field).toHaveCSS('outline-style','solid');
 await expect(search).toHaveCSS('outline-style','none');
 await expect(search).toHaveValue('result');
 await search.press('Escape');
 await expect(search).toHaveCount(0);
 await expect(page.locator('.document-options > button')).toBeFocused();
 await expect(page.locator('.document-reader')).toBeVisible();
});

for(const width of [390,1280]) test(`ready attachment never collapses into a vertical strip: ${width}`,async({page})=>{
 await page.setViewportSize({width,height:1000});
 await setup(page,'en','dark',{...file,status:'ready',error_code:'',coverage:'native_text_only'} as typeof file);
 if(width>1000)await page.getByRole('button',{name:'Open document',exact:true}).click();
 const card=page.locator('.file-attachments li'), summary=card.locator('.file-summary');
 await expect(card).toContainText('Images and figures have not been interpreted');
 const bounds=await summary.boundingBox();expect(bounds!.width).toBeGreaterThan(140);expect(bounds!.width).toBeLessThan(200);expect(bounds!.height).toBeLessThan(160);
 expect((await card.boundingBox())!.height).toBeLessThan(240);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBeTruthy();
 await page.screenshot({path:test.info().outputPath(`attachment-${width}.png`)});
});

for(const theme of ['light','dark'] as const)test(`document workspace focus preserves draft, page and zoom: ${theme}`,async({page})=>{
 await page.setViewportSize({width:1440,height:1000});
 await setup(page,'en',theme,{...file,status:'ready',error_code:''});
 // An explicitly synthetic page gives visual review actual typography/content,
 // rather than the one-pixel raster used in behavioral tests.
 const raster=await page.evaluate(()=>{const c=document.createElement('canvas');c.width=720;c.height=960;const x=c.getContext('2d')!;x.fillStyle='#fff';x.fillRect(0,0,720,960);x.fillStyle='#727272';x.font='14px sans-serif';x.fillText('SYNTHETIC TEST RECORD / 2026',64,62);x.fillStyle='#222';x.font='bold 30px sans-serif';x.fillText('Regional procurement policy',64,124);x.font='17px sans-serif';['1. Scope','This policy applies to regional purchasing teams.','Original records must be retained for review.','','2. Approval','Purchases above 17,250 require prior approval.','Record the approving officer and effective date.','','Exception','Emergency expenditure must be documented','and reviewed within seven days.','','Evidence note','This page is isolated UI test data. It is not an','actual departmental policy or a model answer.'].forEach((line,i)=>x.fillText(line,64,190+i*34));return c.toDataURL().split(',')[1];});
 await page.route('**/attachments/*/preview*',r=>new URL(r.request().url()).searchParams.has('page')?r.fulfill({contentType:'image/png',body:Buffer.from(raster,'base64')}):r.fallback());
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader');await expect(reader.locator('img')).toBeVisible();
 await page.screenshot({path:test.info().outputPath(`workspace-split-${theme}.png`)});
 const width=(await reader.boundingBox())!.width;
 await reader.getByRole('button',{name:'Chat',exact:true}).click();
 await expect(page.locator('.document-chat')).toBeHidden();
 expect((await reader.boundingBox())!.width).toBeGreaterThan(width+200);
 await reader.getByRole('button',{name:'Next page',exact:true}).click();
 await reader.getByRole('combobox',{name:'Zoom',exact:true}).selectOption('125');
 await page.screenshot({path:test.info().outputPath(`workspace-focused-${theme}.png`)});
 await reader.getByRole('button',{name:'Chat',exact:true}).click();
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await expect(reader.getByRole('spinbutton')).toHaveValue('2');
 await expect(reader.getByRole('combobox',{name:'Zoom',exact:true})).toHaveValue('125');
 await reader.getByRole('button',{name:'Close document',exact:true}).click();
 await expect(page.getByRole('button',{name:'Open document',exact:true})).toBeFocused();
});

test('new document chat gives the paper priority without losing the draft',async({page})=>{
 await page.setViewportSize({width:1440,height:900});
 await setup(page,'en','dark',{...file,status:'ready',error_code:''});
 const session={name:'Demo briefing',model_id:'Local-Research-7B-Instruct-Q4_K_M-long-model-name',messages:[]};
 await page.route('**/v1/sessions',r=>r.fulfill({json:{sessions:[session]}}));
 await page.route('**/v1/sessions/Demo%20briefing',r=>r.fulfill({json:session}));
 await page.reload();
 await expect(page.locator('.work-home h2')).toBeVisible();
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 await expect(page.locator('.work-home-actions')).toBeHidden();
 await expect(page.locator('.work-home h2')).toHaveCSS('font-size','24px');
 expect((await page.locator('.work-home').boundingBox())!.height).toBeLessThan(240);
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await page.screenshot({path:test.info().outputPath('document-first-question.png')});
});

test('reopening the current attachment retains the reading position and zoom',async({page})=>{
 await page.setViewportSize({width:1440,height:900});
 await setup(page,'en','dark',{...file,status:'ready',error_code:''});
 const open=page.getByRole('button',{name:'Open document',exact:true});
 await open.click();
 const reader=page.locator('.document-reader');
 await reader.getByRole('button',{name:documentReader('en').next,exact:true}).click();
 await reader.getByRole('combobox',{name:documentReader('en').zoom,exact:true}).selectOption('150');
 await open.click();
 await expect(reader.getByRole('spinbutton')).toHaveValue('2');
 await expect(reader.getByRole('combobox',{name:documentReader('en').zoom,exact:true})).toHaveValue('150');
 await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
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
 await expect(page.locator('.empty-chat')).toHaveCount(0);
 await notice.getByRole('button',{name:'Response settings',exact:true}).click();
 await expect(page.getByRole('region',{name:'Response settings',exact:true})).toBeVisible();
 await page.locator('.composer-settings-link[href="#/settings/responses"]').click();
 await expect(page.locator('.response-settings')).toBeVisible();
 await expect(page.getByRole('combobox',{name:'Context profile',exact:true})).toHaveValue('interactive');
 await expect(page.locator('.response-settings')).toContainText('This limits one answer, not the conversation.');
 await page.getByRole('combobox',{name:'Maximum response tokens',exact:true}).selectOption('4096');
 await expect(page.getByRole('combobox',{name:'Maximum response tokens',exact:true})).toHaveValue('4096');
 await page.locator('.sidebar a[href="#/chat"]').click();
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
 await readerOptions(page);await reader.getByRole('button',{name:'Search extracted text',exact:true}).click();await reader.getByRole('searchbox').fill('42');await reader.locator('.document-search-results').getByRole('button',{name:'Page 2',exact:true}).click();
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
 await readerOptions(page);await reader.getByRole('button',{name:'Search extracted text',exact:true}).click();await reader.getByRole('searchbox').focus();await page.keyboard.press('PageUp');
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
 await expect(page.locator('.document-reader').getByRole('tab',{name:file.name,exact:true})).toBeFocused();
 await readerOptions(page);await page.getByRole('button',{name:copy.search,exact:true}).click();
 await expect(page.getByRole('searchbox',{name:copy.search})).toBeVisible();
 await expect(page.locator('.document-reader')).toContainText(copy.partial);
 await expect(page.getByRole('searchbox',{name:copy.search})).toBeFocused();
 await page.keyboard.press('Escape');await expect(page.getByRole('searchbox',{name:copy.search})).toHaveCount(0);
 await expect(page.locator('.document-reader')).toBeVisible();
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
 // Real PDF rasterization may queue behind another document import in CI.
 await expect(page.locator('.document-reader img')).toBeVisible({timeout:30_000});
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
 expect(await scan.evaluate(element=>getComputedStyle(element,'::after').content)).toBe('none');
 expect(await scan.evaluate(element=>getComputedStyle(element,'::before').animationName)).toBe(reduced?'none':'document-loading');
 expect(await scan.evaluate(element=>parseFloat(getComputedStyle(element,'::before').width))).toBeLessThanOrEqual(16);
 if(!reduced){const transform=await scan.evaluate(element=>getComputedStyle(element,'::before').transform);await expect.poll(()=>scan.evaluate(element=>getComputedStyle(element,'::before').transform)).not.toBe(transform);}
 await page.screenshot({path:test.info().outputPath(`preparation-area-${reduced}.png`)});
 await page.evaluate(()=>document.documentElement.dataset.reduceMotion='true');
 expect(await scan.evaluate(element=>getComputedStyle(element,'::before').animationName)).toBe('none');
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
 await tabs.nth(1).click();await expect(page.locator('.document-page-navigation input')).toHaveValue('2');
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
  await page.locator('.document-reader input[type=number]').press('Enter');
  await expect(page.locator('.document-reader img')).toHaveAttribute('alt',/13$/,{timeout:60_000});
  await expect(page.locator('.document-reader img')).toHaveJSProperty('naturalWidth',1700);
  await expect(page.locator('.document-reader [role=alert]')).toHaveCount(0);
 }
 const destination=process.env.OFFGRID_TEST_DOCUMENT_SAMPLE_OUTPUT;
 if(destination){await page.screenshot({path:resolve(destination,'reader.png')});writeFileSync(resolve(destination,'receipt.json'),JSON.stringify({sha256:sha,pages:result.pages,characters:result.units.reduce((n:number,u:{text:string})=>n+[...u.text].length,0),review_pages:result.units.filter((u:{state:string})=>u.state==='native_text_with_visuals').map((u:{index:number})=>u.index),coverage:'native_text_only'},null,2));}
 expect((await (await request.get(`/v1/sessions/${name}`)).json()).messages).toHaveLength(0);
});

test('page edits commit once and original download failures do not masquerade as page failures',async({page})=>{
 await setup(page);
 const pages:number[]=[];
 await page.route('**/attachments/*/preview*',r=>{
  const u=new URL(r.request().url());
  if(u.searchParams.has('page')){pages.push(Number(u.searchParams.get('page')));return r.fulfill({contentType:'image/png',body:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j5ZkAAAAASUVORK5CYII=','base64')});}
  return r.fulfill({json:{name:file.name,sha256:file.sha256,format:'pdf',state:'complete',pages:37,units:Array.from({length:37},(_,i)=>({kind:'page',index:i+1,text:'Page '+(i+1),state:'ready',source_sha256:file.sha256}))}});
 });
 await page.route('**/attachments/*/original',r=>r.fulfill({status:503,json:{error:{message:'Original temporarily unavailable'}}}));
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader'), number=reader.getByRole('spinbutton');
 await expect(reader.locator('img')).toBeVisible();
 await number.fill('');await number.pressSequentially('13');
 expect(pages).toEqual([1]);await number.press('Enter');await expect.poll(()=>pages).toEqual([1,13]);
 await number.fill('');await number.press('Tab');await expect(number).toHaveValue('13');
 await number.fill('900');await number.press('Enter');await expect(number).toHaveValue('37');
 await reader.getByRole('button',{name:'Download original file',exact:true}).click();
 await expect(reader.locator('.document-download-error')).toContainText('Original temporarily unavailable');
 await expect(reader.locator('.document-page-error')).toHaveCount(0);await expect(reader.locator('img')).toBeVisible();
});

for(const locale of ['en','ar'] as const)test('reader pane resize preserves source and draft '+locale,async({page},info)=>{
 await page.setViewportSize({width:1440,height:960});await setup(page,locale,'dark',{...file,status:'ready',error_code:''});
 const copy=documentReader(locale);await page.getByRole('button',{name:copy.open,exact:true}).click();
 const separator=page.getByRole('separator',{name:copy.document}),reader=page.locator('.document-reader');
 await expect(separator).toHaveAttribute('aria-valuenow','40');const before=(await reader.boundingBox())!.width;
 await separator.focus();await page.keyboard.press(locale==='ar'?'ArrowLeft':'ArrowRight');
 await expect(separator).toHaveAttribute('aria-valuenow','42');expect((await reader.boundingBox())!.width).toBeLessThan(before);
 await reader.getByRole('button',{name:copy.close,exact:true}).click();await page.getByRole('button',{name:copy.open,exact:true}).click();
 await expect(separator).toHaveAttribute('aria-valuenow','42');await expect(page.locator('.composer textarea')).toHaveValue('Explain this paper');
 await page.screenshot({path:info.outputPath('resizable-reader-'+locale+'.png')});
 await page.setViewportSize({width:390,height:844});await expect(separator).not.toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
