import { readFile } from 'node:fs/promises';
import { test, expect } from '@playwright/test';

for (const short of [false, true]) test(`real browser recording converts device-rate audio for native recognition: ${short ? 'short manual Stop' : 'pause completion'}`, async ({page}) => {
  test.skip(!process.env.OFFGRID_E2E_SPEECH_FIXTURE, 'Requires isolated native speech qualification');
  test.setTimeout(90_000);
  test.skip(short && !process.env.OFFGRID_E2E_SHORT_SPEECH_FIXTURE, 'Requires generated short speech fixture');
  const bytes = await readFile((short ? process.env.OFFGRID_E2E_SHORT_SPEECH_FIXTURE : process.env.OFFGRID_E2E_SPEECH_FIXTURE)!);
  await page.addInitScript(base64 => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    const state = (window as any).recordingTest = {released: 0, upload: null};
    Object.defineProperty(navigator, 'mediaDevices', {configurable: true, value: {getUserMedia: async () => {
      const context = new AudioContext({sampleRate: 48000});
      const buffer = await context.decodeAudioData(Uint8Array.from(atob(base64), c => c.charCodeAt(0)).buffer);
      const source = context.createBufferSource(), output = context.createMediaStreamDestination();
      source.buffer = buffer; source.connect(output);
      source.onended = () => {state.ended = true;};
      await context.resume();
      state.start = () => source.start();
      const track = output.stream.getAudioTracks()[0], stop = track.stop.bind(track);
      track.stop = () => {state.released++; stop(); source.stop(); void context.close();};
      return output.stream;
    }}});
    const fetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (String(input).endsWith('/audio/transcriptions')) {
        const file = (init!.body as FormData).get('file') as Blob;
        const view = new DataView(await file.arrayBuffer());
        state.upload = {rate: view.getUint32(24, true), channels: view.getUint16(22, true), bytes: file.size};
      }
      return fetch(input, init);
    };
  }, bytes.toString('base64'));
  await page.goto('/ui/#/chat');
  await page.getByRole('button', {name:'Use microphone', exact:true}).click();
  await expect(page.getByRole('button', {name:'Stop recording', exact:true})).toBeVisible();
  await page.evaluate(() => (window as any).recordingTest.start());
  if (short) {
    await expect.poll(() => page.evaluate(() => (window as any).recordingTest.ended)).toBe(true);
    await page.getByRole('button', {name:'Stop recording', exact:true}).click();
  }
  await expect.poll(() => page.evaluate(() => (window as any).recordingTest.upload), {timeout:20000}).not.toBeNull();
  await expect(page.locator('.composer textarea')).toHaveValue(short ? /hello/i : /fellow Americans|your country/i, {timeout:45000});
  expect(await page.evaluate(() => (window as any).recordingTest.upload.rate)).toBe(16000);
  expect(await page.evaluate(() => (window as any).recordingTest.upload.channels)).toBe(1);
  expect(await page.evaluate(() => (window as any).recordingTest.released)).toBeGreaterThan(0);
});

// Opt-in only: a disposable service with real packaged speech runtimes/models.
// The microphone source is a known recording, never a participant's microphone.
test('native speech transcribes a known recording and plays generated audio in Chat', async ({ page }) => {
  test.skip(!process.env.OFFGRID_E2E_SPEECH_FIXTURE, 'Requires isolated native speech qualification');
  test.setTimeout(90_000);
  const bytes = await readFile(process.env.OFFGRID_E2E_SPEECH_FIXTURE!);
  await page.addInitScript(base64 => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    const recording = Uint8Array.from(atob(base64), char => char.charCodeAt(0));
    (window as any).nativeSpeech = { captureReleased: 0, playbackStarted: 0 };
    Object.defineProperty(navigator, 'mediaDevices', { configurable: true, value: {
      getUserMedia: async () => ({ getTracks: () => [{ stop: () => (window as any).nativeSpeech.captureReleased++ }] }),
    } });
    (window as any).MediaRecorder = class {
      static isTypeSupported() { return true; }
      state = 'inactive'; mimeType = 'audio/wav'; ondataavailable?: (event: any) => void; onstop?: () => void;
      start() { this.state = 'recording'; }
      stop() { this.state = 'inactive'; this.ondataavailable?.({ data: new Blob([recording], { type: 'audio/wav' }) }); this.onstop?.(); }
    };
    const play = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      this.addEventListener('playing', () => (window as any).nativeSpeech.playbackStarted++, { once: true });
      return play.call(this);
    };
  }, bytes.toString('base64'));
  // Saved answer fixture only. Readiness, WAV conversion, transcription,
  // synthesis and browser decoding/playback all use their production paths.
  const session = { name: 'Native speech test', model_id: 'fixture', messages: [{ role: 'assistant', content: 'Welcome to OffGrid. Your speech is processed locally on your computer.' }], updated_at: new Date().toISOString() };
  await page.route('**/v1/models', route => route.fulfill({ json: { data: [{ id: 'fixture', type: 'chat' }] } }));
  await page.route('**/v1/sessions', route => route.fulfill({ json: { sessions: [session] } }));
  await page.route('**/v1/sessions/**', route => route.fulfill({ json: route.request().url().endsWith('/turn') ? { turn: null } : session }));
  await page.goto('/ui/#/chat');
  await page.getByRole('button', { name: 'Use microphone', exact: true }).click();
  await page.getByRole('button', { name: 'Stop recording', exact: true }).click();
  await expect(page.locator('.composer textarea')).toHaveValue(/your country/i, { timeout: 45_000 });
  expect(await page.evaluate(() => (window as any).nativeSpeech.captureReleased)).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Read aloud', exact: true }).click();
  await expect.poll(() => page.evaluate(() => (window as any).nativeSpeech.playbackStarted), { timeout: 30_000 }).toBeGreaterThan(0);
  await page.getByRole('button', { name: 'Stop reading', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Read aloud', exact: true })).toBeVisible();
});
