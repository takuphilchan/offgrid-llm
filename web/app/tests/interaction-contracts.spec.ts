import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const state = { role: 'admin', authRequired: true, failHistory: false, slowHistory: false, failStats: false, knowledge: false, deletes: [] as string[], calls: [] as string[] };
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.onboarding.complete', 'true'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2 } }));
  await page.route(/\/(?:v1|api\/v2\/computer)\//, async r => {
    const path = new URL(r.request().url()).pathname;
    state.calls.push(path);
    if (path === '/v1/agents/tasks' && state.slowHistory) await new Promise(resolve => setTimeout(resolve, 700));
    if ((path === '/v1/agents/tasks' && state.failHistory) || (path === '/v1/stats' && state.failStats)) return r.fulfill({ status: 503, json: { error: 'Fixture unavailable' } });
    if (r.request().method() === 'DELETE') {
      await new Promise(resolve => setTimeout(resolve, 600)); state.deletes.push(path.split('/').at(-1)!);
      return r.fulfill({ json: { success: true } });
    }
    const tasks = ['one', 'two', 'three'].filter(id => !state.deletes.includes(id)).map(id => ({ id, prompt: `Task ${id}`, status: 'completed', deletable: true, created_at: '2026-09-19T10:00:00Z' }));
    const bodies: Record<string, unknown> = {
      '/v1/users/me': { authenticated: true, auth_required: state.authRequired, user: { id: 'alice', username: 'alice', role: state.role } },
      '/v1/models': { data: [{ id: 'model-a', type: 'chat' }, { id: 'model-b', type: 'chat' }] },
      '/v1/catalog': { models: [{ id: 'suggestion', name: 'Suggested model', type: 'chat', repo: 'test/repo', file: 'test.gguf', quant: 'Q4_K_M', size_bytes: 1000, min_ram_gb: 2 }] },
      '/v1/models/download/progress': {}, '/v1/sessions': { sessions: [] },
      '/v1/rag/status': { enabled: state.knowledge, stats: {} },
      '/v1/documents': { documents: [{ id: 'document', name: 'Document without index status', chunk_count: 1, source_retained: true, size: 100 }] },
      '/v1/agents/tasks': tasks, '/v1/agents/tools': { tools: [], enabled_count: 0 }, '/v1/agents/mcp': { servers: [] },
      '/api/v2/computer/status': { available: false }, '/v1/integrations': { integrations: [] },
      '/v1/runs': { runs: [{ id: 'one', status: 'completed', updated_at: '2026-09-19T10:00:00Z', event_count: 0, data: { prompt: 'Saved run' } }] },
      '/v1/runs/one/events': { events: [] }, '/v1/stats': {}, '/v1/system/config': { require_auth: state.authRequired }
    };
    return r.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}

