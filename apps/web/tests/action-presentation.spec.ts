import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

const routes = ['chat/new', 'models', 'knowledge', 'activity', 'agents/profiles', 'agents/tools', 'agents/connections', 'agents/external', 'settings'];
for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
  test(`route action audit at ${width} in ${theme}`, async ({ page }) => {
    const state = await craftFixture(page, 'en', theme);
    await page.setViewportSize({ width, height: 960 });
    await page.route('**/v1/agents/tools?all=true', r => r.fulfill({ json: { enabled_count: 1, tools: [
      { name: 'read_file', description: 'Read documents in the approved workspace.', source: 'builtin', enabled: true },
      { name: 'research_search', description: 'Find public sources through the connected research service.', source: 'mcp:research', enabled: false },
    ] } }));
    await page.route('**/v1/agents/mcp', r => r.fulfill({ json: { servers: [
      { name: 'Research library', transport: 'http', tools: 3, status: 'connected' },
      { name: 'Archive source', transport: 'http', tools: 0, status: 'disconnected' },
    ] } }));
    for (const route of routes) {
      await page.goto(`/ui/#/${route}`);
      await expect(page.locator('main.workspace')).toBeVisible();
      await expect(page.locator('.deferred-page')).toHaveCount(0);
      for (const button of await page.getByRole('button').all()) {
        if (!await button.isVisible()) continue;
        await expect(button).toHaveAccessibleName(/\S/);
        const box = await button.boundingBox();
        expect(box!.height, `${route}: ${await button.textContent()}`).toBeGreaterThanOrEqual(24);
        expect(box!.width, `${route}: ${await button.textContent()}`).toBeGreaterThanOrEqual(24);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), route).toBe(true);
      if (route === 'agents/connections') {
        const form = page.getByRole('form', { name: 'Connect', exact: true });
        const primary = form.getByRole('button', { name: 'Connect', exact: true });
        const secondary = form.getByRole('button', { name: 'Test connection', exact: true });
        expect((await primary.boundingBox())!.height).toBe((await secondary.boundingBox())!.height);
        await expect(form.getByRole('textbox', { name: 'Connection name', exact: true })).toBeVisible();
        if (width === 1440) expect((await form.boundingBox())!.width).toBeLessThanOrEqual(352);
      }
      if (route === 'models' || route === 'agents/tools') {
        const selected = page.locator(route === 'models' ? '.model-library-tabs [aria-selected=true]' : '.task-management [aria-current=page]');
        await expect(selected).toHaveCSS('border-bottom-width', '2px');
        await expect(selected).toHaveCSS('background-color', 'rgba(0, 0, 0, 0)');
      }
      await page.screenshot({ path: test.info().outputPath(`${route.replaceAll('/', '-')}.png`), fullPage: true });
    }
    expect(state.writes).toEqual([]);
  });
}

test('disclosure chevrons retain keyboard, RTL and reduced-motion behavior', async ({page}) => {
  await craftFixture(page, 'en', 'dark');
  await page.goto('/ui/#/models');
  const summary = page.locator('.installed-model summary').first();
  const details = summary.locator('..');
  await summary.focus();
  await expect(summary).toHaveCSS('list-style-type', 'none');
  await page.keyboard.press('Enter');
  await expect(details).toHaveAttribute('open', '');
  await page.keyboard.press('Space');
  await expect(details).not.toHaveAttribute('open', '');
  await page.evaluate(() => document.documentElement.dir = 'rtl');
  const rtl = await summary.evaluate(el => getComputedStyle(el, '::before').transform);
  await page.keyboard.press('Enter');
  await expect.poll(() => summary.evaluate(el => getComputedStyle(el, '::before').transform)).not.toBe(rtl);
  await page.emulateMedia({reducedMotion:'reduce', forcedColors:'active'});
  await expect(summary).toHaveCSS('outline-width', '2px');
  // The global accessibility reset uses .01ms, including pseudo-elements.
  expect(await summary.evaluate(el => parseFloat(getComputedStyle(el, '::before').transitionDuration))).toBeLessThanOrEqual(.00001);
});
