import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { documentProgress } from '../src/i18n/document-progress';
import { knowledgeScope } from '../src/i18n/knowledge-scope';
import type { LocaleCode } from '../src/i18n';

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as LocaleCode[]) {
 test(`returning import becomes usable without refresh or setup: ${locale}`, async ({ page }) => {
  const state = await craftFixture(page, locale, 'light');
  let ready = false, documentReads = 0;
  const mutations: string[] = [];
  const document = { id: 'returning-document', name: 'Report.pdf', size: 512, content_type: 'application/pdf', index_status: 'ready', source_retained: true, chunk_count: 1 };
  await page.route('**/api/v2/system', route => route.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: ['document-imports-v1', 'scoped-knowledge-chat-v1'] } }));
  await page.route('**/v1/documents', route => {
   documentReads++;
   return route.fulfill({ json: { documents: ready ? [document] : [] } });
  });
  await page.route('**/v1/documents/imports*', route => {
   if (route.request().method() !== 'GET') mutations.push(route.request().method());
   return route.fulfill({ json: { imports: [{ id: 'returning-import', name: document.name, state: ready ? 'ready' : 'indexing', document_id: ready ? document.id : undefined }] } });
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/knowledge');
  await expect(page.locator('.knowledge-import-recovery')).toContainText(documentProgress(locale, 'indexing')!);
  const before = documentReads;
  ready = true;
  await expect(page.locator('.knowledge-documents')).toContainText(document.name, { timeout: 7000 });
  await expect(page.locator('.knowledge-import-recovery')).toHaveCount(0);
  await expect(page.locator('.knowledge-setup')).toHaveCount(0);
  expect(documentReads).toBe(before + 1);
  const copy = knowledgeScope[locale];
  await page.getByRole('checkbox', { name: `${copy.select}: ${document.name}` }).check();
  await page.getByRole('button', { name: `${copy.ask} · 1/16`, exact: true }).click();
  await expect(page.locator('.composer-source-scope')).toContainText(document.name);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  expect(mutations).toEqual([]);
  expect(state.writes).toEqual([]);
 });
}
