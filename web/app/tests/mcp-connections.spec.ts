import { expect, test, type Page } from '@playwright/test';
import { mcpConnectionText } from '../src/i18n/mcp-connections';
import type { LocaleCode } from '../src/i18n';

const connectionName = 'Docs & research / 日本語';
async function fixture(page: Page, { fail = false, locale = 'en' as LocaleCode } = {}) {
  let removed = false;
  const requests: string[] = [];
  await page.addInitScript(locale => {
    localStorage.setItem('offgrid.locale', locale);
    localStorage.setItem('offgrid.theme', 'dark');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  }, locale);
  await page.route(/\/(?:api\/v2|v1)\//, async route => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (route.request().method() === 'DELETE') {
      expect(path).toBe('/v1/agents/mcp');
      expect(url.searchParams.get('name')).toBe(connectionName);
      requests.push(route.request().url());
      if (fail) return route.fulfill({ status: 500, json: { error: 'Connection has not been removed. Please retry.', code: 'mcp_remove_failed' } });
      removed = true;
      return route.fulfill({ json: { status: 'removed', server: connectionName, tools_removed: 2 } });
    }
    const data: Record<string, unknown> = {
      '/api/v2/system': {version: 'test', api_version: 2, workspace_id: 'mcp-test', capabilities: ['task-first-agents-v2']},
      '/v1/system/config': {require_auth: false}, '/v1/users/me': {user: null, guest: true, authenticated: false},
      '/v1/models': {data: [{id: 'model', type: 'chat'}]}, '/v1/sessions': {sessions: []}, '/v1/catalog': {models: []},
      '/api/v2/jobs': [], '/v1/agents/tasks': [],
      '/v1/agents/tools': {tools: [{name: 'calculator', description: 'Calculate', enabled: true, source: 'builtin'}, ...(!removed ? [{name: 'mcp__docs__search', description: 'Search documentation', enabled: true, source: 'mcp:docs'}] : [])], enabled_count: removed ? 1 : 2},
      '/v1/agents/mcp': {servers: [...(!removed ? [{name: connectionName, transport: 'http', tools: 2, status: 'connected'}] : []), {name: 'Unavailable server', transport: 'http', tools: 0, status: 'disconnected'}]},
      '/v1/integrations': {integrations: []}, '/api/v2/computer/status': {available: false},
      '/v1/stats': {server: {version: 'test', uptime: '1m'}},
    };
    return route.fulfill({json: data[path] ?? {}});
  });
  await page.goto('/ui/#/agents/connections');
  return requests;
}

test('MCP removal confirms the exact connection, refreshes tools, and stays removed on reload', async ({page}) => {
  const requests = await fixture(page);
  const remove = page.getByRole('button', {name: `Remove connection: ${connectionName}`, exact: true});
  await remove.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText(connectionName);
  await expect(dialog).toContainText('Task history is kept');
  await expect(dialog).toContainText('Actions already sent');
  expect(requests).toHaveLength(0);
  await dialog.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(remove).toBeFocused();
  expect(requests).toHaveLength(0);
  await remove.click();
  await dialog.getByRole('button', {name: 'Remove connection', exact: true}).click();
  await expect(dialog).toHaveCount(0);
  await expect(remove).toHaveCount(0);
  await expect(page.getByRole('status').filter({hasText: 'Connection removed.'})).toBeVisible();
  await expect(page.getByRole('button', {name: 'Remove connection: Unavailable server', exact: true})).toBeEnabled();
  expect(requests).toHaveLength(1);
  await page.reload();
  await expect(remove).toHaveCount(0);
  await page.getByRole('link', {name: 'Available tools', exact: true}).click();
  await expect(page.getByRole('checkbox', {name: 'calculator', exact: true})).toBeVisible();
  await expect(page.getByRole('checkbox', {name: 'mcp__docs__search', exact: true})).toHaveCount(0);
});

test('MCP removal failure retains the row and provides a retry without losing the draft', async ({page}) => {
  const requests = await fixture(page, {fail: true});
  await page.getByLabel('Connection name', {exact: true}).fill('My next connection');
  await page.getByRole('button', {name: `Remove connection: ${connectionName}`, exact: true}).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', {name: 'Remove connection', exact: true}).click();
  await expect(dialog.getByRole('alert')).toContainText('Please retry');
  expect(requests).toHaveLength(1);
  await dialog.getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(page.getByRole('button', {name: `Remove connection: ${connectionName}`, exact: true})).toBeEnabled();
  await expect(page.getByLabel('Connection name', {exact: true})).toHaveValue('My next connection');
});

for (const locale of ['en', 'fr', 'es', 'de', 'ar', 'sw', 'sn', 'nd', 'zu'] as const) {
  test(`MCP removal remains accessible on a narrow screen in ${locale}`, async ({page}, info) => {
    await page.setViewportSize({width: 390, height: 844});
    await fixture(page, {locale});
    const copy = mcpConnectionText(locale);
    await page.getByRole('button', {name: `${copy.remove}: ${connectionName}`, exact: true}).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toHaveAccessibleName(copy.title.replace('{name}', connectionName));
    await expect(dialog.getByRole('button', {name: copy.remove, exact: true})).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    if (locale === 'en' || locale === 'ar') await page.screenshot({path: info.outputPath(`mcp-remove-${locale}.png`)});
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole('button', {name: `${copy.remove}: ${connectionName}`, exact: true})).toBeFocused();
  });
}
