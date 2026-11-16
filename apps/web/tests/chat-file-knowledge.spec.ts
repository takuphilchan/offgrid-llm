import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { fileKnowledge } from '../src/i18n/file-knowledge';
import { fileRecovery } from '../src/i18n/chat-files';
import { knowledgeImport } from '../src/i18n/knowledge-import';
import type { LocaleCode } from '../src/i18n';
import {collectionDestination} from '../src/i18n/collection-destination';
import {languageRegistry} from '../src/i18n/language';

const source = { id: 'file-'+'a'.repeat(32), name: 'Annual report café.txt', media_type: 'text/plain', bytes: 15, sha256: 'b'.repeat(64), status: 'ready', revision: 'c'.repeat(64), created_at:'2026-11-02T00:00:00Z' };
async function setup(page: Page, locale: LocaleCode = 'en', mode = 'ready') {
 await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1',...(mode==='legacy'?[]:['document-attachment-imports-v1','session-document-corrections-v1'])]}}));
 await page.route('**/v1/sessions/*/attachments',r=>r.fulfill({json:{attachments:[source]}}));
 await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:mode!=='disabled',stats:{}}}));
 const policy={id:'collection-private',version:1,revision:2,name:'Private records',owner:'local',readers:['reader-fixture'],editors:[]};
 await page.route('**/v1/collections',r=>r.fulfill({json:{collections:[policy]}}));
 await page.route('**/v1/collections?*',r=>r.fulfill({json:policy}));
 const writes: string[]=[];
 await page.route(/\/v1\/(?:documents|collections)\/imports/,r=>{
  writes.push(r.request().method()+' '+new URL(r.request().url()).search);
  const retry=new URL(r.request().url()).searchParams.get('action')==='retry';
  if (!retry) { expect(new URL(r.request().url()).pathname).toBe('/v1/collections/imports'); expect(r.request().postDataJSON()).toEqual({attachment:{session:'Demo briefing',id:source.id,sha256:source.sha256,revision:source.revision}}); }
  if(mode==='lost')return r.abort('failed');
  return r.fulfill({status:retry?200:202,json:{id:'import-copy',collection_id:policy.id,collection_revision:policy.revision,name:source.name,sha256:source.sha256,state:retry?'ready':mode==='interrupted'?'interrupted':'accepted',...(retry?{document_id:'published'}:{})}});
 });
 await page.goto('/ui/#/chat');
 await page.getByRole('button',{name:'Demo briefing',exact:true}).click();
 await page.locator('.composer textarea').fill('Keep my question');
 await page.locator('.composer-attachments-menu > button').click();
 await page.getByRole('button',{name:fileRecovery(locale).saved,exact:true}).click();
 return writes;
}
for(const locale of Object.keys(languageRegistry) as LocaleCode[])test(`private file publication is explicit and localized: ${locale}`,async({page},info)=>{
 const writes=await setup(page,locale),copy=fileKnowledge(locale);
 await page.setViewportSize({width:390,height:844});
 await page.locator('.saved-file-actions').getByRole('button',{name:copy.title}).click();
 const dialog=page.getByRole('dialog'),destination=collectionDestination(locale);await expect(dialog).toContainText(destination.copy);
 expect(writes).toEqual([]);
 await expect(dialog.getByRole('button',{name:copy.title,exact:true})).toBeDisabled();
 await dialog.getByRole('combobox',{name:destination.destination,exact:true}).selectOption('collection-private');
 await expect(dialog).toContainText('reader-fixture');
 await expect(dialog.getByRole('button',{name:copy.title,exact:true})).toBeDisabled();
 await dialog.getByRole('checkbox',{name:destination.confirm}).check();
 await dialog.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(dialog).toContainText(knowledgeImport(locale).added);
 expect(writes).toEqual(['POST ?collection_id=collection-private&collection_revision=2','POST ?id=import-copy&action=retry']);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 if(locale==='en'||locale==='ar')await page.screenshot({path:info.outputPath(`publication-${locale}.png`),fullPage:true});
 await page.keyboard.press('Escape');
 await expect(page.locator('.composer textarea')).toHaveValue('Keep my question');
 // showModal closes the attachment popover. Focus returns to its visible
 // trigger, and reopening restores the retained source list without reimport.
 await expect(page.locator('.composer-attachments-menu > button')).toBeFocused();
 await page.locator('.composer-attachments-menu > button').click();
 await expect(page.locator('.saved-file-actions').getByRole('button',{name:copy.title})).toBeVisible();
 expect(writes).toHaveLength(2);
});
for(const mode of ['disabled','lost','interrupted','legacy'])test(`publication preserves recovery boundaries: ${mode}`,async({page})=>{
 const writes=await setup(page,'en',mode),copy=fileKnowledge('en');
 if(mode==='legacy'){await expect(page.getByRole('button',{name:copy.title,exact:true})).toHaveCount(0);expect(writes).toEqual([]);return;}
 await page.locator('.saved-file-actions').getByRole('button',{name:copy.title}).click();
 const dialog=page.getByRole('dialog');
 if(mode==='disabled'){await expect(dialog).toContainText(copy.setup);expect(writes).toEqual([]);}
 else{const destination=collectionDestination('en');await dialog.getByRole('combobox',{name:destination.destination,exact:true}).selectOption('collection-private');await dialog.getByRole('checkbox',{name:destination.confirm}).check();await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog).toContainText(knowledgeImport('en').unknown);expect(writes).toEqual(['POST ?collection_id=collection-private&collection_revision=2']);await expect(dialog.getByRole('button',{name:copy.title,exact:true})).toHaveCount(0);}
 await page.keyboard.press('Escape');await expect(page.locator('.composer textarea')).toHaveValue('Keep my question');
});

