import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const theme of ['light', 'dark']) test(`search focus is restrained and remains visible in high contrast: ${theme}`, async ({ page }) => {
  await craftFixture(page, 'en', theme);
  await page.goto('/ui/#/models');
  const search = page.locator('#installed-model-search');
  const field = search.locator('..');
  await search.focus();
  await expect(search).toBeFocused();
  await expect(search).toHaveCSS('outline-style', 'none');
  await expect(field).toHaveCSS('outline-style', 'none');
  await expect(field).toHaveCSS('border-top-width', '1px');
  const focused = await field.evaluate(el => getComputedStyle(el).borderTopColor);
  await search.blur();
  expect(await field.evaluate(el => getComputedStyle(el).borderTopColor)).not.toBe(focused);
  await search.fill('Research');
  await field.getByRole('button').click();
  await expect(search).toHaveValue('');
  await expect(search).toBeFocused();
  await page.screenshot({ path: test.info().outputPath(`search-${theme}.png`) });
  await page.emulateMedia({ forcedColors: 'active' });
  await expect(field).toHaveCSS('outline-width', '2px');
  await expect(field).toHaveCSS('outline-style', 'solid');
  await expect(search).toHaveCSS('outline-style', 'none');
});

for (const theme of ['light', 'dark']) test(`workspace joins the rail with a rounded canvas in ${theme}`, async ({ page }) => {
  await craftFixture(page, 'en', theme);
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto('/ui/#/chat/new');
  await expect(page.locator('.sidebar')).toHaveCSS('border-right-width', '0px');
  await expect(page.locator('.topbar')).toHaveCSS('border-bottom-width', '0px');
  await expect(page.locator('main.workspace')).toHaveCSS('border-top-left-radius', '16px');
  await page.locator('.composer textarea').fill('Preserve this draft');
  await page.locator('.sidebar-collapse').click();
  await expect(page.locator('.composer textarea')).toHaveValue('Preserve this draft');
  await page.screenshot({ path: `../../build/workspace-canvas-${theme}-20261106.png` });
  await page.evaluate(() => document.documentElement.dir = 'rtl');
  await expect(page.locator('main.workspace')).toHaveCSS('border-top-right-radius', '16px');
});

for (const theme of ['light', 'dark']) test(`composer uses one restrained focus boundary in ${theme}`, async ({ page }) => {
  const state = await craftFixture(page, 'en', theme);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/chat/new');
  const editor = page.locator('.composer textarea'), composer = page.locator('.composer');
  await editor.focus();
  await editor.fill('Keep this draft');
  await expect(editor).toBeFocused();
  await expect(editor).toHaveCSS('outline-style', 'none');
  expect(await composer.evaluate(el => getComputedStyle(el).borderTopColor)).toBe(await composer.evaluate(el => {
    const probe = document.createElement('span'); probe.style.color = 'var(--muted)'; el.append(probe);
    const color = getComputedStyle(probe).color; probe.remove(); return color;
  }));
  await page.screenshot({path: `../../build/composer-focus-${theme}-20261106.png`});
  await page.emulateMedia({ forcedColors: 'active' });
  await expect(composer).toHaveCSS('outline-style', 'solid');
  await expect(composer).toHaveCSS('outline-width', '2px');
  await expect(editor).toHaveValue('Keep this draft');
  expect(state.writes).toEqual([]);
});

for (const theme of ['light', 'dark']) test(`settings and compact menus retain usable controls in ${theme}`, async ({ page }) => {
  const state = await craftFixture(page, 'en', theme);
  await page.goto('/ui/#/models');
  await expect(page.locator('.installed-model')).toHaveCount(1);
  await expect(page.locator('.sidebar-foot .service-state')).toHaveCount(0);
  await page.locator('.workspace-options > button').click();
  await expect(page.locator('.workspace-options-panel')).toContainText('Service ready');
  await page.keyboard.press('Escape');
  const trigger = page.locator('.model-maintenance > button');
  await trigger.click();
  const panel = page.locator('.model-maintenance .utility-action-panel');
  await expect(panel).toBeVisible();
  const remove = panel.getByRole('button', { name: 'Delete', exact: true });
  await expect(remove).toHaveCSS('border-top-width', '0px');
  const bounds = await remove.boundingBox();
  expect(bounds!.height).toBeGreaterThanOrEqual(32);
  expect(bounds!.height).toBeLessThanOrEqual(40);
  expect((await panel.boundingBox())!.width).toBeLessThanOrEqual(224);
  await page.screenshot({path:`../../build/model-menu-${theme}-20261106.png`});
  await page.keyboard.press('Escape');
  await expect(trigger).toBeFocused();
  expect(state.writes).toEqual([]);
});

test('late catalog and transfer metadata do not rename or move the installed inventory', async ({ page }) => {
  await craftFixture(page, 'en', 'dark');
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/v1/catalog', async route => {
    await gate;
    await route.fulfill({ json: { models: [{ id: 'Local-Research-7B-Instruct-Q4_K_M-long-model-name', name: 'A different catalog name', file: 'Local-Research-7B-Instruct-Q4_K_M-long-model-name.gguf' }] } });
  });
  await page.route('**/v1/models/download/progress', async route => {
    await gate;
    await route.fulfill({ json: { 'old.gguf': { file_name: 'old.gguf', status: 'failed', percent: 25 } } });
  });
  await page.goto('/ui/#/models');
  const row = page.locator('.installed-model').first();
  await expect(row).toBeVisible();
  const name = await row.locator('h3').innerText();
  const before = await row.boundingBox();
  release();
  await expect(page.locator('.model-transfers')).toBeVisible();
  await expect(row.locator('h3')).toHaveText(name);
  expect(Math.abs((await row.boundingBox())!.y - before!.y)).toBeLessThan(1);
});

test('tool search filters descriptions without changing enabled state', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.route('**/v1/agents/tools?all=true', route => route.fulfill({ json: { enabled_count: 1, tools: [
    { name: 'read_file', description: 'Read approved documents', source: 'builtin', enabled: true },
    { name: 'current_time', description: 'Read the service clock', source: 'builtin', enabled: false },
  ] } }));
  await page.goto('/ui/#/agents/tools');
  const search = page.getByRole('searchbox', { name: 'Available tools', exact: true });
  await expect(page.locator('.tool-list article')).toHaveCount(2);
  await search.fill('documents');
  await expect(page.locator('.tool-list article')).toHaveCount(1);
  await expect(page.getByRole('checkbox', { name: 'read_file', exact: true })).toBeChecked();
  await search.fill('absent');
  await expect(page.locator('.tool-list article')).toHaveCount(0);
  await search.fill('');
  await expect(page.locator('.tool-list article')).toHaveCount(2);
  expect(state.writes).toEqual([]);
});

test('Schedules has one section separator, not a second border below the tabs', async ({ page }) => {
  await craftFixture(page, 'en', 'dark');
  await page.goto('/ui/#/activity');
  await page.getByRole('button', { name: 'Schedules', exact: true }).click();
  await expect(page.locator('.agent-schedules')).toBeVisible();
  await expect(page.locator('.agent-schedules')).toHaveCSS('border-top-width', '0px');
});
