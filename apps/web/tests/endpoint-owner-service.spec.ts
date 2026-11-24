import { expect, test, type Page } from '@playwright/test';
import { readFile, writeFile, stat } from 'node:fs/promises';
import { join } from 'node:path';
import type { EndpointOwnerSnapshot } from '../src/api/endpoint-owner';

async function restart(page:Page, directory:string) {
 const token=`owner-${Date.now()}`;await writeFile(join(directory,'restart-request'),token);
 await expect.poll(()=>readFile(join(directory,'restart-ack'),'utf8').catch(()=>''),{timeout:15_000}).toBe(token);
 await page.reload();
}
async function reconcile(page:Page) {
 await page.locator('.endpoint-owner-record').getByRole('button',{name:'Reconcile recorded outcome',exact:true}).click();
 await page.getByRole('dialog').getByRole('button',{name:'Reconcile recorded outcome',exact:true}).click();
 await expect(page.getByRole('dialog')).toHaveCount(0);
}
test('actual service creates and updates owner configuration, survives a lost receipt, and requires restart plus fresh access review',async({page,request},testInfo)=>{
 const fixture=process.env.OFFGRID_E2E_ENDPOINT_OWNER_FIXTURES;
 test.skip(!fixture||!process.env.OFFGRID_E2E_URL,'Requires the disposable private owner fixture wrapper.');
 test.setTimeout(60_000);
 const ownerPath=join(fixture!,'data','inference-endpoints.json');
 const readState=async()=>{const response=await request.get('/api/v2/inference-endpoints/configuration');expect(response.ok()).toBe(true);return await response.json() as EndpointOwnerSnapshot;};
 const initial=await readState();expect(initial.write_available,initial.write_reason).toBe(true);expect(initial.entries).toEqual([]);
 const posts:string[]=[];page.on('request',request=>{if(request.method()==='POST')posts.push(new URL(request.url()).pathname);});
 await page.addInitScript(()=>{localStorage.setItem('offgrid.onboarding.complete','true');localStorage.setItem('offgrid.locale','en');});
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await page.getByRole('button',{name:'Add endpoint',exact:true}).click();const dialog=page.getByRole('dialog');
 await dialog.getByLabel('Endpoint ID',{exact:true}).fill('isolated-owner');await dialog.getByLabel('New revision',{exact:true}).fill('r1');
 await dialog.getByLabel('HTTPS origin',{exact:true}).fill('https://fixture.example.test');await dialog.getByLabel('Allowed IP addresses (one per line)',{exact:true}).fill('192.0.2.9');
 await dialog.getByLabel('Model',{exact:true}).fill('fixture-model');await dialog.getByLabel('Model SHA-256',{exact:true}).fill(`sha256:${'a'.repeat(64)}`);
 await dialog.getByLabel('Runtime version',{exact:true}).fill('fixture-1');await dialog.getByLabel('Approved credential reference',{exact:true}).selectOption('fixture-credential');
 await dialog.getByLabel('I confirm the deployment uses this exact model and runtime identity.',{exact:true}).check();
 await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();
 await expect(dialog.getByRole('button',{name:'Save reviewed configuration',exact:true})).toBeVisible();
 expect(await stat(ownerPath).then(()=>true,()=>false)).toBe(false);
 await expect(dialog).toContainText('local-admin');await expect(dialog).toContainText('fixture-credential');await expect(dialog).not.toContainText('OFFGRID_ISOLATED_UNAVAILABLE_CREDENTIAL');
 await page.screenshot({path:testInfo.outputPath('owner-actual-service-review.png')});
 await dialog.getByRole('button',{name:'Save reviewed configuration',exact:true}).click();await expect(dialog).toHaveCount(0);
 await expect(page.locator('.endpoint-owner-record')).toContainText('Saved; reconciliation required');
 expect((await request.get('/api/v2/inference-endpoints')).status()).toBe(503);
 let state=await readState();expect(state.records).toHaveLength(1);expect(state.entries[0]).toMatchObject({id:'isolated-owner',revision:'r1',credential_reference:'fixture-credential'});
 expect(JSON.parse(await readFile(ownerPath,'utf8')).endpoints[0].revision).toBe('r1');
 await reconcile(page);await expect(page.locator('.endpoint-owner-record')).toContainText('Saved and reconciled');
 await restart(page,fixture!);
 const card=page.locator('.endpoint-model').filter({hasText:'endpoint:isolated-owner@r1'});
 await expect(card).toContainText('Review required');await expect(card.getByRole('button',{name:'Check connection',exact:true})).toBeDisabled();
 await card.getByRole('button',{name:'Review and enable',exact:true}).click();await dialog.getByRole('button',{name:'Review and enable',exact:true}).click();
 await expect(card).toContainText('Access enabled');await expect(card).toContainText('configured credential is unavailable');
 // Deliberately drop only the UI response after the real service receives and publishes the update.
 await page.route('**/api/v2/inference-endpoints/configuration/commit',async route=>{const response=await route.fetch();expect(response.ok()).toBe(true);await route.abort('failed');});
 await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await page.locator('.endpoint-owner-entry').getByRole('button',{name:'Update endpoint',exact:true}).click();
 await dialog.getByLabel('New revision',{exact:true}).fill('r2');await dialog.getByLabel('Model SHA-256',{exact:true}).fill(`sha256:${'b'.repeat(64)}`);
 await expect(dialog.getByLabel('Approved credential reference',{exact:true})).toHaveValue('fixture-credential');
 await dialog.getByLabel('I confirm the deployment uses this exact model and runtime identity.',{exact:true}).check();await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();
 await dialog.getByRole('button',{name:'Save reviewed configuration',exact:true}).click();await expect(dialog).toHaveCount(0);
 await expect(page.locator('.endpoint-owner-panel')).toContainText('The save outcome is unconfirmed');
 await expect(card).toHaveCount(0); // Do not keep stale enabled access controls after owner publication.
 state=await readState();expect(state.records).toHaveLength(2);expect(state.records[1].state).toBe('published');expect(state.entries[0].revision).toBe('r2');
 expect((await request.get('/api/v2/inference-endpoints')).status()).toBe(503);
 const disabled=await request.post('/v1/chat/completions',{data:{model:'endpoint:isolated-owner@r1',messages:[{role:'user',content:'must not dispatch'}]}});expect(disabled.ok()).toBe(false);
 await page.reload();await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await reconcile(page);await restart(page,fixture!);
 const next=page.locator('.endpoint-model').filter({hasText:'endpoint:isolated-owner@r2'});await expect(next).toContainText('Review required');
 await expect(next.getByRole('button',{name:'Check connection',exact:true})).toBeDisabled();
 state=await readState();expect(state.records.map(record=>record.state)).toEqual(['committed','committed']);
 expect(posts.filter(path=>path.endsWith('/configuration/commit'))).toHaveLength(2);
 expect(posts.some(path=>path.endsWith('/probe'))).toBe(false);
 await page.screenshot({path:testInfo.outputPath('owner-actual-service-restart-review.png'),fullPage:true});
 // Explicit rendered maintenance through the actual service.
 const historical=state.records;
 const ownerBeforeArchive=await readFile(ownerPath,'utf8');
 const policyPath=join(fixture!,'data','inference-endpoint-policy.json');
 const policyBeforeArchive=await readFile(policyPath,'utf8');
 await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await page.getByRole('button',{name:'Archive settled receipts',exact:true}).click();
 await expect(dialog.locator('li')).toHaveCount(2);
 await page.screenshot({path:testInfo.outputPath('owner-actual-archive-review.png')});
 const archiveResponse=page.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith('/configuration/archive'));
 await dialog.getByRole('button',{name:'Archive settled receipts',exact:true}).click();
 const archived=await archiveResponse;await expect(dialog).toHaveCount(0);
 expect(archived.ok(),await archived.text()).toBe(true);expect((await archived.json()).archived).toBe(2);
 await restart(page,fixture!);state=await readState();expect(state.records).toEqual([]);expect(state.entries[0].revision).toBe('r2');
 for(const record of historical){
  const duplicate=await request.post('/api/v2/inference-endpoints/configuration/commit',{data:{request_id:record.request_id,review_id:record.review_id}});
  expect(duplicate.ok()).toBe(true);expect(await duplicate.json()).toEqual(record);
 }
 expect(await readFile(ownerPath,'utf8')).toBe(ownerBeforeArchive);expect(await readFile(policyPath,'utf8')).toBe(policyBeforeArchive);
 await expect(next).toContainText('Review required');
 // Retire only the obsolete decision through the real service. Historical access
 // remains audit evidence and cannot enable the installed replacement.
 expect(state.policy_maintenance?.retirable).toEqual(['isolated-owner@r1']);
 await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await page.getByRole('button',{name:'Retire obsolete decisions',exact:true}).click();
 await expect(dialog.locator('li')).toHaveText(['isolated-owner@r1']);
 await page.screenshot({path:testInfo.outputPath('owner-actual-retirement-review.png')});
 const retireResponse=page.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith('/configuration/retire-decisions'));
 await dialog.getByRole('button',{name:'Retire obsolete decisions',exact:true}).click();
 const retirement=await retireResponse;await expect(dialog).toHaveCount(0);
 expect(retirement.ok(),await retirement.text()).toBe(true);
 const retired=await retirement.json();expect(retired.retired).toEqual(['isolated-owner@r1']);
 const policyHistory=JSON.parse(await readFile(join(fixture!,'data',`inference-endpoint-policy-history-${retired.history_revision}.json`),'utf8'));
 expect(policyHistory.snapshot).toEqual(JSON.parse(policyBeforeArchive));
 await restart(page,fixture!);state=await readState();
 expect(state.policy_maintenance?.retirable).toEqual([]);
 expect(state.policy_maintenance?.decision_count).toBe(1);
 expect(await readFile(ownerPath,'utf8')).toBe(ownerBeforeArchive);
 await expect(next).toContainText('Review required');await expect(next.getByRole('button',{name:'Check connection',exact:true})).toBeDisabled();
 const retiredCall=await request.post('/v1/chat/completions',{data:{model:'endpoint:isolated-owner@r1',messages:[{role:'user',content:'historical access must not dispatch'}]}});expect(retiredCall.ok()).toBe(false);
 await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 const historyPanel=page.locator('.endpoint-owner-history');
 await historyPanel.getByRole('button',{name:'Archived receipts',exact:true}).click();
 await expect(historyPanel.locator('.endpoint-owner-record')).toHaveCount(2);
 await expect(historyPanel).toContainText('These records never grant current access');
 const receiptDownload=page.waitForEvent('download');await historyPanel.getByRole('button',{name:'Export this page',exact:true}).click();
 const receiptExport=JSON.parse(await readFile((await (await receiptDownload).path())!,'utf8'));
 expect(receiptExport.evidence_only).toBe(true);expect(receiptExport.page.receipts.map((item:any)=>item.record).sort((a:any,b:any)=>a.request_id.localeCompare(b.request_id))).toEqual([...historical].sort((a,b)=>a.request_id.localeCompare(b.request_id)));
 await historyPanel.getByRole('button',{name:'Policy history',exact:true}).click();
 await expect(historyPanel.locator('.endpoint-owner-record')).toHaveCount(Object.keys(policyHistory.snapshot.decisions).length);
 const policyDownload=page.waitForEvent('download');await historyPanel.getByRole('button',{name:'Export this page',exact:true}).click();
 const policyExport=JSON.parse(await readFile((await (await policyDownload).path())!,'utf8'));
 expect(policyExport.page.history).toEqual(policyHistory);expect(policyExport.page.revision).toBe(retired.history_revision);
 expect(await readFile(ownerPath,'utf8')).toBe(ownerBeforeArchive);await expect(next).toContainText('Review required');
 await page.screenshot({path:testInfo.outputPath('owner-actual-policy-history.png'),fullPage:true});
 await testInfo.attach('owner-final-state',{body:JSON.stringify(state,null,2),contentType:'application/json'});
 // Deliberate damage is restricted to this private disposable fixture. The UI
 // must keep recovery discoverable without inventing a reset or enabling access.
 const journalPath=join(fixture!,'data','inference-endpoint-owner-journal.json');
 const damagedJournal='{fixture interrupted journal';await writeFile(journalPath,damagedJournal);
 await page.reload();await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await expect(page.locator('.endpoint-owner-panel')).toContainText('Endpoint owner journal is unavailable');
 await expect(page.getByRole('button',{name:'Add endpoint',exact:true})).toBeDisabled();
 await page.getByRole('link',{name:'Workspace recovery',exact:true}).click();
 await expect(page.locator('#workspace-recovery-title')).toBeVisible();
 expect(await readFile(journalPath,'utf8')).toBe(damagedJournal);
});
