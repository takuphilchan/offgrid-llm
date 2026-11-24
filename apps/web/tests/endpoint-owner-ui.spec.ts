import { expect, test, type Page } from '@playwright/test';
import type { EndpointOwnerEntry, EndpointOwnerSnapshot } from '../src/api/endpoint-owner';
import { endpointMaintenance } from '../src/i18n/endpoint-maintenance';
import { endpointOwner } from '../src/i18n/endpoint-owner';
import { createHash } from 'node:crypto';

async function fixture(page: Page, { role = 'admin', language = 'en', unknown = false } = {}) {
 const entry: EndpointOwnerEntry = { id:'fixture', revision:'r1', provider:'ollama', url:'https://inference.example.test', allowed_ips:['192.0.2.9'], model:'fixture-model', model_digest:`sha256:${'a'.repeat(64)}`, runtime_version:'fixture-version', format:'gguf', context_length:4096, actors:['fixture-user'], deployment_attested:true, expected_revision:'', managed:false, credential_configured:false };
 const snapshot: EndpointOwnerSnapshot = { file_revision:'b'.repeat(64), source:'workspace', actor:'fixture-user', write_available:true, entries:[], managed_templates:[], credential_references:[], records:[] };
 const state = { snapshot, entry, posts:[] as {path:string;body:any}[], input:null as any, stale:false, unsupported:false };
 await page.addInitScript(locale => { localStorage.setItem('offgrid.onboarding.complete','true'); localStorage.setItem('offgrid.locale',locale); }, language);
 await page.route('**/health', route => route.fulfill({json:{status:'healthy'}}));
 await page.route(/\/(?:v1|api\/v2)\//, async route => {
  const request=route.request(), path=new URL(request.url()).pathname, action=path.split('/').pop();
  if(request.method()==='POST') state.posts.push({path,body:request.postDataJSON()});
  if(path.includes('/inference-endpoints/configuration')) {
   if(action==='archive') {
    const archived=snapshot.records.length;snapshot.records=[];snapshot.journal_revision='f'.repeat(64);
    if(unknown)return route.abort('failed');
    return route.fulfill({json:{archived,journal_revision:snapshot.journal_revision}});
   }
   if(action==='retire-decisions') {
    const retired=snapshot.policy_maintenance!.retirable;
    snapshot.policy_maintenance={policy_revision:'f'.repeat(64),decision_count:snapshot.policy_maintenance!.decision_count-retired.length,retirable:[]};
    if(unknown)return route.abort('failed');
    return route.fulfill({json:{retired,policy_revision:snapshot.policy_maintenance.policy_revision,history_revision:'a'.repeat(64)}});
   }
   if(action==='prepare') {
    state.input=request.postDataJSON(); if(state.stale)return route.fulfill({status:409,json:{error:{message:'Owner file changed; refresh before preparing'}}});
    const template=snapshot.managed_templates.find(item=>item.id===state.input.managed_template);
    return route.fulfill({json:{review_id:'c'.repeat(64),expected_revision:snapshot.file_revision,expires_at:new Date(Date.now()+300000).toISOString(),entry:{...(template?.entry??entry),...state.input,expected_revision:'',managed:Boolean(template),credential_configured:Boolean(state.input.credential_reference)},restart_required:true,access_review_required:true}});
   }
   if(action==='commit') {
    snapshot.entries=[{...entry,...state.input,expected_revision:''}];snapshot.file_revision='d'.repeat(64);snapshot.write_available=false;snapshot.write_reason='Explicit reconciliation required';
    snapshot.records=[{request_id:request.postDataJSON().request_id,review_id:'c'.repeat(64),actor:'fixture-user',epoch:'e'.repeat(64),before:'b'.repeat(64),after:'d'.repeat(64),endpoint:`${state.input.id}@${state.input.revision}`,state:'published',created_at:new Date().toISOString()}];
    if(unknown)return route.abort('failed');
    return route.fulfill({json:snapshot.records[0]});
   }
   if(action==='reconcile') {
    expect(request.postDataJSON().expected_revision).toBe(snapshot.file_revision);
    snapshot.records[0]={...snapshot.records[0],state:'committed',resolved_at:new Date().toISOString(),resolved_by:'fixture-user',resolved_epoch:'e'.repeat(64)};
    return route.fulfill({json:snapshot.records[0]});
   }
   return route.fulfill({json:snapshot});
  }
  // Recovery must be reachable even when ordinary management is unhealthy.
  if(path.startsWith('/api/v2/inference-endpoints'))return route.fulfill({status:503,json:{error:{message:'Endpoint policy unavailable'}}});
  const bodies:Record<string,unknown>={
   '/api/v2/system':{product:'offgrid',api_version:2,workspace_id:'endpoint-owner-ui-fixture',capabilities:state.unsupported?[]:['inference-endpoint-owner-v1']},
   '/v1/system/config':{require_auth:true},'/v1/users/me':{authenticated:true,auth_required:true,user:{id:'fixture-user',username:'fixture-user',role}},
   '/v1/models':{data:[{id:'fixture-chat',type:'llm'}]},'/v1/catalog':{models:[]},'/v1/models/download/progress':{},'/api/v2/jobs':[], '/v1/sessions':{sessions:[]},
   '/api/v2/models':{models:[]},'/api/v2/models/operations':{operations:[]},'/api/v2/models/packages':{packages:[]}
  };
  return route.fulfill({json:bodies[path]??{}});
 });
 return state;
}

