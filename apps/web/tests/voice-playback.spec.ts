import { voiceText } from '../src/i18n/voice';
import { chooseModel } from './helpers/model-choice';
import { expect, test, type Page } from '@playwright/test';
import { installChatJob, openAgentChat } from './fixtures/chat-job';
import { SpeechPauseGate } from '../src/lib/speech-pause';

test('pause detector ignores silence and a short spike, then ends a spoken utterance', () => {
  const gate = new SpeechPauseGate();
  const ended = [
    gate.sample(0.004, 10), gate.sample(0.004, 1500),
    gate.sample(0.07, 1600), gate.sample(0.004, 1700),
    gate.sample(0.07, 2000), gate.sample(0.07, 2250),
    gate.sample(0.003, 2800), gate.sample(0.003, 3400)
  ];
  expect(ended).toEqual([false, false, false, false, false, false, false, true]);
});

async function fixture(page: Page, content = 'Hello. This is a saved response.') {
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    (window as any).speechRequests = [];
    (window as any).speechAborts = 0;
    const original = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (String(input).endsWith('/generate')) {
        const encoder = new TextEncoder();
        return new Response(new ReadableStream({ start(controller) {
          let output = '';
          const emit = (event: unknown) => controller.enqueue(encoder.encode(`data: ${JSON.stringify(event)}\n\n`));
          (window as any).emitText = (delta: string) => { output += delta; emit({ type: 'delta', delta }); };
          (window as any).finishText = (fail = false) => {
            if (!fail) {
              const message = { role: 'assistant', content: output };
              emit({ type: 'done', message, session: { name: (window as any).voiceSessionName || 'Speech test', messages: [message] }, metrics: { first_text_ms: 100, context_window: 8192 } });
            }
            controller.close();
          };
          init?.signal?.addEventListener('abort', () => controller.error(new DOMException('Stopped', 'AbortError')), { once: true });
        } }), { headers: { 'Content-Type': 'text/event-stream' } });
      }
      if (!String(input).endsWith('/v1/audio/speech')) return original(input, init);
      (window as any).speechRequests.push(JSON.parse(init!.body as string));
      return new Promise<Response>((resolve, reject) => {
        (window as any).finishSpeech = (failure: boolean) => resolve(failure
          ? new Response(JSON.stringify({ error: { message: 'Read-aloud requires CustomVoice; Base is unsupported.' } }), { status: 503, headers: { 'Content-Type': 'application/json' } })
          : new Response(wav(), { headers: { 'Content-Type': 'audio/wav' } }));
        init?.signal?.addEventListener('abort', () => {
          (window as any).speechAborts++;
          reject(new DOMException('Stopped', 'AbortError'));
        }, { once: true });
      });
    };
    function wav() {
      const buffer = new ArrayBuffer(364), view = new DataView(buffer);
      const word = (at: number, s: string) => [...s].forEach((c, i) => view.setUint8(at + i, c.charCodeAt(0)));
      word(0, 'RIFF'); view.setUint32(4, 356, true); word(8, 'WAVE'); word(12, 'fmt ');
      view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true);
      view.setUint32(24, 16000, true); view.setUint32(28, 32000, true); view.setUint16(32, 2, true); view.setUint16(34, 16, true);
      word(36, 'data'); view.setUint32(40, 320, true); return buffer;
    }
    HTMLMediaElement.prototype.play = function () { (window as any).player = this; return Promise.resolve(); };
    HTMLMediaElement.prototype.pause = function () { (window as any).paused = true; };
    HTMLMediaElement.prototype.load = function () {};
  });
  const session = { name: 'Speech test', model_id: 'model', messages: [{ role: 'assistant', content }], updated_at: new Date().toISOString() };
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/**', route => route.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2, capabilities: [] } }));
  await page.route('**/v1/**', route => {
    const path = new URL(route.request().url()).pathname;
    let json: unknown = {};
    if (path === '/v1/users/me') json = { authenticated: false, user: null };
    else if (path === '/v1/models') json = { data: [{ id: 'model', type: 'chat' }] };
    else if (path === '/v1/audio/status') json = { tts: { available: true } };
    else if (path === '/v1/sessions') json = { sessions: [session] };
    else if (path.endsWith('/turn')) json = { turn: null };
    else if (path.startsWith('/v1/sessions/')) json = session;
    return route.fulfill({ json });
  });
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toBeVisible();
}

test('read aloud immediately shows preparation, stops pending work and shows actionable errors', async ({ page }) => {
  await fixture(page);
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  await expect(page.getByRole('button', { name: 'Stop reading' })).toHaveText('Stop reading');
  await expect(page.locator('.message-speech [role="status"]')).toContainText('Generating speech');
  await expect(page.locator('.speech-preparing-indicator')).toBeVisible();
  await page.getByRole('button', { name: 'Stop reading' }).click();
  await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toBeVisible();
  expect(await page.evaluate(() => (window as any).speechAborts)).toBe(1);
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
  await page.evaluate(() => (window as any).finishSpeech(true));
  await expect(page.getByRole('alert')).toContainText('Base is unsupported');
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
});

for (const width of [390, 1440]) for (const theme of ['light', 'dark']) {
  test(`read-aloud preparation stays in a compact action row at ${width} ${theme}`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 900 });
    await page.addInitScript(theme => localStorage.setItem('offgrid.theme', theme), theme);
    await fixture(page);
    const row = page.locator('.message-actions');
    const before = await row.boundingBox();
    await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
    const stop = page.getByRole('button', { name: 'Stop reading', exact: true });
    await expect(stop).toBeVisible();
    expect((await row.boundingBox())!.height).toBe(before!.height);
    const copy = await row.locator('.message-copy').first().boundingBox();
    expect((await stop.boundingBox())!.y).toBe(copy!.y);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath('speech-toolbar.png') });
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await expect(page.locator('.speech-preparing-indicator')).toHaveCSS('animation-name', 'none');
    await stop.click();
    await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toBeVisible();
  });
}

test('long answers start with a bounded chunk, prefetch one chunk, and cancel on navigation', async ({ page }) => {
  await fixture(page, 'This is the first sentence. ' + 'Another useful sentence to read aloud. '.repeat(20));
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  expect(await page.evaluate(() => (window as any).speechRequests[0].input.length)).toBeLessThanOrEqual(160);
  await page.evaluate(() => (window as any).finishSpeech(false));
  await expect(page.getByRole('button', { name: 'Stop reading' })).toHaveText('Stop reading');
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
  await page.goto('/ui/#/models');
  await expect.poll(() => page.evaluate(() => (window as any).speechAborts)).toBeGreaterThan(0);
  expect(await page.evaluate(() => (window as any).paused)).toBe(true);
});

async function startSpokenTurn(page: Page) {
  await fixture(page);
  await openVoicePreferences(page);
  await page.getByText('Speak responses', { exact: true }).click();
  await openVoicePreferences(page);
  await expect(page.getByRole('checkbox', { name: 'Speak responses' })).toBeChecked();
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(0); // no history replay
  await page.locator('.composer textarea').fill('Answer with speech');
  await page.locator('.composer-actions > button').click();
  await expect.poll(() => page.evaluate(() => typeof (window as any).emitText)).toBe('function');
}

test('speech starts during text streaming and flushes the committed tail without replay', async ({ page }) => {
  await startSpokenTurn(page);
  await page.evaluate(() => (window as any).emitText('Hello there. Next'));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.map((r: any) => r.input))).toEqual(['Hello there.']);
  await page.evaluate(() => (window as any).finishSpeech(false));
  await expect(page.locator('.chat-speech-controls')).toContainText('Speaking the draft');
  await expect(page.locator('.composer-actions > button')).toHaveAccessibleName('Stop');
  await page.evaluate(() => { (window as any).player.onended(); (window as any).emitText(' sentence'); });
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  await page.evaluate(() => (window as any).finishText());
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.map((r: any) => r.input))).toEqual(['Hello there.', 'Next sentence']);
  await page.evaluate(() => (window as any).finishSpeech(false));
  await expect.poll(() => page.evaluate(() => Boolean((window as any).player.onended))).toBe(true);
  await page.evaluate(() => (window as any).player.onended());
  await expect(page.locator('.chat-speech-controls').getByRole('button', { name: 'Stop reading' })).toHaveCount(0);
});

test('failed text stream cancels pending audio and never speaks the unfinished tail', async ({ page }) => {
  await startSpokenTurn(page);
  await page.evaluate(() => (window as any).emitText('This is provisional. Unfinished'));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  await page.evaluate(() => (window as any).finishText(true));
  await expect.poll(() => page.evaluate(() => (window as any).speechAborts)).toBe(1);
  await expect(page.getByRole('alert')).toContainText('before the conversation was confirmed saved');
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
});

test('unavailable speech is explained before a synthesis request', async ({ page }) => {
  await fixture(page);
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { tts: { available: false, issue: 'Base requires reference audio. Install CustomVoice in Models.' } } }));
  await openVoicePreferences(page);
  await page.getByText('Speak responses', { exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Install CustomVoice');
  await openVoicePreferences(page);
  await expect(page.getByRole('checkbox', { name: 'Speak responses' })).not.toBeChecked();
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
  await expect(page.locator('.voice-error')).toContainText('Install CustomVoice');
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(0);
});

test('streamed reasoning and unfinished code are not spoken; overloaded audio stops without stopping chat', async ({ page }) => {
  await startSpokenTurn(page);
  await page.evaluate(() => (window as any).emitText('<think>Private reasoning. '));
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(0);
  await page.evaluate(() => (window as any).emitText('</think>Visible sentence. ```secret code. '));
  // No terminal fragment is flushed until the prose is complete/committed.
  await page.evaluate(() => (window as any).emitText('``` More visible text. Tail'));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  expect(await page.evaluate(() => (window as any).speechRequests[0].input)).not.toMatch(/Private|secret|think/);
  await page.evaluate(() => (window as any).emitText(' Many sentences. '.repeat(200)));
  await expect(page.getByRole('alert')).toContainText('cannot keep up');
  await expect(page.locator('.composer-actions > button')).toHaveAccessibleName('Stop');
  await expect.poll(() => page.evaluate(() => (window as any).speechAborts)).toBe(1);
});

test('streamed speech prefetches at most one chunk and Stop reading leaves generation running', async ({ page }) => {
  await startSpokenTurn(page);
  await page.evaluate(() => (window as any).emitText('A useful first sentence. '.repeat(24)));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  await page.evaluate(() => (window as any).finishSpeech(false));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
  await page.evaluate(() => (window as any).finishSpeech(false));
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
  await page.locator('.chat-speech-controls').getByRole('button', { name: 'Stop reading' }).click();
  await expect(page.locator('.composer-actions > button')).toHaveAccessibleName('Stop');
  await page.evaluate(() => { (window as any).emitText(' More text. '); (window as any).finishText(); });
  await expect(page.getByRole('button', { name: 'Talk', exact: true })).toBeVisible();
  await expect(page.locator('.composer-submit')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
});

test('leaving Chat cancels live speech and does not restore playback on return', async ({ page }) => {
  await startSpokenTurn(page);
  await page.evaluate(() => (window as any).emitText('A useful first sentence. Tail'));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  await page.goto('/ui/#/models');
  await expect.poll(() => page.evaluate(() => (window as any).speechAborts)).toBe(1);
  await page.goto('/ui/#/chat');
  await openVoicePreferences(page);
  await expect(page.getByRole('checkbox', { name: 'Speak responses' })).not.toBeChecked();
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
});

async function microphoneFixture(page: Page) {
  await page.addInitScript(() => {
    const state = (window as any).micTest = { starts: 0, stops: 0, permissions: 0, requests: 0 };
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: {
      getUserMedia: () => { state.permissions++; return new Promise(resolve => { state.allow = () => resolve({ getTracks: () => [{ stop: () => state.stops++ }] }); }); }
    } });
    (window as any).AudioContext = undefined;
    (window as any).MediaRecorder = class {
      static isTypeSupported() { return true; }
      state = 'inactive'; mimeType = 'audio/wav'; ondataavailable?: (event: any) => void; onstop?: () => void;
      start() { this.state = 'recording'; state.starts++; }
      stop() { this.state = 'inactive'; this.ondataavailable?.({ data: new Blob(['synthetic fixture'], { type: 'audio/wav' }) }); this.onstop?.(); }
    };
    const original = window.fetch.bind(window);
    window.fetch = (input, init) => {
      if (!String(input).endsWith('/audio/transcriptions')) return original(input, init);
      state.requests++; state.model = (init!.body as FormData).get('model');
      return new Promise((resolve, reject) => {
        state.finish = (text = 'spoken text') => resolve(new Response(JSON.stringify({ text }), { headers: { 'Content-Type': 'application/json' } }));
        init?.signal?.addEventListener('abort', () => reject(new DOMException('Cancelled', 'AbortError')), { once: true });
      });
    };
  });
  await fixture(page);
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { asr: { available: true }, tts: { available: true }, profiles: [
    { id: 'asr-a', revision: 'r1', name: 'Recognition A', capabilities: ['transcription'], available: true },
    { id: 'asr-b', revision: 'r2', name: 'Recognition B', capabilities: ['transcription'], available: true },
    { id: 'tts-a', revision: 'r1', name: 'Speech A', capabilities: ['speech_synthesis'], available: true, voices: [{ id: '0', language: 'en' }] },
    { id: 'tts-b', revision: 'r2', name: 'Speech B', capabilities: ['speech_synthesis'], available: true }
  ] } }));
}

test('missing speech runtime offers settings without capture or a model download', async ({ page }) => {
  await microphoneFixture(page);
  let downloads = 0;
  await page.route('**/v1/audio/status', route => route.fulfill({ json: {
    asr: { available: false, issue: 'Install the matching qwen3-asr speech runtime for this model.' },
    tts: { available: false }, profiles: [{ id: 'qwen-asr', revision: 'r1', name: 'Qwen ASR', capabilities: ['transcription'], available: false, issue: 'Runtime missing' }]
  } }));
  await page.route('**/download**', route => { downloads++; return route.abort(); });
  await page.goto('/ui/#/chat');
  const mic = page.locator('.voice-input-control[data-capture-phase]');
  await mic.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect(mic).toContainText('qwen3-asr');
  await expect(mic.locator('.composer-voice-recovery > .utility-popover-panel')).toBeVisible();
  await expect(page.locator('.voice-settings-panel:visible')).toContainText('Re-downloading a model will not fix a missing runtime');
  await expect(page.locator('.voice-settings-panel:visible').getByRole('link', { name: 'Manage speech models' })).toHaveAttribute('href', '#/models');
  expect(downloads).toBe(0);
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(0);
  expect(await page.evaluate(() => (window as any).micTest.requests)).toBe(0);
});