test('real service copies exact private bytes into one durable import without indexing',async({request})=>{
 test.skip(!process.env.OFFGRID_E2E_URL,'Requires disposable service');
 const session='private-copy-test';
 expect((await request.post('/v1/sessions',{data:{name:session,model_id:'fixture-model'}})).ok()).toBeTruthy();
 const bytes=Buffer.from('Original café 日本語\n');
 const upload=await request.post(`/v1/sessions/${session}/attachments`,{multipart:{file:{name:'copy.txt',mimeType:'text/plain',buffer:bytes}}});
 expect(upload.status()).toBe(200);const {attachment}=await upload.json();
 const data={attachment:{session,id:attachment.id,sha256:attachment.sha256}};
 const created=await request.post('/v1/collections',{data:{name:'Private attachment test'}});expect(created.status()).toBe(201);const collection=await created.json();
 const target=`/v1/collections/imports?collection_id=${collection.id}&collection_revision=${collection.revision}`;
 const accepted=await request.post(target,{data});expect(accepted.status()).toBe(202);const record=await accepted.json();
 expect(record).toMatchObject({state:'accepted',attempt:0,collection_id:collection.id,collection_revision:collection.revision,sha256:attachment.sha256,bytes:bytes.length});
 const repeated=await request.post(target,{data});expect((await repeated.json()).id).toBe(record.id);
 const original=await request.get(`/v1/sessions/${session}/attachments/${attachment.id}/original`);expect(await original.body()).toEqual(bytes);
 const removed=await request.delete(`/v1/documents/imports?id=${record.id}`);expect(removed.ok()).toBeTruthy();
 expect((await request.get(`/v1/sessions/${session}/attachments/${attachment.id}/original`)).ok()).toBeTruthy();
});

test('changing or reloading the destination discards consent; a stale audience is never silently accepted',async({page})=>{
 const writes=await setup(page),copy=collectionDestination('en');
 await page.route('**/v1/collections',r=>r.fulfill({json:{collections:[{id:'workspace',version:1,revision:1,name:'Workspace',readers:[],editors:[]},{id:'collection-private',version:1,revision:2,name:'Private',owner:'local',readers:[],editors:[]}]}}));
 await page.route('**/v1/collections?*',r=>r.fulfill({json:new URL(r.request().url()).searchParams.get('id')==='workspace'?{id:'workspace',version:1,revision:1,name:'Workspace',readers:[],editors:[]}:{id:'collection-private',version:1,revision:2,name:'Private',owner:'local',readers:[],editors:[]}}));
 await page.locator('.saved-file-actions').getByRole('button',{name:fileKnowledge('en').title}).click();
 const dialog=page.getByRole('dialog'),select=dialog.getByRole('combobox',{name:copy.destination}),publish=dialog.getByRole('button',{name:fileKnowledge('en').title,exact:true});
 await select.selectOption('workspace');await expect(dialog).toContainText(copy.shared);await dialog.getByRole('checkbox').check();await expect(publish).toBeEnabled();
 await select.selectOption('collection-private');await expect(dialog.getByRole('checkbox')).not.toBeChecked();await expect(publish).toBeDisabled();
 await dialog.getByRole('checkbox').check();await dialog.getByRole('button',{name:copy.refresh}).click();await expect(select).toHaveValue('');await expect(publish).toBeDisabled();
 await select.selectOption('collection-private');await dialog.getByRole('checkbox').check();
 let attempts=0;await page.route('**/v1/collections/imports?*',r=>{attempts++;expect(new URL(r.request().url()).searchParams.get('collection_revision')).toBe('2');return r.fulfill({status:409,json:{error_code:'collection_conflict'}});});
 await publish.click();await expect(dialog.getByRole('alert')).toContainText('Collection details could not be confirmed');
 await expect(dialog).toContainText(knowledgeImport('en').unknown);expect(attempts).toBe(1);expect(writes).toEqual([]);
});

