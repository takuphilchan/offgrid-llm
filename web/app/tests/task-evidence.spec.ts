import { openWorkDetails } from './helpers/workspace-actions';
import { expect, test, type Page } from '@playwright/test';
import { taskEvidenceText } from '../src/i18n/task-evidence';
import { documentWorkflowText } from '../src/i18n/document-workflow';

const id = 'run-' + 'c'.repeat(32);
const verification = { version: 1, revision: 1, instruction_id: 'initial', instruction_sha256: 'a'.repeat(64), state: 'partial', checks: [
 { expectation: {id:'open', title:'Browser ready', kind:'application_open'}, state:'verified', provenance:'model_proposed', revision:1, evidence_steps:[3], checked_at:'2026-10-11T02:03:04Z' },
 { expectation: {id:'research', title:'Research tourism', kind:'research'}, state:'unavailable', provenance:'caller_explicit', revision:1, reason:'No independent research verifier.' },
] };
async function fixture(page: Page, evidence: unknown, locale = 'en', status = 'completed', extra: Record<string,unknown> = {}) {
 const commands: string[] = [];
 await page.addInitScript(locale => { localStorage.setItem('offgrid.locale', locale); localStorage.setItem('offgrid.onboarding.complete','true'); }, locale);
 const snapshot = { run_id:id, task_id:id, prompt:'Research tourism', model:'fixture', status, steps:[], output:'Saved answer', pending_approval:null, resumable:false, verification:evidence, ...extra };
 await page.route('**/health', r => r.fulfill({json:{status:'healthy'}}));
 await page.route(/\/(?:api\/v2|v1)\//, async r => {
  const path = new URL(r.request().url()).pathname;
  if (r.request().method() !== 'GET') commands.push(path);
  let data: unknown = {};
  if(path === '/api/v2/system') data = {product:'offgrid', api_version:2, version:'test', workspace_id:'fixture', capabilities:['task-first-agents-v2','durable-agent-events-v2']};
  else if(path === '/v1/users/me') data = {user:null,authenticated:false,guest:true};
  else if(path === '/v1/system/config') data = {version:'test',require_auth:false};
  else if(path === '/v1/models') data = {data:[{id:'fixture',type:'chat',context_window:8192}]};
  else if(path === '/api/v2/jobs' || path === '/v1/agents/tasks') data = [{id,prompt:snapshot.prompt,status,created_at:'2026-10-10T00:00:00Z'}];
  else if(path === `/api/v2/jobs/${id}`) data = snapshot;
  else if(path.endsWith('/events')) return r.fulfill({contentType:'text/event-stream',body:`data: ${JSON.stringify({...snapshot,type:'snapshot'})}\n\n`});
  else if(path === '/api/v2/computer/sessions') data = {sessions:[]};
  else if(path === '/v1/catalog') data = {models:[]};
  await r.fulfill({json:data});
 });
 await page.goto(`/ui/#/agents/task/${id}`);
 await expect(page.locator('.task-surface')).toBeVisible().catch(() => expect(page.locator('.task-result')).toBeVisible());
 return commands;
}

test('partial checks preserve provenance; reload and details never replay work', async ({page}) => {
 const commands=await fixture(page, verification);
 await openWorkDetails(page);
 const panel=page.getByRole('region',{name:'Outcome checks'});
 await expect(panel).toContainText('Partially verified');
 await expect(panel).toContainText('Model interpretation');
 await expect(panel).toContainText('Caller-defined');
 await panel.locator('summary').focus();await page.keyboard.press('Enter');
 await expect(panel).toContainText('No independent research verifier.');
 await expect(panel.locator('time')).toHaveAttribute('datetime','2026-10-11T02:03:04Z');
 await expect(panel.locator('time')).toBeVisible();
 await page.reload();await openWorkDetails(page);await expect(panel).toBeVisible();expect(commands).toEqual([]);
});