test('desktop application picker connects an existing window without browser-only setup',async({page})=>{
 await fixture(page);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'desktop-workspace'}}));
 let connected=false;
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:connected?[{id:'native-session',origin:'Notes — existing application',driver:'windows-uia',state:'ready',remaining_actions:100}]:[]}}));
 await page.addInitScript(()=>{
   const targets=[{id:'opaque-window',title:'Notes — existing application',driver:'windows-uia'}];
   (window as any).__nativeCalls=[];let state:any={state:'idle',installed:true};
   (window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',getComputerStatus:async()=>state,
     startComputerBrowser:async()=>{throw Error('must not start browser');},
     discoverComputerApps:async(request:unknown)=>{(window as any).__nativeCalls.push(request);state={state:'selecting',installed:true,targets};return state;},
     startComputerApp:async(request:unknown)=>{(window as any).__nativeCalls.push(request);state={state:'ready',installed:true,target:{id:'native-session',origin:targets[0].title,driver:'windows-uia'}};return state;},
     stopComputerBrowser:async()=>{state={state:'stopped',installed:true};return state;}};
 });
 await page.goto('/ui/#/agents');await page.getByRole('button',{name:'Application window',exact:true}).click();
 await page.getByText('Advanced settings',{exact:true}).click();
 await page.getByRole('combobox',{name:'Action approvals',exact:true}).selectOption('full_task');
 const computerSetup=page.getByRole('group',{name:'Use this computer'});
 await expect(computerSetup.getByText(/approval mode above applies within this scope/i).first()).toBeVisible();
 await expect(computerSetup.getByText(/changes require approval/i)).toHaveCount(0);
 await expect(page.getByLabel('Website (HTTPS)',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'Choose an application',exact:true}).click();
 await page.getByRole('combobox',{name:'Select a window',exact:true}).selectOption('opaque-window');
 connected=true;
 await page.getByRole('button',{name:'Connect application',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).__nativeCalls)).toEqual([{workspace:'desktop-workspace'},{workspace:'desktop-workspace',target:'opaque-window',approvalMode:'full_task'}]);
 await expect(page.locator('.computer-activity')).toContainText('Application connected');
 await expect(page.locator('.computer-activity').getByRole('button',{name:'Stop control'})).toBeEnabled();
 await expect(page.locator('.computer-pair-code')).toHaveCount(0);
 await page.locator('.primary-nav a[href="#/models"]').click();
 await expect(page.locator('.computer-activity').getByRole('button',{name:'Stop control'})).toBeEnabled();
});

