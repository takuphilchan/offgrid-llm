import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgeRecovery } from '../src/i18n/knowledge-recovery';
import type { LocaleCode } from '../src/i18n';
import { documentRecovery } from '../src/i18n/document-recovery';

const saved = { id: 'import-saved', name: 'Annual report 日本語.pdf', state: 'needs_input', error_code: 'document_ocr_required', bytes: 100, sha256: 'a'.repeat(64), attempt: 1 };
async function setup(page: Page, locale: LocaleCode = 'en') {
  await craftFixture(page, locale, 'light');
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'fixture', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
  const state = { records: [{ ...saved }], actions: [] as string[], failRemove: false };
  await page.route('**/v1/documents/imports*', r => {
    const url = new URL(r.request().url()), method = r.request().method();
    if (method === 'GET') return r.fulfill({ json: { imports: state.records } });
    state.actions.push(`${method} ${url.searchParams.get('id')} ${url.searchParams.get('action')}`);
    if (method === 'DELETE' && state.failRemove) return r.fulfill({ status: 503, json: { error: 'Storage unavailable. Retry when ready.' } });
    state.records = [];
    return r.fulfill({ json: method === 'DELETE' ? { success: true } : { ...saved, state: 'ready', document_id: 'published' } });
  });
  return state;
}

for (const code of ['document_invalid', 'document_encrypted', 'document_empty', 'document_limit', 'document_unsupported_format', 'document_ocr_required', 'document_ocr_language_missing']) test(`saved import explains recovery after restart: ${code}`, async ({ page }) => {
  const state = await setup(page);
  state.records[0].error_code = code;
  await page.goto('/ui/#/knowledge');
  await page.reload();
  await expect(page.locator('.knowledge-import-recovery')).toContainText(documentRecovery('en', code)!);
  expect(state.actions).toEqual([]);
});

test('saved import survives reload and retries its identity without uploading again', async ({ page }) => {
  const state = await setup(page);
  await page.goto('/ui/#/knowledge');
  const recovery = page.locator('.knowledge-import-recovery');
  await expect(recovery).toContainText(saved.name);
  await expect(recovery).toContainText(documentRecovery('en', saved.error_code)!);
  expect(state.actions).toEqual([]);
  await page.reload();
  await expect(recovery).toContainText(saved.name);
  await recovery.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(recovery).toHaveCount(0);
  expect(state.actions).toEqual(['POST import-saved retry']);
});

test('removing a pending import requires clear confirmation and keeps a failed removal visible', async ({ page }) => {
  const state = await setup(page);
  state.failRemove = true;
  await page.goto('/ui/#/knowledge');
  await page.getByRole('button', { name: knowledgeRecovery('en').remove, exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText(knowledgeRecovery('en').removeHelp);
  await dialog.locator('.danger-button').click();
  await expect(dialog.getByRole('alert')).toContainText('Storage unavailable');
  await expect(page.locator('.knowledge-import-recovery')).toContainText(saved.name);
  state.failRemove = false;
  await dialog.locator('.danger-button').click();
  await expect(dialog).toHaveCount(0);
  await expect(page.locator('.knowledge-import-recovery')).toHaveCount(0);
  expect(state.actions).toEqual(['DELETE import-saved null', 'DELETE import-saved null']);
});

test('busy saved imports offer cancellation without a second extraction', async ({ page }) => {
  const state = await setup(page);
  state.records[0].state = 'extracting';
  await page.goto('/ui/#/knowledge');
  const recovery = page.locator('.knowledge-import-recovery');
  await expect(recovery.getByRole('button', { name: 'Try again', exact: true })).toHaveCount(0);
  await recovery.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(recovery).toHaveCount(0);
  expect(state.actions).toEqual(['POST import-saved cancel']);
});

test('cancel during import stops the remaining batch and keeps saved recovery', async ({ page }) => {
  const state = await setup(page);
  state.records = [];
  let accepted = 0, processing = 0, release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/v1/documents/imports*', async r => {
    const url = new URL(r.request().url());
    if (r.request().method() === 'GET') return r.fulfill({ json: { imports: state.records } });
    if (!url.searchParams.has('id')) {
      accepted++; state.records = [{ ...saved, state: 'accepted' }];
      return r.fulfill({ status: 202, json: state.records[0] });
    }
    processing++;
    await gate;
    await r.fulfill({ status: 499, json: { error: 'Cancelled', error_code: 'document_cancelled' } }).catch(() => {});
  });
  await page.goto('/ui/#/knowledge');
  const file = { name: 'first.txt', mimeType: 'text/plain', buffer: Buffer.from('retained source') };
  await page.locator('[data-knowledge-import]').setInputFiles([file, { ...file, name: 'never-sent.txt' }]);
  await expect.poll(() => processing).toBe(1);
  await page.locator('.knowledge-import-actions').getByRole('button', { name: 'Cancel', exact: true }).click();
  release();
  await expect(page.locator('.knowledge-import-recovery')).toContainText(saved.name);
  await expect(page.locator('.knowledge-import-results [data-state="pending"]')).toContainText('never-sent.txt');
  expect(accepted).toBe(1);
  expect(processing).toBe(1);
});