test('maintenance review lists exact settled receipts and cancellation has no effect',async({page})=>{
 const state=await fixture(page);state.snapshot.journal_revision='e'.repeat(64);
 state.snapshot.records=[{request_id:'settled-request',review_id:'c'.repeat(64),actor:'fixture-user',epoch:'e'.repeat(64),before:'b'.repeat(64),after:'d'.repeat(64),endpoint:'fixture@r1',state:'committed',created_at:'2026-11-21T00:00:00Z',resolved_at:'2026-11-21T00:00:01Z',resolved_by:'fixture-user',resolved_epoch:'e'.repeat(64)}];
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 const archive=page.getByRole('button',{name:'Archive settled receipts',exact:true});await archive.click();
 const dialog=page.getByRole('dialog');await expect(dialog).toContainText('settled-request');await expect(dialog).toContainText('Configuration and access stay unchanged');
 expect(state.posts).toHaveLength(0);await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);await expect(archive).toBeFocused();
 await archive.click();await dialog.getByRole('button',{name:'Archive settled receipts',exact:true}).click();await expect(dialog).toHaveCount(0);
 await expect(page.locator('.endpoint-owner-record')).toHaveCount(0);await expect(archive).toBeDisabled();
 expect(state.posts).toEqual([{path:'/api/v2/inference-endpoints/configuration/archive',body:{expected_revision:'b'.repeat(64),expected_journal_revision:'e'.repeat(64)}}]);
});

test('uncertain retirement consumes the displayed review and refresh never repeats the operation',async({page})=>{
 const state=await fixture(page,{unknown:true});state.snapshot.entries=[state.entry];state.snapshot.policy_maintenance={policy_revision:'c'.repeat(64),decision_count:2,retirable:['fixture@old']};
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 const retire=page.getByRole('button',{name:'Retire obsolete decisions',exact:true});await retire.click();
 const dialog=page.getByRole('dialog');await expect(dialog.locator('li')).toHaveText(['fixture@old']);await expect(dialog).toContainText('This does not enable access');
 expect(state.posts).toHaveLength(0);await dialog.getByRole('button',{name:'Retire obsolete decisions',exact:true}).click();await expect(dialog).toHaveCount(0);
 await expect(page.locator('.endpoint-owner-panel')).toContainText('The maintenance outcome is unconfirmed');await expect(retire).toBeDisabled();
 await page.locator('.endpoint-owner-panel').getByRole('button',{name:'Refresh',exact:true}).click();
 expect(state.posts).toEqual([{path:'/api/v2/inference-endpoints/configuration/retire-decisions',body:{expected_revision:'b'.repeat(64),expected_policy_revision:'c'.repeat(64),endpoints:['fixture@old']}}]);
});

test('maintenance has no eligible action without a bounded preview',async({page})=>{
 await fixture(page);await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await expect(page.getByRole('button',{name:'Archive settled receipts',exact:true})).toBeDisabled();
 await expect(page.getByRole('button',{name:'Retire obsolete decisions',exact:true})).toBeDisabled();
});

