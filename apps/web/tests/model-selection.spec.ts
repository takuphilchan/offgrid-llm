import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const locale of ['en', 'ar']) for (const width of [320, 390, 768, 1440]) test(`long model trigger stays inside composer without overlapping controls: ${locale} ${width}`, async ({ page }) => {
  const state = await craftFixture(page, locale, 'light');
  await page.setViewportSize({ width, height: 900 });
  await page.goto('/ui/#/chat');
  const trigger = page.locator('.model-select-trigger');
  await expect(trigger).toBeVisible();
  const errors = await page.locator('.composer').evaluate(composer => {
    const box = composer.getBoundingClientRect();
    const controls = [...composer.querySelectorAll<HTMLButtonElement>('.composer-context > .utility-popover > button, .model-select-trigger, .composer-footer button')].filter(node => node.getClientRects().length);
    const problems: string[] = [];
    for (const [index, control] of controls.entries()) {
      const rect = control.getBoundingClientRect();
      if (rect.left < box.left - 1 || rect.right > box.right + 1) problems.push(`outside: ${control.getAttribute('aria-label') ?? control.textContent}`);
      for (const other of controls.slice(index + 1)) {
        const next = other.getBoundingClientRect();
        if (Math.min(rect.right, next.right) - Math.max(rect.left, next.left) > 1 && Math.min(rect.bottom, next.bottom) - Math.max(rect.top, next.top) > 1) problems.push(`overlap: ${control.textContent} / ${other.textContent}`);
      }
    }
    return problems;
  });
  expect(errors).toEqual([]);
  await trigger.click();
  await expect(page.locator('.model-select-panel')).toBeVisible();
  expect(state.writes).toEqual([]);
});

test('model picker searches exact identifiers, shows actual readiness and preserves selection authority', async ({ page }) => {
  const state = await craftFixture(page, 'en', 'light');
  const models = [
    { id: 'Qwen2.5-7B-Instruct-Q4_K_M', type: 'chat', loaded: true, context_window: 32768 },
    { id: 'Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589', type: 'chat', runtime_ready: false },
    { id: 'voice-only', type: 'tts' },
  ];
  await page.route('**/v1/models', r => r.fulfill({ json: { data: models } }));
  await page.goto('/ui/#/chat');
  const picker = page.getByRole('button', { name: 'Model', exact: true });
  await picker.click();
  await expect(page.getByRole('searchbox', { name: 'Filter installed models' })).toBeFocused();
  await expect(page.getByRole('option')).toHaveCount(2);
  await expect(page.getByRole('option').first()).toContainText('Loaded');
  await expect(page.getByRole('option').last()).toContainText('Runtime ready: No');
  await page.getByRole('searchbox', { name: 'Filter installed models' }).fill('f1e3673a9589');
  await expect(page.getByRole('option')).toHaveCount(1);
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('option')).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(picker).toContainText('Meta Llama 3.1 8B Instruct');
  await expect(picker).toBeFocused();
  await expect(picker).toHaveAttribute('aria-expanded', 'false');
  expect(state.writes).toEqual([]);
  await picker.click();
  await expect(page.getByRole('option', { selected: true })).toContainText(models[1].id);
  await page.keyboard.press('Escape');
  await expect(picker).toBeFocused();
});

for (const locale of ['en', 'ar']) test(`model picker fits narrow ${locale} and distinguishes same-family variants`, async ({ page }, info) => {
  const state = await craftFixture(page, locale, 'dark');
  await page.route('**/v1/models', r => r.fulfill({ json: { data: ['Q4_K_M', 'Q8_0'].map(quant => ({ id: `Qwen2.5-7B-Instruct-${quant}`, type: 'chat' })) } }));
  await page.setViewportSize({ width: 390, height: 640 });
  await page.goto('/ui/#/chat');
  await page.locator('.model-select-trigger').click();
  await expect(page.getByRole('option')).toHaveCount(2);
  await expect(page.getByRole('option').first()).toContainText('Q4_K_M');
  await expect(page.getByRole('option').last()).toContainText('Q8_0');
  await expect(page.getByRole('option').first().locator('strong')).toBeVisible();
  await expect(page.locator('.model-select-trigger > span')).toBeVisible();
  expect(await page.getByRole('option').first().evaluate(node => getComputedStyle(node).display)).toBe('grid');
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath(`model-picker-${locale}.png`), fullPage: true, animations: 'disabled' });
  expect(state.writes).toEqual([]);
});