test('native permission failures explain OS recovery without raw error codes',async({page})=>{
 await fixture(page);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'desktop-workspace'}}));
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[]}}));
 await page.addInitScript(()=>{(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',getComputerStatus:async()=>({state:'idle',installed:true}),startComputerBrowser:async()=>{},discoverComputerApps:async()=>{throw Error('computer_permission_denied');}};});
 await page.goto('/ui/#/agents');await page.getByRole('button',{name:'Application window',exact:true}).click();
 await page.getByRole('button',{name:'Choose an application',exact:true}).click();
 await expect(page.getByRole('alert').filter({hasText:'Allow accessibility access in your operating system settings'})).toBeVisible();
 await expect(page.getByText('computer_permission_denied',{exact:true})).toHaveCount(0);
});

test('native runtime failures never prescribe browser-package repair',async({page})=>{
 await fixture(page);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'desktop-workspace'}}));
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[]}}));
 await page.addInitScript(()=>{(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',getComputerStatus:async()=>({state:'idle',installed:true}),startComputerBrowser:async()=>{},discoverComputerApps:async()=>{throw Error('computer_worker_unavailable');}};});
 await page.goto('/ui/#/agents');await page.getByRole('button',{name:'Application window',exact:true}).click();
 await page.getByRole('button',{name:'Choose an application',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('Application control could not connect');
 await expect(page.getByRole('alert')).not.toContainText('Browser runtime unavailable');
});

test('desktop browser onboarding uses private IPC without terminal commands or pairing codes', async ({ page }) => {
 await fixture(page);
 await page.route('**/api/v2/system', r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'desktop-workspace'}}));
 await page.route('**/api/v2/computer/sessions', r=>r.fulfill({json:{sessions:[]}}));
 await page.route('**/api/v2/computer/pairing',r=>r.fulfill({json:{code:'a'.repeat(64)}}));
 await page.addInitScript(()=>{
   (window as any).__starts=[];
   (window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',getComputerStatus:async()=>({state:'idle',installed:true}),
     startComputerBrowser:async(request:unknown)=>{(window as any).__starts.push(request);return {state:'idle',code:'consent_declined'};},stopComputerBrowser:async()=>{}};
 });
 await page.goto('/ui/#/agents');
 await page.getByRole('button',{name:'Separate browser',exact:true}).click();
 await page.getByText('Advanced settings',{exact:true}).click();
 await expect(page.getByRole('combobox',{name:'Action approvals',exact:true})).toHaveValue('scoped_changes');
 await expect(page.getByText('Automatically allow reversible work in the selected scope.',{exact:true})).toBeVisible();
 await expect(page.getByRole('button',{name:'Try a practice page',exact:true})).toBeEnabled();
 await expect(page.getByText('Developer connection',{exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Pair browser',exact:true})).toHaveCount(0);
 const task=await page.locator('.agent-task-editor').boundingBox();
 const setup=await page.locator('.computer-setup').boundingBox();
 expect(task!.y+task!.height).toBeLessThan(setup!.y);
 await page.getByRole('button',{name:'Try a practice page',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).__starts.length)).toBe(1);
 expect(await page.evaluate(()=>(window as any).__starts[0])).toEqual({origin:'demo',workspace:'desktop-workspace',networkMode:'direct',approvalMode:'scoped_changes'});
 await expect(page.locator('.computer-pair-code')).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Try a practice page',exact:true})).toBeEnabled();
 await page.getByLabel('Website (HTTPS)',{exact:true}).fill('https://playwright.dev/docs/intro');
 await page.getByText('Network settings',{exact:true}).first().click();
 await page.getByRole('combobox',{name:'Network settings',exact:true}).selectOption('trusted-vpn');
 await page.getByRole('combobox',{name:'Action approvals',exact:true}).selectOption('full_task');
 await expect(page.getByText(/Credentials, payments, privilege changes, installation, permanent deletion, scripts, and uncertain retries stay blocked/)).toBeVisible();
 await expect(page.getByText(/Only enable this for a VPN you trust/)).toBeVisible();
 await page.getByRole('button',{name:'Open browser',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).__starts.length)).toBe(2);
 expect(await page.evaluate(()=>(window as any).__starts[1])).toEqual({origin:'https://playwright.dev/docs/intro',workspace:'desktop-workspace',networkMode:'trusted-vpn',approvalMode:'full_task'});
 await expect(page.getByText('Permission was declined. No browser session started.',{exact:true})).toBeVisible();
});

test('known browser network errors explain recovery rather than prescribing reinstall',async({page})=>{
 await fixture(page);
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'desktop-workspace'}}));
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[]}}));
 await page.addInitScript(()=>{(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',getComputerStatus:async()=>({state:'idle',installed:true}),startComputerBrowser:async()=>{throw Error('network_blocked');}};});
 await page.goto('/ui/#/agents');await page.getByRole('button',{name:'Separate browser',exact:true}).click();
 await page.getByLabel('Website (HTTPS)',{exact:true}).fill('https://example.com/article');
 await page.getByRole('button',{name:'Open browser',exact:true}).click();
 await expect(page.getByRole('alert').filter({hasText:'select Trusted VPN routing'})).toBeVisible();
 await expect(page.getByText(/Repair the matching desktop package/)).toHaveCount(0);
});

test('web browser onboarding leads with desktop handoff, not developer setup', async ({ page }) => {
 await fixture(page);
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[]}}));
 await page.goto('/ui/#/agents');await page.getByRole('button',{name:'Application window',exact:true}).click();
 await expect(page.getByRole('link',{name:'Open desktop app'})).toHaveAttribute('href','offgrid://computer');
 await expect(page.getByText('Developer connection',{exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Pair browser',exact:true})).not.toBeVisible();
 await expect(page.getByRole('button',{name:'Open desktop app',exact:true})).toBeDisabled();
 await expect(page.locator('.agent-metrics')).toHaveCount(0);
});

test('missing saved agent selection clears once without losing the task draft', async ({ page }) => {
 await fixture(page);
 const key = 'offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:';
 await page.addInitScript(({key}) => {
   if (sessionStorage.getItem('seeded-missing-run')) return;
   sessionStorage.setItem('seeded-missing-run', 'true');
   localStorage.setItem(key, 'missing-run');
   localStorage.setItem('offgrid.draft.v1:alice:agent-task:', 'Keep my research draft');
 }, {key});
 let calls = 0;
 await page.route('**/v1/agents/tasks/missing-run', r => { calls++; return r.fulfill({status:404, json:{error:'Agent run not found'}}); });
 await page.goto('/ui/#/agents');
 await expect(page.getByRole('status').filter({hasText:'selected task is no longer available'})).toBeVisible();
 await expect(page.getByRole('textbox', {name:'Task', exact:true})).toHaveValue('Keep my research draft');
 await expect(page.getByText('Something went wrong', {exact:true})).toHaveCount(0);
 expect(await page.evaluate(key => localStorage.getItem(key), key)).toBeNull();
 // React StrictMode may duplicate the initial read, but recovery must survive reload.
 const initialCalls = calls;
 expect(initialCalls).toBeGreaterThan(0);
 await page.reload();
 await expect(page.getByRole('textbox', {name:'Task', exact:true})).toHaveValue('Keep my research draft');
 await expect(page.locator('.task-history article')).toHaveCount(3);
 expect(calls).toBe(initialCalls);
});

test('agent selections from another workspace are never fetched', async ({ page }) => {
 const state = await fixture(page);
 await page.route('**/api/v2/system', r => r.fulfill({json:{product:'offgrid', api_version:2, workspace_id:'new-workspace'}}));
 await page.addInitScript(() => localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Aold-workspace:agent-run:', 'old-run'));
 await page.goto('/ui/#/agents');
 await expect(page.locator('.task-history article')).toHaveCount(3);
 expect(state.calls).not.toContain('/v1/agents/tasks/old-run');
});

test('computer tasks ignore retired verification drafts and need no extra configuration', async ({ page }) => {
 await fixture(page);
 const obsoleteKey = 'offgrid.draft.v1:alice%3Aworkspace%3Alegacy:computer-expected-text:';
 const unrelatedKey = 'offgrid.draft.v1:bob%3Aworkspace%3Aother:computer-expected-text:';
 await page.addInitScript(({obsoleteKey,unrelatedKey}) => {
   localStorage.setItem(obsoleteKey, 'done');
   localStorage.setItem(unrelatedKey, 'Do not touch another workspace');
   localStorage.setItem('offgrid.draft.v1:alice:agent-task:', 'Keep this task draft');
 }, {obsoleteKey,unrelatedKey});
 let passed = false;
 let submissions = 0;
 await page.route('**/api/v2/computer/sessions', r => r.fulfill({json:{sessions:[{id:'demo-session',origin:'offgrid-demo://research',remaining_actions:100,state:'ready'}]}}));
 await page.route('**/api/v2/computer/model-check', r => r.fulfill({json:{model:r.request().postDataJSON().model,passed,code:passed?'computer_tool_check_passed':'computer_tool_calling_unavailable',message:passed?'Two-step smoke check passed.':'Template cannot use tools. Choose a compatible model.',retryable:false}}));
 await page.route('**/v1/agents/run', r => { submissions++; expect(r.request().postDataJSON()).not.toHaveProperty('computer_expected_text'); return r.fulfill({status:422,json:{error:{message:'Model changed; recheck compatibility.'}}}); });
 await page.goto('/ui/#/agents');
 await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Keep this task draft');
 await page.getByRole('button',{name:'Separate browser',exact:true}).click();
 await page.getByRole('combobox',{name:'Target'}).selectOption('demo-session');
 await page.getByRole('textbox',{name:'Task',exact:true}).fill('Inspect the demo');
 const run = page.getByRole('button',{name:'Run task',exact:true});
 await expect(run).toBeEnabled();
 await expect(page.locator('#computer-expected-text')).toHaveCount(0);
 await expect(page.getByText('Verification settings (optional)',{exact:true})).toHaveCount(0);
 expect(await page.evaluate(key=>localStorage.getItem(key),obsoleteKey)).toBeNull();
 expect(await page.evaluate(key=>localStorage.getItem(key),unrelatedKey)).toBe('Do not touch another workspace');
 await page.getByText('Advanced settings',{exact:true}).click();
 await page.getByRole('button',{name:'Check selected model'}).click();
 await expect(page.getByRole('status').filter({hasText:'Template cannot use tools'})).toBeVisible();
 await expect(run).toBeEnabled();
 expect(submissions).toBe(0);
 passed = true;
 await page.getByRole('button',{name:'Check selected model'}).click();
 await expect(run).toBeEnabled();
 await page.getByRole('combobox',{name:'Model',exact:true}).selectOption('model-b');
 await expect(run).toBeEnabled();
 await expect(page.getByText('Two-step smoke check passed.',{exact:true})).toHaveCount(0);
 expect(submissions).toBe(0);
 await run.click();
 await expect(page.getByRole('alert').filter({hasText:'Model changed; recheck compatibility.'})).toBeVisible();
 expect(submissions).toBe(1);
 // An older tab can recreate this setting; the request boundary still ignores it.
 await page.evaluate(key=>localStorage.setItem(key,'done'.repeat(1000)),obsoleteKey);
 await run.click();
 await expect.poll(()=>submissions).toBe(2);
 await page.reload();
 await expect(page.getByRole('textbox',{name:'Task',exact:true})).toHaveValue('Keep this task draft');
 await expect(page.locator('#computer-expected-text')).toHaveCount(0);
 expect(await page.evaluate(key=>localStorage.getItem(key),obsoleteKey)).toBeNull();
});

test('consumed browser sessions cannot submit or pair again until stopped', async ({ page }) => {
 await fixture(page);
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[{id:'used',origin:'offgrid-demo://research',remaining_actions:90,state:'finished',run_id:'old'}]}}));
 await page.goto('/ui/#/agents');
 await page.getByRole('button',{name:'Separate browser',exact:true}).click();
 await expect(page.getByRole('option',{name:/Finished — start a new session/})).toHaveAttribute('disabled','');
 await expect(page.getByText('Developer connection',{exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Pair browser',exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'Open desktop app',exact:true})).toBeDisabled();
 await expect(page.getByText('Stop this session and restart the companion to approve another task.')).toBeVisible();
});

test('browser stop and task mode survive navigation and reload during approval', async ({ page }) => {
 await fixture(page);
 let stops=0;
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:stops?[]:[{id:'paired',origin:'offgrid-demo://research',remaining_actions:98,state:'in_use',run_id:'browser-run'}]}}));
 await page.route('**/api/v2/computer/stop',r=>{stops++;return r.fulfill({status:202,json:{status:'stopping'}})});
 await page.addInitScript(()=>localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:','browser-run'));
 await page.route('**/v1/agents/tasks/browser-run',r=>r.fulfill({json:{run_id:'browser-run',task_id:'browser-run',status:'waiting_for_approval',output:'',steps:[],computer_session:'paired',computer_expected_text:'Draft saved: OffGrid test',pending_approval:{id:'approval',tool:'browser_fill',arguments:{text:'OffGrid test'},expires_at:new Date(Date.now()+60000).toISOString()}}}));
 await page.goto('/ui/#/agents');
 await expect(page.getByRole('button',{name:'Separate browser',exact:true})).toHaveAttribute('aria-pressed','true');
 await expect(page.locator('#computer-expected-text')).toHaveCount(0);
 await page.locator('.primary-nav a[href="#/models"]').click();
 await expect(page.locator('.computer-activity').getByRole('button',{name:'Stop browser'})).toBeEnabled();
 await page.locator('.primary-nav a[href="#/agents"]').click();
 await expect(page.getByRole('button',{name:'Separate browser',exact:true})).toHaveAttribute('aria-pressed','true');
 await page.reload();
 await expect(page.getByRole('button',{name:'Separate browser',exact:true})).toHaveAttribute('aria-pressed','true');
 await page.locator('.computer-activity').getByRole('button',{name:'Stop browser'}).click();
 await expect(page.locator('.computer-activity')).toContainText('Stop requested');
 expect(stops).toBe(1);
});

