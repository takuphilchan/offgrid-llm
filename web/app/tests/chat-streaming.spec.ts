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
          (window as any).emitTimedDelta = (delta: string) => emit({type:'delta',delta});
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
    } else if (path.endsWith('/turn/cancel')) json = { success: true };
    else if (path.endsWith('/turn')) json = { turn: null };
    else if (path.startsWith('/v1/sessions/')) json = sessions[decodeURIComponent(path.split('/')[3])];
    return route.fulfill({ json });
  });
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Stream my answer');
  await page.locator('.composer-actions > button').click();
  await expect(page.locator('.streaming-response')).toContainText('Hello 世界');
}

test('chat renders text before completion, decodes split UTF-8, then shows saved result and metrics', async ({ page }, info) => {
  await streamingWorkspace(page);
  await expect(page.locator('.composer-actions > button')).toHaveText('Stop');
  expect(await page.evaluate(() => (window as any).generationRequest)).toMatchObject({ stream: true, profile: 'interactive', max_tokens: 1024 });
  await page.evaluate(() => (window as any).finishGeneration());
  await expect(page.locator('.message.assistant')).toContainText('Hello 世界 — finished');
  await expect(page.locator('.streaming-response')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  const answer = page.locator('.message.assistant');
  const details = answer.locator('footer').getByRole('button', { name: 'Response details', exact: true });
  const metrics = answer.getByRole('region', { name: 'Response details', exact: true });
  await expect(metrics).toBeHidden();
  await expect(page.locator('.conversation > .response-details')).toHaveCount(0);
  expect(await answer.evaluate(el => el.querySelector('.message-body')!.compareDocumentPosition(el.querySelector('footer')!) & Node.DOCUMENT_POSITION_FOLLOWING)).toBeTruthy();
  await details.click();
  await expect(metrics.getByText('0.25 s', { exact: true })).toBeVisible();
  await expect(metrics.getByText('8,192', { exact: true })).toBeVisible();
  await page.screenshot({ path: info.outputPath('answer-owned-details.png') });
  await page.keyboard.press('Escape');
  await expect(metrics).toBeHidden();
  await expect(details).toBeFocused();
  await details.click();
  await page.locator('.composer textarea').click();
  await expect(metrics).toBeHidden();
  await expect(page.getByText(/Response reached the token limit/)).toBeVisible();
  await page.locator('.composer textarea').fill('Next response');
  await page.locator('.composer-actions > button').click();
  await expect(page.locator('.streaming-response')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Response details', exact: true })).toHaveCount(0);
});

test('unexpected stream closure retains the draft and labels partial text as unsaved', async ({ page }) => {
  await streamingWorkspace(page);
  await page.evaluate(() => (window as any).breakGeneration());
  await expect(page.getByRole('alert')).toContainText('before the conversation was confirmed saved');
  await expect(page.locator('.streaming-response')).toContainText('Partial response');
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
  await expect(page.locator('.composer-actions > button')).toHaveText('Send');
});

test('stop aborts generation and preserves the draft and partial text', async ({ page }) => {
  await streamingWorkspace(page);
  await page.locator('.composer-actions > button').click();
  await expect(page.getByRole('alert')).toContainText('Stopped');
  await expect(page.locator('.streaming-response')).toContainText('Hello 世界');
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
});

test('stream rendering and local stop feedback meet the 250ms UI fixture budget', async ({page}, info) => {
  await streamingWorkspace(page);
  const samples = await page.evaluate(async () => {
    const durations:number[]=[];
    for(let i=0;i<30;i++) {
      const marker=` token-${i} `,start=performance.now();
      await new Promise<void>((resolve,reject)=>{
        const timer=setTimeout(()=>{observer.disconnect();reject(Error('Stream did not render'));},2000);
        const observer=new MutationObserver(()=>{
          if(!document.querySelector('.streaming-response')?.textContent?.includes(marker.trim()))return;
          observer.disconnect();clearTimeout(timer);durations.push(performance.now()-start);resolve();
        });
        observer.observe(document.querySelector('.streaming-response')!,{childList:true,subtree:true,characterData:true});
        (window as any).emitTimedDelta(marker);
      });
    }
    return durations.sort((a,b)=>a-b);
  });
  const stop = await page.locator('.composer-actions > button').evaluate(async button=>{
    const start=performance.now();
    return new Promise<{milliseconds:number;feedback:string}>((resolve,reject)=>{
      const observer=new MutationObserver(()=>{
        const feedback=document.querySelector('.conversation [role="alert"]')?.textContent??'';
        if(!feedback.includes('Stopped.'))return;
        observer.disconnect();clearTimeout(timer);resolve({milliseconds:performance.now()-start,feedback});
      });
      const timer=setTimeout(()=>{observer.disconnect();reject(Error('Stop feedback did not render'));},2000);
      observer.observe(document.querySelector('.conversation')!,{childList:true,subtree:true,characterData:true});
      (button as HTMLButtonElement).click();
    });
  });
  const p95=samples[Math.ceil(samples.length*.95)-1];
  expect(p95).toBeLessThanOrEqual(250);expect(stop.milliseconds).toBeLessThanOrEqual(250);
  // The Send button returns only after saved-state reconciliation. That network
  // completion is distinct from the immediately rendered local Stop feedback.
  await expect(page.locator('.composer-actions > button')).toHaveText('Send');
  console.log(JSON.stringify({fixture:'stream-ui-feedback',p95,stop}));
  await info.attach('stream-ui-performance.json',{body:JSON.stringify({samples,p95,stop,kind:'Synthetic deltas, not inference latency'}),contentType:'application/json'});
});