test('empty voice setup identifies missing models and opens the matching discovery category', async ({ page }) => {
  await microphoneFixture(page);
  await page.route('**/api/v2/models**', route => route.fulfill({ json: { models: [], repositories: [], operations: [] } }));
  await page.route('**/v1/catalog', route => route.fulfill({ json: { models: [] } }));
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { asr: { available: false }, tts: { available: false }, profiles: [] } }));
  await page.goto('/ui/#/chat');
  const mic = page.locator('.voice-input-control[data-capture-phase]');
  await mic.getByRole('button', { name: 'Use microphone', exact: true }).click();
  const panel = page.locator('.voice-settings-panel:visible');
  await expect(panel.getByText('No model is installed for this speech feature.')).toHaveCount(2);
  await expect(panel.getByRole('link', { name: 'Set up recognition', exact: true })).toBeVisible();
  await panel.getByRole('link', { name: 'Set up spoken responses', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Speech generation', exact: true })).toHaveAttribute('aria-pressed', 'true');
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(0);
});

test('microphone readiness and permission waiting have distinct cancellable feedback', async ({ page }) => {
  await microphoneFixture(page);
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/v1/audio/status', async route => {
    await held;
    await route.fulfill({ json: { asr: { available: true } } });
  });
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect(page.getByText('Checking voice availability…', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cancel microphone setup', exact: true })).toBeEnabled();
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(0);
  release();
  await expect(page.getByText('Waiting for microphone permission…', { exact: true })).toBeVisible();
  await expect(page.getByText('Transcribing…', { exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'Cancel microphone setup', exact: true }).click();
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.stops)).toBe(1);
  expect(await page.evaluate(() => (window as any).micTest.starts)).toBe(0);
  await expect(page.getByRole('button', { name: 'Use microphone', exact: true })).toBeVisible();
});