test('Stop revokes the service even when local shutdown is unconfirmed', async ({ page }) => {
 await fixture(page);let stops=0;
 await page.addInitScript(()=>{(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',stopComputerBrowser:async()=>({state:'error',code:'stop_unconfirmed'})};});
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:stops?[]:[{id:'paired',origin:'offgrid-demo://research',state:'in_use',remaining_actions:10}]}}));
 await page.route('**/api/v2/computer/stop',r=>{stops++;return r.fulfill({status:202,json:{status:'stopping'}})});
 await page.goto('/ui/#/agents');
 const strip=page.locator('.computer-activity');await strip.getByRole('button',{name:'Stop browser'}).click();
 await expect(strip.getByRole('alert')).toContainText('Browser shutdown was not confirmed');
 await expect.poll(()=>stops).toBe(1);
 await expect(strip.getByRole('button',{name:'Stop browser'})).toBeEnabled();
 await expect(strip.getByRole('button',{name:'Close',exact:true})).toHaveCount(0);
});

test('Stop still closes the local browser when the service is offline', async ({ page }) => {
 await fixture(page);
 await page.addInitScript(()=>{(window as any).__localStops=0;(window as any).electron={onThemeChange:()=>()=>{},getSystemTheme:async()=>'light',stopComputerBrowser:async()=>{(window as any).__localStops++;return {state:'stopped'};}};});
 await page.route('**/api/v2/computer/sessions',r=>r.fulfill({json:{sessions:[{id:'paired',origin:'offgrid-demo://research',state:'in_use',remaining_actions:10}]}}));
 await page.route('**/api/v2/computer/stop',r=>r.fulfill({status:503,json:{error:'Offline fixture'}}));
 await page.goto('/ui/#/agents');const strip=page.locator('.computer-activity');
 await strip.getByRole('button',{name:'Stop browser'}).click();
 await expect(strip.getByRole('alert')).toContainText('The browser is closed locally');
 expect(await page.evaluate(()=>(window as any).__localStops)).toBe(1);
});

