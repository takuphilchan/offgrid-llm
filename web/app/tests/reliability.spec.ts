import { expect, test, type Page } from '@playwright/test';

const runID = 'run-' + 'a'.repeat(32);

async function workspace(page: Page) {
  let actor = 'alice';
  const sessions: Record<string, any> = {};
  let generationStatus = 503;
  let creates = 0;
  const actions: string[] = [];
  let runStatus = 'waiting_for_approval';
  const approval = { id: 'approval-1', run_id: 'run-1', call_id: 'call-1', actor, tool: 'write_file', arguments: { path: 'notes.txt', content: 'hello' }, expires_at: new Date(Date.now() + 600_000).toISOString() };
  const run = () => ({ run_id: runID, task_id: runID, prompt: 'Write notes', model: 'test-model', status: runStatus, output: runStatus === 'completed' ? 'Saved once' : '', steps: [], pending_approval: runStatus === 'waiting_for_approval' ? approval : null });
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  // Every API request is intercepted: these tests never execute real tools,
  // download models, or change the user's running OffGrid service.
  await page.route(/\/(?:v1|api\/v2)\//, async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    let json: any = {};
    let status = 200;
    if (path === '/api/v2/system') json = {product:'offgrid',version:'test',api_version:2,workspace_id:'reliability-fixture',capabilities:['task-first-agents-v2']};
    else if (path === '/v1/users/me') json = { authenticated: true, user: { id: actor, username: actor, role: 'admin' } };
    else if (path === '/v1/models') json = { data: [{ id: 'test-model', type: 'chat' }] };
    else if (path === '/v1/sessions' && request.method() === 'GET') json = { sessions: Object.values(sessions) };
    else if (path === '/v1/sessions' && request.method() === 'POST') {
      const body = request.postDataJSON();
      json = sessions[body.name] = { ...body, messages: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() }; status = 201;
    } else if (path.endsWith('/generate')) { status = generationStatus; json = { error: 'Knowledge retrieval unavailable' }; }
    else if (path.startsWith('/v1/sessions/')) json = sessions[decodeURIComponent(path.split('/')[3])] ?? {};
    else if (path === '/api/v2/jobs' && request.method() === 'POST') {
      const body = request.postDataJSON();
      expect(body.approved_tool_calls).toBeUndefined();
      expect(body.request_id).toBeTruthy();
      creates++; json = run(); status = 202;
    } else if (path === `/api/v2/jobs/${runID}`) json = run();
    else if (path === `/api/v2/jobs/${runID}/events`) return route.fulfill({contentType:'text/event-stream',body:': heartbeat\n\n'});
    else if (path.startsWith(`/api/v2/jobs/${runID}/`)) {
      const action = path.split('/').at(-1)!;
      const body = request.postDataJSON();
      expect(body.prompt).toBeUndefined();
      expect(body.approval_id).toBe('approval-1');
      actions.push(action); runStatus = action === 'approve' ? 'completed' : 'cancelled'; json = run();
    } else if (path === '/api/v2/jobs' || path === '/v1/agents/tasks') json = creates ? [{ id: runID, prompt: 'Write notes', status: runStatus, created_at: new Date().toISOString() }] : [];
    else if (path === '/v1/agents/tools') json = { tools: [], enabled_count: 0 };
    else if (path === '/v1/agents/mcp') json = { servers: [] };
    else if (path === '/v1/integrations') json = { integrations: [] };
    else if (path === '/v1/catalog') json = { models: [] };
    await route.fulfill({ status, json });
  });
  return { switchUser: (id: string) => { actor = id; }, creates: () => creates, actions, generationStatus: (status: number) => { generationStatus = status; } };
}

test('chat and agent drafts survive navigation and reload without leaking to another account', async ({ page }) => {
  const state = await workspace(page);
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Private Alice draft');
  await page.locator('.primary-nav a[href="#/agents"]').click();
  await page.locator('.task-composer textarea').fill('Unsent agent task');
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await expect(page.locator('.composer textarea')).toHaveValue('Private Alice draft');
  await page.reload();
  await expect(page.locator('.composer textarea')).toHaveValue('Private Alice draft');
  state.switchUser('bob');
  await page.reload();
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await page.locator('.primary-nav a[href="#/agents"]').click();
  await expect(page.locator('.task-composer textarea')).toHaveValue('');
  state.switchUser('alice');
  await page.reload();
  await expect(page.locator('.task-composer textarea')).toHaveValue('Unsent agent task');
});

