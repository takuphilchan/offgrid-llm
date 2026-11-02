import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgeImport } from '../src/i18n/knowledge-import';
import { documentRecovery } from '../src/i18n/document-recovery';
import { knowledgeUnitLabels } from '../src/i18n/knowledge-locators';

const formats = [
  { id: 'text', extensions: ['.txt'], runtime_state: 'available', max_file_bytes: 50 * 1024 * 1024 },
  { id: 'pdf', extensions: ['.pdf'], runtime_state: 'unavailable', ocr_available: false },
];
const file = (name: string) => ({ name, mimeType: name.endsWith('.pdf') ? 'application/pdf' : 'text/plain', buffer: Buffer.from('café 日本語') });

async function setup(page: Page, locale = 'en') {
  const state = await craftFixture(page, locale, 'light');
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: true, embedding_model: 'local-search', stats: {}, document_formats: formats } }));
  return state;
}

test('batch import retains individual outcomes and rejects unavailable PDFs before transfer', async ({ page }, info) => {
  await setup(page);
  const uploaded: string[] = [];
  await page.route('**/v1/documents/ingest', route => {
    const body = route.request().postData() ?? '';
    uploaded.push(body);
    return route.fulfill({ json: { success: true, document: { id: `d${uploaded.length}`, index_status: 'ready' } } });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles([file('Alpha 日本語.txt'), file('Report.pdf'), file('Bravo.txt')]);
  await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(2);
  await expect(page.locator('.knowledge-import-results [data-state="failed"]')).toContainText('Report.pdf');
  await expect(page.locator('.knowledge-import-results')).toContainText(knowledgeImport('en').missing);
  expect(uploaded).toHaveLength(2);
  expect(uploaded.join('')).not.toContain('Report.pdf');
  await page.locator('.knowledge-document-options > summary').click();
  await page.locator('.knowledge-format-details summary').click();
  await expect(page.locator('.knowledge-format-details')).toContainText(knowledgeImport('en').ocr);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath('knowledge-import-narrow.png'), fullPage: true });
});

test('lost acknowledgment stops the batch without replay or false success', async ({ page }) => {
  await setup(page);
  let attempts = 0;
  await page.route('**/v1/documents/ingest', route => { attempts++; return route.abort('failed'); });
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles([file('uncertain.txt'), file('not-sent.txt')]);
  await expect(page.locator('.knowledge-import-results [data-state="unknown"]')).toContainText(knowledgeImport('en').unknown);
  await expect(page.locator('.knowledge-import-results [data-state="pending"]')).toContainText('not-sent.txt');
  expect(attempts).toBe(1);
  await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(0);
  await expect(page.locator('.knowledge-import-results button')).toHaveCount(1); // clear, never automatic retry
});

test('a parser rejection does not discard other files and no incomplete receipt counts as added', async ({ page }) => {
  await setup(page);
  let attempts = 0;
  await page.route('**/v1/documents/ingest', route => {
    attempts++;
    return attempts === 1 ? route.fulfill({ status: 422, json: { error_code: 'document_invalid', error: 'Invalid document.' } })
      : route.fulfill({ json: { success: true, document: { id: 'unready', index_status: 'indexing' } } });
  });
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles([file('invalid.txt'), file('incomplete.txt'), file('later.txt')]);
  await expect(page.locator('.knowledge-import-results [data-state="failed"]')).toContainText(documentRecovery('en', 'document_invalid')!);
  await expect(page.locator('.knowledge-import-results [data-state="unknown"]')).toContainText('incomplete.txt');
  await expect(page.locator('.knowledge-import-results [data-state="pending"]')).toContainText('later.txt');
  await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(0);
  expect(attempts).toBe(2);
});

