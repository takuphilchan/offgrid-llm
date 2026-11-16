import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';
import {ocrSetup} from '../src/i18n/ocr-setup';
import {ocrChoice} from '../src/i18n/ocr-choice';
import {ocrMixed} from '../src/i18n/ocr-mixed';
import catalog from '../../../internal/documentocr/languages.json' with {type:'json'};
import {reviewKnowledgeImport,collectionImportReceipt} from './helpers/collection-review';

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`mixed OCR retains explicit language order: ${locale}`,async({page},info)=>{
 await craftFixture(page,locale,locale==='ar'?'dark':'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1','document-ocr-language-selection-v1','document-ocr-mixed-languages-v1']}}));
 await page.route('**/v1/document-ocr-languages',r=>r.fulfill({json:{selection_mode:'ordered_languages',max_languages:3,languages:catalog.packs.map(p=>({...p,state:['chi_sim','fra','eng'].includes(p.language)?'installed_verified':'missing'})),unavailable_interface_locales:catalog.unavailable_interface_locales}}));
 const uploads:string[]=[];
 await page.route('**/v1/sessions/*/attachments*',r=>{
  if(r.request().method()==='GET')return r.fulfill({json:{attachments:[]}});
  const language=new URL(r.request().url()).searchParams.get('ocr_language');uploads.push(language??'');
  return r.fulfill({json:{attachment:{id:'file-'+'a'.repeat(32),name:'mixed.txt',bytes:9,sha256:'b'.repeat(64),revision:'c'.repeat(64),status:'ready',ocr_language:language}}});
 });
 if(locale==='ar')await page.setViewportSize({width:390,height:844});
 await page.goto('/ui/#/chat');
 await page.locator('.composer-attachments-menu > button').click();
 await page.getByRole('button',{name:ocrSetup(locale).title,exact:true}).click();
 const dialog=page.getByRole('dialog'),copy=ocrMixed(locale);
 await dialog.getByLabel(ocrChoice(locale).language,{exact:true}).selectOption('chi_sim');
 await dialog.getByRole('button',{name:copy.add,exact:true}).click();
 const next=dialog.locator('.ocr-additional-language select');
 await expect(next).toHaveValue(''); // No implicit English pack when adding a row.
 await next.selectOption('fra');
 await dialog.getByRole('button',{name:copy.add,exact:true}).click();
 await dialog.locator('.ocr-additional-language select').last().selectOption('eng');
 await expect(dialog.getByRole('button',{name:copy.add,exact:true})).toHaveCount(0);
 await expect(dialog.locator('.ocr-additional-language select').first().locator('option[value="eng"]')).toBeDisabled();
 await expect(dialog).toContainText(copy.hint);
 await expect(dialog).toContainText(copy.installed);
 if(locale==='ar')await page.evaluate(()=>document.documentElement.style.fontSize='200%');
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`mixed-ocr-${locale}.png`)});
 await dialog.getByRole('button',{name:ocrChoice(locale).close,exact:true}).click();
 await page.locator('input[data-attachment="files"]').setInputFiles({name:'mixed.txt',mimeType:'text/plain',buffer:Buffer.from('new text')});
 await expect.poll(()=>uploads).toEqual(['chi_sim+fra+eng']);
 if(locale==='en'){
  await page.locator('.composer-attachments-menu > button').click();
  await page.getByRole('button',{name:ocrSetup(locale).title,exact:true}).click();
  const remove=page.getByRole('dialog').getByRole('button',{name:`${copy.remove}: Français`,exact:true});
  await remove.focus();await page.keyboard.press('Enter');
  await expect(page.getByRole('dialog').getByLabel(ocrChoice(locale).language,{exact:true})).toBeFocused();
  await expect(page.getByRole('dialog').locator('.ocr-additional-language select')).toHaveCount(1);
  expect(uploads).toEqual(['chi_sim+fra+eng']); // Existing sources retain their original selection.
 }
});

test('Knowledge retains ordered OCR packs with its reviewed destination',async({page})=>{
 await craftFixture(page,'en','light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['knowledge-collections-v1','document-imports-v1','document-ocr-language-selection-v1','document-ocr-mixed-languages-v1']}}));
 await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:true,stats:{},document_formats:[{id:'image_document',extensions:['.png'],runtime_state:'installed_unchecked',max_file_bytes:24*1024*1024}]}}));
 await page.route('**/v1/documents/ocr-languages',r=>r.fulfill({json:{selection_mode:'ordered_languages',max_languages:3,languages:catalog.packs.map(p=>({...p,state:'installed_verified'})),document_formats:[]}}));
 const uploads:string[]=[];
 await page.route('**/v1/collections/imports?*',r=>{uploads.push(new URL(r.request().url()).searchParams.get('ocr_language')??'');return r.fulfill({status:202,json:{...collectionImportReceipt('import-fixture'),ocr_language:'eng+fra'}});});
 await page.route('**/v1/documents/imports*',r=>r.fulfill({json:new URL(r.request().url()).searchParams.has('id')?{...collectionImportReceipt('import-fixture','ready'),ocr_language:'eng+fra'}:{imports:[]}}));
 await page.goto('/ui/#/knowledge');
 await page.locator('[data-knowledge-import]').setInputFiles({name:'record.png',mimeType:'image/png',buffer:Buffer.from('fixture')});
 const dialog=page.getByRole('dialog');
 await dialog.getByRole('button',{name:ocrMixed('en').add,exact:true}).click();
 await dialog.locator('.ocr-additional-language select').selectOption('fra');
 await reviewKnowledgeImport(page,'en');
 await expect.poll(()=>uploads).toEqual(['eng+fra']);
});

test('old services receive no mixed-language upload',async({page})=>{
 await craftFixture(page,'en','light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',capabilities:['document-ocr-language-selection-v1']}}));
 let uploads=0;await page.route('**/v1/collections/imports?*',r=>{uploads++;return r.abort();});
 await page.goto('/ui/#/chat');
 const rejected=await page.evaluate(async()=>{
  const {api}=await import('/ui/src/api/client.ts');
  try{await api.collections.acceptImport('workspace',1,new File(['test'],'test.txt'),'key',undefined,'eng+fra');return false;}catch{return true;}
 });
 expect(rejected).toBe(true);expect(uploads).toBe(0);
});
