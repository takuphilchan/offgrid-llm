import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgeOptions } from '../src/i18n/knowledge-options';
import { knowledgeScope } from '../src/i18n/knowledge-scope';
import type { LocaleCode } from '../src/i18n';

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
  test(`document work precedes optional configuration: ${locale}`, async ({ page }, info) => {
    const state = await craftFixture(page, locale, 'light');
    let languageReads = 0;
    await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: true, stats: {}, document_formats: [
      { id: 'pdf', extensions: ['.pdf'], runtime_state: 'installed_unchecked', ocr_state: 'language_missing' },
    ] } }));
    await page.route('**/v1/documents/ocr-languages*', route => {
      languageReads++;
      return route.fulfill({ json: { languages: [] } });
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/ui/#/knowledge');
    await expect(page.locator('.knowledge-heading-import button')).toBeEnabled();
    await expect(page.locator('.knowledge-document-options > summary')).toHaveText(knowledgeOptions(locale));
    await expect(page.locator('.knowledge-format-details > summary')).not.toBeVisible();
    await expect(page.locator('.knowledge-ocr-setup > summary')).not.toBeVisible();
    await expect(page.locator('.knowledge-documents')).toBeVisible();
    expect(languageReads).toBe(0);
    expect(state.writes).toEqual([]);
    if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`knowledge-simple-${locale}.png`), fullPage: true });
    await page.locator('.knowledge-document-options > summary').focus();
    await page.keyboard.press('Enter');
    await expect(page.locator('.knowledge-format-details > summary')).toBeVisible();
    await expect(page.locator('.knowledge-ocr-setup > summary')).toBeVisible();
    expect(languageReads).toBe(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  });
}

test('refresh removes unavailable source selections instead of leaving a disabled invisible selection', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  let available = true;
  await page.route('**/v1/documents', route => route.fulfill({ json: { documents: available ? [
    { id: 'source', name: 'Report.pdf', index_status: 'ready', size: 100 },
  ] : [] } }));
  await page.goto('/ui/#/knowledge');
  await page.getByRole('checkbox', { name: `${knowledgeScope.en.select}: Report.pdf` }).check();
  await expect(page.locator('.knowledge-selection')).toBeVisible();
  available = false;
  await page.evaluate(async () => {
    const pending: Promise<unknown>[] = [];
    window.dispatchEvent(new CustomEvent('offgrid:refresh-workspace', { detail: pending }));
    await Promise.all(pending);
  });
  await expect(page.locator('.knowledge-selection')).toHaveCount(0);
  await expect(page.locator('.knowledge-documents')).toHaveCount(0);
});

test('failed refresh does not display previously readable document names', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-documents')).toBeVisible();
  state.mode = 'error';
  await page.evaluate(async () => {
    const pending: Promise<unknown>[] = [];
    window.dispatchEvent(new CustomEvent('offgrid:refresh-workspace', { detail: pending }));
    await Promise.all(pending);
  });
  await expect(page.locator('.knowledge-documents')).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('temporarily unavailable');
  expect(state.writes).toEqual([]);
});
