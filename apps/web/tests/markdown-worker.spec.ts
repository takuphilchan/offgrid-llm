import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

const longText = 'A paragraph with café 日本語 and **emphasis**. &amp; &copy;\n\n'.repeat(110);
const syntax = '\n[reference][later]\n\n```ts\nconst exact = "café 日本語";\n```\n\n| First | Second |\n| --- | --- |\n| one | two |\n\n<script>window.executed = true</script>\n\n[unsafe](javascript:alert(1)) ![private image](https://never-fetch.invalid/image.png)\n\n[later]: https://example.com/evidence\n';

test('long Markdown keeps complete-document semantics, inert content and exact copy', async ({ page }) => {
  const canonical = longText + syntax;
  const foreign: string[] = [];
  page.on('request', request => { if (request.url().includes('never-fetch.invalid')) foreign.push(request.url()); });
  await craftFixture(page, 'en', 'light');
  await page.addInitScript(() => { Object.defineProperty(navigator, 'clipboard', { value: { writeText: async (value: string) => { (window as any).copiedSource = value; } } }); });
  const session = { name: 'Long answer', model_id: 'model', messages: [{ role: 'assistant', content: canonical }] };
  await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [session] } }));
  await page.route('**/v1/sessions/Long%20answer', route => route.fulfill({ json: session }));
  await page.goto('/ui/#/chat');
  await expect(page.locator('.message-body table')).toBeVisible();
  await expect(page.locator('.message-body')).toContainText('emphasis. & ©');
  await expect(page.getByRole('link', { name: 'reference', exact: true })).toHaveAttribute('href', 'https://example.com/evidence');
  await expect(page.getByRole('link', { name: 'unsafe', exact: true })).toHaveAttribute('href', '');
  expect(await page.evaluate(() => (window as any).executed)).toBeUndefined();
  expect(foreign).toEqual([]);
  await page.getByRole('button', { name: 'Copy code', exact: true }).click();
  expect(await page.evaluate(() => (window as any).copiedSource)).toBe('const exact = "café 日本語";');
  await page.getByRole('button', { name: 'Copy response', exact: true }).click();
  expect(await page.evaluate(() => (window as any).copiedSource)).toBe(canonical);
});

test('streaming worker resolves references supplied later without changing text', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Continue');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.locator('.streaming-response')).toBeVisible();
  await page.evaluate(value => (window as any).emitCraftDelta(value), longText + '\n\n[reference][later]\n');
  await expect(page.locator('.streaming-response')).toContainText('[reference][later]');
  await page.evaluate(() => (window as any).emitCraftDelta('\n[later]: https://example.com/evidence\n'));
  await expect(page.locator('.streaming-response').getByRole('link', { name: 'reference' })).toHaveAttribute('href', 'https://example.com/evidence');
  await page.getByRole('button', { name: 'New chat', exact: true }).click();
  await page.locator('.composer textarea').fill('A new private draft');
  await expect(page.locator('.streaming-response')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('A new private draft');
});

test('worker startup failure preserves readable exact source without breaking Send or Stop', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.addInitScript(() => { (window as any).Worker = class { constructor() { throw Error('Worker unavailable in fixture'); } }; });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Continue');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.locator('.streaming-response')).toBeVisible();
  await page.evaluate(value => (window as any).emitCraftDelta(value), longText + syntax);
  await expect(page.locator('.streaming-response .markdown-pending')).toContainText('const exact = "café 日本語";');
  await expect(page.getByRole('button', { name: 'Stop', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: 'Stop', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeVisible();
});

test('a stalled parser releases obsolete rendered text and preserves the newest source', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.addInitScript(() => {
    (window as any).Worker = class {
      onmessage?: (event: { data: unknown }) => void;
      calls = 0;
      postMessage(request: { id: number; content: string }) {
        if (!request.content || ++this.calls === 1) queueMicrotask(() => this.onmessage?.({ data: { id: request.id, tree: { type: 'root', children: [{ type: 'element', tagName: 'p', properties: {}, children: [{ type: 'text', value: request.content }] }] } } }));
        else (window as any).stalledMarkdown = true;
      }
      terminate() {}
    };
  });
  await page.clock.install();
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Continue');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.locator('.streaming-response p')).toBeVisible();
  await page.evaluate(value => (window as any).emitCraftDelta(value), '\nNewest exact tail — café 日本語.');
  await expect.poll(() => page.evaluate(() => (window as any).stalledMarkdown)).toBe(true);
  await page.clock.fastForward(10_100);
  await expect(page.locator('.streaming-response .markdown-pending')).toContainText('Newest exact tail — café 日本語.');
  await expect(page.getByRole('button', { name: 'Stop', exact: true })).toBeEnabled();
});
