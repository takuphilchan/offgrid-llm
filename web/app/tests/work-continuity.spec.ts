import { expect, test, type Page } from '@playwright/test';
const id = 'run-' + 'a'.repeat(32);
async function fixture(page: Page, count = 1) {
  const state = { actor: 'alice', workspace: 'one', role: 'admin', writes: [] as string[], reads: [] as string[], completed: false };
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(api\/v2|v1)\//, r => {
    const path = new URL(r.request().url()).pathname;
    (r.request().method() === 'GET' ? state.reads : state.writes).push(path);
    const task = { id, prompt: 'Keep the same task', status: state.completed ? 'completed' : 'waiting_for_approval', model: 'model', created_at: '2026-10-08T00:00:00Z', deletable: false };
    const chats = ['First conversation', 'Second conversation'].map(name => ({ name, messages: [{ role: 'assistant', content: name }], created_at: '2026-10-08T00:00:00Z', updated_at: '2026-10-08T00:00:00Z' }));
    const data: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: state.workspace, capabilities: ['task-first-agents-v2'] },
      '/v1/users/me': { authenticated: true, auth_required: true, user: { id: state.actor, role: state.role, username: state.actor } },
      '/v1/models': { data: [{ id: 'model', type: 'chat' }] }, '/v1/catalog': { models: [] },
      '/v1/sessions': { sessions: chats }, '/v1/rag/status': { enabled: false },
      '/api/v2/jobs': state.actor === 'alice' && state.workspace === 'one' ? Array.from({ length: count }, (_, i) => ({ ...task, id: i ? 'run-' + String(i).padStart(32, '0') : id, prompt: i ? `Task ${i}` : task.prompt })) : [],
      [`/api/v2/jobs/${id}`]: { ...task, run_id: id, output: '', steps: [] },
    };
    if (path.endsWith('/turn')) return r.fulfill({ json: { turn: { id: 'turn', status: 'completed', prompt: '', output: '' } } });
    return r.fulfill({ json: data[path] ?? {} });
  });
  return state;
}

test('known task opens its canonical identity from another page without writes', async ({ page }) => {
  const state = await fixture(page);
  await page.goto('/ui/#/models');
  await page.getByRole('button', { name: 'Known work', exact: true }).click();
  await page.getByRole('link', { name: 'Keep the same task' }).click();
  await expect(page).toHaveURL(new RegExp(`#\/agents\/task\/${id}$`));
  await expect(page.locator('#task-title')).toHaveText('Keep the same task');
  await page.locator('.primary-nav a[href="#/settings"]').click();
  await page.getByRole('button', { name: 'Known work', exact: true }).click();
  await page.getByRole('link', { name: 'Keep the same task' }).click();
  await expect(page.locator('#task-title')).toHaveText('Keep the same task');
  expect(state.writes).toEqual([]);
  expect(state.reads.filter(path => path === `/api/v2/jobs/${id}`).length).toBeLessThanOrEqual(3);
});

test('work list bounds its metadata and makes overflow explicit', async ({ page }) => {
  await fixture(page, 40); await page.goto('/ui/#/settings');
  const trigger = page.getByRole('button', { name: 'Known work', exact: true });
  await expect(trigger).toContainText('16+'); await trigger.click();
  await expect(page.locator('.known-work-list li')).toHaveCount(16);
  await expect(page.getByRole('region', { name: 'Known work' }).getByRole('link', { name: 'Recent tasks' })).toHaveAttribute('href', '#/agents');
});

