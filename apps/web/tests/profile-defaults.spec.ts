import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { agentProfilesText } from '../src/i18n/agent-profiles';
import type { LocaleCode } from '../src/i18n';

const model = 'installed-writing-model';
async function setup(page: Page, locale: LocaleCode = 'en') {
  const state = await craftFixture(page, locale, 'light');
  await page.route('**/v1/models', route => route.fulfill({ json: { data: [{ id: model, type: 'chat', runtime_ready: true }] } }));
  await page.route('**/api/v2/agent-profiles', route => route.fulfill({ json: { profiles: [] } }));
  return state;
}

test('new profile decision and layout capture', async ({ page }, info) => {
  const state = await setup(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  const editor = page.locator('.profile-editor');
  await expect(editor.locator('.model-select-trigger')).toBeVisible();
  expect((await editor.locator('.model-select-trigger').boundingBox())!.width).toBeLessThanOrEqual(449);
  const decisions = await editor.locator('input:visible, textarea:visible, button:visible, summary:visible').evaluateAll(nodes => nodes.map(node => ({
    kind: node.tagName, text: node.getAttribute('aria-label') || node.closest('label')?.textContent || node.textContent,
  })));
  await info.attach('visible-decisions', { body: JSON.stringify(decisions, null, 2), contentType: 'application/json' });
  await page.screenshot({ path: info.outputPath('profile-new-wide.png'), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({ path: info.outputPath('profile-new-narrow.png'), fullPage: true });
  expect(state.writes).toEqual([]);
});

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
  test(`create from purpose with no infrastructure decisions: ${locale}`, async ({ page }) => {
    const state = await setup(page, locale), copy = agentProfilesText[locale];
    const saves: any[] = [];
    await page.route('**/api/v2/agent-profiles/created/effective*', route => route.fulfill({ json: { ready: true, issues: [], capabilities: {} } }));
    await page.route('**/api/v2/agent-profiles', route => {
      if (route.request().method() === 'GET') return route.fulfill({ json: { profiles: [] } });
      const body = route.request().postDataJSON(); saves.push(body);
      return route.fulfill({ json: { id: 'created', revision: 1, settings: body.settings } });
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/ui/#/agents/profiles');
    await page.getByRole('button', { name: copy.create, exact: true }).click();
    const editor = page.locator('.profile-editor');
    await expect(editor.locator('input:visible, textarea:visible')).toHaveCount(2);
    await expect(editor.locator('.model-select-trigger')).toContainText('installed writing model');
    await expect(editor.locator('.profile-resource-settings')).not.toHaveAttribute('open');
    await expect(editor.locator('.profile-advanced')).not.toHaveAttribute('open');
    await editor.getByRole('textbox', { name: copy.name, exact: true }).fill('My writing helper');
    await editor.getByRole('textbox', { name: copy.instructions, exact: true }).fill('Help me draft clear letters.');
    expect(saves).toEqual([]);
    await editor.getByRole('button', { name: copy.save, exact: true }).click();
    await expect.poll(() => saves.length).toBe(1);
    expect(saves[0].settings).toEqual({ name: 'My writing helper', instructions: 'Help me draft clear letters.', model,
      max_iterations: 10, tools: [], knowledge: [], delegates: [], computer: false });
    await expect(editor.getByRole('button', { name: copy.use, exact: true })).toBeEnabled();
    expect(state.writes).toEqual([]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });
}

test('new profile suggestion excludes unusable runtimes and never repairs an existing draft silently', async ({ page }) => {
  await setup(page);
  let inventory = [{ id: 'unavailable', type: 'chat', runtime_ready: false }, { id: model, type: 'chat', runtime_ready: true }];
  await page.route('**/v1/models', route => route.fulfill({ json: { data: inventory } }));
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  await expect(page.locator('.profile-editor .model-select-trigger')).toContainText('installed writing model');
  // Merely accepting a suggestion is not an edit requiring a discard dialog.
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('Keep my draft');
  inventory = [{ id: 'replacement', type: 'chat', runtime_ready: true }];
  await page.getByRole('button', { name: 'Workspace options', exact: true }).click();
  await page.locator('.workspace-options-panel button[aria-busy]').click();
  await page.keyboard.press('Escape');
  await expect(page.locator('.profile-editor .model-select-trigger')).toContainText('installed writing model');
  await expect(page.getByRole('textbox', { name: 'Name', exact: true })).toHaveValue('Keep my draft');
  await expect(page.getByRole('button', { name: 'Save agent', exact: true })).toBeDisabled();
});

test('no installed model leaves an explicit choice rather than selecting a remote or speech runtime', async ({ page }) => {
  const state = await setup(page);
  await page.route('**/v1/models', route => route.fulfill({ json: { data: [{ id: 'speech', type: 'speech' }, { id: 'embedding', type: 'embedding' }] } }));
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('Needs a model');
  await expect(page.getByRole('button', { name: 'Save agent', exact: true })).toBeDisabled();
  expect(state.writes).toEqual([]);
});

test('optional resource outages do not block writing-profile setup or request configuration upfront', async ({ page }) => {
  const state = await setup(page);
  let requests = 0;
  await page.route('**/api/v2/agent-profiles/resources', route => { requests++; return route.fulfill({ status: 503, json: { error: 'Connections temporarily unavailable' } }); });
  await page.goto('/ui/#/agents/profiles');
  await page.getByRole('button', { name: 'New agent', exact: true }).click();
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('Writing helper');
  await expect(page.getByRole('button', { name: 'Save agent', exact: true })).toBeEnabled();
  expect(requests).toBe(0);
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.locator('.profile-resource-settings > summary').click();
  await expect(page.locator('.profile-resource-settings').getByRole('alert')).toContainText('Connections temporarily unavailable');
  await expect(page.getByRole('button', { name: 'Save agent', exact: true })).toBeEnabled();
  await page.locator('.profile-resource-settings > summary').click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  expect(requests).toBe(1);
  expect(state.writes).toEqual([]);
});
