import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const state = { enabled: true, failStatus: false, revokeSource: false, removed: false, writes: [] as string[] };
  const document = { id: 'guide', name: 'Project guide.txt', size: 256, content_type: 'text/plain', chunk_count: 3, source_retained: true, index_status: 'ready' };
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:api\/v2|v1)\//, r => {
    const path = new URL(r.request().url()).pathname;
    if (r.request().method() !== 'GET') state.writes.push(path);
    if (path === '/v1/documents/delete') state.removed = true;
    if (path === '/v1/documents/source' && state.revokeSource) return r.fulfill({ status: 403, json: { error: 'Source access revoked' } });
    if (path === '/v1/rag/status' && state.failStatus) return r.fulfill({ status: 503, json: { error: 'Knowledge status unavailable' } });
    const bodies: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: 'knowledge-fixture', capabilities: [] },
      '/v1/system/config': { require_auth: false }, '/v1/users/me': { authenticated: false, user: null },
      '/v1/models': { data: [{ id: 'chat-fixture', type: 'chat' }, { id: 'embedding-fixture', type: 'embedding' }] },
      '/v1/catalog': { models: [] }, '/v1/models/download/progress': {}, '/v1/sessions': { sessions: [] },
      '/v1/documents': { documents: state.removed ? [] : [document], count: state.removed ? 0 : 1 },
      '/v1/rag/status': { enabled: state.enabled, embedding_model: 'embedding-fixture', stats: {} },
      '/v1/documents/source': { document, content: '<img src=x onerror="alert(1)">\nSource text.', truncated: false },
    };
    return r.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}

test('documents and source inspection precede index administration; revoked sources fail safely', async ({ page }, info) => {
  const state = await fixture(page); await page.goto('/ui/#/knowledge');
  const card = page.locator('.resource-card');
  await expect(card.getByText('Indexed', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add document', exact: true })).toBeEnabled();
  await expect(card.getByRole('button', { name: 'Reindex', exact: true })).toHaveCount(0);
  await card.getByRole('button', { name: 'View source', exact: true }).click();
  await expect(page.getByRole('dialog').locator('pre')).toContainText('<img src=x');
  await expect(page.getByRole('dialog').locator('img')).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(card.getByRole('button', { name: 'View source', exact: true })).toBeFocused();
  state.revokeSource = true; await card.getByRole('button', { name: 'View source', exact: true }).click();
  await expect(page.getByRole('dialog').getByRole('alert')).toContainText('Source access revoked');
  await page.keyboard.press('Escape'); expect(state.writes).toEqual([]);
  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 600 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`knowledge-${width}.png`), fullPage: true });
  }
});

test('explicit Knowledge handoff preserves the current draft, states broad scope and never submits', async ({ page }) => {
  const state = await fixture(page); await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep this draft — 日本語');
  await page.locator('.primary-nav a[href="#/knowledge"]').click();
  await page.locator('.primary-nav a[href="#/chat"]').click();
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).not.toBeChecked();
  await page.keyboard.press('Escape'); await page.locator('.primary-nav a[href="#/knowledge"]').click();
  await page.getByRole('button', { name: 'Ask using Knowledge', exact: true }).click();
  await expect(page).toHaveURL(/#\/chat$/);
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this draft — 日本語');
  await expect(page.locator('.composer-retrieval-scope')).toContainText('not just one selected document');
  await page.getByRole('button', { name: 'Context & response', exact: true }).click();
  await expect(page.getByRole('checkbox', { name: 'Use knowledge base' })).toBeChecked();
  expect(state.writes).toEqual([]);
});

test('status failures do not erase documents or imply enabled retrieval', async ({ page }) => {
  const state = await fixture(page); state.failStatus = true;
  await page.goto('/ui/#/knowledge');
  await expect(page.getByRole('heading', { name: 'Project guide.txt' })).toBeVisible();
  await expect(page.getByRole('alert')).toContainText('Knowledge status unavailable');
  await expect(page.getByRole('button', { name: 'Add document', exact: true })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Ask using Knowledge', exact: true })).toHaveCount(0);
  state.failStatus = false; await page.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Add document', exact: true })).toBeEnabled();
  expect(state.writes).toEqual([]);
});

test('document removal stays explicit and manageable while retrieval is disabled', async ({ page }) => {
  const state = await fixture(page); state.enabled = false;
  await page.goto('/ui/#/knowledge');
  await page.getByRole('button', { name: 'Manage document: Project guide.txt', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Reindex', exact: true })).toBeDisabled();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  expect(state.writes).toEqual([]);
  await page.getByRole('dialog').getByRole('button', { name: 'Confirm delete', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Project guide.txt' })).toHaveCount(0);
  expect(state.writes).toEqual(['/v1/documents/delete']);
});
