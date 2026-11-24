import { test, expect } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const [locale, theme] of [['en', 'light'], ['ar', 'dark']]) test(`draft height follows window reflow without editing or submitting: ${locale}`, async ({ page }, info) => {
 const fixture = await craftFixture(page, locale, theme);
 await page.setViewportSize({ width: 1440, height: 900 });
 await page.goto('/ui/#/chat/new');
 const editor = page.locator('.composer textarea');
 const draft = 'Prepare a concise plan for the next workshop, with time for questions and a short summary of the decisions we need to make.';
 await editor.fill(draft);
 const wide = (await editor.boundingBox())!.height;
 await page.setViewportSize({ width: 390, height: 760 });
 await expect.poll(async () => (await editor.boundingBox())!.height).toBeGreaterThan(wide + 16);
 await expect(editor).toHaveValue(draft);
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
 await page.screenshot({ path: info.outputPath(`composer-reflow-${locale}.png`), fullPage: true });
 await page.setViewportSize({ width: 1440, height: 900 });
 await expect.poll(async () => (await editor.boundingBox())!.height).toBeLessThanOrEqual(wide + 1);
 await editor.fill(Array.from({ length: 30 }, (_, i) => `Line ${i + 1}`).join('\n'));
 await expect.poll(async () => (await editor.boundingBox())!.height).toBeLessThanOrEqual(160);
 await expect.poll(() => editor.evaluate(el => el.scrollHeight > el.clientHeight)).toBe(true);
 await editor.fill('');
 await expect.poll(async () => (await editor.boundingBox())!.height).toBeLessThanOrEqual(64);
 expect(fixture.writes).toEqual([]);
});

test('older browsers retain bounded content sizing through the existing fallback', async ({ page }) => {
 const fixture = await craftFixture(page, 'en', 'light');
 await page.addInitScript(() => {
  const supports = CSS.supports.bind(CSS);
  CSS.supports = ((property: string, value?: string) => property === 'field-sizing' ? false : value === undefined ? supports(property) : supports(property, value)) as typeof CSS.supports;
 });
 await page.goto('/ui/#/chat/new');
 await page.addStyleTag({ content: '.composer textarea { field-sizing: fixed !important; }' });
 const editor = page.locator('.composer textarea');
 await editor.fill(Array.from({ length: 30 }, (_, i) => `Line ${i + 1}`).join('\n'));
 await expect.poll(async () => (await editor.boundingBox())!.height).toBe(160);
 await expect.poll(() => editor.evaluate(el => el.scrollHeight > el.clientHeight)).toBe(true);
 await editor.fill('Short draft');
 await expect.poll(async () => (await editor.boundingBox())!.height).toBeLessThanOrEqual(64);
 await expect(editor).toHaveValue('Short draft');
 await page.setViewportSize({ width: 1440, height: 900 });
 await editor.fill('Prepare a concise plan for the next workshop, with time for questions and a short summary of the decisions we need to make.');
 const wide = (await editor.boundingBox())!.height;
 await page.setViewportSize({ width: 390, height: 760 });
 await expect.poll(async () => (await editor.boundingBox())!.height).toBeGreaterThan(wide + 16);
 expect(fixture.writes).toEqual([]);
});
