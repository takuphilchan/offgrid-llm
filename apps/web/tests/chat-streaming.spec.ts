import { expect, test, type Page } from '@playwright/test';
import { chatRecovery, chatBusyRecovery } from '../src/i18n/chat-recovery';

async function streamingWorkspace(page: Page, locale = 'en') {
  await page.addInitScript(locale => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.locale', locale);
    const original = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (!String(input).endsWith('/generate')) return original(input, init);
      const request = JSON.parse(init!.body as string);
      (window as any).generationRequest = request;
      const name = decodeURIComponent(String(input).split('/')[3]);
      (window as any).generationSession = name;
      (window as any).generationCount = ((window as any).generationCount ?? 0) + 1;
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
          (window as any).failGeneration = (error: string, error_code: string) => { emit({type:'error',error,error_code}); controller.close(); };
          init!.signal?.addEventListener('abort', () => controller.error(new DOMException('Stopped', 'AbortError')), { once: true });
        }
      });
      return new Response(stream, { headers: { 'Content-Type': 'text/event-stream' } });
    };
  }, locale);
  const sessions: Record<string, any> = {};
  await page.route('**/api/v2/**', route => {
    const path = new URL(route.request().url()).pathname;
    const json = path === '/api/v2/system'
      ? { product: 'offgrid', api_version: 2, version: 'fixture', workspace_id: 'stream-fixture', capabilities: ['task-first-agents-v2'] }
      : path === '/api/v2/jobs' ? [] : {};
    return route.fulfill({ json });
  });
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

test('typed memory failure offers model recovery, retains the draft and stops claiming running work', async ({ page }, info) => {
  await streamingWorkspace(page);
  await expect(page.getByRole('button',{name:'Known work',exact:true})).toContainText('Running');
  const name = await page.evaluate(() => (window as any).generationSession as string);
  const diagnostic = 'insufficient memory for the requested 512-token allocation (estimated available capacity: 0)';
  const turn = {id:'capacity-turn',status:'failed',phase:'loading',prompt:'Stream my answer',model:'stream-model',profile:'interactive',knowledge:false,max_tokens:1024,output:'Hello 世界',error:diagnostic,error_code:'insufficient_runtime_capacity'};
  const session = {name,model_id:'stream-model',messages:[],turn,updated_at:new Date().toISOString()};
  await page.route('**/v1/sessions/**', route => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/v1/sessions/'+encodeURIComponent(name)) return route.fulfill({json:session});
    if (path.endsWith('/turn')) return route.fulfill({json:{turn}});
    return route.fallback();
  });
  await page.evaluate(({diagnostic}) => (window as any).failGeneration(diagnostic,'insufficient_runtime_capacity'), {diagnostic});
  const notice = page.getByRole('alert');
  await expect(notice).toContainText('Not enough available memory for this model and context');
  await expect(notice.getByRole('button',{name:'Check request status'})).toHaveCount(0);
  await expect(notice.locator('details p')).toBeHidden();
  await page.screenshot({path:info.outputPath('capacity-recovery.png')});
  await notice.getByText('Technical details',{exact:true}).click();
  await expect(notice.locator('details p')).toHaveText(diagnostic);
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
  await expect(page.getByRole('button',{name:'Known work',exact:true})).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).generationCount)).toBe(1);
  await page.reload();
  await expect(page.getByRole('alert')).toContainText('Not enough available memory for this model and context');
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
  await page.getByRole('alert').getByRole('button',{name:'Choose a chat model',exact:true}).click();
  await expect(page).toHaveURL(/#\/models/);
  expect(await page.evaluate(() => (window as any).generationCount ?? 0)).toBe(0);
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
});

test('busy inference preserves the draft without a misleading status action or automatic retry', async ({page}) => {
  await streamingWorkspace(page);
  await page.evaluate(() => (window as any).failGeneration('All request slots occupied','inference_busy'));
  const notice=page.getByRole('alert');
  await expect(notice).toContainText(chatBusyRecovery.en);
  await expect(notice.getByRole('button')).toHaveCount(0);
  await expect(notice.locator('details p')).toBeHidden();
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
  expect(await page.evaluate(()=>(window as any).generationCount)).toBe(1);
});

test('unknown stream failure codes preserve the diagnostic and do not suggest memory recovery', async ({ page }) => {
  await streamingWorkspace(page);
  await page.evaluate(() => (window as any).failGeneration('Provider unavailable','future_provider_failure'));
  await expect(page.getByRole('alert')).toContainText('Provider unavailable');
  await expect(page.getByRole('alert').getByRole('button',{name:'Check request status',exact:true})).toBeVisible();
  await expect(page.getByRole('alert').getByRole('button',{name:'Choose a chat model',exact:true})).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('Stream my answer');
});

