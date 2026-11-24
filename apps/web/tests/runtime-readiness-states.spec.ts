import { test, expect, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { languageRegistry } from '../src/i18n/language';
import translations from '../src/i18n/feature-packs.json' with { type: 'json' };

const readiness = (state = 'missing') => ({ state, mode: 'cpu', gpu_detected: false, download_required: state !== 'ready', download_bytes: 18506923, downloaded_bytes: 0, revision: 'b10516' });
const setup = (page: Page) => page.locator('.runtime-preparation');
async function firstUse(page: Page, locale = 'en', theme = 'light') {
 await craftFixture(page, locale, theme);
 await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [] } }));
 await page.addInitScript(() => { localStorage.removeItem('offgrid.onboarding.complete'); localStorage.removeItem('offgrid.onboarding.stage'); });
}

test('lost preparation acknowledgement and status failure require a successful read before another attempt', async ({ page }) => {
 await firstUse(page);
 let writes = 0, readsFail = false;
 await page.route('**/api/v2/runtime', route => {
  if (route.request().method() === 'POST') { writes++; readsFail = true; return route.abort('failed'); }
  return route.fulfill(readsFail ? { status: 503, json: { error: 'Status temporarily unavailable' } } : { json: readiness() });
 });
 await page.goto('/ui/');
 await setup(page).getByRole('button', { name: 'Prepare engine', exact: true }).click();
 await expect(setup(page).getByRole('button', { name: 'Inference engine: Try again', exact: true })).toBeVisible();
 await expect(setup(page).getByRole('button', { name: 'Prepare engine', exact: true })).toHaveCount(0);
 await expect(setup(page)).not.toContainText('17.6 MiB');
 await setup(page).getByRole('button', { name: 'Inference engine: Try again', exact: true }).click();
 await expect(setup(page).getByRole('button', { name: 'Inference engine: Try again', exact: true })).toBeEnabled();
 expect(writes).toBe(1);
 readsFail = false;
 await setup(page).getByRole('button', { name: 'Inference engine: Try again', exact: true }).click();
 await expect(setup(page).getByRole('button', { name: 'Prepare engine', exact: true })).toBeVisible();
 expect(writes).toBe(1);
 expect(await page.evaluate(() => localStorage.getItem('offgrid.onboarding.complete'))).not.toBe('true');
});

test('initial status stays checking without preparation or permission advice', async ({ page }) => {
 await firstUse(page);
 await page.route('**/v1/users/me', route => route.fulfill({ json: { authenticated: true, auth_required: true, user: { id: 'reader', role: 'user' } } }));
 let release!: () => void;
 const held = new Promise<void>(resolve => { release = resolve; });
 await page.route('**/api/v2/runtime', async route => { await held; await route.fulfill({ json: readiness() }); });
 await page.goto('/ui/');
 await expect(setup(page).getByRole('status')).toContainText('Checking');
 await expect(setup(page).getByRole('button')).toHaveCount(0);
 await expect(setup(page)).not.toContainText('administrator');
 release();
 await expect(setup(page)).toContainText('Ask a workspace administrator');
 await expect(setup(page).getByRole('button')).toHaveCount(0);
});

for (const invalid of [
 { state: 'future-state' }, { mode: 'future-mode' }, { gpu_detected: null },
 { download_required: null }, { download_bytes: -1 }, { downloaded_bytes: '10' },
 { downloaded_bytes: Number.MAX_SAFE_INTEGER + 1 }, { revision: null },
]) test(`invalid engine status cannot enable preparation or claim readiness: ${JSON.stringify(invalid)}`, async ({ page }) => {
 await firstUse(page);
 let writes = 0;
 await page.route('**/api/v2/runtime', route => { if (route.request().method() === 'POST') writes++; return route.fulfill({ json: { ...readiness('ready'), ...invalid } }); });
 await page.goto('/ui/');
 await expect(setup(page).getByRole('status')).toHaveText('Could not check engine status. Try again to read its current state.');
 await expect(setup(page).getByRole('button', { name: 'Prepare engine', exact: true })).toHaveCount(0);
 await expect(setup(page)).not.toContainText('Engine ready');
 await expect(setup(page)).not.toContainText('MiB');
 expect(writes).toBe(0);
});

test('malformed preparation acknowledgement is reconciled by a read without claiming ready', async ({ page }) => {
 await firstUse(page);
 let writes = 0, reads = 0;
 await page.route('**/api/v2/runtime', route => {
  if (route.request().method() === 'POST') { writes++; return route.fulfill({ json: { state: 'ready', mode: 'cpu' } }); }
  reads++; return route.fulfill({ json: readiness(writes ? 'failed' : 'missing') });
 });
 await page.goto('/ui/');
 await setup(page).getByRole('button', { name: 'Prepare engine', exact: true }).click();
 await expect(setup(page)).toContainText('Preparation failed or stopped');
 await expect(setup(page)).not.toContainText('Engine ready');
 expect(writes).toBe(1); expect(reads).toBeGreaterThanOrEqual(2);
});