test('form approval cards show the observed option label and explicit checkbox state', async ({ page }) => {
 await fixture(page);
 await page.addInitScript(()=>localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:','browser-form'));
 let checked=false;
 await page.route('**/v1/agents/tasks/browser-form',r=>r.fulfill({json:{run_id:'browser-form',task_id:'browser-form',status:'waiting_for_approval',output:'',computer_session:'paired',
  steps:[{id:'observe',tool_name:'browser_observe',tool_result:JSON.stringify({observation_id:'current',elements:[{id:'1',label:'Region',options:[{id:'2',label:'Zimbabwe — 2026'}]},{id:'2',label:'Include sources'}]})}],
  pending_approval:{id:'approval',tool:checked?'browser_set_checked':'browser_select',arguments:checked?{observation_id:'current',element:'2',checked:false}:{observation_id:'current',element:'1',option:'2'},expires_at:new Date(Date.now()+60000).toISOString()}}}));
 await page.goto('/ui/#/agents');
 const card=page.locator('.browser-action-summary');
 await expect(card).toContainText('Select an option');await expect(card).toContainText('Region');await expect(card).toContainText('Zimbabwe — 2026');
 checked=true;await page.reload();
 await expect(card).toContainText('Set a checkbox');await expect(card).toContainText('Include sources');await expect(card).toContainText('Unchecked');
});

test('computer task history distinguishes automatic authorization from verification', async ({page})=>{
 await fixture(page);
 await page.addInitScript(()=>localStorage.setItem('offgrid.draft.v1:alice%3Aworkspace%3Alegacy:agent-run:','approval-audit'));
 await page.route('**/v1/agents/tasks/approval-audit',r=>r.fulfill({json:{run_id:'approval-audit',task_id:'approval-audit',status:'completed',output:'Draft saved.',computer_session:'paired',computer_approval_mode:'scoped_changes',steps:[{id:1,tool_name:'browser_fill',tool_result:'{"verified":true}',authorization:'automatic:scoped_changes'}]}}));
 await page.goto('/ui/#/agents');
 const timeline=page.locator('.browser-task-timeline');
 await expect(timeline).toContainText('Automatically approved · Approve scoped changes');
 await expect(timeline).not.toContainText('Verified · scoped changes');
});

test('history toolbar and search field have deliberate vertical separation', async ({ page }, testInfo) => {
  await fixture(page); await page.goto('/ui/#/agents');
  await expect(page.locator('.task-history article')).toHaveCount(3);
  const heading = await page.locator('.agent-history-panel > .section-heading').boundingBox();
  const search = await page.locator('.agent-history-panel .history-search').boundingBox();
  expect(search!.y - heading!.y - heading!.height).toBeGreaterThanOrEqual(12);
  await page.locator('.agent-history-panel').screenshot({ path: testInfo.outputPath('history-desktop.png') });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.locator('.agent-history-panel .history-search')).toBeVisible();
  expect(await page.locator('.agent-history-panel').evaluate(e => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await page.locator('.agent-history-panel').screenshot({ path: testInfo.outputPath('history-mobile.png') });
});

test('command palette contains focus, handles Escape outside input and ignores composition Enter', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: /Quick actions/ }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('combobox').dispatchEvent('keydown', { key: 'Enter', isComposing: true });
  await expect(dialog).toBeVisible();
  for (let i = 0; i < 18; i++) { await page.keyboard.press('Tab'); expect(await page.evaluate(() => !!document.activeElement?.closest('dialog'))).toBe(true); }
  await page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Quick actions/ })).toBeFocused();
});

