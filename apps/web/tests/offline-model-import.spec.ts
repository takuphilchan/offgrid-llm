import {test,expect,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {modelImport} from '../src/i18n/model-import';

const file={name:'offline.gguf',mimeType:'application/octet-stream',buffer:Buffer.alloc(1024**2)};
async function setup(page:Page,capability=true){
 const state=await craftFixture(page,'en','dark');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:capability?['gguf-offline-import-v1']:[]}}));
 await page.goto('/ui/#/models');
 return state;
}

test('offline import requires review, reports stored receipt and preserves selection',async({page})=>{
 await page.addInitScript(()=>localStorage.setItem('offgrid.model','Saved-unavailable-model'));
 const state=await setup(page);const requests:string[]=[];
 await page.route('**/api/v2/models/import/gguf?*',async r=>{requests.push(r.request().url());expect(r.request().postDataBuffer()?.length).toBe(file.buffer.length);await r.fulfill({status:201,json:{model_id:'offline',filename:file.name,size:file.buffer.length,sha256:'a'.repeat(64),inventory_refreshed:true}});});
 const selected=await page.evaluate(()=>localStorage.getItem('offgrid.model'));
 await page.getByText('Import a model file',{exact:true}).click();
 const panel=page.locator('.offline-model-import');
 await panel.getByLabel('Choose GGUF file',{exact:true}).setInputFiles(file);
 expect(requests).toEqual([]);
 await panel.getByRole('button',{name:'Import file',exact:true}).click();
 await expect(panel.locator('p[role=status]')).toContainText('Runtime compatibility is not yet verified');
 expect(requests).toHaveLength(1);
 expect(await page.evaluate(()=>localStorage.getItem('offgrid.model'))).toBe(selected);
 expect(state.writes).toEqual([]);
 await page.screenshot({path:test.info().outputPath('offline-model-import.png'),fullPage:true});
});

test('unsupported files never become an import request',async({page})=>{
 const state=await setup(page);await page.getByText('Import a model file',{exact:true}).click();
 const panel=page.locator('.offline-model-import');
 for(const invalid of [{...file,name:'notes.txt'},{...file,buffer:Buffer.alloc(20)},{...file,name:'model-00001-of-00002.gguf'},{...file,name:'mmproj-model.gguf'}]){
  await panel.getByLabel('Choose GGUF file',{exact:true}).setInputFiles(invalid);
  await expect(panel.getByRole('alert')).toBeVisible();
  await expect(panel.getByRole('button',{name:'Import file',exact:true})).toHaveCount(0);
 }
 expect(state.writes).toEqual([]);
});

test('lost response requires inspection and never automatically replays the upload',async({page})=>{
 await setup(page);let calls=0;
 await page.route('**/api/v2/models/import/gguf?*',async r=>{calls++;await r.abort('failed');});
 await page.getByText('Import a model file',{exact:true}).click();
 const panel=page.locator('.offline-model-import');
 await panel.getByLabel('Choose GGUF file',{exact:true}).setInputFiles(file);
 await panel.getByRole('button',{name:'Import file',exact:true}).click();
 await expect(panel.locator('p[role=status]')).toContainText('not confirmed');
 await expect(panel.getByRole('button',{name:'Choose GGUF file',exact:true})).toBeDisabled();
 await page.goto('/ui/#/chat');await page.goto('/ui/#/models');
 await page.getByText('Import a model file',{exact:true}).click();
 await expect(panel.locator('p[role=status]')).toContainText('not confirmed');
 await panel.getByRole('button',{name:'Refresh installed models',exact:true}).click();
 await expect(panel.getByRole('button',{name:'Choose GGUF file',exact:true})).toBeEnabled();
 expect(calls).toBe(1);
});

test('confirmed rejection remains distinct from an unknown outcome',async({page})=>{
 await setup(page);
 await page.route('**/api/v2/models/import/gguf?*',r=>r.fulfill({status:409,json:{error:{code:'gguf_import_exists',message:'Existing model preserved.'}}}));
 await page.getByText('Import a model file',{exact:true}).click();
 const panel=page.locator('.offline-model-import');await panel.getByLabel('Choose GGUF file',{exact:true}).setInputFiles(file);
 await panel.getByRole('button',{name:'Import file',exact:true}).click();
 await expect(panel.getByRole('alert')).toHaveText('Existing model preserved.');
 await expect(panel.getByRole('button',{name:'Import file',exact:true})).toHaveCount(0);
});

test('older services do not expose an unsupported import',async({page})=>{
 await setup(page,false);await expect(page.locator('.installed-model')).toBeVisible();
 await expect(page.locator('.offline-model-import')).toHaveCount(0);
});

