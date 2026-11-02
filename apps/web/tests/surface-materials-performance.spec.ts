import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

test('floating surfaces remain responsive over a streaming answer at 4x and 6x CPU', async ({ browser }, info) => {
  test.skip(!process.env.OFFGRID_PAINT_MEASURE, 'Run isolated, with one worker.');
  test.setTimeout(180_000);
  const samples: unknown[] = [];
  for (const rate of [4, 6]) for (const mode of ['off', 'on']) for (const theme of ['light', 'dark']) {
    const context = await browser.newContext(), page = await context.newPage();
    try {
      await craftFixture(page, 'en', theme);
      const cdp = await context.newCDPSession(page);
      await cdp.send('Emulation.setCPUThrottlingRate', { rate });
      await page.goto('/ui/#/chat');
      await page.evaluate(mode => document.documentElement.dataset.gradients = mode, mode);
      await page.locator('.composer textarea').fill('Continue the briefing');
      await page.locator('.composer-actions > button:last-child').click();
      await expect(page.locator('.streaming-response')).toBeVisible();
      const openMs = await page.evaluate(async () => {
        const start = performance.now();
        document.querySelector<HTMLButtonElement>('.composer-context-menu > button')!.click();
        await new Promise(requestAnimationFrame); return performance.now() - start;
      });
      const panel = page.locator('.composer-context-menu > .utility-popover-panel');
      await expect(panel).toHaveCSS('opacity', '1');
      await expect(panel).toHaveCSS('backdrop-filter', mode === 'on' ? 'blur(12px)' : 'none');
      const events: { name: string; dur?: number }[] = [];
      cdp.on('Tracing.dataCollected', ({ value }) => events.push(...value));
      await cdp.send('Tracing.start', { categories: 'devtools.timeline', transferMode: 'ReportEvents' });
      const frames = await page.evaluate(async () => {
        const frames: number[] = []; let active = true, last = performance.now();
        const tick = (now: number) => { frames.push(now - last); last = now; if (active) requestAnimationFrame(tick); };
        requestAnimationFrame(tick);
        for (let i = 0; i < 60; i++) {
          (window as unknown as { emitCraftDelta: (text: string) => void }).emitCraftDelta('\n\nA long streamed paragraph remains readable. café 日本語. '.repeat(6));
          await new Promise(resolve => setTimeout(resolve, 55));
        }
        active = false; return frames.sort((a, b) => a - b);
      });
      const finished = new Promise<void>(resolve => cdp.once('Tracing.tracingComplete', () => resolve()));
      await cdp.send('Tracing.end'); await finished;
      await page.keyboard.press('Escape');
      const stopMs = await page.evaluate(async () => {
        const start = performance.now(); document.querySelector<HTMLButtonElement>('.composer-actions > button:last-child')!.click();
        await new Promise(requestAnimationFrame); return performance.now() - start;
      });
      const frameP95 = frames[Math.floor(frames.length * .95)];
      const sample = { rate, mode, theme, openMs, stopMs, frameP95, paintMs: events.filter(event => event.name === 'Paint').reduce((sum, event) => sum + (event.dur ?? 0), 0) / 1000 };
      samples.push(sample);
      console.log(JSON.stringify(sample));
      expect.soft(openMs, `${rate}x ${mode} ${theme} open`).toBeLessThan(250);
      expect.soft(stopMs, `${rate}x ${mode} ${theme} Stop`).toBeLessThan(250);
      expect.soft(frameP95, `${rate}x ${mode} ${theme} frame p95`).toBeLessThanOrEqual(250);
    } finally { await context.close(); }
  }
  await info.attach('surface-performance.json', { body: JSON.stringify({ browser: browser.version(), samples }, null, 2), contentType: 'application/json' });
});