test('memory rejection before streaming uses the typed HTTP error without another submission', async ({page}) => {
  await streamingWorkspace(page);
  await page.evaluate(() => (window as any).finishGeneration());
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await page.evaluate(() => {
    const original = window.fetch.bind(window);
    (window as any).capacityAttempts = 0;
    window.fetch = async (input, init) => {
      if (!String(input).endsWith('/generate')) return original(input, init);
      (window as any).capacityAttempts++;
      return Response.json({error:{message:'Allocation rejected',code:'insufficient_runtime_capacity'}},{status:503});
    };
  });
  await page.locator('.composer textarea').fill('Keep my next prompt');
  await page.locator('.composer-actions > button').click();
  await expect(page.getByRole('alert')).toContainText('Not enough available memory');
  await expect(page.getByRole('alert').getByRole('button',{name:'Choose a chat model',exact:true})).toBeVisible();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep my next prompt');
  expect(await page.evaluate(() => (window as any).capacityAttempts)).toBe(1);
});

for (const [locale, message] of Object.entries(chatRecovery)) test(`memory recovery is localized and reachable at narrow width (${locale})`, async ({page}, info) => {
  await page.setViewportSize({width:390,height:844});
  await streamingWorkspace(page, locale);
  await page.evaluate(() => (window as any).failGeneration('Private runtime diagnostic','insufficient_runtime_capacity'));
  const notice = page.getByRole('alert');
  await expect(notice).toContainText(message);
  await expect(notice.locator('details p')).toBeHidden();
  await expect(notice.getByRole('button')).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  if(locale === 'ar') await page.screenshot({path:info.outputPath('capacity-recovery-rtl.png')});
  await notice.getByRole('button').click();
  await expect(page).toHaveURL(/#\/models/);
  expect(await page.evaluate(() => (window as any).generationCount)).toBe(1);
});

for (const backgroundCheck of [false, true]) test(`new chat detaches and returns to saved work without replay (background check: ${backgroundCheck})`, async ({ page }) => {
  if (backgroundCheck) await page.clock.install();
  await streamingWorkspace(page);
  const source = await page.evaluate(() => (window as any).generationSession as string);
  const cancels: string[] = [];
  page.on('request', request => { if (request.url().endsWith('/turn/cancel')) cancels.push(request.url()); });
  await page.getByRole('button', { name: 'New chat', exact: true }).click();
  await expect(page.locator('.streaming-response')).toHaveCount(0);
  await page.locator('.composer textarea').fill('A separate unsent draft');
  const session = { name: source, model_id: 'stream-model', updated_at: new Date().toISOString(), messages: [{ role: 'user', content: 'Stream my answer' }, { role: 'assistant', content: 'Saved in the background.' }] };
  let reads = 0;
  await page.route('**/v1/sessions/*/turn', route => { reads++; return route.fulfill({ json: { turn: { id: 'original-turn', status: 'completed', prompt: 'Stream my answer', output: 'Saved in the background.' } } }); });
  await page.route(`**/v1/sessions/${encodeURIComponent(source)}`, route => route.fulfill({ json: session }));
  if (backgroundCheck) { await page.clock.fastForward(6000); await expect.poll(() => reads).toBeGreaterThan(0); }
  await page.locator('.history-open').first().click();
  await expect(page.locator('.message.assistant')).toContainText('Saved in the background.');
  expect(await page.evaluate(() => (window as any).generationCount)).toBe(1);
  expect(cancels).toEqual([]);
  await page.getByRole('button', { name: 'New chat', exact: true }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('A separate unsent draft');
});

test('late Stop acknowledgment for a previous conversation cannot abort the new conversation', async ({ page }) => {
  await streamingWorkspace(page);
  let release!: () => void;
  const delayed = new Promise<void>(resolve => { release = resolve; });
  let requested = false;
  await page.route('**/turn/cancel', async route => { requested = true; await delayed; await route.fulfill({ json: { success: true } }); });
  await page.locator('.composer-actions > button').click();
  await expect.poll(() => requested).toBe(true);
  await page.getByRole('button', { name: 'New chat', exact: true }).click();
  await page.locator('.composer textarea').fill('Second independent question');
  await page.locator('.composer-actions > button').click();
  await expect(page.locator('.streaming-response')).toContainText('Hello 世界');
  release();
  await page.evaluate(() => (window as any).emitTimedDelta(' Still running in the second conversation.'));
  await expect(page.locator('.streaming-response')).toContainText('Still running in the second conversation.');
  await expect(page.locator('.composer-actions > button')).toHaveText('Stop');
  await page.evaluate(() => (window as any).finishGeneration());
  await expect(page.locator('.message.assistant')).toContainText('Hello 世界 — finished');
  expect(await page.evaluate(() => (window as any).generationCount)).toBe(2);
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
