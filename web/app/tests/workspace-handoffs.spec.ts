import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const state = { workspace: 'workspace-a', actor: 'alice', role: 'admin', models: false, knowledge: false, writes: [] as string[] };
  await page.addInitScript(() => { localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.onboarding.complete', 'true'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(api\/v2|v1)\//, r => {
    const path = new URL(r.request().url()).pathname;
    if (r.request().method() !== 'GET') state.writes.push(path);
    const data: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: state.workspace, capabilities: ['task-first-agents-v2'] },
      '/v1/users/me': { authenticated: true, auth_required: true, user: { id: state.actor, username: state.actor, role: state.role } },
      '/v1/models': { data: state.models ? [{ id: 'fixture-model', type: 'chat' }] : [] },
      '/v1/sessions': { sessions: [] }, '/v1/catalog': { models: [] }, '/api/v2/jobs': [],
      '/v1/documents': { documents: [], count: 0 },
      '/v1/rag/status': { enabled: state.knowledge, stats: {} },
    };
    return r.fulfill({ json: data[path] ?? {} });
  });
  return state;
}

test('model setup returns to the latest chat draft, refreshes inventory and never submits', async ({ page }) => {
  const state = await fixture(page);
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Private draft — not a URL');
  await page.getByRole('button', { name: 'Choose a chat model', exact: true }).click();
  await expect(page).toHaveURL(/#\/models$/);
  await expect(page.getByRole('button', { name: 'Return to your draft' })).toBeVisible();
  expect(page.url()).not.toContain('Private');
  state.models = true;
  // Another tab edits the same existing draft while setup is open.
  await page.evaluate(() => {
    const key = `offgrid.draft.v2:${encodeURIComponent(JSON.stringify(['alice', 'workspace-a']))}:chat:`;
    localStorage.setItem(key, 'A newer draft');
    window.dispatchEvent(new StorageEvent('storage', { key, newValue: 'A newer draft', storageArea: localStorage }));
  });
  await page.getByRole('button', { name: 'Return to your draft' }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('A newer draft');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  expect(state.writes).toEqual([]);
});

test('Knowledge setup return does not silently turn retrieval on', async ({ page }) => {
  const state = await fixture(page); state.models = true;
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep this question');
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await page.getByRole('link', { name: 'Set up Knowledge' }).click();
  state.knowledge = true;
  await page.getByRole('button', { name: 'Return to your draft' }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this question');
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).toBeEnabled();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).not.toBeChecked();
  expect(state.writes).toEqual([]);
});

test('reload drops the transient return link but not the existing draft', async ({ page }) => {
  await fixture(page); await page.goto('/ui/#/agents/new');
  await page.getByRole('textbox', { name: 'Task', exact: true }).fill('Retain my task');
  await page.locator('.task-composer').getByRole('link', { name: 'Models', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Return to your draft' })).toBeVisible();
  await page.reload();
  await expect(page.getByRole('button', { name: 'Return to your draft' })).toHaveCount(0);
  await page.locator('.primary-nav a[href="#/agents"]').click();
  await expect(page.getByRole('textbox', { name: 'Task', exact: true })).toHaveValue('Retain my task');
});

for (const change of ['workspace', 'actor', 'permission'] as const) test(`${change} change discards a private setup return`, async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Do not move this to another account');
  await page.getByRole('button', { name: 'Choose a chat model', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Return to your draft' })).toBeVisible();
  if (change === 'workspace') state.workspace = 'workspace-b';
  else if (change === 'actor') state.actor = 'bob';
  else state.role = 'guest';
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Return to your draft' })).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

const scopedKey = (actor: string, workspace: string, kind = 'chat', id = '') => `offgrid.draft.v2:${encodeURIComponent(JSON.stringify([actor, workspace]))}:${kind}:${encodeURIComponent(id)}`;
async function seedLegacy(page: Page) {
  await page.addInitScript(() => {
    if (sessionStorage.getItem('legacy-seeded')) return;
    sessionStorage.setItem('legacy-seeded', 'true');
    localStorage.setItem('offgrid.draft.v1:alice:chat:', 'Old private question');
    localStorage.setItem('offgrid.draft.v1:alice:agent-task:', 'Old task draft');
    localStorage.setItem('offgrid.draft.v1:bob:chat:', 'Another account');
    localStorage.setItem('offgrid.active-session:alice', 'Never automatically select');
  });
}

test('legacy restoration is explicit, copy-only, one-time, and never submits or migrates authority', async ({ page }) => {
  const state = await fixture(page); state.models = true;
  await seedLegacy(page); await page.goto('/ui/#/chat');
  const restore = page.getByRole('button', { name: 'Restore previous drafts here' });
  await expect(restore).toBeVisible();
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await expect(page.getByText('Old private question', { exact: true })).toHaveCount(0);
  await restore.click();
  await expect(page.locator('.composer textarea')).toHaveValue('Old private question');
  await page.locator('.primary-nav a[href="#/agents"]').click();
  await expect(page.getByRole('textbox', { name: 'Task', exact: true })).toHaveValue('Old task draft');
  await page.reload();
  await expect(restore).toHaveCount(0);
  await expect(page.getByRole('textbox', { name: 'Task', exact: true })).toHaveValue('Old task draft');
  expect(await page.evaluate(() => [localStorage.getItem('offgrid.draft.v1:alice:chat:'), localStorage.getItem('offgrid.draft.v1:alice:agent-task:'), localStorage.getItem('offgrid.draft.v1:bob:chat:'), localStorage.getItem('offgrid.active-session:alice')])).toEqual(['Old private question', 'Old task draft', 'Another account', 'Never automatically select']);
  expect(state.writes).toEqual([]);
});

for (const value of ['A newer draft', '']) test(`restoration never overwrites a newer ${value ? 'draft' : 'cleared draft'}`, async ({ page }) => {
  await fixture(page); await seedLegacy(page); await page.goto('/ui/#/chat');
  const editor = page.locator('.composer textarea');
  await editor.fill('A newer draft');
  if (!value) await editor.fill('');
  await page.getByRole('button', { name: 'Restore previous drafts here' }).click();
  await expect(editor).toHaveValue(value);
  await page.reload(); await expect(editor).toHaveValue(value);
  expect(await page.evaluate(() => localStorage.getItem('offgrid.draft.v1:alice:chat:'))).toBe('Old private question');
});

test('a concurrent edit wins even between the restore check and copy', async ({ page }) => {
  await fixture(page); await seedLegacy(page); await page.goto('/ui/#/chat');
  await page.evaluate(key => {
    const original = Storage.prototype.setItem;
    Storage.prototype.setItem = function (name, value) {
      if (name === key.replace('offgrid.draft.v2:', 'offgrid.draft.recovered.v2:')) original.call(this, key, 'Concurrent newer draft');
      original.call(this, name, value);
    };
  }, scopedKey('alice', 'workspace-a'));
  await page.getByRole('button', { name: 'Restore previous drafts here' }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Concurrent newer draft');
  await page.reload(); await expect(page.locator('.composer textarea')).toHaveValue('Concurrent newer draft');
});

test('failed restoration acknowledgment retains originals and safely retries', async ({ page }) => {
  const state = await fixture(page); await seedLegacy(page); await page.goto('/ui/#/chat');
  await page.evaluate(() => {
    const original = Storage.prototype.setItem;
    Storage.prototype.setItem = function (key, value) {
      if (key.startsWith('offgrid.draft.restore.v2:')) { Storage.prototype.setItem = original; throw new DOMException('Full', 'QuotaExceededError'); }
      original.call(this, key, value);
    };
  });
  const restore = page.getByRole('button', { name: 'Restore previous drafts here' });
  await restore.click(); await expect(page.getByRole('alert').filter({ hasText: 'restoration could not be saved' })).toBeVisible();
  await expect(restore).toBeVisible(); await restore.click();
  await expect(restore).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('Old private question');
  expect(state.writes).toEqual([]);
});

for (const change of ['actor', 'workspace'] as const) test(`${change} isolates drafts, selection and restore decisions`, async ({ page }) => {
  const state = await fixture(page); state.models = true;
  await seedLegacy(page); await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Keep separate', exact: true }).click();
  await page.locator('.composer textarea').fill('Only in workspace A for Alice');
  state[change] = change === 'actor' ? 'bob' : 'workspace-b';
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await expect(page.getByRole('button', { name: 'Restore previous drafts here' })).toBeVisible();
  state[change] = change === 'actor' ? 'alice' : 'workspace-a';
  await page.locator('.topbar').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Only in workspace A for Alice');
  await expect(page.getByRole('button', { name: 'Restore previous drafts here' })).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('unidentified service keeps session-memory drafts without loading or persisting legacy text', async ({ page }) => {
  const state = await fixture(page); state.workspace = ''; state.models = true;
  await seedLegacy(page); await page.goto('/ui/#/chat');
  await expect(page.getByText(/This service has no workspace identity/)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Restore previous drafts here' })).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await page.locator('.composer textarea').fill('Memory only');
  await page.locator('.primary-nav a[href="#/models"]').click();
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await expect(page.locator('.composer textarea')).toHaveValue('Memory only');
  expect(await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('offgrid.draft.memory:')))).toEqual([]);
  await page.reload(); await expect(page.locator('.composer textarea')).toHaveValue('');
  expect(state.writes).toEqual([]);
});

test('a removed conversation on setup return retains its draft and does not select another conversation', async ({ page }) => {
  const state = await fixture(page);
  let sessions = ['Original chat', 'Unrelated chat'];
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: sessions.map(name => ({ name, messages: [], created_at: new Date().toISOString(), updated_at: new Date().toISOString() })) } }));
  await page.goto('/ui/#/chat');
  await expect(page.locator('.history-row.active')).toContainText('Original chat');
  await page.locator('.composer textarea').fill('Keep bound to original');
  await page.getByRole('button', { name: 'Choose a chat model', exact: true }).click();
  sessions = ['Unrelated chat']; state.models = true;
  await page.getByRole('button', { name: 'Return to your draft' }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep bound to original');
  await expect(page.getByText(/This conversation is no longer available/)).toBeVisible();
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeDisabled();
  await expect(page.locator('.history-row.active')).toHaveCount(0);
  expect(state.writes).toEqual([]);
});