test('leaving after acceptance never starts extraction in the abandoned page', async ({ page }) => {
  await setup(page);
  let accepted = 0, processing = 0, release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/v1/documents/imports*', async r => {
    if (r.request().method() === 'GET') return r.fulfill({ json: { imports: [] } });
    if (new URL(r.request().url()).searchParams.has('id')) { processing++; return r.fulfill({ json: saved }); }
    accepted++; await gate;
    await r.fulfill({ status: 202, json: { ...saved, state: 'accepted' } }).catch(() => {});
  });
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles({ name: 'saved.txt', mimeType: 'text/plain', buffer: Buffer.from('retained source') });
  await expect.poll(() => accepted).toBe(1);
  await page.evaluate(() => { location.hash = '#/chat'; });
  await expect(page.locator('.composer textarea')).toBeVisible();
  release();
  await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 100)));
  expect(processing).toBe(0);
});

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) test(`recovery fits a narrow layout and remains localized: ${locale}`, async ({ page }, info) => {
  const state = await setup(page, locale);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  const recovery = page.locator('.knowledge-import-recovery');
  await expect(recovery).toContainText(knowledgeRecovery(locale).body);
  await expect(recovery).toContainText(documentRecovery(locale, saved.error_code)!);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  expect(state.actions).toEqual([]);
  if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`recovery-${locale}.png`), fullPage: true });
});

test('real service retains rejected Office bytes across reload and removes only the import', async ({ page, request }) => {
  test.skip(!process.env.OFFGRID_E2E_URL, 'Requires disposable real service');
  await setup(page);
  // All import requests reach the disposable service; the unrelated UI data is a fixture.
  await page.route('**/v1/documents/imports*', r => r.continue());
  const accepted = await request.post('/v1/documents/imports', { multipart: { file: { name: 'broken.docx', mimeType: 'application/octet-stream', buffer: Buffer.from('invalid ZIP document') } } });
  expect(accepted.status()).toBe(202);
  const record = await accepted.json();
  const failed = await request.post(`/v1/documents/imports?id=${record.id}&action=retry`);
  expect(failed.status()).toBe(422);
  await page.goto('/ui/#/knowledge');
  const recovery = page.locator('.knowledge-import-recovery');
  await expect(recovery).toContainText('broken.docx');
  await page.reload();
  await expect(recovery).toContainText('broken.docx');
  await recovery.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(recovery.getByRole('alert')).toBeVisible();
  const status = await request.get(`/v1/documents/imports?id=${record.id}`);
  expect(await status.json()).toMatchObject({ id: record.id, state: 'needs_input', attempt: 2, sha256: record.sha256 });
  await recovery.getByRole('button', { name: knowledgeRecovery('en').remove, exact: true }).click();
  await page.getByRole('dialog').locator('.danger-button').click();
  await expect(recovery).toHaveCount(0);
  expect((await request.get(`/v1/documents/imports?id=${record.id}`)).status()).toBe(404);
});
