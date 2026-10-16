import { expect, test, type Page } from '@playwright/test';
import type { AgentProfile, AgentProfileSettings } from '../src/api/client';

const model = 'installed-local-model';
const profile = (id: string, settings: Partial<AgentProfileSettings> = {}, revision = 1): AgentProfile => ({
  id, actor: 'local', workspace: 'profile-test', revision, digest: 'fixture-digest', archived: false, created_at: '2026-10-13T00:00:00Z',
  settings: { name: id, model, instructions: 'Answer accurately.', max_iterations: 10, ...settings },
});

async function fixture(page: Page, initial = [profile('Researcher')]) {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  let profiles = structuredClone(initial), conflict = false, supported = true;
  const mutations: { path: string; body: any }[] = [];
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:api\/v2|v1)\//, async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    let data: unknown = {};
    if (path === '/api/v2/system') data = { product: 'offgrid', api_version: 2, version: 'fixture', workspace_id: 'profile-test', capabilities: supported ? ['task-first-agents-v2', 'durable-agent-events-v2', 'agent-profiles-v1'] : [] };
    else if (path === '/v1/system/config') data = { version: 'fixture', require_auth: false };
    else if (path === '/v1/users/me') data = { user: null, authenticated: false, guest: true };
    else if (path === '/v1/models') data = { data: [{ id: model, type: 'chat', context_window: 8192 }] };
    else if (path === '/v1/documents') data = { documents: [{ id: 'doc-one', name: 'Design notes' }] };
    else if (path === '/v1/rag/status') data = { enabled: true };
    else if (path === '/v1/sessions') data = { sessions: [] };
    else if (path === '/api/v2/jobs' || path === '/v1/agents/tasks') data = [];
    else if (path === '/api/v2/computer/sessions') data = { sessions: [] };
    else if (path === '/api/v2/agent-profiles/resources') data = { computer_permitted: true, connection_scope: 'workspace-admin', tools: {
      read_docs: { name: 'read_docs', namespace: 'mcp', source: 'docs-connection', revision: 'schema-1', location: 'remote', kind: 'read', risk: 'low', description: 'Search approved documentation.' },
    } };
    else if (path.startsWith('/api/v2/agent-profiles')) {
      const [, tail = ''] = path.split('/api/v2/agent-profiles'), [, id, action] = tail.split('/');
      if (request.method() === 'GET') {
        const selected = profiles.find(item => item.id === id);
        if (!id) data = { profiles: profiles.filter(item => !item.archived) };
        else if (!selected) return route.fulfill({ status: 404, json: { error: 'Profile not found' } });
        else if (action === 'effective') data = { profile: selected, ready: selected.settings.model === model, capabilities: {}, issues: selected.settings.model === model ? [] : [{kind:'model',reference:selected.settings.model,code:'model_unavailable',message:'Select an installed model.'}] };
        else data = selected;
      } else {
        const body = request.postDataJSON(); mutations.push({ path, body });
        if (conflict && request.method() === 'PUT') return route.fulfill({ status: 409, json: { error: { message: 'Revision conflict' } } });
        if (action === 'archive') { profiles = profiles.map(item => item.id === id ? { ...item, archived: true } : item); data = { id, archived: true }; }
        else { const saved = profile(id || `created-${mutations.length}`, body.settings, id ? body.expected_revision + 1 : 1); profiles = [saved, ...profiles.filter(item => item.id !== saved.id)]; data = saved; }
      }
    }
    return route.fulfill({ json: data });
  });
  return { mutations, conflict: () => { conflict = true; }, get: () => profiles, withdraw:()=>{supported=false;} };
}

test('a temporary capability withdrawal cannot restore task composition over the profile home',async({page})=>{
  const f=await fixture(page);await page.goto('/ui/#/agents');
  await page.getByRole('button',{name:/Researcher/}).click();
  await page.getByRole('textbox',{name:'Instructions',exact:true}).fill('Keep this draft.');
  f.withdraw();await page.getByRole('button',{name:'Workspace options',exact:true}).click();await page.locator('.workspace-options-panel button[aria-busy]').click();await page.keyboard.press('Escape');
  await expect(page.getByRole('heading',{name:'Agent profiles',exact:true})).toBeVisible();
  await expect(page.getByRole('textbox',{name:'Instructions',exact:true})).toHaveValue('Keep this draft.');
  await expect(page.getByRole('button',{name:'Start task',exact:true})).toHaveCount(0);
  expect(f.mutations).toEqual([]);
});

