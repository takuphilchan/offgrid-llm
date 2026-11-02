import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgeStart } from '../src/i18n/knowledge-start';
import type { LocaleCode } from '../src/i18n';

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
 test(`first-use search offers one local default without configuration: ${locale}`, async ({ page }, info) => {
  const state = await craftFixture(page, locale, 'light');
  await page.route('**/v1/models', route => route.fulfill({ json: { data: [{ id: 'local-search', type: 'embedding' }] } }));
  await page.route('**/v1/rag/status', route => route.fulfill({ json: { enabled: false, stats: {} } }));
  await page.route('**/v1/documents', route => route.fulfill({ json: { documents: [], count: 0 } }));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  const copy = knowledgeStart(locale), panel = page.locator('.knowledge-setup');
  await expect(panel.getByRole('heading', { name: copy.title })).toBeVisible();
  await expect(panel).toContainText(copy.body);
  await expect(panel).toContainText('local search');
  await expect(panel.locator('.primary-button')).toBeEnabled();
  await expect(panel.locator('.knowledge-search-options')).not.toHaveAttribute('open');
  await expect(panel.getByRole('button', { name: copy.model, exact: true })).toHaveCount(0);
  expect(state.writes).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  if (locale === 'en' || locale === 'ar') await page.screenshot({ path: info.outputPath(`knowledge-start-${locale}.png`), fullPage: true });
  await panel.locator('summary').focus(); await page.keyboard.press('Enter');
  await expect(panel.getByRole('button', { name: copy.model, exact: true })).toBeVisible();
  expect(state.writes).toEqual([]);
 });
}
