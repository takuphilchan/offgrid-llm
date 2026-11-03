import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { workflow } from '../src/i18n/workflow';

for (const retained of [false,true]) for (const kind of ['page', 'sheet'] as const) for (const locale of ['en', 'ar'] as const) test(`saved ${kind} citations open exact source and restore keyboard focus: ${locale}, original ${retained}`, async ({ page }) => {
  const state = await craftFixture(page, locale, 'light');
  const content = 'Exact café 日本語 <script>not executable</script>';
  const source = { kind, index: 42, start_byte: 210000, end_byte: 210058, source_sha256: 'b'.repeat(64), extractor_revision: 'ocr-fixture', ...(kind === 'sheet' ? { name: 'Budget', cells: ['B2', 'C3'] } : {}) };
  const contentHash = 'a'.repeat(32);
  const session = {
    name: 'Demo briefing', messages: [{ role: 'assistant', content: 'Answer [1]', knowledge_sources: [{ document_id: 'doc', chunk_id: 'chunk', name: 'manual.pdf', content, content_hash: contentHash, source }] }],
  };
  await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [session] } }));
  await page.route('**/v1/sessions/Demo%20briefing', route => route.fulfill({ json: session }));
  let response: 'verified' | 'stale' | 'legacy' = 'verified', reads = 0;
  await page.route('**/v1/documents/source?**', route => {
    reads++;
    const query = new URL(route.request().url()).searchParams;
    if(query.get('format')==='preview') {
      expect(query.get('sha256')).toBe(source.source_sha256);
      return route.fulfill({json:{name:'manual.pdf',sha256:source.source_sha256,format:'pdf',pages:1,state:'partial',units:[{kind,index:42,text:kind==='page'?content:'Native sheet text',source_sha256:source.source_sha256,state:'ocr_required'}]}});
    }
    for (const key of ['start_byte', 'end_byte', 'source_sha256', 'extractor_revision'] as const) expect(query.get(key)).toBe(String(source[key]));
    expect(query.get('content_hash')).toBe(contentHash);
    if (response === 'stale') return route.fulfill({ status: 409, json: { error: { message: 'This source no longer matches the saved citation.' } } });
    return route.fulfill({ json: { document: { id: 'doc', name: 'manual.pdf',original_retained:retained }, content: response === 'legacy' ? 'Unrelated document prefix' : content, truncated: false, ...(response === 'verified' ? { citation_verified: true } : {}) } });
  });
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Demo briefing', exact: true }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator('.knowledge-passages > summary').click();
  if (kind === 'sheet') await expect(page.locator('.knowledge-passages .workspace-secondary')).toHaveText('B2, C3');
  const view = page.locator('.knowledge-passages').getByRole('button', { name: workflow[locale].viewSource });
  await view.click();
  const dialog = page.getByRole('dialog');
  const surface=retained?page.locator('.document-reader'):dialog;
  await expect(surface.locator(retained&&kind==='sheet'?'.document-verified-passage blockquote':'pre')).toHaveText(content);
  if(retained&&kind==='page')await expect(surface.locator('mark')).toHaveText(content);
  await expect(surface.locator('script')).toHaveCount(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.keyboard.press('Escape');
  await expect(view).toBeFocused();
  for (const mode of ['stale', 'legacy'] as const) {
    response = mode;
    await view.click();
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(dialog.locator('pre')).toHaveCount(0);
    await expect(page.locator('.knowledge-passages blockquote')).toHaveText(content);
  }
  expect(reads).toBeGreaterThanOrEqual(3); // React StrictMode replays mount-time reads in development.
  expect(state.writes).toEqual([]);
});