test('cancelling transcription preserves the draft and ignores late text', async ({ page }) => {
  await microphoneFixture(page);
  await page.locator('.composer textarea').fill('Keep my draft');
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect(page.getByText('Transcribing…', { exact: true })).toBeVisible();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(1);
  await page.getByRole('button', { name: 'Cancel transcription', exact: true }).click();
  await page.evaluate(() => (window as any).micTest.finish());
  await expect(page.locator('.composer textarea')).toHaveValue('Keep my draft');
  await expect(page.getByRole('button', { name: 'Use microphone', exact: true })).toBeVisible();
});

test('dictation feedback follows input levels, fits narrow screens and discards without transcribing', async ({ page }, info) => {
  await microphoneFixture(page);
  await page.evaluate(() => {
    (window as any).microphoneLevel = 0.06;
    (window as any).AudioContext = class {
      createMediaStreamSource() { return { connect() {} }; }
      createAnalyser() { return { fftSize: 1024, getFloatTimeDomainData(samples: Float32Array) { samples.fill((window as any).microphoneLevel); } }; }
      close() { return Promise.resolve(); }
    };
  });
  await page.locator('.composer textarea').fill('Keep this typed draft');
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await expect(page.locator('.voice-capture-feedback')).toContainText('Recording');
  await expect.poll(() => page.locator('.voice-wave i').last().evaluate(el => el.getBoundingClientRect().height)).toBeGreaterThan(3);
  await page.evaluate(() => { (window as any).microphoneLevel = 0; });
  await expect.poll(() => page.locator('.voice-wave i').last().evaluate(el => el.getBoundingClientRect().height)).toBe(3);
  await page.evaluate(() => { (window as any).microphoneLevel = 0.06; });
  await page.screenshot({ path: info.outputPath('dictation-recording-desktop.png'), fullPage: true });
  await page.setViewportSize({ width: 320, height: 640 });
  await expect(page.getByRole('button', { name: 'Stop recording', exact: true })).toBeInViewport();
  await expect(page.locator('.voice-discard')).toBeInViewport();
  await expect(page.locator('.voice-wave')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: info.outputPath('dictation-recording-narrow.png'), fullPage: true });
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.locator('.workspace-options > button').click();
  await page.getByLabel('Appearance', { exact: true }).selectOption('dark');
  await page.keyboard.press('Escape');
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.setViewportSize({ width: 320, height: 640 });
  await page.screenshot({ path: info.outputPath('dictation-recording-dark.png'), fullPage: true });
  await page.locator('.voice-discard').click();
  await expect(page.getByRole('button', { name: 'Use microphone', exact: true })).toBeVisible();
  expect(await page.evaluate(() => (window as any).micTest.requests)).toBe(0);
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this typed draft');
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(2);
  await page.evaluate(() => (window as any).micTest.allow());
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect(page.getByText('Transcribing…', { exact: true })).toBeVisible();
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await expect(page.locator('.voice-progress')).toHaveCSS('animation-name', 'none');
  await page.screenshot({ path: info.outputPath('dictation-transcribing-narrow.png'), fullPage: true });
  await page.getByRole('button', { name: 'Cancel transcription', exact: true }).click();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep this typed draft');
});