test('profile creation, revision editing, duplication and archive do not execute work', async ({ page }) => {
  const f = await fixture(page);
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  const editor = page.locator('.profile-editor');
  await editor.getByRole('textbox', { name: 'Name', exact: true }).fill('Writing assistant');
  await editor.getByRole('combobox', { name: 'Model', exact: true }).selectOption(model);
  await editor.getByRole('textbox', { name: 'Instructions', exact: true }).fill('Use short paragraphs. Cite supporting documents.');
  await editor.getByText('Tools · 0', { exact: true }).click();
  await editor.getByRole('checkbox', { name: /read_docs/ }).check();
  await editor.getByText('Knowledge · 0', { exact: true }).click();
  await editor.getByRole('checkbox', { name: 'Design notes', exact: true }).check();
  await editor.getByText('Advanced', { exact: true }).click();
  await editor.getByRole('checkbox', { name: 'Researcher', exact: true }).check();
  await editor.getByRole('button', { name: 'Save agent', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Agent saved' })).toBeVisible();
  expect(f.mutations[0].body.settings).toMatchObject({ name: 'Writing assistant', model, knowledge: ['doc-one'], delegates: ['Researcher'], tools: [{ provider:'docs-connection', tool:'read_docs', schema_revision:'schema-1', location:'remote' }] });
  await editor.getByRole('textbox', { name: 'Description', exact: true }).fill('Draft with evidence');
  await editor.getByRole('button', { name: 'Save agent', exact: true }).click();
  await expect(editor).toContainText('Revision 2');
  expect(f.mutations[1].body.expected_revision).toBe(1);
  await editor.getByRole('button', { name: 'Duplicate', exact: true }).click();
  await expect(editor.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Copy of Writing assistant');
  await editor.getByRole('button', { name: 'Save agent', exact: true }).click();
  await expect(editor).toContainText('Revision 1');
  await editor.getByRole('button', { name: 'Archive', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Existing conversations and recorded revisions are kept');
  await dialog.getByRole('button', { name: 'Archive', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Agent archived' })).toBeVisible();
  expect(f.mutations).toHaveLength(4);
});

test('conflict keeps draft, cannot overwrite, and allows explicit save as a new profile', async ({ page }) => {
  const f = await fixture(page); f.conflict();
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: /Researcher/ }).click();
  const editor = page.locator('.profile-editor');
  await editor.getByRole('textbox', { name: 'Instructions', exact: true }).fill('Keep my uncommitted edit.');
  await editor.getByRole('button', { name: 'Save agent', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Your draft is kept');
  await expect(editor.getByRole('button', { name: 'Save agent', exact: true })).toBeDisabled();
  await expect(editor.getByRole('textbox', { name: 'Instructions', exact: true })).toHaveValue('Keep my uncommitted edit.');
  await editor.getByRole('button', { name: 'Save as new agent', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Agent saved' })).toBeVisible();
  expect(f.get().find(item => item.id === 'Researcher')?.settings.instructions).toBe('Answer accurately.');
  expect(f.mutations).toHaveLength(2);
});

test('unavailable model and changed tools stay explicit without substituting resources', async ({ page }) => {
  const f = await fixture(page, [profile('Old profile', { model: 'removed-model', tools: [{ provider:'docs-connection', tool:'read_docs', schema_revision:'old-schema', location:'remote' }] })]);
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: /Old profile/ }).click();
  const editor = page.locator('.profile-editor');
  await expect(editor).toContainText('nothing will be downloaded');
  await expect(editor.getByRole('button', { name: 'Save agent', exact: true })).toBeDisabled();
  const unavailable = editor.getByRole('checkbox', { name: /read_docs.*Unavailable selection/ });
  await expect(unavailable).toBeChecked();
  await expect(editor.getByRole('checkbox', { name: /read_docs Search approved/ })).not.toBeChecked();
  await unavailable.click();
  await expect(unavailable).toHaveCount(0);
  await editor.getByRole('combobox', { name: 'Model', exact: true }).selectOption(model);
  await editor.getByRole('button', { name: 'Save agent', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Agent saved' })).toBeVisible();
  expect(f.mutations[0].body.settings.tools).toEqual([]);
});

test('profile editor preserves unsaved work across navigation and fits narrow and short screens', async ({ page }, info) => {
  const f = await fixture(page);
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: /Researcher/ }).click();
  await page.getByRole('textbox', { name: 'Instructions', exact: true }).fill('An unsaved instruction');
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel', exact: true }).click();
  await page.evaluate(() => { location.hash = '#/chat'; });
  await page.evaluate(() => { location.hash = '#/agents/profiles'; });
  await expect(page.getByRole('textbox', { name: 'Instructions', exact: true })).toHaveValue('An unsaved instruction');
  for (const theme of ['light', 'dark']) {
    await page.evaluate(theme => { document.documentElement.dataset.theme = theme; }, theme);
    await page.setViewportSize({ width: 1100, height: 700 });
    await page.screenshot({ path: info.outputPath(`profiles-${theme}.png`), fullPage: true });
    await page.setViewportSize({ width: 390, height: 540 });
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1)).toBe(true);
    await page.getByRole('button', { name: 'Save agent', exact: true }).scrollIntoViewIfNeeded();
    await expect(page.getByRole('button', { name: 'Save agent', exact: true })).toBeInViewport();
    await page.screenshot({ path: info.outputPath(`profiles-narrow-${theme}.png`), fullPage: true });
  }
  expect(f.mutations).toEqual([]);
});

test('profile search matches compact history controls and filters without submitting work', async ({ page }, info) => {
  const entries = Array.from({ length: 24 }, (_, i) => profile(`Research ${i + 1}`));
  const f = await fixture(page, [...entries, profile('Document specialist')]);
  await page.setViewportSize({ width: 900, height: 620 });
  await page.goto('/ui/#/agents/profiles');
  const search = page.getByRole('searchbox', { name: 'Search agents' });
  await expect(search).toHaveAttribute('placeholder', 'Search agents');
  await expect(page.locator('.profile-library-list .profile-search svg')).toBeVisible();
  const list = page.locator('.profile-library-list ul');
  await expect.poll(() => list.evaluate(el => el.scrollHeight > el.clientHeight)).toBe(true);
  expect(await list.evaluate(el => getComputedStyle(el).scrollbarWidth)).toBe('none');
  await search.fill('Document');
  await expect(page.locator('.profile-list-item')).toHaveCount(1);
  await expect(page.getByRole('button', { name: /Document specialist/ })).toBeVisible();
  await page.screenshot({ path: info.outputPath('profile-search.png') });
  expect(f.mutations).toEqual([]);
});
