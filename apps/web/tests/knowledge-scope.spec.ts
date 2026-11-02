import { expect, test, type Page } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { knowledgeScope } from '../src/i18n/knowledge-scope';

async function scopeFixture(page: Page, locale = 'en', capable = true) {
  const state = await craftFixture(page, locale, 'light');
  await page.route('**/api/v2/system', route => route.fulfill({ json: { product: 'offgrid', workspace_id: 'craft', capabilities: capable ? ['scoped-knowledge-chat-v1'] : [] } }));
  await page.route('**/v1/documents', route => route.fulfill({ json: { documents: [
    { id: 'a', name: 'Alpha 日本語.txt', size: 24, index_status: 'ready', chunk_count: 1 },
    { id: 'b', name: 'Beta café.txt', size: 24, index_status: 'ready', chunk_count: 1 },
    { id: 'c', name: 'Unavailable.txt', size: 24, index_status: 'failed', chunk_count: 0 },
  ] } }));
  return state;
}

for (const locale of Object.keys(knowledgeScope) as (keyof typeof knowledgeScope)[]) test(`selected source handoff preserves draft, scope and authority: ${locale}`, async ({ page }) => {
  const state = await scopeFixture(page, locale), copy = knowledgeScope[locale];
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep this question');
  await page.evaluate(() => { location.hash = '#/knowledge'; });
  await page.getByRole('checkbox', { name: `${copy.select}: Alpha 日本語.txt` }).check();
  await expect(page.getByRole('checkbox', { name: `${copy.select}: Unavailable.txt` })).toBeDisabled();
  await page.getByRole('button', { name: new RegExp(copy.ask) }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this question');
  await expect(page.locator('.composer-source-scope')).toContainText('Alpha 日本語.txt');
  await expect(page.locator('.composer-source-scope')).not.toContainText('Beta');
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  expect(state.writes).toEqual([]);
  await page.getByRole('button', { name: copy.clear }).click();
  await expect(page.locator('.composer-source-scope')).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('a collection remains selected through filtering and old services cannot submit it', async ({ page }) => {
  const state = await scopeFixture(page, 'en', false);
  await page.goto('/ui/#/knowledge');
  await page.getByRole('checkbox', { name: 'Select source: Alpha 日本語.txt' }).check();
  await page.getByRole('checkbox', { name: 'Select source: Beta café.txt' }).check();
  await page.getByRole('searchbox').fill('Alpha');
  await expect(page.getByRole('button', { name: /Ask about selected sources/ })).toHaveText('Ask about selected sources · 2/16');
  await expect(page.getByRole('button', { name: /Ask about selected sources/ })).toBeDisabled();
  await expect(page.getByText(knowledgeScope.en.unsupported)).toBeVisible();
  expect(state.writes).toEqual([]);
});

for (const fail of [false, true]) test(`scoped Chat sends exact IDs and ${fail ? 'retains a failed question' : 'renders inert supporting passages'}`, async ({ page }, info) => {
  await scopeFixture(page);
  await page.goto('/ui/#/knowledge');
  await page.getByRole('checkbox', { name: 'Select source: Alpha 日本語.txt' }).check();
  await page.getByRole('button', { name: /Ask about selected sources/ }).click();
  await page.locator('.composer textarea').fill('What is Alpha?');
  await page.evaluate(fail => {
    const prior = window.fetch;
    window.fetch = async (url, init) => {
      if (!String(url).endsWith('/generate')) return prior(url, init);
      const body = JSON.parse(String(init?.body));
      (window as any).scopedPayload = body;
      if (fail) return new Response(JSON.stringify({ error: { message: 'Selected document unavailable. No broader search was performed.' } }), { status: 422 });
      const message = { role: 'assistant', content: 'Answer [1]', knowledge_sources: [{ document_id: 'a', chunk_id: 'chunk-a', name: 'Alpha 日本語.txt', content: '<script>window.sourceExecuted = true</script>\ncafé 日本語', content_hash: 'a'.repeat(64), source: { kind: 'page', index: 3, start_byte: 12, end_byte: 80, source_sha256: 'b'.repeat(64), extractor_revision: 'fixture' } }] };
      const session = { name: decodeURIComponent(String(url).split('/')[3]), model_id: body.model_id, messages: [{ role: 'user', content: body.content }, message] };
      return new Response(`data: ${JSON.stringify({ type: 'done', session, message, finish_reason: 'stop' })}\n\n`, { headers: { 'Content-Type': 'text/event-stream' } });
    };
  }, fail);
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).scopedPayload)).toMatchObject({ use_knowledge_base: true, knowledge_documents: ['a'] });
  if (fail) {
    await expect(page.getByText('Selected document unavailable. No broader search was performed.')).toBeVisible();
    await expect(page.locator('.composer textarea')).toHaveValue('What is Alpha?');
    await expect(page.locator('.knowledge-passages')).toHaveCount(0);
  } else {
    await page.locator('.knowledge-passages > summary').click();
    await expect(page.locator('.knowledge-passages blockquote')).toContainText('<script>window.sourceExecuted = true</script>');
    expect(await page.evaluate(() => (window as any).sourceExecuted)).toBeUndefined();
    await expect(page.locator('.knowledge-passages')).toContainText('Alpha 日本語.txt');
    await expect(page.locator('.knowledge-passages')).toContainText('Page 3');
    await expect(page.locator('.composer textarea')).toHaveValue('');
    await page.screenshot({ path: info.outputPath('knowledge-passages.png'), fullPage: true });
  }
});

test('a selected scope does not become broad retrieval in a different conversation', async ({ page }) => {
  await scopeFixture(page);
  await page.goto('/ui/#/knowledge');
  await page.getByRole('checkbox', { name: 'Select source: Alpha 日本語.txt' }).check();
  await page.getByRole('button', { name: /Ask about selected sources/ }).click();
  await expect(page.locator('.composer-source-scope')).toContainText('Alpha');
  await page.getByRole('button', { name: 'New chat', exact: true }).click();
  await expect(page.locator('.composer-source-scope')).toHaveCount(0);
  await expect(page.locator('.composer-retrieval-scope')).toHaveCount(0);
  await page.getByRole('button', { name: 'Demo briefing', exact: true }).click();
  await expect(page.locator('.composer-source-scope')).toContainText('Alpha');
});
