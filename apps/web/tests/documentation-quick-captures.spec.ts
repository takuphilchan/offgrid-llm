import { expect, test } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { craftFixture } from './helpers/craft-fixture';

// Capture the real compact renderer with an isolated desktop bridge fixture.
// No global shortcut registration, native app actions or model inference.
test('capture documentation quick access input and reply', async ({ page }) => {
  test.skip(!process.env.OFFGRID_DOC_CAPTURES, 'Set OFFGRID_DOC_CAPTURES=1 to refresh documentation images.');
  await craftFixture(page, 'en', 'light');
  await page.addInitScript(() => {
    let state = { mode: 'compact', shortcut: 'CommandOrControl+Shift+Space', registered: true };
    const listeners = new Set<(value: typeof state) => void>();
    const change = async (mode: string) => { state = { ...state, mode }; listeners.forEach(callback => callback(state)); return state; };
    Object.assign(window, { electron: {
      isDesktop: true, platform: 'win32', commandWindowProtocol: 1,
      getSystemTheme: async () => 'light', onThemeChange: () => () => {},
      getCommandWindow: async () => state,
      onCommandWindow: (callback: (value: typeof state) => void) => { listeners.add(callback); return () => { listeners.delete(callback); }; },
      openCommandWindow: () => change('compact'), expandCommandWindow: () => change('full'),
      setCommandLayout: async (layout: string) => { (window as any).documentationLayout = layout; return state; },
    } });
  });
  const model = 'Qwen2.5-7B-Instruct-Q4_K_M';
  const session = { name: 'Project launch checklist', model_id: model,
    updated_at: '2026-11-16T09:00:00Z', messages: [
      { role: 'user', content: 'Give me a short checklist for sharing my local project.' },
      { role: 'assistant', content: '**Before you share it**\n\n1. Run the checks and review any failures.\n2. Remove credentials and personal sample data.\n3. Include setup instructions and a working example.\n\nKeep a copy of the version you tested.' },
    ] };
  await page.route('**/v1/models', r => r.fulfill({ json: { data: [{ id: model, type: 'chat' }] } }));
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [session] } }));
  await page.route('**/v1/sessions/Project%20launch%20checklist', r => r.fulfill({ json: session }));
  await page.setViewportSize({ width: 720, height: 164 });
  await page.goto('/ui/#/chat/new');
  await expect(page.locator('.app-shell')).toHaveClass(/command-window/);
  const editor = page.locator('.composer textarea');
  await editor.fill('Help me prepare this project for launch.');
  await expect(editor).toBeInViewport();
  await expect(page.locator('.composer-actions .model-select-trigger')).toBeInViewport();
  await page.evaluate(() => document.fonts.ready);
  const output = resolve('../../build/documentation-captures');
  mkdirSync(output, { recursive: true });
  await page.screenshot({ path: resolve(output, 'quick-access-input.png'), animations: 'disabled' });
  await page.evaluate(() => window.electron!.expandCommandWindow!());
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.locator('.history-open').filter({ hasText: session.name }).click();
  await expect(page.locator('.message.assistant')).toContainText('Before you share it');
  await page.evaluate(() => window.electron!.openCommandWindow!());
  await page.setViewportSize({ width: 720, height: 540 });
  await expect(page.locator('#workspace-sidebar')).toBeHidden();
  await expect.poll(() => page.evaluate(() => (window as any).documentationLayout)).toBe('work');
  await page.locator('.conversation').hover();
  await page.mouse.wheel(0, -1000);
  await expect.poll(() => page.locator('.conversation').evaluate(el => el.scrollTop)).toBe(0);
  await page.screenshot({ path: resolve(output, 'quick-access-reply.png'), animations: 'disabled' });
});
