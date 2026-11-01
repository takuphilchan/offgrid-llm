import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { agentProfilesText } from '../src/i18n/agent-profiles';

for (const locale of ['en', 'ar'] as const) for (const width of [320, 768, 1280]) test(`composer preserves pinned model and explicit controls: ${locale} ${width}`, async ({ page }, info) => {
  const state = await craftFixture(page, locale, 'dark');
  const model = 'Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589';
  const profile = { id: 'writer', revision: 7, settings: { name: 'Writer', model, tools: [], knowledge: [], computer: false } };
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', api_version: 2, workspace_id: 'craft', capabilities: ['agent-profiles-v1', 'agent-conversations-v1'] } }));
  await page.route('**/api/v2/agent-profiles', r => r.fulfill({ json: { profiles: [profile] } }));
  await page.setViewportSize({ width, height: 720 });
  await page.goto('/ui/#/chat');
  const editor = page.locator('.composer textarea');
  await editor.fill('Keep this exact draft — café 日本語');
  await page.getByRole('button', { name: agentProfilesText[locale].selector, exact: true }).click();
  await page.getByRole('radio', { name: /Writer/ }).check(); await page.keyboard.press('Escape');
  const trigger = page.locator('.pinned-model-trigger');
  await expect(trigger).toContainText('Meta Llama 3.1 8B Instruct');
  await trigger.click();
  await expect(page.locator('.chat-agent-model code')).toHaveText(model);
  await expect(page.locator('.chat-agent-model strong')).toContainText(agentProfilesText[locale].revision.replace('{revision}', '7'));
  await expect(page.locator('.chat-agent-model input, .chat-agent-model select')).toHaveCount(0);
  await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
  await expect(editor).toHaveValue('Keep this exact draft — café 日本語');
  await expect(page.locator('.composer-context .chat-execution-workspace')).toBeVisible();
  await expect(page.locator('.composer-context .chat-agent-tools')).toBeVisible();
  await expect(page.locator('.composer-footer .voice-settings > button')).toHaveCount(0);
  const preferences=page.locator('.composer-context-menu > button');await preferences.click();
  await expect(page.locator('.voice-settings-inline')).toBeVisible();
  await page.keyboard.press('Escape');await expect(preferences).toBeFocused();
  await expect(page.locator('.composer-context .composer-context-menu')).toHaveCount(0);
  await expect(page.locator('.composer-footer .composer-utilities .composer-context-menu > button')).toBeVisible();
  const identity=await page.locator('.composer-context').boundingBox(),utilities=await page.locator('.composer-utilities').boundingBox(),actions=await page.locator('.composer-actions').boundingBox();
  expect(utilities!.y).toBeGreaterThanOrEqual(identity!.y+identity!.height);
  expect(actions!.y).toBeGreaterThanOrEqual(identity!.y+identity!.height);
  await expect(page.locator('.composer-actions > button:last-child')).toBeEnabled();
  const problems = await page.locator('.composer').evaluate(composer => {
    const boundary = composer.getBoundingClientRect();
    const controls = [...composer.querySelectorAll<HTMLElement>('button')].filter(node => node.getClientRects().length).map(node => node.getBoundingClientRect());
    return controls.flatMap((box, index) => [
      ...(box.left < boundary.left - 1 || box.right > boundary.right + 1 ? ['outside'] : []),
      ...controls.slice(index + 1).filter(other => Math.min(box.right, other.right) - Math.max(box.left, other.left) > 1 && Math.min(box.bottom, other.bottom) - Math.max(box.top, other.top) > 1).map(() => 'overlap'),
    ]);
  });
  expect(problems).toEqual([]); expect(state.writes).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  await page.screenshot({ path: info.outputPath(`composer-${locale}-${width}.png`), fullPage: true });
});
