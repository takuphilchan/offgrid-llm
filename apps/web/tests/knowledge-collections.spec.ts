import {expect,test,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {reviewKnowledgeImport} from './helpers/collection-review';
import {collectionDestination} from '../src/i18n/collection-destination';
import {knowledgeImport} from '../src/i18n/knowledge-import';
import {languageRegistry} from '../src/i18n/language';
import type {LocaleCode} from '../src/i18n';

const files=['first 日本語.txt','second.txt','third.txt'].map(name=>({name,mimeType:'text/plain',buffer:Buffer.from('Source café 日本語')}));
const policy={id:'collection-records',name:'Department records',version:1,revision:4,owner:'local',readers:['records-reader'],editors:[]};
async function setup(page:Page,locale:LocaleCode='en',theme='light'){
 await craftFixture(page,locale,theme);
 const writes:string[]=[],pending:Record<string,object>={};
 await page.route('**/v1/collections',r=>r.fulfill({json:{collections:[policy]}}));
 await page.route('**/v1/collections?*',r=>r.fulfill({json:policy}));
 await page.route('**/v1/collections/imports?*',r=>{
  const query=new URL(r.request().url()).searchParams;expect(query.get('collection_id')).toBe(policy.id);expect(query.get('collection_revision')).toBe('4');
  const id=`import-${Object.keys(pending).length+1}`;
  writes.push(`accept ${id}`);pending[id]={id,collection_id:policy.id,collection_revision:4,sha256:'a'.repeat(64)};
  return r.fulfill({status:202,json:{...pending[id],state:'accepted'}});
 });
 await page.route('**/v1/documents/imports*',r=>{
  const query=new URL(r.request().url()).searchParams,id=query.get('id');
  if(r.request().method()==='GET')return r.fulfill({json:{imports:[]}});
  expect(query.get('action')).toBe('retry');expect(id&&pending[id]).toBeTruthy();writes.push(`retry ${id}`);
  return r.fulfill({json:{...pending[id!],state:'ready',document_id:`doc-${id}`}});
 });
 await page.goto('/ui/#/knowledge');return writes;
}

for(const [index,locale] of (Object.keys(languageRegistry) as LocaleCode[]).entries())test(`private batch review and results use the selected language: ${locale}`,async({page},info)=>{
 const writes=await setup(page,locale,index%2?'dark':'light');await page.setViewportSize({width:390,height:844});
 await page.locator('[data-knowledge-import]').setInputFiles(files.slice(0,2));
 const dialog=page.getByRole('dialog');await expect(dialog).toContainText(files[0].name);expect(writes).toEqual([]);
 await reviewKnowledgeImport(page,locale,policy.id);
 await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(2);
 expect(writes).toEqual(['accept import-1','retry import-1','accept import-2','retry import-2']);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 if(locale==='ar'||locale==='en')await page.screenshot({path:info.outputPath(`private-library-${locale}.png`),fullPage:true});
});

test('dismissing review sends no files and a later selection needs fresh consent',async({page})=>{
 const writes=await setup(page),copy=collectionDestination('en');
 const [chooser]=await Promise.all([page.waitForEvent('filechooser'),page.getByRole('button',{name:'Add document',exact:true}).click()]);
 await chooser.setFiles(files[0]);
 const dialog=page.getByRole('dialog');await dialog.getByRole('combobox').selectOption(policy.id);await dialog.getByRole('checkbox',{name:copy.confirmFiles}).check();
 await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);expect(writes).toEqual([]);
 await expect(page.getByRole('button',{name:'Add document',exact:true})).toBeFocused();
 await page.locator('[data-knowledge-import]').setInputFiles(files[1]);await expect(dialog.getByRole('combobox')).toHaveValue('');
 await expect(dialog.locator('button.primary-button')).toBeDisabled();expect(writes).toEqual([]);
});

test('audience conflict stops later files and never creates shared copies',async({page})=>{
 const writes=await setup(page);let accepted=0;
 await page.route('**/v1/collections/imports?*',r=>{
  accepted++;if(accepted===1)return r.fallback();
  return r.fulfill({status:409,json:{error_code:'collection_conflict'}});
 });
 await page.locator('[data-knowledge-import]').setInputFiles(files);await reviewKnowledgeImport(page,'en',policy.id);
 await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(1);
 await expect(page.locator('.knowledge-import-results [data-state="unknown"]')).toContainText('second.txt');
 await expect(page.locator('.knowledge-import-results [data-state="pending"]')).toContainText('third.txt');
 expect(accepted).toBe(2);expect(writes).toEqual(['accept import-1','retry import-1']);
});

test('unavailable collections do not activate a legacy shared upload',async({page})=>{
 const writes=await setup(page);let legacy=0;
 await page.route('**/v1/collections',r=>r.fulfill({status:404,json:{error:'unsupported'}}));
 await page.route('**/v1/documents/ingest',r=>{legacy++;return r.abort();});
 await page.locator('[data-knowledge-import]').setInputFiles(files[0]);
 const dialog=page.getByRole('dialog');await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.locator('button.primary-button')).toBeDisabled();
 expect(writes).toEqual([]);expect(legacy).toBe(0);
});

test('a mismatched completion receipt does not label the file added or continue the batch',async({page})=>{
 const writes=await setup(page);
 await page.route('**/v1/documents/imports*',r=>r.request().method()==='GET'?r.fulfill({json:{imports:[]}}):r.fulfill({json:{id:'another-import',collection_id:'workspace',collection_revision:1,sha256:'b'.repeat(64),state:'ready',document_id:'wrong-document'}}));
 await page.locator('[data-knowledge-import]').setInputFiles(files.slice(0,2));await reviewKnowledgeImport(page,'en',policy.id);
 await expect(page.locator('.knowledge-import-results [data-state="unknown"]')).toContainText(knowledgeImport('en').unknown);
 await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(0);expect(writes).toEqual(['accept import-1']);
});
