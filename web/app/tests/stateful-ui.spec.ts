import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('offgrid.onboarding.complete', 'true'));
});

test('saved conversations survive reload and can be deleted', async ({ page, request }) => {
  const name = `E2E Chat ${Date.now()}`;
  const created = await request.post('/v1/sessions', { data: { name, model_id: '' } });
  expect(created.status()).toBe(201);

  await page.goto('/ui/#/chat');
  const row = page.locator('.history-row', { hasText: name });
  await expect(row).toBeVisible();
  await row.locator('.history-open').click();
  await page.reload();
  await expect(page.locator('.history-row.active', { hasText: name })).toBeVisible();

  const deleteButton = page.locator('.history-row', { hasText: name }).locator('.history-delete');
  await deleteButton.click();
  await expect(page.getByRole('dialog')).toContainText(name);
  await page.getByRole('dialog').getByRole('button', { name: 'Confirm delete', exact: true }).click();
  await expect(page.locator('.history-row', { hasText: name })).toHaveCount(0);
  expect((await request.get(`/v1/sessions/${encodeURIComponent(name)}`)).status()).toBe(404);
});

test('model catalog renders real actionable entries', async ({ page }) => {
  await page.goto('/ui/#/models');
  await expect(page.locator('.catalog-card').first()).toBeVisible();
  await expect(page.locator('.catalog-card .primary-button').first()).toBeEnabled();
  await expect(page.locator('.catalog-card').first()).toContainText(/Q\d/i);
});

test('authentication-required services present a login flow', async ({ page }) => {
  let authenticated = false;
  await page.route('**/v1/models', route => route.fulfill(authenticated
    ? { status: 200, contentType: 'application/json', body: JSON.stringify({ data: [] }) }
    : { status: 401, contentType: 'text/plain', body: 'Unauthorized' }));
  await page.route('**/v1/auth/login', async route => {
    authenticated = true;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ user: { id: '1', username: 'admin', role: 'admin' }, expires_at: new Date(Date.now() + 60_000).toISOString(), auth_method: 'local' }) });
  });
  await page.route('**/v1/users/me', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ user: { id: '1', username: 'admin', role: 'admin' }, authenticated: true }) }));

  await page.goto('/ui/#/chat');
  await expect(page.getByRole('heading', { name: 'Sign in to your workspace' })).toBeVisible();
  await page.getByLabel('Username').fill('admin');
  await page.getByLabel('Password').fill('correct horse battery staple');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.locator('.app-shell')).toBeVisible();
});