test('mobile conversation drawer contains focus and Escape restores its trigger', async ({ page }) => {
  await fixture(page); await page.setViewportSize({ width: 390, height: 844 }); await page.goto('/ui/#/chat');
  const trigger = page.getByRole('button', { name: 'Show conversations', exact: true });
  await trigger.click();
  for (let i = 0; i < 8; i++) { await page.keyboard.press('Tab'); expect(await page.evaluate(() => !!document.activeElement?.closest('#conversation-history'))).toBe(true); }
  await page.keyboard.press('Escape'); await expect(page.locator('#conversation-history')).not.toBeVisible(); await expect(trigger).toBeFocused();
});

test('member UI does not call administrator endpoints or offer privileged actions', async ({ page }) => {
  const state = await fixture(page); state.role = 'user';
  await page.goto('/ui/#/agents'); await expect(page.getByText('Administrator access required')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Run task', exact: true })).toHaveCount(0);
  await page.goto('/ui/#/settings'); await expect(page.locator('.settings-page')).toContainText('Administrator access required');
  await page.goto('/ui/#/models'); await expect(page.locator('.catalog-card .primary-button')).toBeDisabled();
  await page.goto('/ui/#/knowledge'); await expect(page.getByRole('button', { name: 'Add document' })).toBeDisabled();
  expect(state.calls.filter(path => path.startsWith('/v1/agents/') || path === '/api/v2/computer/status' || path === '/v1/models/download/progress')).toEqual([]);
});

test('anonymous local workspace remains usable when authentication is explicitly disabled', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/users/me', r => r.fulfill({ json: { authenticated: false, guest: true, auth_required: false, user: null } }));
  await page.goto('/ui/#/agents'); await expect(page.getByRole('button', { name: 'Run task', exact: true })).toBeVisible();
});