test('Activity links only known job identities and keeps history when diagnostics fail', async ({ page }) => {
  const state = await fixture(page); state.completed = true;
  await page.route('**/v1/stats', r => r.fulfill({ status: 503, json: { error: 'Counters unavailable' } }));
  await page.route('**/v1/runs', r => r.fulfill({ json: { runs: [
    { id, status: 'completed', updated_at: '2026-10-08T00:00:00Z', data: { prompt: 'Same saved job' } },
    { id: 'legacy-import', status: 'running', updated_at: '2026-10-07T00:00:00Z', data: { prompt: 'Legacy import' } },
  ] } }));
  await page.route('**/v1/runs/*/events', r => r.fulfill({ json: { events: [] } }));
  await page.goto('/ui/#/activity');
  await expect(page.locator('.run-row').first()).toContainText('Legacy import');
  await page.getByRole('button', { name: /^Legacy import/ }).click();
  await expect(page.getByRole('link', { name: 'Open task', exact: true })).toHaveCount(0);
  await expect(page.getByText('Counters unavailable')).toBeHidden();
  await page.getByText('Diagnostics', { exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Counters unavailable');
  await page.getByRole('button', { name: /^Same saved job/ }).click();
  await page.getByRole('link', { name: 'Open task', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`#/agents/task/${id}$`));
  await expect(page.locator('#task-title')).toContainText('Keep the same task');
  expect(state.writes).toEqual([]);
});

test('Activity does not fetch privileged history for a member', async ({ page }) => {
  const state = await fixture(page); state.role = 'user';
  await page.goto('/ui/#/activity');
  await expect(page.getByText('Administrator access required')).toBeVisible();
  expect(state.reads.filter(path => path === '/v1/runs' || path === '/api/v2/jobs')).toEqual([]);
  await expect(page.getByRole('link', { name: 'Open task', exact: true })).toHaveCount(0);
});

for (const field of ['actor', 'workspace', 'role'] as const) test(`${field} change clears known private work`, async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/models');
  await expect(page.getByRole('button', { name: 'Known work', exact: true })).toBeVisible();
  state[field] = field === 'role' ? 'user' : 'another';
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Known work', exact: true })).toHaveCount(0);
  await expect(page.getByText('Keep the same task', { exact: true })).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('known chat selection returns to that conversation without scanning other turns or speaking', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/chat');
  await expect(page.locator('.message.assistant')).toContainText('First conversation');
  await page.getByRole('button', { name: /^Second conversation 1/ }).click();
  await expect(page.locator('.message.assistant')).toContainText('Second conversation');
  await page.locator('.primary-nav a[href="#/models"]').click();
  await page.getByRole('button', { name: 'Known work', exact: true }).click();
  await page.getByRole('region', { name: 'Known work' }).getByRole('button', { name: 'First conversation', exact: true }).click();
  await expect(page.locator('.message.assistant')).toContainText('First conversation');
  expect(state.writes).toEqual([]);
  expect(state.reads.filter(path => path.endsWith('/turn')).length).toBeLessThanOrEqual(4);
});

test('terminal task followers stop polling and logout removes private work', async ({ page }) => {
  const state = await fixture(page); state.completed = true;
  await page.clock.install();
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('#task-title')).toHaveText('Keep the same task');
  const reads = state.reads.filter(path => path === `/api/v2/jobs/${id}`).length;
  await page.clock.fastForward(15_000);
  await expect(page.locator('.task-state')).toContainText('Completed');
  expect(state.reads.filter(path => path === `/api/v2/jobs/${id}`).length).toBe(reads);
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await expect(page.getByRole('button', { name: 'Known work', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Workspace options', exact: true }).click();
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Known work', exact: true })).toHaveCount(0);
  expect(state.writes).toEqual(['/v1/auth/logout']);
});

async function durableChat(page: Page) {
  const base = await fixture(page);
  const state = { started: false, completed: false, recoverable: true };
  const session = () => ({ name: 'First conversation', messages: state.completed ? [{ role: 'assistant', content: 'Saved after navigation' }] : [], updated_at: '2026-10-08T00:00:00Z' });
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [session(), { name: 'Second conversation', messages: [{ role: 'assistant', content: 'Other saved answer' }], updated_at: '2026-10-08T00:00:00Z' }] } }));
  await page.route('**/v1/sessions/First%20conversation', r => r.fulfill({ json: session() }));
  await page.route('**/v1/sessions/First%20conversation/turn', r => r.fulfill({ json: { turn: state.started && state.recoverable ? { id: 'same-turn', status: state.completed ? 'completed' : 'running', prompt: 'Keep this request', output: '' } : null } }));
  await page.addInitScript(() => {
    (window as any).submissions = 0;
    const original = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (!String(input).endsWith('/generate')) return original(input, init);
      (window as any).submissions++;
      return new Response(new ReadableStream({ start(controller) {
        const encoder = new TextEncoder();
        controller.enqueue(encoder.encode('data: {"type":"delta","delta":"Old provisional answer"}\n\n'));
        (window as any).loseAck = () => controller.close();
        init?.signal?.addEventListener('abort', () => controller.error(new DOMException('Detached', 'AbortError')), { once: true });
      } }), { headers: { 'Content-Type': 'text/event-stream' } });
    };
  });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep this request');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  state.started = true;
  await expect(page.locator('.streaming-response')).toContainText('Old provisional answer');
  return { base, state };
}

