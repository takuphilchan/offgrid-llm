import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';
import {ocrProgress} from '../src/i18n/ocr-progress';
import {chatFiles} from '../src/i18n/chat-files';

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`saved OCR progress is visible in Chat and Knowledge: ${locale}`,async({page},info)=>{
 await craftFixture(page,locale,locale==='ar'?'dark':'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1','document-imports-v1','document-ocr-page-recovery-v1']}}));
 const file={id:'file-'+'a'.repeat(32),name:'scan.tiff',bytes:100,sha256:'b'.repeat(64),status:'failed',error_code:'document_cancelled',ocr_completed_pages:2,media_type:'image/tiff',created_at:'2026-11-14T00:00:00Z'};
 const record={id:'import-saved',name:'library-scan.tiff',state:'cancelled',error_code:'document_cancelled',bytes:100,sha256:'b'.repeat(64),attempt:1,collection_id:'workspace',collection_revision:1,ocr_completed_pages:2};
 let prepares=0;
 await page.route('**/v1/sessions/*/attachments*',r=>{
  if(r.request().method()==='GET')return r.fulfill({json:{attachments:[file]}});
  prepares++;
  return r.fulfill({status:422,json:{attachment:file,error:{code:'document_cancelled',message:'Interrupted fixture'}}});
 });
 await page.route('**/v1/documents/imports*',r=>r.fulfill({json:{imports:[record]}}));
 if(locale==='ar')await page.setViewportSize({width:390,height:844});
 await page.goto('/ui/#/chat');
 await page.locator('input[data-attachment="files"]').setInputFiles({name:'scan.tiff',mimeType:'image/tiff',buffer:Buffer.from('authored transport fixture')});
 const draft=page.locator('.composer .file-attachments');
 await expect(draft).toContainText(ocrProgress(locale,2));
 await expect(draft.getByRole('button',{name:chatFiles(locale).retry,exact:true})).toBeEnabled();
 await page.reload();
 await expect(draft).toContainText(ocrProgress(locale,2));
 expect(prepares).toBe(1);
 await page.goto('/ui/#/knowledge');
 const recovery=page.locator('.knowledge-import-recovery');
 await expect(recovery).toContainText(ocrProgress(locale,2));
 expect(await recovery.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`ocr-progress-${locale}.png`)});
});