test('history loading, failure and successful recovery stay distinct', async ({ page }) => {
  const state = await fixture(page); state.slowHistory = true; state.failHistory = true;
  await page.goto('/ui/#/agents'); await expect(page.locator('.agent-history-panel')).toContainText('Preparing history');
  await expect(page.locator('.agent-history-panel')).not.toContainText('No agent tasks');
  await expect(page.locator('.agent-history-panel')).toContainText('History refresh failed');
  state.failHistory = false; state.slowHistory = false;
  await page.locator('.topbar').getByRole('button', { name: 'Refresh' }).click();
  await expect(page.locator('.agent-history-panel')).toContainText('Task one'); await expect(page.getByRole('alert')).toHaveCount(0);
});

test('statistics failure does not hide available run history', async ({ page }) => {
  const state = await fixture(page); state.failStats = true;
  await page.goto('/ui/#/activity'); await page.getByRole('button', { name: /Saved run/ }).click();
  await expect(page.locator('.event-list')).toContainText('No events recorded');
});

test('connector drafts and selected Activity survive navigation without refetching online searches', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/agents/connections');
  await page.locator('.connector-panel input').nth(0).fill('Private connector'); await page.locator('.connector-panel input').nth(1).fill('http://localhost:3000/mcp');
  await page.locator('.primary-nav a[href="#/models"]').click(); await page.getByLabel('Model name or publisher', { exact: true }).fill('Research model');
  await page.locator('.primary-nav a[href="#/agents"]').click(); await page.getByRole('tab', { name: 'Connections' }).click();
  await expect(page.locator('.connector-panel input').nth(0)).toHaveValue('Private connector');
  await page.locator('.primary-nav a[href="#/models"]').click(); await expect(page.getByLabel('Model name or publisher', { exact: true })).toHaveValue('Research model');
  await page.locator('.primary-nav a[href="#/activity"]').click(); await page.getByRole('button', { name: /Saved run/ }).click();
  await page.locator('.primary-nav a[href="#/models"]').click(); await page.locator('.primary-nav a[href="#/activity"]').click();
  await expect(page.locator('.run-row.selected')).toContainText('Saved run');
});