test('late microphone permission after navigation releases tracks without starting capture', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.goto('/ui/#/models');
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.stops)).toBe(1);
  expect(await page.evaluate(() => (window as any).micTest.starts)).toBe(0);
});

test('dictation retains concurrent draft edits and sends the selected recognition model', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await chooseModel(page, 'Recognition model', 'asr-b@r2');
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await page.locator('.composer textarea').fill('original');
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.starts)).toBe(1);
  await page.locator('.composer textarea').fill('original plus typed edits');
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(1);
  await page.evaluate(() => (window as any).micTest.finish());
  await expect(page.locator('.composer textarea')).toHaveValue('original plus typed edits spoken text');
  expect(await page.evaluate(() => (window as any).micTest.model)).toBe('asr-b@r2');
});

test('dictation ends after speech and a pause without sending the editable draft', async ({ page }) => {
  await microphoneFixture(page);
  await page.evaluate(() => {
    (window as any).micLevel = 0.06;
    (window as any).AudioContext = class {
      createMediaStreamSource() { return { connect() {} }; }
      createAnalyser() { return { fftSize: 1024, getFloatTimeDomainData(samples: Float32Array) { samples.fill((window as any).micLevel); } }; }
      close() { return Promise.resolve(); }
    };
  });
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.starts)).toBe(1);
  await page.waitForTimeout(350);
  await page.evaluate(() => { (window as any).micLevel = 0; });
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests), { timeout: 4000 }).toBe(1);
  await page.evaluate(() => (window as any).micTest.finish());
  await expect(page.locator('.composer textarea')).toHaveValue('spoken text');
  await expect(page.locator('.message.user')).toHaveCount(0);
});

