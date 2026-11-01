import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

for (const theme of ['light', 'dark']) for (const locale of ['en', 'ar']) for (const width of [390, 1440]) {
  test(`floating material stays legible, bounded and flat when requested: ${theme} ${locale} ${width}`, async ({ page }, info) => {
    const state = await craftFixture(page, locale, theme);
    await page.setViewportSize({ width, height: 700 });
    await page.goto('/ui/#/settings');
    if (width < 1024) await page.locator('.navigation-trigger').click();
    const trigger = page.locator('.workspace-options > button');
    await trigger.click();
    const panel = page.locator('.workspace-options-panel');
    await expect(panel).toHaveCSS('opacity', '1');
    await expect(panel).toHaveCSS('background-image', /linear-gradient/);
    await expect(panel).toHaveCSS('backdrop-filter', 'blur(12px)');
    const geometry = await panel.boundingBox();
    expect(geometry!.x).toBeGreaterThanOrEqual(0);
    expect(geometry!.y).toBeGreaterThanOrEqual(0);
    expect(geometry!.x + geometry!.width).toBeLessThanOrEqual(width);
    expect(geometry!.y + geometry!.height).toBeLessThanOrEqual(700);
    await page.screenshot({ path: info.outputPath('material.png'), animations: 'disabled' });
    const opaque = async () => {
      await expect(panel).toHaveCSS('backdrop-filter', 'none');
      await expect(panel).toHaveCSS('background-image', 'none');
      const fill = await panel.evaluate(el => getComputedStyle(el).backgroundColor);
      expect(fill).toMatch(/^rgb\(/); // Fully opaque, not rgba.
      expect(await panel.boundingBox()).toEqual(geometry);
    };
    await page.evaluate(() => document.documentElement.dataset.gradients = 'off');
    await opaque();
    await page.evaluate(() => delete document.documentElement.dataset.gradients);
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-reduced-transparency', value: 'reduce' }] });
    await opaque();
    await cdp.send('Emulation.setEmulatedMedia', { features: [] });
    await page.emulateMedia({ contrast: 'more' }); await opaque();
    await page.emulateMedia({ contrast: 'no-preference', forcedColors: 'active' }); await opaque();
    await page.emulateMedia({ forcedColors: 'none', reducedMotion: 'reduce' });
    await expect(panel).toHaveCSS('animation-name', 'none');
    await expect(panel).toHaveCSS('backdrop-filter', 'blur(12px)');
    await page.keyboard.press('Escape');
    await expect(trigger).toBeFocused();
    expect(state.writes).toEqual([]);
  });
}

for (const theme of ['light', 'dark']) test(`material text contrast includes extreme underlying content: ${theme}`, async ({ page }, info) => {
  await craftFixture(page, 'en', theme);
  await page.goto('/ui/#/settings');
  const result = await page.evaluate(() => {
    const probe = document.createElement('i'); document.body.append(probe);
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true })!;
    const color = (value: string) => { probe.style.color = value; return getComputedStyle(probe).color; };
    const luminance = () => {
      const rgb = ctx.getImageData(0, 0, 1, 1).data;
      return [.2126, .7152, .0722].reduce((sum, weight, i) => {
        const c = rgb[i] / 255; return sum + weight * (c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4);
      }, 0);
    };
    const minima: Record<string, number> = {};
    for (const name of ['text', 'muted', 'faint']) {
      ctx.fillStyle = color(`var(--${name})`); ctx.fillRect(0, 0, 1, 1); const foreground = luminance();
      let minimum = Infinity;
      for (const underlay of ['black', 'white']) for (let tint = 0; tint <= 35; tint++) {
        ctx.fillStyle = underlay; ctx.fillRect(0, 0, 1, 1);
        ctx.fillStyle = color('var(--material-fill)'); ctx.fillRect(0, 0, 1, 1);
        ctx.fillStyle = color(`color-mix(in oklab, var(--surface-2) ${tint}%, transparent)`); ctx.fillRect(0, 0, 1, 1);
        const background = luminance();
        minimum = Math.min(minimum, (Math.max(foreground, background) + .05) / (Math.min(foreground, background) + .05));
      }
      minima[name] = minimum;
    }
    probe.remove(); return minima;
  });
  for (const [name, contrast] of Object.entries(result)) expect(contrast, name).toBeGreaterThanOrEqual(4.5);
  await info.attach('composited-contrast.json', { body: JSON.stringify(result, null, 2), contentType: 'application/json' });
  console.log(JSON.stringify({ theme, materialContrast: result }));
});

test('primary hover preserves text opacity and modal material preserves keyboard control', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'dark');
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Do not submit this draft');
  const send = page.locator('.composer-actions > button:last-child');
  await send.hover();
  await expect(send).toHaveCSS('opacity', '1');
  await expect(send).toHaveCSS('background-image', 'none');
  await page.locator('.command-trigger').click();
  const palette = page.locator('dialog.command-palette');
  await expect(palette).toHaveCSS('backdrop-filter', 'blur(12px)');
  for (let i = 0; i < 16; i++) {
    await page.keyboard.press('Tab');
    expect(await palette.evaluate(el => el.contains(document.activeElement))).toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(page.locator('.command-trigger')).toBeFocused();
  await expect(page.locator('.composer textarea')).toHaveValue('Do not submit this draft');
  expect(state.writes).toEqual([]);
});

test('generated defaults remain opaque when enhancement support is absent', async ({ page }) => {
  // Exercise the real token defaults without depending on an older browser
  // installation. Unsupported declarations must not be required for readability.
  const tokens = readFileSync(resolve('src/tokens.css'), 'utf8')
    .replace(/@supports[^\{]+\{/g, '@supports (offgrid-unsupported-property: never) {');
  await page.setContent(`<style>${tokens}
    .probe { width: 300px; height: 100px; color: var(--text); background-color: var(--material-fill);
      background-image: var(--gradient-floating); backdrop-filter: var(--material-blur); }
    </style><div class="probe">Readable fallback</div>`);
  const probe = page.locator('.probe');
  for (const theme of ['light', 'dark']) {
    await page.evaluate(theme => document.documentElement.dataset.theme = theme, theme);
    await expect(probe).toHaveCSS('backdrop-filter', 'none');
    await expect(probe).toHaveCSS('background-image', 'none');
    await expect(probe).toHaveCSS('background-color', theme === 'light' ? 'rgb(255, 255, 255)' : 'rgb(24, 24, 25)');
  }
});