test('failed chat send retains the draft in the created conversation', async ({ page }) => {
  await workspace(page);
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Question that must not disappear');
  await page.locator('.composer button').click();
  await expect(page.getByRole('alert')).toContainText('Knowledge retrieval unavailable');
  await expect(page.locator('.composer textarea')).toHaveValue('Question that must not disappear');
  await page.reload();
  await expect(page.locator('.composer textarea')).toHaveValue('Question that must not disappear');
});

test('approval after reload continues the same run exactly once', async ({ page }) => {
  const state = await workspace(page);
  await page.goto('/ui/#/agents');
  await page.locator('.task-composer textarea').fill('Write notes');
  await page.getByRole('button', { name: 'Start task', exact: true }).click();
  await expect(page.locator('.approval-card')).toBeVisible();
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await page.locator('.primary-nav a[href="#/agents"]').click();
  await page.reload();
  await expect(page.locator('.approval-card')).toContainText('notes.txt');
  await page.getByRole('button', { name: 'Approve exact call' }).dblclick();
  await expect(page.locator('.task-detail')).toContainText('Saved once');
  expect(state.creates()).toBe(1);
  expect(state.actions).toEqual(['approve']);
});

test('denial reaches the server and stays denied after reload', async ({ page }) => {
  const state = await workspace(page);
  await page.goto('/ui/#/agents');
  await page.locator('.task-composer textarea').fill('Write notes');
  await page.getByRole('button', { name: 'Start task', exact: true }).click();
  await page.getByRole('button', { name: 'Deny', exact: true }).click();
  await expect(page.locator('.task-detail')).toContainText('Cancelled');
  await page.reload();
  await expect(page.locator('.approval-card')).toHaveCount(0);
  await expect(page.locator('.task-detail')).toContainText('Cancelled');
  expect(state.actions).toEqual(['deny']);
  expect(state.creates()).toBe(1);
});

test('a late chat response cannot erase a newer draft', async ({ page }) => {
  await workspace(page);
  let finish: (() => void) | undefined;
  await page.route('**/v1/sessions/*/generate', async route => {
    await new Promise<void>(resolve => { finish = resolve; });
    const name = decodeURIComponent(new URL(route.request().url()).pathname.split('/')[3]);
    const message = { role: 'assistant', content: 'Saved answer' };
    await route.fulfill({ contentType: 'text/event-stream', body: `data: ${JSON.stringify({ type: 'done', session: { name, model_id: 'test-model', messages: [{ role: 'user', content: 'Original' }, message], updated_at: new Date().toISOString() }, message })}\n\n` });
  });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Original');
  await page.locator('.composer button').click();
  await expect.poll(() => !!finish).toBe(true);
  await page.locator('.composer textarea').fill('My next question');
  finish!();
  await expect(page.locator('.message.assistant')).toContainText('Saved answer');
  await expect(page.locator('.composer textarea')).toHaveValue('My next question');
  await page.reload();
  await expect(page.locator('.composer textarea')).toHaveValue('My next question');
});

test('failed browser draft storage warns without discarding typed text', async ({ page }) => {
  await workspace(page);
  await page.addInitScript(() => {
    const set = Storage.prototype.setItem;
    Storage.prototype.setItem = function(key: string, value: string) {
      if (key.startsWith('offgrid.draft.')) throw new DOMException('Storage full', 'QuotaExceededError');
      set.call(this, key, value);
    };
  });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Do not lose this text');
  await expect(page.getByRole('alert')).toContainText(/draft/i);
  await expect(page.locator('.composer textarea')).toHaveValue('Do not lose this text');
  await page.locator('.primary-nav a[href="#/agents"]').click();
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await expect(page.locator('.composer textarea')).toHaveValue('Do not lose this text');
});
