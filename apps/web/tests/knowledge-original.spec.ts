import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgeOriginal } from '../src/i18n/knowledge-original';
import { workflow } from '../src/i18n/workflow';
import { readFile } from 'node:fs/promises';

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as const) {
 test(`source preview distinguishes original and legacy text in ${locale}`, async ({ page }) => {
  await craftFixture(page, locale, 'light');
  let original = false, downloads = 0;
  await page.route('**/v1/documents/source?**', route => {
   if (new URL(route.request().url()).searchParams.get('format') === 'original') {
    downloads++; return route.fulfill({ status: 410, json: { error: { message: 'Original file was removed.' } } });
   }
   return route.fulfill({ json: { document: { id: 'notes', name: 'source.pdf', original_retained: original }, content: 'Exact source café 日本語', truncated: false } });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  await page.getByRole('button', { name: workflow[locale].viewSource, exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText(knowledgeOriginal(locale).missing);
  await expect(dialog.getByRole('button', { name: knowledgeOriginal(locale).download })).toHaveCount(0);
  await page.keyboard.press('Escape');
  original = true;
  await page.getByRole('button', { name: workflow[locale].viewSource, exact: true }).click();
  await dialog.getByRole('button', { name: knowledgeOriginal(locale).download }).click();
  await expect(dialog.getByRole('alert')).toContainText('Original file was removed.');
  await expect(dialog.locator('pre')).toHaveText('Exact source café 日本語');
  expect(downloads).toBe(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('button', { name: workflow[locale].viewSource, exact: true })).toBeFocused();
 });
}

test('original download preserves exact bytes and filename', async ({ page }) => {
 await craftFixture(page, 'en', 'light');
 const bytes = Buffer.from('Original binary bytes: café 日本語\u0000\u0001');
 await page.route('**/v1/documents/source?**', route => new URL(route.request().url()).searchParams.get('format') === 'original'
  ? route.fulfill({ body: bytes, contentType: 'application/octet-stream' })
  : route.fulfill({ json: { document: { id: 'notes', name: 'café.pdf', original_retained: true }, content: 'Extracted text', truncated: false } }));
 await page.goto('/ui/#/knowledge');
 await page.getByRole('button', { name: workflow.en.viewSource, exact: true }).click();
 const downloading = page.waitForEvent('download');
 await page.getByRole('button', { name: knowledgeOriginal('en').download }).click();
 const download = await downloading;
 expect(download.suggestedFilename()).toBe('café.pdf');
 expect(await readFile((await download.path())!)).toEqual(bytes);
 await expect(page.getByRole('dialog').locator('pre')).toHaveText('Extracted text');
});
