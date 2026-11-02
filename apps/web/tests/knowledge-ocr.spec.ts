import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { ocrSetup } from '../src/i18n/ocr-setup';
import { documentRecovery } from '../src/i18n/document-recovery';
import type { LocaleCode } from '../src/i18n';

for (const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as LocaleCode[]) test(`OCR setup is optional and uses explicit local import: ${locale}`, async ({ page }, info) => {
  await craftFixture(page, locale, 'light');
  const formats = [{ id: 'pdf', extensions: ['.pdf'], runtime_state: 'installed_unchecked', ocr_state: 'language_missing', ocr_available: false }];
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: true, stats: {}, document_formats: formats } }));
  let installs = 0, reads = 0;
  await page.route('**/v1/documents/ocr-languages*', route => {
    if (route.request().method() === 'POST') {
      installs++;
      expect(route.request().headers()['content-type']).toBe('application/octet-stream');
      expect(route.request().postDataBuffer()?.equals(Buffer.from('local language data'))).toBe(true);
    } else reads++;
    return route.fulfill({ json: { languages: [{ language: 'eng', name: 'English', sha256: 'fixture', state: installs ? 'installed_verified' : 'missing', qualification: 'unverified' }], document_formats: formats } });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  const panel = page.locator('.knowledge-ocr-setup'), copy = ocrSetup(locale);
  await expect(panel.locator('summary')).not.toBeVisible();
  await page.locator('.knowledge-document-options > summary').click();
  await page.locator('.knowledge-format-details summary').click();
  await expect(page.locator('.knowledge-format-details')).toContainText(documentRecovery(locale, 'document_ocr_language_missing')!);
  await page.locator('.knowledge-format-details summary').click();
  await expect(panel).not.toHaveAttribute('open');
  expect(reads).toBe(0); expect(installs).toBe(0);
  await panel.locator('summary').focus(); await page.keyboard.press('Enter');
  await expect(panel.getByRole('button', { name: copy.importFile })).toBeVisible();
  await panel.locator('input[type=file]').setInputFiles({ name: 'eng.traineddata', mimeType: 'application/octet-stream', buffer: Buffer.from('local language data') });
  await expect(panel).toContainText(copy.installed);
  expect(installs).toBe(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`ocr-setup-${locale}.png`), fullPage: true });
});

test('local OCR setup does not require enabling Knowledge or downloading a search model', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: false, stats: {}, document_formats: [{ id: 'pdf', extensions: ['.pdf'], runtime_state: 'installed_unchecked', ocr_state: 'language_missing' }] } }));
  let installs = 0;
  await page.route('**/v1/documents/ocr-languages*', route => {
    if (route.request().method() === 'POST') installs++;
    return route.fulfill({ json: { languages: [{ language: 'eng', name: 'English', state: installs ? 'installed_verified' : 'missing' }] } });
  });
  const mutations: string[] = [];
  page.on('request', request => { if (request.method() === 'POST') mutations.push(new URL(request.url()).pathname); });
  await page.goto('/ui/#/knowledge');
  const panel = page.locator('.knowledge-ocr-setup');
  await page.locator('.knowledge-document-options > summary').click();
  await panel.locator('summary').click();
  await expect(panel.getByRole('button', { name: ocrSetup('en').importFile })).toBeEnabled();
  await panel.locator('input[type=file]').setInputFiles({ name: 'eng.traineddata', mimeType: 'application/octet-stream', buffer: Buffer.from('fixture pack') });
  await expect(panel).toContainText(ocrSetup('en').installed);
  expect(mutations).toEqual(['/v1/documents/ocr-languages']);
});

test('saved scan failure opens the relevant setup without retrying or changing configuration', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.route('**/api/v2/system', route => route.fulfill({ json: { system_id: 'fixture', workspace_id: 'craft', capabilities: ['document-imports-v1'] } }));
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: true, stats: {}, document_formats: [{ id: 'pdf', extensions: ['.pdf'], runtime_state: 'installed_unchecked', ocr_state: 'language_missing' }] } }));
  let mutations = 0;
  await page.route('**/v1/documents/imports*', route => {
    if (route.request().method() !== 'GET') mutations++;
    return route.fulfill({ json: { imports: [{ id: 'saved-scan', name: 'Scanned receipt.pdf', state: 'needs_input', error_code: 'document_ocr_language_missing' }] } });
  });
  await page.route('**/v1/documents/ocr-languages*', route => {
    if (route.request().method() !== 'GET') mutations++;
    return route.fulfill({ json: { languages: [{ language: 'eng', name: 'English', state: 'missing' }] } });
  });
  await page.goto('/ui/#/knowledge');
  const panel = page.locator('.knowledge-ocr-setup');
  await expect(panel).not.toHaveAttribute('open');
  await page.locator('.knowledge-import-recovery').getByRole('button', { name: ocrSetup('en').title }).click();
  await expect(panel).toHaveAttribute('open');
  await expect(panel.locator('summary')).toBeFocused();
  await expect(panel.getByRole('button', { name: ocrSetup('en').importFile })).toBeVisible();
  expect(mutations).toBe(0);
});

test('lost OCR setup acknowledgment does not retry or claim installation', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: true, stats: {}, document_formats: [{ id: 'pdf', extensions: ['.pdf'], runtime_state: 'installed_unchecked', ocr_state: 'language_missing' }] } }));
  let attempts = 0;
  await page.route('**/v1/documents/ocr-languages*', route => {
    if (route.request().method() === 'POST') { attempts++; return route.abort('failed'); }
    return route.fulfill({ json: { languages: [{ language: 'eng', name: 'English', state: 'missing' }], document_formats: [] } });
  });
  await page.goto('/ui/#/knowledge');
  const panel = page.locator('.knowledge-ocr-setup');
  await page.locator('.knowledge-document-options > summary').click();
  await panel.locator('summary').click();
  await panel.locator('input[type=file]').setInputFiles({ name: 'eng.traineddata', mimeType: 'application/octet-stream', buffer: Buffer.from('data') });
  await expect(panel.getByRole('alert')).toBeVisible();
  await expect(panel).not.toContainText(ocrSetup('en').installed);
  await expect(panel.getByRole('button', { name: ocrSetup('en').check })).toBeEnabled();
  expect(attempts).toBe(1);
});