test('slow status polling does not overlap and an old read cannot replace a successful acknowledgement', async ({ page }) => {
 await firstUse(page);
 let polls = 0, writes = 0;
 let releasePost!: () => void, releaseRead!: () => void;
 const postHeld = new Promise<void>(resolve => { releasePost = resolve; });
 const readHeld = new Promise<void>(resolve => { releaseRead = resolve; });
 await page.route('**/api/v2/runtime', async route => {
  if (route.request().method() === 'POST') { writes++; await postHeld; return route.fulfill({ json: readiness('ready') }); }
  const polling = writes > 0;
  if (polling) { polls++; await readHeld; }
  return route.fulfill({ json: readiness(polling ? 'downloading' : 'missing') });
 });
 await page.goto('/ui/');
 await setup(page).getByRole('button', { name: 'Prepare engine', exact: true }).dblclick();
 await expect.poll(() => polls).toBe(1);
 // Hold a read across two polling periods to exercise a genuinely slow service.
 await page.waitForTimeout(2200);
 expect(polls).toBe(1); expect(writes).toBe(1);
 releasePost();
 await expect(setup(page)).toContainText('Engine ready');
 releaseRead();
 await expect(setup(page).getByRole('progressbar')).toHaveCount(0);
 await expect(setup(page).getByRole('button', { name: 'Prepare engine', exact: true })).toHaveCount(0);
 await expect(setup(page)).toContainText('Engine ready');
});

for (const locale of Object.keys(languageRegistry)) test(`unsupported engine setup is translated without a dead prepare form: ${locale}`, async ({ page }, testInfo) => {
 await firstUse(page, locale, ['ar', 'de', 'zh-Hans'].includes(locale) ? 'dark' : 'light');
 await page.setViewportSize({ width: ['ar', 'de', 'zh-Hans'].includes(locale) ? 390 : 1280, height: 760 });
 let writes = 0;
 await page.route('**/api/v2/runtime', route => { if (route.request().method() === 'POST') writes++; return route.fulfill({ status: 404, body: 'not found' }); });
 await page.goto('/ui/');
 const copy = (translations as Record<string, Record<string, Record<string, string>>>)[locale]['i18n/runtime-setup:labels'];
 await expect(setup(page).getByRole('status')).toHaveText(copy['8']);
 await expect(setup(page).locator('strong')).toHaveText(copy['0']);
 await expect(setup(page).getByRole('button')).toHaveCount(0);
 expect(writes).toBe(0);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
 if (['en', 'ar', 'de', 'zh-Hans'].includes(locale)) await page.screenshot({ path: testInfo.outputPath(`engine-unsupported-${locale}.png`), fullPage: true });
});

test('failed status read stays retryable and does not suggest an unsupported service or available update', async ({ page }, testInfo) => {
 await firstUse(page);
 let failed = true, writes = 0;
 await page.route('**/api/v2/runtime', route => {
  if (route.request().method() === 'POST') writes++;
  return route.fulfill(failed ? { status: 503, json: { error: 'Unavailable' } } : { json: readiness('ready') });
 });
 await page.goto('/ui/');
 await expect(setup(page).getByRole('status')).toContainText('Could not check engine status');
 await expect(setup(page)).not.toContainText('does not support');
 await expect(setup(page)).not.toContainText('update');
 await page.screenshot({ path: testInfo.outputPath('engine-read-failed-en.png'), fullPage: true });
 failed = false;
 await setup(page).getByRole('button', { name: 'Inference engine: Try again', exact: true }).click();
 await expect(setup(page)).toContainText('Engine ready');
 expect(writes).toBe(0);
});

for (const mode of ['cpu', 'nvidia', 'amd']) test(`checking an installed ${mode} engine does not promise a runtime download`, async ({ page }) => {
 await firstUse(page);
 let writes = 0;
 await page.route('**/api/v2/runtime', route => {
  if (route.request().method() === 'POST') writes++;
  return route.fulfill({ json: { ...readiness('unchecked'), mode, gpu_detected: mode !== 'cpu', download_required: false, download_bytes: 0 } });
 });
 await page.goto('/ui/');
 await expect(setup(page).getByRole('status')).toHaveText('Checks the configured engine without downloading models or GPU drivers.');
 await expect(setup(page)).not.toContainText('may download');
 await expect(setup(page).getByRole('button', { name: 'Prepare engine', exact: true })).toBeVisible();
 expect(writes).toBe(0);
});
