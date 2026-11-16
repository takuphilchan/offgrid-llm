import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';
import {ocrSetup} from '../src/i18n/ocr-setup';
import {ocrChoice} from '../src/i18n/ocr-choice';
import catalog from '../../../internal/documentocr/languages.json' with {type:'json'};
import {reviewKnowledgeImport,collectionImportReceipt} from './helpers/collection-review';

for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`explicit OCR selection stays separate from interface language: ${locale}`,async({page},info)=>{
 await craftFixture(page,locale,locale==='ar'?'dark':'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1','document-ocr-language-selection-v1']}}));
 await page.route('**/v1/document-ocr-languages',r=>r.fulfill({json:{selection_mode:'single_language',manifest_revision:catalog.source_revision,languages:catalog.packs.map(p=>({...p,state:p.language==='chi_sim'?'installed_verified':'missing'})),unavailable_interface_locales:catalog.unavailable_interface_locales,document_formats:[]}}));
 const uploads:string[]=[];
 await page.route('**/v1/sessions/*/attachments*',r=>{
  if(r.request().method()==='GET')return r.fulfill({json:{attachments:[]}});
  const language=new URL(r.request().url()).searchParams.get('ocr_language');uploads.push(language??'');
  return r.fulfill({json:{attachment:{id:'file-'+'a'.repeat(32),name:'new-file.txt',bytes:9,sha256:'b'.repeat(64),revision:'c'.repeat(64),status:'ready',media_type:'text/plain',created_at:'2026-11-14T00:00:00Z',ocr_language:language}}});
 });
 if(locale==='ar')await page.setViewportSize({width:390,height:844});
 await page.goto('/ui/#/chat');
 await page.locator('.composer-attachments-menu > button').click();
 await page.getByRole('button',{name:ocrSetup(locale).title,exact:true}).click();
 const dialog=page.getByRole('dialog'),select=dialog.getByLabel(ocrChoice(locale).language,{exact:true});
 await expect(select).toHaveValue('eng');await expect(select.locator('option')).toHaveCount(25);
 await expect(dialog).toContainText('ChiShona');await expect(dialog).toContainText('isiNdebele');
 await select.selectOption('chi_sim');await expect(dialog).toContainText(ocrSetup(locale).installed);
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 if(['en','ar'].includes(locale))await page.screenshot({path:info.outputPath(`ocr-language-${locale}.png`)});
 await dialog.getByRole('button',{name:ocrChoice(locale).close,exact:true}).click();
 await page.locator('input[data-attachment="files"]').setInputFiles({name:'new-file.txt',mimeType:'text/plain',buffer:Buffer.from('new text')});
 await expect.poll(()=>uploads).toEqual(['chi_sim']);
 await expect(page.locator('.composer .file-attachments')).toContainText('new-file.txt');
});

for(const locale of ['en','ar'] as LocaleCode[])test(`Knowledge saves its explicit OCR language with the reviewed audience: ${locale}`,async({page})=>{
 await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1','knowledge-collections-v1','document-imports-v1','document-ocr-language-selection-v1']}}));
 await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:true,stats:{},document_formats:[{id:'image_document',extensions:['.png'],runtime_state:'installed_unchecked',max_file_bytes:24*1024*1024}]}}));
 await page.route('**/v1/documents/ocr-languages',r=>r.fulfill({json:{selection_mode:'single_language',languages:catalog.packs.map(p=>({...p,state:'missing'})),unavailable_interface_locales:catalog.unavailable_interface_locales,document_formats:[]}}));
 const selections:string[]=[];
 await page.route('**/v1/collections/imports?*',r=>{selections.push(new URL(r.request().url()).searchParams.get('ocr_language')??'');return r.fulfill({status:202,json:{...collectionImportReceipt('import-fixture'),ocr_language:'fra'}});});
 await page.route('**/v1/documents/imports*',r=>r.fulfill({json:new URL(r.request().url()).searchParams.has('id')?{...collectionImportReceipt('import-fixture','ready'),ocr_language:'fra'}:{imports:[]}}));
 await page.goto('/ui/#/knowledge');
 await page.locator('[data-knowledge-import]').setInputFiles({name:'record.png',mimeType:'image/png',buffer:Buffer.from('authored upload transport fixture')});
 await page.getByRole('dialog').getByLabel(ocrChoice(locale).language,{exact:true}).selectOption('fra');
 await reviewKnowledgeImport(page,locale);
 await expect.poll(()=>selections).toEqual(['fra']);
 await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(1);
});

test('old service explains unavailable OCR selection and allows closing by keyboard',async({page})=>{
 await craftFixture(page,'en','light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1']}}));
 await page.goto('/ui/#/chat');await page.locator('.composer-attachments-menu > button').click();
 await page.getByRole('button',{name:ocrSetup('en').title,exact:true}).click();
 await expect(page.getByRole('dialog').getByRole('combobox')).toHaveCount(0);
 await expect(page.getByRole('dialog').getByRole('status')).toBeVisible();
 await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);
});