test('new private collection still needs audience acknowledgement and stays keyboard accessible at 200 percent',async({page},info)=>{
 const writes=await setup(page,'ar'),copy=collectionDestination('ar');
 let creates=0;
 await page.route('**/v1/collections',r=>{
  if(r.request().method()==='GET')return r.fulfill({json:{collections:[]}});
  creates++;expect(r.request().postDataJSON()).toEqual({name:'Review records'});
  return r.fulfill({status:201,json:{id:'collection-new',version:1,revision:1,name:'Review records',owner:'local',readers:[],editors:[]}});
 });
 await page.locator('.saved-file-actions').getByRole('button',{name:fileKnowledge('ar').title}).click();
 await page.setViewportSize({width:780,height:1000});
 await page.evaluate(()=>{document.documentElement.style.zoom='2';});
 const dialog=page.getByRole('dialog');await dialog.locator('summary').click();
 await dialog.getByRole('textbox',{name:copy.name}).fill('Review records');
 await dialog.getByRole('button',{name:copy.create,exact:true}).click();
 await expect(dialog.getByRole('combobox')).toHaveValue('collection-new');await expect(dialog).toContainText('local');
 const publish=dialog.getByRole('button',{name:fileKnowledge('ar').title,exact:true});await expect(publish).toBeDisabled();
 await dialog.getByRole('checkbox').focus();await page.keyboard.press('Space');await expect(publish).toBeEnabled();
 expect(creates).toBe(1);expect(writes).toEqual([]);
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 await page.screenshot({path:info.outputPath('private-collection-rtl-zoom.png'),fullPage:true});
 await page.keyboard.press('Escape');await expect(page.locator('.composer textarea')).toHaveValue('Keep my question');
});

test('lost collection creation requires inspection and does not replay or publish',async({page})=>{
 const writes=await setup(page),copy=collectionDestination('en');let creates=0;
 await page.route('**/v1/collections',r=>r.request().method()==='GET'?r.fulfill({json:{collections:[]}}):(creates++,r.abort('failed')));
 await page.locator('.saved-file-actions').getByRole('button',{name:fileKnowledge('en').title}).click();
 const dialog=page.getByRole('dialog');await dialog.locator('summary').click();await dialog.getByRole('textbox',{name:copy.name}).fill('Private');
 await dialog.getByRole('button',{name:copy.create,exact:true}).click();await expect(dialog.getByRole('alert')).toBeVisible();
 await expect(dialog.getByRole('button',{name:copy.create,exact:true})).toBeDisabled();
 await expect(dialog.getByRole('button',{name:fileKnowledge('en').title,exact:true})).toBeDisabled();expect(creates).toBe(1);expect(writes).toEqual([]);
});

test('older collection service cannot fall back to shared attachment publication',async({page})=>{
 const writes=await setup(page);
 await page.route('**/v1/collections',r=>r.fulfill({status:404,json:{error:'unsupported'}}));
 await page.locator('.saved-file-actions').getByRole('button',{name:fileKnowledge('en').title}).click();
 const dialog=page.getByRole('dialog');await expect(dialog.getByRole('alert')).toBeVisible();
 await expect(dialog.getByRole('button',{name:fileKnowledge('en').title,exact:true})).toBeDisabled();expect(writes).toEqual([]);
});
