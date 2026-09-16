import { expect, test, type Page } from '@playwright/test';

async function streamingWorkspace(page: Page) {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.locale', 'en');
    const original = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (!String(input).endsWith('/generate')) return original(input, init);
      const request = JSON.parse(init!.body as string);
      (window as any).generationRequest = request;
      const name = decodeURIComponent(String(input).split('/')[3]);
      const encoder = new TextEncoder();
      const stream = new ReadableStream({
        start(controller) {
          const emit = (data: unknown) => controller.enqueue(encoder.encode(`data: ${JSON.stringify(data)}\r\n\r\n`));
          emit({ type: 'phase', phase: 'processing' });
          // Intentionally split every byte, including UTF-8 multibyte characters.
          const partial = encoder.encode('data: {"type":"delta","delta":"Hello 世界"}\r\n\r\n');
          for (const byte of partial) controller.enqueue(new Uint8Array([byte]));
          (window as any).finishGeneration = () => {
            const message = { role: 'assistant', content: 'Hello 世界 — finished' };
            emit({ type: 'done', session: { name, messages: [{ role: 'user', content: request.content }, message], updated_at: new Date().toISOString() }, message, finish_reason: 'length', metrics: { first_text_ms: 250, context_window: 8192, tokens_per_second: 20 } });
            controller.close();
          };
          (window as any).breakGeneration = () => controller.close();
          init!.signal?.addEventListener('abort', () => controller.error(new DOMException('Stopped', 'AbortError')), { once: true });
        }
      });
      return new Response(stream, { headers: { 'Content-Type': 'text/event-stream' } });
    };
  });
  const sessions: Record<string, any> = {};
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/v1/**', route => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    let json: any = {};
    if (path === '/v1/users/me') json = { authenticated: false, user: null };
    if (path === '/v1/models') json = { data: [{ id: 'stream-model', type: 'chat' }] };
    if (path === '/v1/sessions') {
      if (request.method() === 'GET') json = { sessions: Object.values(sessions) };
      else { const data = request.postDataJSON(); json = sessions[data.name] = { ...data, messages: [], updated_at: new Date().toISOString() }; }
    } else if (path.startsWith('/v1/sessions/')) json = sessions[decodeURIComponent(path.split('/')[3])];
    return route.fulfill({ json });
  });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Stream my answer');
  await page.locator('.composer button').click();
  await expect(page.locator('.streaming-response')).toContainText('Hello 世界');
}

test('chat renders text before completion, decodes split UTF-8, then shows saved result and metrics', async ({ page }) => {
  await streamingWorkspace(page);
  await expect(page.locator('.composer button')).toHaveText('Stop');
  expect(await page.evaluate(() => (window as any).generationRequest)).toMatchObject({ stream: true, profile: 'interactive', max_tokens: 1024 });
  await page.evaluate(() => (window as any).finishGeneration());
  await expect(page.locator('.message.assistant')).toContainText('Hello 世界 — finished');
  await expect(page.locator('.streaming-response')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await expect(page.getByText(/First text: 0.25/)).toBeVisible();
  await expect(page.getByText(/Response reached the token limit/)).toBeVisible();
});

test('unexpected stream closure retains the draft and labels partial text as unsaved', async ({ page }) => {
  await streamingWorkspace(page);
  await page.evaluate(() => (window as any).breakGeneration());
  await expect(page.getByRole('alert')).toContainText('before the conversation was confirmed saved');
  await expect(page.locator('.streaming-response')).toContainText('Partial response');
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
  await expect(page.locator('.composer button')).toHaveText('Send');
});

test('stop aborts generation and preserves the draft and partial text', async ({ page }) => {
  await streamingWorkspace(page);
  await page.locator('.composer button').click();
  await expect(page.getByRole('alert')).toContainText('Stopped');
  await expect(page.locator('.streaming-response')).toContainText('Hello 世界');
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
});