test('an unreadable owner file keeps the existing workspace recovery route reachable',async({page})=>{
 const state=await fixture(page);
 await page.route('**/inference-endpoints/configuration',route=>route.fulfill({status:409,json:{error:{message:'Endpoint owner file is unavailable'}}}));
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await expect(page.locator('.endpoint-owner-panel')).toContainText('Endpoint owner file is unavailable');
 await expect(page.getByRole('button',{name:'Add endpoint',exact:true})).toBeDisabled();
 await page.getByRole('link',{name:'Workspace recovery',exact:true}).click();
 await expect(page.locator('#workspace-recovery-title')).toBeVisible();await expect(page).toHaveURL(/#\/settings\/recovery$/);
 expect(state.posts).toHaveLength(0);
});

test('historical paging rejects a changed journal and explicit refresh starts a fresh read without mutations',async({page})=>{
 const state=await fixture(page);state.snapshot.history_available=true;
 const pin='a'.repeat(64),journal='b'.repeat(64),changed='c'.repeat(64);
 const hash=(id:string)=>createHash('sha256').update(id).digest('hex');
 const receipts=Array.from({length:32},(_,index)=>({version:1,archive_id:pin,archived_by:'fixture-user',archived_at:'2026-11-22T00:00:00Z',record:{request_id:'history-'+index,review_id:pin,actor:'fixture-user',epoch:pin,before:pin,after:journal,endpoint:'fixture@r1',state:'committed',created_at:'2026-11-21T00:00:00Z',resolved_at:'2026-11-21T00:00:01Z',resolved_by:'fixture-user',resolved_epoch:pin}})).sort((a,b)=>hash(a.record.request_id).localeCompare(hash(b.record.request_id)));
 let roots=0;const reads:string[]=[];
 await page.route('**/configuration/history*',route=>{
  const query=new URL(route.request().url()).searchParams;reads.push(query.toString());
  if(query.has('after'))return route.fulfill({status:409,json:{error:{message:'Endpoint history changed; restart browsing'}}});
  roots++;return route.fulfill({json:{journal_revision:roots===1?journal:changed,archive_id:pin,receipts:roots===1?receipts:[],...(roots===1?{next:hash(receipts.at(-1)!.record.request_id)}:{})}});
 });
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();const history=page.locator('.endpoint-owner-history');
 await history.getByRole('button',{name:'Archived receipts',exact:true}).click();await expect(history.locator('.endpoint-owner-record')).toHaveCount(32);
 await history.getByRole('button',{name:'Show more',exact:true}).click();await expect(history.getByRole('alert')).toContainText('history changed');await expect(history.getByRole('button',{name:'Export this page',exact:true})).toBeDisabled();
 await history.getByRole('button',{name:'Refresh',exact:true}).click();await expect(history).toContainText('No matching history');expect(reads).toHaveLength(3);expect(reads[2]).toBe('');expect(reads[1]).toContain('journal_revision='+journal);expect(state.posts).toHaveLength(0);
});

for(const language of ['en','ar','zh-Hans'] as const) test(`maintenance review contains long identities on a narrow ${language} screen`,async({page},testInfo)=>{
 const state=await fixture(page,{language});const copy=endpointMaintenance[language];
 const identities=Array.from({length:64},(_,index)=>`${String(index).padStart(3,'0')}${'x'.repeat(125)}@${'r'.repeat(128)}`);
 state.snapshot.policy_maintenance={policy_revision:'c'.repeat(64),decision_count:64,retirable:identities};
 await page.setViewportSize({width:390,height:844});await page.goto('/ui/#/models');
 await page.getByRole('button',{name:endpointOwner[language].configure,exact:true}).click();
 const opener=page.getByRole('button',{name:copy.retire,exact:true});await opener.click();const dialog=page.getByRole('dialog');
 await expect(dialog).toContainText(copy.retireHint);await expect(dialog.locator('li')).toHaveCount(64);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth)).toBe(true);
 expect(await dialog.evaluate(node=>{const bounds=node.getBoundingClientRect();return bounds.top>=0&&bounds.bottom<=window.innerHeight;})).toBe(true);
 await dialog.getByRole('list',{name:copy.retire,exact:true}).focus();await page.keyboard.press('End');
 await expect(dialog.locator('li').last()).toBeInViewport();
 await dialog.getByRole('button',{name:copy.retire,exact:true}).scrollIntoViewIfNeeded();
 await page.screenshot({path:testInfo.outputPath(`endpoint-maintenance-${language}-narrow.png`)});
 await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);await expect(opener).toBeFocused();expect(state.posts).toHaveLength(0);
});
async function openForm(page:Page) {
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await page.getByRole('button',{name:'Add endpoint',exact:true}).click();
 const dialog=page.getByRole('dialog');
 await dialog.getByLabel('Endpoint ID',{exact:true}).fill('fixture');await dialog.getByLabel('New revision',{exact:true}).fill('r1');
 await dialog.getByLabel('HTTPS origin',{exact:true}).fill('https://inference.example.test');
 await dialog.getByLabel('Allowed IP addresses (one per line)',{exact:true}).fill('192.0.2.9');
 await dialog.getByLabel('Model',{exact:true}).fill('fixture-model');await dialog.getByLabel('Model SHA-256',{exact:true}).fill(`sha256:${'a'.repeat(64)}`);
 await dialog.getByLabel('Runtime version',{exact:true}).fill('fixture-version');
 await dialog.getByLabel('I confirm the deployment uses this exact model and runtime identity.',{exact:true}).check();
 await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();
 await expect(dialog.getByRole('button',{name:'Save reviewed configuration',exact:true})).toBeVisible();
 return dialog;
}

