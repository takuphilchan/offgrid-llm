import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgePartial } from '../src/i18n/knowledge-partial';
import { workflow } from '../src/i18n/workflow';
import { readFile } from 'node:fs/promises';

for (const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as const) test(`partial PDF review stays explicit and private: ${locale}`, async ({ page }, info) => {
 const state = await craftFixture(page, locale, 'light'), copy = knowledgePartial(locale);
 await page.route('**/api/v2/system', route => route.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
 const digest = 'a'.repeat(64);
 const item = { id: 'partial', name: 'mixed.pdf', sha256: digest, bytes: 100, state: 'needs_input', error_code: 'document_ocr_required', review_available: true, attempt: 1 };
 let revoked = false, mutations = 0;
 await page.route('**/v1/documents/imports*', route => {
  if (route.request().method() !== 'GET') mutations++;
  const query = new URL(route.request().url()).searchParams;
  if (query.get('view') !== 'extraction') return route.fulfill({ json: { imports: [item] } });
  if (revoked) return route.fulfill({ status: 404, json: { error: 'Import not found' } });
  expect(query.get('id')).toBe(item.id);
  return route.fulfill({ json: { version: 1, state: 'partial', source_sha256: digest, warnings: [], source_units: [
   { kind: 'page', index: 1, text: 'Native café 日本語 <script>untrusted</script>', state: 'ready', source_sha256: digest, extractor_revision: 'native' },
   { kind: 'page', index: 2, text: 'Uncertain OCR', state: 'ocr_partial', source_sha256: digest, extractor_revision: 'ocr' },
  ] } });
 });
 await page.setViewportSize({ width: 390, height: 844 });
 await page.goto('/ui/#/knowledge');
 const view = page.locator('.knowledge-import-recovery').getByRole('button', { name: workflow[locale].viewSource });
 await view.click();
 const dialog = page.getByRole('dialog');
 await expect(dialog).toContainText(copy.title);
 await expect(dialog.locator('pre')).toHaveText('Native café 日本語 <script>untrusted</script>');
 await expect(dialog.locator('script')).toHaveCount(0);
 await dialog.getByRole('combobox').selectOption('1');
 await expect(dialog.getByRole('status')).toHaveText(copy.review);
 await expect(dialog.locator('pre')).toHaveText('Uncertain OCR');
 expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`partial-review-${locale}.png`), fullPage: true });
 await page.keyboard.press('Escape'); await expect(view).toBeFocused();
 revoked = true; await view.click();
 await expect(dialog.getByRole('alert')).toHaveText('Import not found');
 await expect(dialog.locator('pre')).toHaveCount(0);
 expect(mutations).toBe(0); expect(state.writes).toEqual([]);
});

test('real mixed PDF retains readable pages through the built web UI', async ({ page, request }) => {
 test.skip(!process.env.OFFGRID_E2E_URL || process.env.OFFGRID_E2E_DOCUMENT_WORKERS !== '1', 'Requires disposable real service with app-local parser workers');
 await craftFixture(page, 'en', 'light');
 await page.route('**/api/v2/system', route => route.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
 await page.route('**/v1/documents/imports*', route => route.continue());
 const accepted = await request.post('/v1/documents/imports', { multipart: { file: { name: 'mixed.pdf', mimeType: 'application/pdf', buffer: await readFile('../../internal/rag/testdata/documents/corpus/mixed.pdf') } } });
 expect(accepted.status()).toBe(202);
 const item = await accepted.json();
 try {
  expect((await request.post(`/v1/documents/imports?id=${item.id}&action=retry`)).status()).toBe(422);
  const status = await request.get(`/v1/documents/imports?id=${item.id}`);
  expect(await status.json()).toMatchObject({ state: 'needs_input', review_available: true, sha256: item.sha256, attempt: 1 });
  await page.goto('/ui/#/knowledge');
  await page.reload();
  await page.locator('.knowledge-import-recovery').getByRole('button', { name: 'View source', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('Partial extraction · not indexed');
  await expect(dialog.locator('pre')).toContainText('Native page');
  await dialog.getByRole('combobox').selectOption('1');
  await expect(dialog.getByRole('status')).toHaveText('Needs review');
  expect((await (await request.get('/v1/documents')).json()).documents).toHaveLength(0);
  expect((await (await request.get(`/v1/documents/imports?id=${item.id}`)).json()).attempt).toBe(1);
 } finally {
  expect((await request.delete(`/v1/documents/imports?id=${item.id}`)).ok()).toBe(true);
 }
 expect((await request.get(`/v1/documents/imports?id=${item.id}&view=extraction`)).status()).toBe(404);
});