test('file and batch limits are checked before upload', async ({ page }) => {
  const state = await setup(page);
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: true, stats: {}, document_formats: [{ ...formats[0], max_file_bytes: 3 }] } }));
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles([file('large.txt')]);
  await expect(page.locator('.knowledge-import-results')).toContainText(knowledgeImport('en').sizeLimit);
  await page.locator('[data-knowledge-import]').setInputFiles(Array.from({ length: 21 }, (_, index) => file(`${index}.txt`)));
  await expect(page.locator('.knowledge-import [role="alert"]')).toContainText(knowledgeImport('en').batchLimit);
  expect(state.writes).toEqual([]);
});

for (const code of ['document_storage_limit', 'document_unsupported_format']) test(`known rejection stays actionable without replay: ${code}`, async ({ page }) => {
  await setup(page);
  let attempts = 0;
  await page.route('**/v1/documents/ingest', route => {
    attempts++;
    return route.fulfill({ status: 422, json: { error_code: code, error: 'Import rejected before publication.' } });
  });
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles(file('rejected.txt'));
  await expect(page.locator('.knowledge-import-results [data-state="failed"]')).toContainText(documentRecovery('en', code)!);
  await expect(page.locator('.knowledge-import-results [data-state="unknown"]')).toHaveCount(0);
  await expect(page.locator('.knowledge-import-results [data-state="added"]')).toHaveCount(0);
  expect(attempts).toBe(1);
});

test('a failed library refresh preserves the import acknowledgment', async ({ page }) => {
  await setup(page);
  let imported = false;
  await page.route('**/v1/rag/status', route => route.fulfill(imported ? { status: 503, json: { error: 'Status unavailable' } } : { json: { enabled: true, stats: {}, document_formats: formats } }));
  await page.route('**/v1/documents/ingest', route => { imported = true; return route.fulfill({ json: { success: true, document: { id: 'saved', index_status: 'ready' } } }); });
  await page.goto('/ui/#/knowledge');
  await page.locator('[data-knowledge-import]').setInputFiles(file('saved.txt'));
  await expect(page.locator('.knowledge-import-results [data-state="added"]')).toContainText('saved.txt');
  await expect(page.getByRole('alert')).toContainText('Status unavailable');
  await expect(page.locator('.knowledge-import .primary-button')).toBeDisabled();
});

test('navigating away does not dispatch the rest of a batch', async ({ page }) => {
  await setup(page);
  let release: (() => void) | undefined;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let attempts = 0;
  await page.route('**/v1/documents/ingest', async route => { attempts++; await gate; await route.fulfill({ json: { success: true, document: { id: 'one', index_status: 'ready' } } }); });
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-import-actions .primary-button')).toBeEnabled();
  await page.locator('[data-knowledge-import]').setInputFiles([file('one.txt'), file('two.txt')]);
  await expect.poll(() => attempts).toBe(1);
  await page.evaluate(() => { location.hash = '#/chat'; });
  await expect(page.locator('.composer textarea')).toBeVisible();
  release!();
  await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 100)));
  expect(attempts).toBe(1);
});

test('search setup offers the installed default without opening model configuration or downloading', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.route('**/v1/models', route => route.fulfill({ json: { data: [{ id: 'existing-search', type: 'embedding' }] } }));
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: false, stats: {}, document_formats: formats } }));
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-setup')).toContainText('existing search');
  await expect(page.locator('.knowledge-search-options')).not.toHaveAttribute('open');
  await expect(page.locator('.knowledge-setup .primary-button')).toBeEnabled();
  expect(state.writes).toEqual([]);
  await page.getByText(knowledgeImport('en').changeModel, { exact: true }).click();
  await expect(page.locator('.knowledge-search-options')).toHaveAttribute('open');
});

for (const locale of Object.keys(knowledgeUnitLabels) as (keyof typeof knowledgeUnitLabels)[]) test(`format help is optional and localized: ${locale}`, async ({ page }) => {
  await setup(page, locale);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  const copy = knowledgeImport(locale);
  await expect(page.locator('.knowledge-format-details')).not.toHaveAttribute('open');
  await page.locator('.knowledge-document-options > summary').click();
  await page.getByText(copy.formats, { exact: true }).click();
  await expect(page.locator('.knowledge-format-details')).toContainText(copy.unavailable);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});
