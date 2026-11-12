import { expect, test } from '@playwright/test';
import { newSessionName, visibleSessionName } from '../src/features/chat/chat-presentation';
import { craftFixture } from './helpers/craft-fixture';

test('question and multilingual titles remain valid Windows session filenames', () => {
  for (const prompt of ['So, how are you?', 'CON', 'A/B: "What is this?" <file> | *', '日本語'.repeat(50), '😀'.repeat(50)]) {
    const name = newSessionName(prompt);
    expect(name).not.toMatch(/[<>:"/\\|?*\u0000-\u001f]/);
    expect(new TextEncoder().encode(name).length).toBeLessThanOrEqual(128);
    expect(name).not.toContain('\ufffd');
  }
  expect(visibleSessionName(newSessionName('So, how are you?'))).toBe('So, how are you');
});

for (const installed of [true, false, 'unknown', 'browser'] as const) {
  test(`computer status distinguishes ${installed} component from zero sessions`, async ({ page }) => {
    await craftFixture(page, 'en', 'light');
    if (installed !== 'browser') await page.addInitScript(value => {
      Object.assign(window, { electron: {
        getPaths: async () => ({}),
        getSystemTheme: async () => 'light',
        onThemeChange: () => () => {},
        getComputerStatus: async () => { if (value === 'unknown') throw Error('Unavailable'); return { installed: value, state: 'idle' }; }
      } });
    }, installed);
    await page.goto('/ui/#/settings');
    await page.getByRole('button',{name:'Access & data',exact:true}).click();
    const panel = page.locator('.settings-panel').filter({ has: page.getByRole('heading', { name: 'Computer use', exact: true }) });
    await expect(panel).toContainText(installed === true ? 'Computer access component installed' : installed === false ? 'component is missing' : installed === 'browser' ? 'No computer session connected' : 'Status unavailable');
    await expect(panel).not.toContainText('No platform driver installed');
    await expect(panel.getByRole('button')).toHaveCount(0);
  });
}