test('Talk sends one completed utterance, does not capture speaker output, and relistens after playback', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Talk', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.starts)).toBe(1);
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(1);
  await page.evaluate(() => (window as any).micTest.finish());
  await expect.poll(() => page.evaluate(() => typeof (window as any).emitText)).toBe('function');
  await page.evaluate(() => { (window as any).emitText('Hello there.'); (window as any).finishText(); });
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).finishSpeech(false));
  await expect.poll(() => page.evaluate(() => Boolean((window as any).player?.onended))).toBe(true);
  await page.evaluate(() => (window as any).player.onended());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(2);
  await page.getByRole('button', { name: 'Stop Talk', exact: true }).click();
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.stops)).toBeGreaterThanOrEqual(2);
  expect(await page.evaluate(() => (window as any).micTest.starts)).toBe(1);
});

test('Talk creates one new conversation and keeps the transcribed turn in it', async ({ page }) => {
  await microphoneFixture(page);
  let createdName = '';
  let creates = 0;
  await page.route('**/v1/sessions', async route => {
    if (route.request().method() !== 'POST') return route.fallback();
    creates++;
    const request = route.request().postDataJSON();
    createdName = request.name;
    await route.fulfill({ json: { name: createdName, model_id: request.model_id, messages: [], updated_at: new Date().toISOString() } });
  });
  await page.getByRole('button', { name: 'New chat', exact: true }).click();
  await page.getByRole('button', { name: 'Talk', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(1);
  await page.evaluate(() => (window as any).micTest.finish());
  await expect.poll(() => createdName).not.toBe('');
  await expect.poll(() => page.evaluate(() => typeof (window as any).finishText)).toBe('function');
  await page.evaluate(name => { (window as any).voiceSessionName = name; (window as any).emitText('A saved reply.'); (window as any).finishText(); }, createdName);
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  expect(creates).toBe(1);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  await page.getByRole('button', { name: 'Stop Talk', exact: true }).click();
});

test('Stop Talk during generation aborts the turn and never starts speech or another capture', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Talk', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(1);
  await page.evaluate(() => (window as any).micTest.finish());
  await expect.poll(() => page.evaluate(() => typeof (window as any).finishText)).toBe('function');
  await page.getByRole('button', { name: 'Stop Talk', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeVisible();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(0);
  await expect(page.locator('.composer textarea')).toHaveValue('spoken text');
});

test('Talk failure preserves its transcript and does not submit another request', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Talk', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(1);
  await page.evaluate(() => (window as any).micTest.finish());
  await expect.poll(() => page.evaluate(() => typeof (window as any).finishText)).toBe('function');
  await page.evaluate(() => (window as any).finishText(true));
  await expect(page.locator('.talk-status')).toContainText('Talk paused');
  await expect(page.locator('.composer textarea')).toHaveValue('spoken text');
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
});

test('typing exits Talk without losing text or submitting microphone audio', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Talk', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.starts)).toBe(1);
  await page.locator('.composer textarea').fill('Typed instead');
  await expect(page.locator('.composer textarea')).toHaveValue('Typed instead');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  await expect(page.getByRole('button', { name: 'Talk', exact: true })).toHaveCount(0);
  await expect.poll(() => page.evaluate(() => (window as any).micTest.stops)).toBeGreaterThan(0);
  expect(await page.evaluate(() => (window as any).micTest.requests)).toBe(0);
});

test('Talk control remains usable in a narrow Chat composer', async ({ page }) => {
  await microphoneFixture(page);
  await page.setViewportSize({ width: 390, height: 740 });
  await expect(page.getByRole('button', { name: 'Talk', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Use microphone', exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('one primary composer action follows the draft without starting capture', async ({ page }, info) => {
  await microphoneFixture(page);
  const editor = page.locator('.composer textarea');
  await expect(page.getByRole('button', { name: 'Talk', exact: true })).toBeVisible();
  await expect(page.locator('.composer-submit')).toHaveCount(0);
  await page.screenshot({ path: info.outputPath('composer-voice-idle.png') });
  await editor.fill('Tell me a story');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeEnabled();
  await expect(page.getByRole('button', { name: 'Talk', exact: true })).toHaveCount(0);
  await expect(editor).toBeFocused();
  await page.screenshot({ path: info.outputPath('composer-send-ready.png') });
  await editor.fill('  ');
  await expect(page.getByRole('button', { name: 'Talk', exact: true })).toBeVisible();
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(0);
});

test('Stop recording remains usable while a text response is generating', async ({ page }) => {
  await microphoneFixture(page);
  await page.locator('.composer textarea').fill('typed request');
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(1);
  await page.evaluate(() => (window as any).micTest.allow());
  await expect.poll(() => page.evaluate(() => (window as any).micTest.starts)).toBe(1);
  await page.locator('.composer-actions > button').click();
  await expect(page.locator('.composer-actions > button')).toHaveAccessibleName('Stop');
  await expect(page.getByRole('button', { name: 'Stop recording', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.stops)).toBeGreaterThan(0);
});

test('speech selection stays pinned across chunks when preferences change', async ({ page }, info) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await chooseModel(page, 'Speech model', 'tts-a@r1');
  await page.getByLabel('Voice', { exact: true }).selectOption('0');
  await page.screenshot({ path: info.outputPath('voice-settings.png'), fullPage: true });
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await openVoicePreferences(page);
  await page.getByText('Speak responses', { exact: true }).click();
  await openVoicePreferences(page);
  await expect(page.getByRole('checkbox', { name: 'Speak responses' })).toBeChecked();
  await page.locator('.composer textarea').fill('speak');
  await page.locator('.composer-actions > button').click();
  await expect.poll(() => page.evaluate(() => typeof (window as any).emitText)).toBe('function');
  await page.evaluate(() => (window as any).emitText('First sentence. Tail'));
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(1);
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await chooseModel(page, 'Speech model', 'tts-b@r2');
  await page.evaluate(() => (window as any).finishSpeech(false));
  await expect.poll(() => page.evaluate(() => Boolean((window as any).player?.onended))).toBe(true);
  await page.evaluate(() => { (window as any).player.onended(); (window as any).finishText(); });
  await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(2);
  expect(await page.evaluate(() => (window as any).speechRequests.map((r: any) => [r.model, r.voice]))).toEqual([['tts-a@r1', '0'], ['tts-a@r1', '0']]);
});

test('a removed explicit speech selection never falls back or opens the microphone', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await chooseModel(page, 'Recognition model', 'asr-b@r2');
  await chooseModel(page, 'Speech model', 'tts-b@r2');
  await page.getByRole('button', { name: 'Response settings', exact: true }).click();
  await page.route('**/v1/audio/status', route => route.fulfill({ json: { asr: { available: true }, tts: { available: true }, profiles: [] } }));
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await expect(page.locator('.composer .voice-error')).toContainText('no longer installed');
  expect(await page.evaluate(() => (window as any).micTest.permissions)).toBe(0);
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
  await expect(page.locator('.message .voice-error')).toContainText('no longer installed');
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(0);
});

test('voice settings remain visible and keyboard reachable on narrow light and dark layouts', async ({ page }, info) => {
  await microphoneFixture(page);
  await page.setViewportSize({ width: 390, height: 844 });
  for (const theme of ['light', 'dark']) {
    await page.evaluate(theme => localStorage.setItem('offgrid.theme', theme), theme);
    await page.reload();
    const summary = page.getByRole('button', { name: 'Response settings', exact: true });
    await summary.focus(); await page.keyboard.press('Enter');
    const panel = page.locator('.composer-context-menu > .utility-popover-panel');
    await expect(panel).toBeVisible();
    // Readiness temporarily disables selectors while installed profiles load.
    await expect(page.getByRole('button', { name: 'Recognition model', exact: true })).toContainText('Recognition A');
    await expect(page.getByLabel('Recognition model', { exact: true })).toBeEnabled();
    await page.getByLabel('Recognition model', { exact: true }).focus();
    await expect(page.getByLabel('Recognition model', { exact: true })).toBeFocused();
    const box = await panel.boundingBox();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath(`voice-${theme}-mobile.png`), fullPage: true });
  }
});

test('voice utility remains bounded after resizing to a short viewport', async ({ page }) => {
  await microphoneFixture(page);
  await page.setViewportSize({ width: 320, height: 480 });
  const trigger = page.getByRole('button', { name: 'Response settings', exact: true });
  await trigger.click();
  const panel = page.getByRole('region', { name: 'Response settings', exact: true });
  await expect(panel).toBeVisible();
  await expect.poll(async () => {
    const box = await panel.boundingBox();
    return !!box && box.x >= 0 && box.x + box.width <= 320 && box.y >= 0 && box.y + box.height <= 480;
  }).toBe(true);
  await panel.getByRole('link', { name: 'Manage speech models' }).focus();
  await expect(panel.getByRole('link', { name: 'Manage speech models' })).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(panel).toBeHidden();
  await expect(trigger).toBeFocused();
});

test('dismissed voice metadata is cancelled and cannot replace a reopened panel', async ({ page }) => {
  await fixture(page);
  await page.evaluate(() => {
    const original = window.fetch.bind(window);
    const pending = (window as any).metadata = { replies: [] as { signal: AbortSignal; reply: (name: string) => void }[] };
    window.fetch = (input, init) => {
      if (!String(input).endsWith('/v1/audio/status')) return original(input, init);
      // Deliberately allow a late response after abort to exercise the stale guard.
      return new Promise(resolve => pending.replies.push({ signal: init!.signal!, reply: (name: string) => resolve(new Response(JSON.stringify({ profiles: [
        { id: name, revision: 'r1', name, available: true, capabilities: ['transcription'] },
      ] }), { headers: { 'Content-Type': 'application/json' } })) }));
    };
  });
  const trigger = page.getByRole('button', { name: 'Response settings', exact: true });
  await trigger.click();
  // Development StrictMode mounts effects twice; assert the live operation,
  // not the total number of correctly cancelled development-only requests.
  await expect.poll(() => page.evaluate(() => (window as any).metadata.replies.filter((item: any) => !item.signal.aborted).length)).toBe(1);
  await page.keyboard.press('Escape');
  await expect.poll(() => page.evaluate(() => (window as any).metadata.replies.every((item: any) => item.signal.aborted))).toBe(true);
  await trigger.click();
  await expect.poll(() => page.evaluate(() => (window as any).metadata.replies.filter((item: any) => !item.signal.aborted).length)).toBe(1);
  await page.evaluate(() => (window as any).metadata.replies.find((item: any) => !item.signal.aborted).reply('Current profile'));
  await expect(page.getByLabel('Recognition model', { exact: true })).toContainText('Current profile');
  await page.evaluate(() => (window as any).metadata.replies.filter((item: any) => item.signal.aborted).forEach((item: any) => item.reply('Stale profile')));
  await expect(page.getByLabel('Recognition model', { exact: true })).not.toContainText('Stale profile');
});

for (const surface of ['chat', 'agents']) {
  test(`${surface} voice settings dismiss outside and with Escape, retaining selections`, async ({ page }, info) => {
    await microphoneFixture(page);
    if (surface === 'agents') {
      await installChatJob(page,{workspace:'voice-agent',model:'model',snapshot:()=>({}),submit:()=>{throw Error('Voice preferences must not submit work');}});
      await page.route('**/api/v2/jobs*', route => route.fulfill({ json: [] }));
      await page.route('**/v1/agents/tasks', route => route.fulfill({ json: [] }));
      await page.route('**/v1/agents/tools', route => route.fulfill({ json: { tools: [], enabled_count: 0 } }));
      await page.route('**/v1/agents/mcp', route => route.fulfill({ json: { servers: [] } }));
      await openAgentChat(page);
    }
    const button = page.getByRole('button', { name: 'Response settings', exact: true });
    const panel = page.getByRole('region', { name: 'Response settings', exact: true });
    const editor = page.locator('.composer textarea');
    const outside = page.locator('.topbar h1');
    await expect(panel).toBeHidden();
    await button.click();
    await expect(button).toHaveAttribute('aria-expanded', 'true');
    await chooseModel(page, 'Recognition model', 'asr-b@r2');
    await expect(panel).toBeVisible();
    await outside.click();
    await expect(panel).toBeHidden();
    await expect(button).toHaveAttribute('aria-expanded', 'false');
    if (surface === 'chat') { await editor.click(); await expect(editor).toBeFocused(); }
    await button.click();
    await expect(page.getByRole('button', { name: 'Recognition model', exact: true })).toContainText('Recognition B');
    await page.getByLabel('Speech model', { exact: true }).focus();
    await page.keyboard.press('Escape');
    await expect(panel).toBeHidden();
    await expect(button).toBeFocused();
    await button.press('Enter');
    await expect(panel).toBeVisible();
    await button.click();
    await expect(panel).toBeHidden();
    if (surface === 'chat') {
      await expect(page.locator('.composer').getByRole('button', { name: 'Response settings', exact: true })).toBeVisible();
      await openVoicePreferences(page);
      await expect(page.locator('.composer').getByRole('checkbox', { name: 'Speak responses' })).toBeVisible();
    }
    await page.screenshot({ path: info.outputPath(`${surface}-voice-controls.png`), fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.evaluate(() => document.documentElement.dir = 'rtl');
    await openVoicePreferences(page);
    const box = await panel.boundingBox();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    await page.screenshot({ path: info.outputPath(`${surface}-voice-rtl.png`), fullPage: true });
    // On narrow layouts the panel legitimately overlays the editor. Use the
    // exposed header as the outside target, rather than clicking through it.
    await page.locator('.topbar h1').click();
    await expect(panel).toBeHidden();
    await expect(page.getByRole('alert')).toHaveCount(0);
  });
}
async function openVoicePreferences(page: import('@playwright/test').Page) {
  const trigger = page.getByRole('button', { name: 'Response settings', exact: true });
  if (await trigger.getAttribute('aria-expanded') !== 'true') await trigger.click();
}


test('silence keeps the draft and reports an actionable result without sending', async ({page}) => {
  await microphoneFixture(page);
  const editor=page.locator('.composer textarea');
  await editor.fill('Keep my words');
  await page.getByRole('button',{name:'Use microphone',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).micTest.permissions)).toBe(1);
  await page.evaluate(()=>(window as any).micTest.allow());
  await page.getByRole('button',{name:'Stop recording',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).micTest.requests)).toBe(1);
  await page.evaluate(()=>(window as any).micTest.finish(''));
  await expect(page.getByText('No speech was detected. Try again or type your message. Your draft is unchanged.',{exact:true})).toBeVisible();
  await expect(editor).toHaveValue('Keep my words');
});


test('changing language during microphone permission keeps one request and localizes its failure', async ({page}) => {
  await microphoneFixture(page);
  await page.evaluate(()=>Object.defineProperty(navigator,'mediaDevices',{configurable:true,value:{getUserMedia:()=>{
    (window as any).micTest.permissions++;
    return new Promise((_resolve,reject)=>{(window as any).micTest.deny=()=>reject(new DOMException('denied','NotAllowedError'));});
  }}}));
  await page.getByRole('button',{name:'Use microphone',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).micTest.permissions)).toBe(1);
  await page.locator('.workspace-options > button').click();
  await page.locator('.workspace-options-panel select').nth(0).selectOption('ar');
  await page.keyboard.press('Escape');
  await page.evaluate(()=>(window as any).micTest.deny());
  await expect(page.getByText(voiceText('ar').permission,{exact:true})).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('dir','rtl');
  expect(await page.evaluate(()=>(window as any).micTest.permissions)).toBe(1);
});


