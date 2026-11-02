import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { fileKnowledge } from '../src/i18n/file-knowledge';
import { fileRecovery } from '../src/i18n/chat-files';
import { knowledgeImport } from '../src/i18n/knowledge-import';
import type { LocaleCode } from '../src/i18n';

const source = { id: 'file-'+'a'.repeat(32), name: 'Annual report café.txt', media_type: 'text/plain', bytes: 15, sha256: 'b'.repeat(64), status: 'ready', revision: 'c'.repeat(64), created_at:'2026-11-02T00:00:00Z' };
async function setup(page: Page, locale: LocaleCode = 'en', mode = 'ready') {
 await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['session-files-v1',...(mode==='legacy'?[]:['document-attachment-imports-v1'])]}}));
 await page.route('**/v1/sessions/*/attachments',r=>r.fulfill({json:{attachments:[source]}}));
 await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:mode!=='disabled',stats:{}}}));
 const writes: string[]=[];
 await page.route('**/v1/documents/imports*',r=>{
  writes.push(r.request().method()+' '+new URL(r.request().url()).search);
  const retry=new URL(r.request().url()).searchParams.get('action')==='retry';
  if (!retry) expect(r.request().postDataJSON()).toEqual({attachment:{session:'Demo briefing',id:source.id,sha256:source.sha256}});
  if(mode==='lost')return r.abort('failed');
  return r.fulfill({status:retry?200:202,json:{id:'import-copy',name:source.name,sha256:source.sha256,state:retry?'ready':mode==='interrupted'?'interrupted':'accepted',...(retry?{document_id:'published'}:{})}});
 });
 await page.goto('/ui/#/chat');
 await page.getByRole('button',{name:'Demo briefing',exact:true}).click();
 await page.locator('.composer textarea').fill('Keep my question');
 await page.locator('.composer-attachments-menu > button').click();
 await page.getByRole('button',{name:fileRecovery(locale).saved,exact:true}).click();
 return writes;
}
for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as LocaleCode[])test(`private file publication is explicit and localized: ${locale}`,async({page},info)=>{
 const writes=await setup(page,locale),copy=fileKnowledge(locale);
 await page.setViewportSize({width:390,height:844});
 await page.locator('.saved-file-actions').getByRole('button',{name:copy.title}).click();
 const dialog=page.getByRole('dialog');await expect(dialog).toContainText(copy.sharing);
 expect(writes).toEqual([]);
 await dialog.getByRole('button',{name:copy.title,exact:true}).click();
 await expect(dialog).toContainText(knowledgeImport(locale).added);
 expect(writes).toEqual(['POST ','POST ?id=import-copy&action=retry']);
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
 else{await dialog.getByRole('button',{name:copy.title,exact:true}).click();await expect(dialog).toContainText(knowledgeImport('en').unknown);expect(writes).toEqual(['POST ']);await expect(dialog.getByRole('button',{name:copy.title,exact:true})).toHaveCount(0);}
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
 const accepted=await request.post('/v1/documents/imports',{data});expect(accepted.status()).toBe(202);const record=await accepted.json();
 expect(record).toMatchObject({state:'accepted',attempt:0,sha256:attachment.sha256,bytes:bytes.length});
 const repeated=await request.post('/v1/documents/imports',{data});expect((await repeated.json()).id).toBe(record.id);
 const original=await request.get(`/v1/sessions/${session}/attachments/${attachment.id}/original`);expect(await original.body()).toEqual(bytes);
 const removed=await request.delete(`/v1/documents/imports?id=${record.id}`);expect(removed.ok()).toBeTruthy();
 expect((await request.get(`/v1/sessions/${session}/attachments/${attachment.id}/original`)).ok()).toBeTruthy();
});
