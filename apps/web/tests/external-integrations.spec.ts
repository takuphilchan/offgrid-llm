import { expect, test } from '@playwright/test';
import { externalIntegrations } from '../src/i18n/external-integrations';

test('registry-defined providers use one setup flow and distinguish service from host readiness', async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    Object.defineProperty(navigator, 'clipboard', { value: { writeText: async (value: string) => { (window as unknown as { copied: string }).copied = value; } } });
  });
  await page.route('**/api/v2/**', route => route.fulfill({ json: new URL(route.request().url()).pathname === '/api/v2/system'
    ? { product: 'offgrid', version: 'test', api_version: 2, workspace_id: 'integration-fixture', capabilities: ['task-first-agents-v2'] } : {} }));
  await page.route('**/v1/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/v1/integrations') return route.fulfill({ json: { integrations: [{ id: 'fixture-third', name: 'Third fixture', provider_id: 'offgrid', transport: 'openai-chat-completions', ready: true, model_id: 'downloaded-model', context_window: 32768, minimum_context: 8192, warnings: [], readiness_scope: 'service_model', host_state: 'not_inspected', workflow_state: 'unverified' }] } });
    if (path === '/v1/integrations/fixture-third/setup') return route.fulfill({ json: { setup: { install_command: 'offgrid integrations configure fixture-third', environment: { TOKEN: 'placeholder' }, config_file: 'fixture.json', content: '{"manual":true}', verify: ['offgrid integrations test fixture-third'] } } });
    if (path === '/v1/models') return route.fulfill({ json: { data: [{ id: 'downloaded-model', type: 'llm' }] } });
    return route.fulfill({ json: { servers: [], tools: [], tasks: [], profiles: [], authenticated: false } });
  });
  await page.goto('/ui/#/agents/connections');
  await expect(page.getByRole('heading', { name: 'Third fixture' })).toBeVisible();
  await expect(page.getByText('Service/model compatible', { exact: true })).toBeVisible();
  await expect(page.getByText('Host setup: not inspected', { exact: false })).toBeVisible();
  await expect(page.getByText('Tool workflow: unverified', { exact: false })).toBeVisible();
  await page.locator('.provider-card button').click();
  await expect(page.locator('.provider-setup')).toBeVisible();
  await expect(page.getByText('{"manual":true}', { exact: true })).not.toBeVisible();
  await page.locator('.provider-setup .section-heading button').click();
  await expect.poll(() => page.evaluate(() => (window as unknown as { copied: string }).copied)).toBe('offgrid integrations configure fixture-third');
  await page.getByText('Manual configuration reference', { exact: true }).click();
  await expect(page.getByText('{"manual":true}', { exact: true })).toBeVisible();
});

test('external provider state labels exist in all nine locales', () => {
  expect(Object.keys(externalIntegrations)).toHaveLength(9);
  for (const messages of Object.values(externalIntegrations)) {
    for (const value of Object.values(messages)) expect(value.length).toBeGreaterThan(0);
  }
});