test('bulk deletion can stop after the in-flight item without deleting remaining entries', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/agents');
  await page.getByRole('button', { name: 'Clear removable tasks' }).click();
  const dialog = page.getByRole('dialog'); await dialog.getByRole('button', { name: 'Confirm delete' }).click();
  await expect(dialog.getByRole('status')).toHaveText('0 of 3 processed');
  await dialog.getByRole('button', { name: 'Stop after current item' }).click();
  await expect(dialog.getByRole('button', { name: 'Cancel', exact: true })).toBeEnabled();
  expect(state.deletes).toHaveLength(1); await expect(dialog).not.toContainText('Task one');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.locator('.task-history article')).toHaveCount(2);
});

test('unknown index status is never presented as ready and unavailable knowledge cannot be enabled in chat', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/knowledge');
  await expect(page.locator('.resource-card')).toContainText('Status unavailable');
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).toBeDisabled();
});

test('late provider setup cannot overwrite instructions for a newly selected model', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/integrations?*', r => r.fulfill({ json: { integrations: [{ id: 'hermes', name: 'Hermes', provider_id: 'offgrid', transport: 'openai-chat-completions', model_id: new URL(r.request().url()).searchParams.get('model') || 'model-a', context_window: 8192, minimum_context: 8192, warnings: [], ready: true }] } }));
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let requested = false;
  await page.route('**/v1/integrations/hermes/setup*', async r => {
    requested = true; await held;
    await r.fulfill({ json: { setup: { install_command: 'STALE MODEL A', verify: [], environment: {} } } });
  });
  await page.goto('/ui/#/agents/connections');
  await page.locator('.provider-card button').click();
  await expect.poll(() => requested).toBe(true);
  await page.getByRole('tab', { name: 'Work', exact: true }).click();
  await page.getByRole('combobox', { name: 'Model', exact: true }).selectOption('model-b');
  release();
  await page.getByRole('tab', { name: 'Connections' }).click();
  await expect(page.locator('.provider-card')).toContainText('model-b');
  await expect(page.locator('.provider-setup')).toHaveCount(0);
});