test('exact review, single publication and explicit reconciliation remain reachable with unhealthy access policy',async({page},testInfo)=>{
 const state=await fixture(page);const dialog=await openForm(page);
 await expect(dialog).toContainText('https://inference.example.test');await expect(dialog).toContainText('fixture-user');await expect(dialog).toContainText('No provider is contacted');
 await dialog.getByRole('button',{name:'Update endpoint',exact:true}).click();
 await expect(dialog.getByLabel('New revision',{exact:true})).toHaveValue('r1');
 await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();
 await dialog.getByRole('button',{name:'Save reviewed configuration',exact:true}).click();
 await expect(dialog).toHaveCount(0);await expect(page.locator('.endpoint-owner-record')).toContainText('Saved; reconciliation required');
 await expect(page.getByRole('button',{name:'Add endpoint',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Reconcile recorded outcome',exact:true}).click();
 await expect(page.getByRole('dialog')).toContainText('d'.repeat(64));
 await page.getByRole('dialog').getByRole('button',{name:'Reconcile recorded outcome',exact:true}).click();
 await expect(page.locator('.endpoint-owner-record')).toContainText('Saved and reconciled');
 expect(state.posts.map(item=>item.path.split('/').pop())).toEqual(['prepare','prepare','commit','reconcile']);
 await page.screenshot({path:testInfo.outputPath('endpoint-owner-reconciled.png'),fullPage:true});
});
test('lost publication response consumes review, shows durable recovery and never repeats commit',async({page})=>{
 const state=await fixture(page,{unknown:true});const dialog=await openForm(page);
 await dialog.getByRole('button',{name:'Save reviewed configuration',exact:true}).click();
 await expect(dialog).toHaveCount(0);await expect(page.locator('.endpoint-owner-panel')).toContainText('The save outcome is unconfirmed');
 await expect(page.locator('.endpoint-owner-record')).toContainText(state.snapshot.records[0].request_id);
 await page.locator('.endpoint-owner-panel').getByRole('button',{name:'Refresh',exact:true}).click();
 await expect(page.getByRole('button',{name:'Save reviewed configuration',exact:true})).toHaveCount(0);
 expect(state.posts.filter(item=>item.path.endsWith('/commit'))).toHaveLength(1);
 await page.reload();await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await expect(page.locator('.endpoint-owner-record')).toContainText('Saved; reconciliation required');
 expect(state.posts.filter(item=>item.path.endsWith('/commit'))).toHaveLength(1);
});
test('an update cannot silently discard an existing credential',async({page})=>{
 const state=await fixture(page);state.snapshot.entries=[{...state.entry,credential_configured:true}];
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();
 await page.getByRole('button',{name:'Update endpoint',exact:true}).click();const dialog=page.getByRole('dialog');
 await expect(dialog.getByLabel('Endpoint ID',{exact:true})).toBeDisabled();await dialog.getByLabel('New revision',{exact:true}).fill('r2');
 await dialog.getByLabel('I confirm the deployment uses this exact model and runtime identity.',{exact:true}).check();
 await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();expect(state.posts).toHaveLength(0);
 await dialog.getByLabel('Explicitly remove the existing credential',{exact:true}).check();
 await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();
 await expect(dialog.getByRole('button',{name:'Save reviewed configuration',exact:true})).toBeVisible();
 expect(state.input.replace).toBe('fixture@r1');expect(state.input.credential_reference).toBeUndefined();
});
test('credential selection is exact-origin bound and has a stable accessible label',async({page})=>{
 const state=await fixture(page);state.snapshot.entries=[state.entry];state.snapshot.credential_references=[{id:'fixture-credential',origins:[state.entry.url!]},{id:'other-credential',origins:['https://other.example.test']}];
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();await page.getByRole('button',{name:'Update endpoint',exact:true}).click();
 const dialog=page.getByRole('dialog'),credential=dialog.getByLabel('Approved credential reference',{exact:true});
 await credential.selectOption('fixture-credential');await expect(credential).toHaveValue('fixture-credential');await expect(credential.locator('option')).toHaveCount(2);
 await dialog.getByLabel('HTTPS origin',{exact:true}).fill('https://other.example.test');await expect(credential).toHaveValue('');
 await expect(credential.locator('option')).toHaveText(['None','other-credential']);
});
test('managed configuration sends only an approved template and explicit actor/model revision identity',async({page})=>{
 const state=await fixture(page);state.snapshot.managed_templates=[{id:'local-fixture',entry:{...state.entry,url:'http://127.0.0.1:11434',managed:true,managed_template:'local-fixture'}}];
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();await page.getByRole('button',{name:'Add endpoint',exact:true}).click();
 const dialog=page.getByRole('dialog');await dialog.getByLabel('Endpoint ID',{exact:true}).fill('local-fixture');await dialog.getByLabel('New revision',{exact:true}).fill('r1');
 await dialog.getByLabel('Transport',{exact:true}).selectOption('managed');await dialog.getByLabel('Approved local template',{exact:true}).selectOption('local-fixture');
 await expect(dialog.getByLabel('HTTPS origin',{exact:true})).toHaveCount(0);await expect(dialog).toContainText('http://127.0.0.1:11434');
 await dialog.getByLabel('I confirm the deployment uses this exact model and runtime identity.',{exact:true}).check();await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();
 await expect(dialog.getByRole('button',{name:'Save reviewed configuration',exact:true})).toBeVisible();
 expect(Object.keys(state.input).sort()).toEqual(['actors','deployment_attested','expected_revision','id','managed_template','revision']);expect(state.posts).toHaveLength(1);
});
test('stale preparation retains editable inputs and never offers a commit',async({page})=>{
 const state=await fixture(page);state.snapshot.entries=[state.entry];state.stale=true;
 await page.goto('/ui/#/models');await page.getByRole('button',{name:'Configure endpoints',exact:true}).click();await page.getByRole('button',{name:'Update endpoint',exact:true}).click();
 const dialog=page.getByRole('dialog');await dialog.getByLabel('New revision',{exact:true}).fill('r2');await dialog.getByLabel('I confirm the deployment uses this exact model and runtime identity.',{exact:true}).check();
 await dialog.getByRole('button',{name:'Review configuration',exact:true}).click();await expect(dialog.getByRole('alert')).toContainText('Owner file changed');await expect(dialog.getByLabel('New revision',{exact:true})).toHaveValue('r2');
 await expect(dialog.getByRole('button',{name:'Save reviewed configuration',exact:true})).toHaveCount(0);expect(state.posts).toHaveLength(1);
});
test('ordinary users have no owner control',async({page})=>{
 await fixture(page,{role:'user'});await page.goto('/ui/#/models');await expect(page.locator('.endpoint-model')).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Configure endpoints',exact:true})).toHaveCount(0);
});
test('unsupported service has no owner control',async({page})=>{
 const state=await fixture(page);state.unsupported=true;await page.goto('/ui/#/models');
 await expect(page.getByRole('button',{name:'Configure endpoints',exact:true})).toHaveCount(0);
});
test('Arabic owner dialog preserves keyboard focus and fits a narrow viewport',async({page},testInfo)=>{
 await fixture(page,{language:'ar'});await page.setViewportSize({width:390,height:844});await page.goto('/ui/#/models');
 await page.getByRole('button',{name:'إعداد نقاط الاتصال',exact:true}).click();const add=page.getByRole('button',{name:'إضافة نقطة اتصال',exact:true});await add.click();
 await expect(page.getByRole('dialog')).toContainText('أصل HTTPS');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);
 expect(await page.getByRole('dialog').evaluate(node=>node.scrollWidth<=node.clientWidth)).toBe(true);
 expect(await page.getByRole('dialog').evaluate(node=>{const bounds=node.getBoundingClientRect();return bounds.top>=0&&bounds.bottom<=window.innerHeight;})).toBe(true);
 await page.screenshot({path:testInfo.outputPath('endpoint-owner-ar-narrow.png')});
 await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(add).toBeFocused();
});
