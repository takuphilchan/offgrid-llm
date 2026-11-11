import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

test('first-use engine preparation is explicit and does not complete model verification', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [] } }));
 await page.addInitScript(() => { localStorage.removeItem('offgrid.onboarding.complete'); localStorage.removeItem('offgrid.onboarding.stage'); });
 let writes = 0, state = 'missing';
 const status = () => ({ state, mode: 'cpu', gpu_detected: true, download_required: state !== 'ready', download_bytes: 18506923, downloaded_bytes: state === 'downloading' ? 4096 : 0, revision: 'b10516' });
 await page.route('**/api/v2/runtime', async route => {
  if (route.request().method() === 'POST') {
   writes++; state = 'downloading';
   await new Promise(resolve => setTimeout(resolve, 1500));
   state = 'ready';
  }
  await route.fulfill({ json: status() });
 });
 await page.goto('/ui/');
 const setup = page.getByRole('region', { name: 'Inference engine' });
 await expect(setup).toContainText('17.6 MiB');
 await expect(setup).toContainText('CPU');
 await expect(setup).toContainText('GPU detected');
 expect(writes).toBe(0);
 await setup.getByRole('button', { name: 'Prepare engine' }).click();
 await expect(setup.getByRole('progressbar')).toBeVisible();
 await expect(setup).toContainText('Engine ready. Send a prompt');
 expect(writes).toBe(1);
 expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).not.toBe('true');
});

test('saved replies do not verify a newly prepared installation', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.addInitScript(() => { localStorage.removeItem('offgrid.onboarding.complete'); localStorage.removeItem('offgrid.onboarding.stage'); });
 await page.goto('/ui/');
 await expect(page.getByRole('dialog')).toBeVisible();
 await expect(page.locator('.onboarding-connection')).toContainText('Service ready');
 expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).not.toBe('true');
});

test('setup completion is isolated by workspace and account', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 let workspace = 'first', actor = 'owner';
 await page.route('**/api/v2/system', route => route.fulfill({ json: { product: 'offgrid', version: 'fixture', workspace_id: workspace, api_version: 2, capabilities: [] } }));
 await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: true, auth_required: true, user: { id: actor, role: 'admin' } } }));
 await page.goto('/ui/');
 await expect(page.locator('.composer textarea')).toBeVisible();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 workspace = 'second';
 await page.reload();
 await expect(page.getByRole('dialog')).toBeVisible();
 await page.getByRole('dialog').locator('.onboarding-close').click();
 await page.reload();
 await expect(page.locator('.composer textarea')).toBeVisible();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 actor = 'another-owner';
 await page.reload();
 await expect(page.getByRole('dialog')).toBeVisible();
});

test('cancelling engine preparation permits retry and never marks setup complete', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [] } }));
 await page.addInitScript(() => { localStorage.removeItem('offgrid.onboarding.complete'); localStorage.removeItem('offgrid.onboarding.stage'); });
 let state = 'missing', attempts = 0;
 let release: () => void = () => {};
 const pending = new Promise<void>(resolve => { release = resolve; });
 await page.route('**/api/v2/runtime', async route => {
  if (route.request().method() === 'POST') {
   attempts++;
   if (attempts === 1) { state = 'downloading'; await pending; }
   else state = 'ready';
  }
  await route.fulfill({ json: { state, mode: 'cpu', gpu_detected: false, download_required: state !== 'ready', download_bytes: 1000, downloaded_bytes: 0, revision: 'fixture' } });
 });
 await page.goto('/ui/');
 const setup = page.getByRole('region', { name: 'Inference engine' });
 await setup.getByRole('button', { name: 'Prepare engine' }).click();
 await expect.poll(() => attempts).toBe(1);
 state = 'cancelled';
 await setup.getByRole('button', { name: 'Cancel', exact: true }).click();
 release();
 await expect(setup).toContainText('Preparation failed or stopped');
 await setup.getByRole('button', { name: 'Prepare engine' }).click();
 await expect(setup).toContainText('Engine ready. Send a prompt');
 expect(attempts).toBe(2);
 expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).not.toBe('true');
});

test('runtime preparation failure can be retried without marking setup complete', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [] } }));
 await page.addInitScript(() => { localStorage.removeItem('offgrid.onboarding.complete'); localStorage.removeItem('offgrid.onboarding.stage'); });
 let state = 'missing', attempts = 0;
 await page.route('**/api/v2/runtime', route => {
  if (route.request().method() === 'POST') { attempts++; state = 'failed'; }
  return route.fulfill({ status: route.request().method() === 'POST' ? 503 : 200, json: { state, mode: 'cpu', gpu_detected: false, download_required: true, download_bytes: 18506923, downloaded_bytes: 0, revision: 'b10516' } });
 });
 await page.goto('/ui/');
 const setup = page.getByRole('region', { name: 'Inference engine' });
 await setup.getByRole('button', { name: 'Prepare engine' }).click();
 await expect(setup).toContainText('Preparation failed or stopped');
 await setup.getByRole('button', { name: 'Prepare engine' }).click();
 await expect.poll(() => attempts).toBe(2);
 expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).not.toBe('true');
});