for(const locale of ['en','fr','es','ar','sw','sn','nd','zu','de'] as const) test(`evidence is readable on narrow screens in ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:720});
 await fixture(page,verification,locale);
 await openWorkDetails(page);
 const panel=page.getByRole('region',{name:taskEvidenceText(locale).title});await expect(panel).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
 if(locale==='ar') await expect(page.locator('html')).toHaveAttribute('dir','rtl');
});

for (const [name, evidence, label] of [
 ['pending', {...verification,state:'pending',checks:[{...verification.checks[0],state:'pending'}]}, 'Pending checks'],
 ['unverified', {...verification,state:'unverified',checks:[verification.checks[1]]}, 'Not independently verified'],
 ['passed', {...verification,state:'checks_passed',checks:[verification.checks[0]]}, 'Declared checks passed'],
 ['unknown', {version:99,state:'checks_passed'}, 'Verification unavailable'],
] as const) test(`${name} is not confused with execution success`,async({page})=>{
 await fixture(page,evidence);await expect(page.locator('.task-evidence')).toContainText(label);
});
test('legacy absence is not retrospective certification',async({page})=>{
 await fixture(page,undefined);await expect(page.locator('.task-evidence')).toHaveCount(0);await expect(page.locator('.task-result')).toContainText('Saved answer');
});

const documentVerification = {...verification, version:2, state:'pending', checks:[{
 expectation:{id:'edit',title:'Requested document text',kind:'document_edit_matches',document:'tab',snapshot:'before',edit_mode:'insert_text',text:'café 日本語'},
 state:'pending',provenance:'model_proposed',revision:1,
 binding:{session:'host',target_sha256:'b'.repeat(64),observation:'read',step:2,document:{id:'tab',generation:'g',container:'tab-control',editor:'shared-editor',revision:'r',snapshot:'before',action_id:'one-action',operation:'insert_text',preimage_sha256:'c'.repeat(64),expected_sha256:'d'.repeat(64)}}
}]};

for(const locale of ['en','fr','es','ar','sw','sn','nd','zu','de'] as const) test(`document v2 distinguishes dispatched input from verification in ${locale}`,async({page})=>{
 await page.setViewportSize({width:390,height:620});
 const commands=await fixture(page,documentVerification,locale,'interrupted',{steps:[{id:3,type:'action',tool_name:'computer_insert_text',tool_result:'{}'}],resumable:true});
 await expect(page.locator('.task-evidence')).toContainText(documentWorkflowText(locale).verification);
 await expect(page.locator('.native-document-progress')).toHaveText(documentWorkflowText(locale).dispatched);
 await expect(page.locator('.task-evidence')).toContainText(taskEvidenceText(locale).pending);
 await page.locator('.task-evidence summary').focus();await page.keyboard.press('Enter');
 await expect(page.locator('.task-evidence code')).toHaveText('document_edit_matches');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
 if(locale==='ar')await expect(page.locator('html')).toHaveAttribute('dir','rtl');
 await page.reload();await expect(page.locator('.native-document-progress')).toBeVisible();expect(commands).toEqual([]);
});

test('a trusted interrupted read has no mutation reconciliation and resumes the same job only explicitly',async({page})=>{
 const commands=await fixture(page,documentVerification,'en','interrupted',{resumable:true,error_code:'computer_read_repair_limit',steps:[{id:4,type:'error',tool_name:'computer_read_text',tool_result:JSON.stringify({status:'failed',code:'computer_document_changed',effects:'none',verified:false})}]});
 await expect(page.locator('.native-document-progress')).toContainText(documentWorkflowText('en').readFailed);
 await expect(page.getByRole('button',{name:'Record verified outcome',exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Resume',exact:true})).toBeVisible();expect(commands).toEqual([]);
 await page.getByRole('button',{name:'Resume',exact:true}).click();
 await expect.poll(()=>commands).toEqual([`/api/v2/jobs/${id}/resume`]);
});

test('uncertain input stays uncertain even if output claims no effects',async({page})=>{
 const commands=await fixture(page,documentVerification,'en','uncertain',{uncertain_call_id:'call',uncertain_call:{tool:'computer_insert_text',arguments:'{}'},steps:[{id:4,type:'error',tool_name:'computer_insert_text',tool_result:JSON.stringify({code:'computer_document_changed',effects:'none',verified:false})}]});
 await expect(page.locator('.native-document-progress')).toHaveCount(0);
 await page.getByText('Report what you observed', {exact:true}).first().click();
 await expect(page.getByRole('button',{name:'Record observation',exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'Resume',exact:true})).toHaveCount(0);expect(commands).toEqual([]);
});
