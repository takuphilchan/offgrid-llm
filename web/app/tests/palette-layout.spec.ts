import { expect, test } from '@playwright/test';

for (const profile of [
  { width: 1440, height: 900, theme: 'light', locale: 'en' },
  { width: 1000, height: 596, theme: 'light', locale: 'en' },
  { width: 390, height: 600, theme: 'dark', locale: 'de' },
  { width: 320, height: 480, theme: 'light', locale: 'ar' }
]) test(`palette fits and every action is reachable at ${profile.width}x${profile.height} ${profile.locale}`, async ({ page }, info) => {
  await page.setViewportSize(profile);
  await page.addInitScript(({ theme, locale }) => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.theme', theme); localStorage.setItem('offgrid.locale', locale);
  }, profile);
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/v1/**', r => {
    const path = new URL(r.request().url()).pathname;
    return r.fulfill({ json: path.endsWith('/users/me') ? { authenticated: false, auth_required: false } : path === '/v1/models' ? { data: [] } : {} });
  });
  await page.goto('/ui/#/chat');
  await expect(page.locator('.app-shell')).toBeVisible();
  await page.keyboard.press('Control+k');
  const dialog = page.getByRole('dialog'), search = dialog.getByRole('combobox');
  await expect(search).toBeFocused();
  await expect(page.locator('.command-search')).toHaveCSS('outline-style', 'none');
  expect(await page.locator('.command-search').evaluate(e => getComputedStyle(e).boxShadow)).not.toBe('none');
  const box = await dialog.boundingBox();
  expect(box!.y).toBeGreaterThanOrEqual(8);
  expect(box!.y + box!.height).toBeLessThanOrEqual(profile.height - 8);
  const start = await page.evaluate(() => window.scrollY);
  await search.press('ArrowUp'); // wrap to the last appearance action
  const last = dialog.getByRole('option').last();
  await expect(last).toHaveAttribute('aria-selected', 'true');
  const item = await last.boundingBox(), pane = await page.locator('.command-results').boundingBox();
  expect(item!.y).toBeGreaterThanOrEqual(pane!.y);
  expect(item!.y + item!.height).toBeLessThanOrEqual(pane!.y + pane!.height);
  expect(await page.evaluate(() => window.scrollY)).toBe(start);
  expect(await dialog.evaluate(e => e.scrollWidth - e.clientWidth)).toBeLessThanOrEqual(1);
  await dialog.screenshot({ path: info.outputPath('palette.png') });
  await search.fill('no-matching-action-12345');
  await expect(dialog.getByRole('option')).toHaveCount(0);
  await search.press('Enter'); await expect(dialog).toBeVisible();
  await dialog.locator('.command-dismiss').click(); await expect(dialog).toHaveCount(0);
});