test('leaving a live turn detaches reads and returning reconciles saved work without cancellation or replay', async ({ page }) => {
  const { base, state } = await durableChat(page);
  await page.locator('.primary-nav a[href="#/models"]').click();
  state.completed = true;
  await page.getByRole('button', { name: 'Known work', exact: true }).click();
  await page.getByRole('region', { name: 'Known work' }).getByRole('button', { name: 'First conversation', exact: true }).click();
  await expect(page.locator('.message.assistant')).toContainText('Saved after navigation');
  await expect(page.locator('.streaming-response')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  expect(await page.evaluate(() => (window as any).submissions)).toBe(1);
  expect(base.writes).toEqual([]);
  await page.reload();
  await expect(page.locator('.message.assistant')).toContainText('Saved after navigation');
  expect(base.writes).toEqual([]);
});

test('switching from a live turn cannot let its late error replace another conversation', async ({ page }) => {
  const { base } = await durableChat(page);
  // The shared work menu can switch while the old visible stream is running.
  await page.evaluate(() => window.dispatchEvent(new CustomEvent('offgrid:select-known-chat', { detail: 'Second conversation' })));
  await expect(page.locator('.message.assistant')).toContainText('Other saved answer');
  await expect(page.locator('.streaming-response')).toHaveCount(0);
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeDisabled();
  expect(base.writes).toEqual([]);
});

test('lost acknowledgment checks the accepted turn and never resubmits the prompt', async ({ page }) => {
  const { base, state } = await durableChat(page);
  await page.evaluate(() => (window as any).loseAck());
  await expect(page.getByRole('button', { name: 'Check request status' })).toBeVisible();
  state.completed = true;
  await page.getByRole('button', { name: 'Check request status' }).click();
  await expect(page.locator('.message.assistant')).toContainText('Saved after navigation');
  expect(await page.evaluate(() => (window as any).submissions)).toBe(1);
  expect(base.writes).toEqual([]);
});

test('unsupported turn recovery retains the draft and explicitly prevents uncertain resubmission', async ({ page }) => {
  const { base, state } = await durableChat(page);
  await page.locator('.primary-nav a[href="#/models"]').click(); state.recoverable = false;
  await page.getByRole('button', { name: 'Known work', exact: true }).click();
  await page.getByRole('region', { name: 'Known work' }).getByRole('button', { name: 'First conversation', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('cannot recover the previous turn');
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this request');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeDisabled();
  expect(await page.evaluate(() => (window as any).submissions)).toBe(1);
  expect(base.writes).toEqual([]);
});

test('stale task snapshots cannot restore an older approval', async ({ page }) => {
  const state = await fixture(page);
  let cursor = '20';
  await page.route(`**/api/v2/jobs/${id}`, r => r.fulfill({ json: {
    run_id: id, prompt: 'Keep the same task', status: 'waiting_for_approval', output: '', steps: [], event_cursor: cursor,
    pending_approval: { id: `approval-${cursor}`, tool: `tool-${cursor}`, arguments: { version: cursor }, expires_at: '2099-01-01T00:00:00Z' },
  } }));
  await page.clock.install(); await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.approval-card')).toContainText('20');
  cursor = '19'; await page.clock.fastForward(2600);
  await expect(page.locator('.approval-card')).toContainText('20');
  await expect(page.locator('.approval-card')).not.toContainText('19');
  cursor = '21'; await page.clock.fastForward(2600);
  await expect(page.locator('.approval-card')).toContainText('21');
  expect(state.writes).toEqual([]);
});

test('removed task loses its controls and stops following without mutation', async ({ page }) => {
  const state = await fixture(page);
  await page.clock.install(); await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('button', { name: 'Stop', exact: true })).toBeVisible();
  let reads = 0;
  await page.route(`**/api/v2/jobs/${id}`, r => { reads++; return r.fulfill({ status: 404, json: { error: 'This task is no longer available' } }); });
  await page.clock.fastForward(2600);
  await expect(page.getByRole('alert')).toContainText('no longer available');
  await expect(page.getByRole('button', { name: 'Stop', exact: true })).toHaveCount(0);
  await page.clock.fastForward(15_000); expect(reads).toBe(1);
  expect(state.writes).toEqual([]);
});
