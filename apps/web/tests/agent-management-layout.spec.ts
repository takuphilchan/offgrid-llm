import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
  test(`external agents stay separate and readable at ${width} ${theme}`, async ({ page }, info) => {
    const state = await craftFixture(page, 'en', theme);
    await page.setViewportSize({ width, height: 960 });
    const reads: string[] = [];
    await page.route('**/v1/integrations*', r => {
      reads.push(r.request().url());
      return r.fulfill({ json: { integrations: ['Hermes Agent', 'OpenClaw'].map((name, index) => ({
        id: index ? 'openclaw' : 'hermes', name,
        description: index ? 'Use OffGrid as your OpenClaw inference provider.' : 'Run Hermes with local OffGrid inference.',
        provider_id: 'offgrid', transport: 'openai-chat-completions',
        model_id: 'Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589',
        context_window: 32768, minimum_context: index ? 8192 : 64000,
        ready: Boolean(index), warnings: index ? [] : ['The selected model needs more context for this integration.'],
        readiness_scope: 'service_model', host_state: 'not_inspected', workflow_state: 'unverified',
      })) } });
    });
    await page.goto('/ui/#/agents/tools');
    await expect(page.getByRole('link', { name: 'Tools & MCP', exact: true })).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('form', { name: 'Connect', exact: true })).toHaveCount(0);
    expect(reads).toHaveLength(0);
    await page.getByRole('link', { name: 'External agents', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Hermes Agent', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'OpenClaw', exact: true })).toBeVisible();
    await expect(page.locator('#agent-connections-panel')).toHaveCount(0);
    await expect(page.locator('.provider-card details[open]')).toHaveCount(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath('external-agents.png'), fullPage: true });
    await page.locator('.provider-card summary').first().click();
    await expect(page.locator('.provider-card').first()).toContainText('32,768 / 64,000');
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect(state.writes).toEqual([]);
  });
}
