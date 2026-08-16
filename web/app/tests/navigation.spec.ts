import { expect, test } from '@playwright/test';

const routes = ['chat', 'knowledge', 'agents', 'models', 'activity', 'settings'] as const;

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
});

test('all application routes render and survive browser history', async ({ page }) => {
  const browserErrors: string[] = [];
  page.on('pageerror', error => browserErrors.push(error.stack ?? error.message));
  page.on('console', message => {
    if (message.type() === 'error') browserErrors.push(message.text());
  });

  await page.goto('/ui/#/chat');
  await expect(page.locator('.app-shell')).toBeVisible();

  for (const route of routes) {
    const link = page.locator(`.primary-nav a[href="#/${route}"]`);
    await link.click();
    await expect(page).toHaveURL(new RegExp(`#/${route}$`));
    await page.waitForTimeout(100);
    expect(browserErrors, browserErrors.join('\n\n')).toEqual([]);
    await expect(link).toHaveAttribute('aria-current', 'page');
    await expect(page.locator('.topbar h1')).not.toHaveText('');
    await expect(page.locator('.page-content')).toBeVisible();
    await expect(page.locator('.page-failure')).toHaveCount(0);
  }

  await page.goBack();
  await expect(page).toHaveURL(/#\/activity$/);
  await expect(page.locator('.primary-nav a[href="#/activity"]')).toHaveAttribute('aria-current', 'page');

  await page.goForward();
  await expect(page).toHaveURL(/#\/settings$/);
  await page.reload();
  await expect(page.locator('.primary-nav a[href="#/settings"]')).toHaveAttribute('aria-current', 'page');
  await expect(page.locator('.page-failure')).toHaveCount(0);

  expect(browserErrors, browserErrors.join('\n\n')).toEqual([]);
});
