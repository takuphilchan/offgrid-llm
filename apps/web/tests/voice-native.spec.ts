import { readFile } from 'node:fs/promises';
import { test, expect } from '@playwright/test';

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