test('cancelling a moving upload indicator preserves uncertainty without replay',async({page})=>{
 await setup(page);let release!:()=>void;const pending=new Promise<void>(resolve=>{release=resolve;});let calls=0;
 await page.route('**/api/v2/models/import/gguf?*',async r=>{calls++;await pending;await r.abort('aborted').catch(()=>{});});
 await page.getByText('Import a model file',{exact:true}).click();const panel=page.locator('.offline-model-import');
 await panel.getByLabel('Choose GGUF file',{exact:true}).setInputFiles(file);
 await panel.getByRole('button',{name:'Import file',exact:true}).click();
 await expect(panel.locator('.offline-import-spinner')).toHaveCSS('animation-name','offline-import-spin');
 await page.emulateMedia({reducedMotion:'reduce'});
 await expect(panel.locator('.offline-import-spinner')).toHaveCSS('animation-name','none');
 await expect.poll(()=>calls).toBe(1);
 await panel.getByRole('button',{name:'Cancel',exact:true}).click();release();
 await expect(panel.locator('p[role=status]')).toContainText('not confirmed');
 await expect(panel.getByRole('button',{name:'Choose GGUF file',exact:true})).toBeDisabled();
 expect(calls).toBe(1);
});

test('read-only users cannot open offline import',async({page})=>{
 await craftFixture(page,'en','light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['gguf-offline-import-v1']}}));
 await page.route('**/v1/users/me',r=>r.fulfill({json:{authenticated:true,auth_required:true,user:{id:'reader',username:'Reader',role:'user'}}}));
 await page.route('**/v1/system/config',r=>r.fulfill({json:{require_auth:true}}));
 await page.goto('/ui/#/models');await expect(page.locator('.installed-model')).toBeVisible();
 await expect(page.locator('.offline-model-import')).toHaveCount(0);
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as const) test(`offline import remains readable at narrow width: ${locale}`,async({page})=>{
 const state=await craftFixture(page,locale,'light');
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['gguf-offline-import-v1']}}));
 await page.setViewportSize({width:390,height:844});await page.goto('/ui/#/models');
 const copy=modelImport(locale);await page.getByText(copy.title,{exact:true}).click();
 await expect(page.locator('.offline-model-import').getByRole('button',{name:copy.choose,exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true);
 expect(state.writes).toEqual([]);
});

test('real isolated service stores exact uploaded bytes without selecting or loading the model',async({page})=>{
 test.skip(!process.env.OFFGRID_E2E_URL,'Requires an explicitly configured disposable real service, never the installed workspace');
 await page.addInitScript(()=>{localStorage.setItem('offgrid.onboarding.complete','true');localStorage.setItem('offgrid.locale','en');});
 await page.goto('/ui/#/models');
 const data=Buffer.alloc(1024**2);let p=0;
 const text=(s:string)=>{data.writeBigUInt64LE(BigInt(Buffer.byteLength(s)),p);p+=8;p+=data.write(s,p);};
 data.write('GGUF',p);p+=4;data.writeUInt32LE(3,p);p+=4;data.writeBigUInt64LE(1n,p);p+=8;data.writeBigUInt64LE(1n,p);p+=8;
 text('general.architecture');data.writeUInt32LE(8,p);p+=4;text('qwen2');
 const name=`offline-storage-fixture-${Date.now()}.gguf`;
 await page.getByText('Import a model file',{exact:true}).click();const panel=page.locator('.offline-model-import');
 const selected=await page.evaluate(()=>localStorage.getItem('offgrid.model'));
 await panel.getByLabel('Choose GGUF file',{exact:true}).setInputFiles({name,mimeType:'application/octet-stream',buffer:data});
 try {
  const response=page.waitForResponse(r=>r.url().includes('/api/v2/models/import/gguf?'));
  await panel.getByRole('button',{name:'Import file',exact:true}).click();const result=await response;
  expect(result.status()).toBe(201);const receipt=await result.json();
  const {createHash}=await import('node:crypto');expect(receipt.sha256).toBe(createHash('sha256').update(data).digest('hex'));
  await expect(panel.locator('p[role=status]')).toContainText('File imported');
  await expect(page.locator('.installed-model').filter({hasText:name.slice(0,-5)}).getByRole('button',{name:'Select model',exact:true})).toBeVisible();
  expect(await page.evaluate(()=>localStorage.getItem('offgrid.model'))).toBe(selected);
  const inventory=await (await page.request.get('/v1/models')).json();expect(inventory.data.some((m:{id:string})=>m.id===name.slice(0,-5))).toBe(true);
  await page.reload();
  await expect(page.locator('.installed-model').filter({hasText:name.slice(0,-5)}).getByRole('button',{name:'Select model',exact:true})).toBeVisible();
  expect(await page.evaluate(()=>localStorage.getItem('offgrid.model'))).toBe(selected);
 } finally {
  // The service is disposable, but keep later real-service tests independent.
  const removed=await page.request.delete('/v1/models/delete',{data:{model_id:name.slice(0,-5)}});
  expect([200,404]).toContain(removed.status());
 }
});