test('Talk survives three replies, shows one transcription status and recovers from silence without model setup', async ({ page }) => {
  await microphoneFixture(page);
  await page.getByRole('button', { name: 'Talk', exact: true }).click();
  for (let turn = 1; turn <= 3; turn++) {
    await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(turn);
    await page.evaluate(() => (window as any).micTest.allow());
    await expect.poll(() => page.evaluate(() => (window as any).micTest.starts)).toBe(turn);
    await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
    await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(turn);
    await expect(page.getByText('Transcribing\u2026', { exact: true })).toHaveCount(1);
    await expect(page.locator('.talk-status')).toHaveCount(0);
    await page.evaluate(() => { (window as any).emitText = null; (window as any).micTest.finish(); });
    await expect.poll(() => page.evaluate(() => typeof (window as any).emitText)).toBe('function');
    await page.evaluate(() => { (window as any).emitText('Hello again.'); (window as any).finishText(); });
    await expect.poll(() => page.evaluate(() => (window as any).speechRequests.length)).toBe(turn);
    await page.evaluate(() => (window as any).finishSpeech(false));
    await expect.poll(() => page.evaluate(() => Boolean((window as any).player?.onended))).toBe(true);
    await page.evaluate(() => (window as any).player.onended());
  }
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(4);
  await page.evaluate(() => (window as any).micTest.allow());
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.requests)).toBe(4);
  await page.evaluate(() => (window as any).micTest.finish(''));
  await expect(page.locator('.chat-speech-controls [role=alert]')).toContainText('No speech was detected');
  await expect(page.locator('.chat-speech-controls a[href="#/models"]')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  expect(await page.evaluate(() => (window as any).speechRequests.length)).toBe(3);
  await page.getByRole('button', { name: 'Speak again', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).micTest.permissions)).toBe(5);
  await expect(page.locator('.chat-speech-controls [role=alert]')).toHaveCount(0);
  await page.getByRole('button', { name: 'Stop Talk', exact: true }).click();
});
