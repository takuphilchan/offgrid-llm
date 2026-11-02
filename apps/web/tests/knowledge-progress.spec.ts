import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { documentProgress, documentBusy } from '../src/i18n/document-progress';
import { knowledgeImport } from '../src/i18n/knowledge-import';
import type { LocaleCode } from '../src/i18n';

test('accepted imports show live phases without a second mutation and stop polling after completion', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
 const record = { id: 'progress-source', name: 'report.txt', state: 'queued', sha256: 'a'.repeat(64) };
 let release!: () => void, reads = 0; const writes: string[] = [];
 const processing = new Promise<void>(resolve => { release = resolve; });
 await page.route('**/v1/documents/imports*', async route => {
  const url = new URL(route.request().url());
  if (route.request().method() === 'GET') {
   if (!url.searchParams.has('id')) return route.fulfill({ json: { imports: [] } });
   reads++; return route.fulfill({ json: record });
  }
  writes.push(url.searchParams.get('action') ?? 'accept');
  if (!url.searchParams.has('id')) return route.fulfill({ status: 202, json: { ...record, state: 'accepted' } });
  await processing; return route.fulfill({ json: { ...record, state: 'ready', document_id: 'published' } });
 });
 await page.goto('/ui/#/knowledge');
 await page.locator('[data-knowledge-import]').setInputFiles({ name: record.name, mimeType: 'text/plain', buffer: Buffer.from('source') });
 const results = page.locator('.knowledge-import-results');
 await expect(results).toContainText(documentProgress('en', 'queued')!);
 record.state = 'extracting'; await expect(results).toContainText(documentProgress('en', 'extracting')!);
 record.state = 'indexing'; await expect(results).toContainText(documentProgress('en', 'indexing')!);
 expect(writes).toEqual(['accept', 'retry']);
 release(); await expect(results.locator('[data-state="added"]')).toBeVisible();
 const finalReads = reads; await page.waitForTimeout(1700);
 expect(reads).toBe(finalReads); expect(writes).toEqual(['accept', 'retry']);
});

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
 test(`saved busy sources offer localized recovery without automatic retry: ${locale}`, async ({ page }) => {
  const state = await craftFixture(page, locale, 'light');
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
  await page.route('**/v1/documents/imports*', r => r.fulfill({ json: { imports: [{ id: 'busy-source', name: 'report.pdf', state: 'failed', error_code: 'document_busy' }] } }));
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-import-recovery')).toContainText(documentBusy(locale));
  expect(state.writes).toEqual([]);
 });
}

test('busy admission stops a batch and preserves pending files without claiming success', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
 let writes = 0;
 await page.route('**/v1/documents/imports*', r => {
  if (r.request().method() === 'GET') return r.fulfill({ json: { imports: [] } });
  writes++;
  return writes === 1 ? r.fulfill({ status: 202, json: { id: 'accepted', state: 'accepted' } }) : r.fulfill({ status: 503, json: { error_code: 'document_busy', error: 'Document queue full' } });
 });
 await page.goto('/ui/#/knowledge');
 await page.locator('[data-knowledge-import]').setInputFiles(['first.txt', 'later.txt'].map(name => ({ name, mimeType: 'text/plain', buffer: Buffer.from('source') })));
 const results = page.locator('.knowledge-import-results');
 await expect(results.locator('[data-state="failed"]')).toContainText(documentBusy('en'));
 await expect(results.locator('[data-state="pending"]')).toContainText(knowledgeImport('en').pending);
 await expect(results.locator('[data-state="added"]')).toHaveCount(0);
 expect(writes).toBe(2);
});

test('cancelling a pending import also aborts its progress observation without replay', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
 // Keep both requests pending in the browser until their supplied signal is
 // cancelled; this checks disposal, not a route fixture returning early.
 await page.addInitScript(() => {
  const original = window.fetch;
  (window as any).progressRequests = [];
  window.fetch = (input, init) => {
   const url = new URL(String(input), location.href);
   if (url.pathname !== '/v1/documents/imports') return original(input, init);
   if (!url.searchParams.has('id')) return Promise.resolve(new Response(JSON.stringify(init?.method === 'POST' ? { id: 'held', state: 'accepted' } : { imports: [] }), { status: init?.method === 'POST' ? 202 : 200, headers: { 'Content-Type': 'application/json' } }));
   const entry = { method: init?.method ?? 'GET', aborted: false };
   (window as any).progressRequests.push(entry);
   return new Promise<Response>((_, reject) => {
    const abort = () => { entry.aborted = true; reject(new DOMException('Cancelled', 'AbortError')); };
    if (init?.signal?.aborted) abort(); else init?.signal?.addEventListener('abort', abort, { once: true });
   });
  };
 });
 await page.goto('/ui/#/knowledge');
 await page.locator('[data-knowledge-import]').setInputFiles({ name: 'pending.txt', mimeType: 'text/plain', buffer: Buffer.from('Retain this source') });
 await expect.poll(() => page.evaluate(() => (window as any).progressRequests.map((r: any) => r.method))).toEqual(['POST', 'GET']);
 await page.locator('.knowledge-import-actions').getByRole('button', { name: 'Cancel', exact: true }).click();
 await expect.poll(() => page.evaluate(() => (window as any).progressRequests.every((r: any) => r.aborted))).toBe(true);
 await page.waitForTimeout(1700);
 expect(await page.evaluate(() => (window as any).progressRequests.length)).toBe(2);
 await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(0);
});
