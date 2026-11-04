import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

test('browser workspace has no terminal control', async ({ page }) => {
  await craftFixture(page, 'en', 'light');
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('button', { name: 'Open terminal' })).toHaveCount(0);
});

for (const available of [false, true]) {
  test(`desktop terminal entry is ${available ? 'available' : 'absent'} without starting a shell on mount`, async ({ page }) => {
    await craftFixture(page, 'en', 'light');
    await page.addInitScript(value => {
      (window as any).terminalOpens = 0;
      (window as any).electron = {
        isDesktop: true,
        terminalProtocol: 1,
        getApiUrl: async () => location.origin,
        getSystemTheme: async () => 'light',
        onThemeChange: () => () => {},
        getTerminalStatus: async () => ({ available: value }),
        openTerminal: async () => { (window as any).terminalOpens++; return { visible: true }; },
      };
    }, available);
    await page.goto('/ui/#/chat');
    const entry = page.getByRole('button', { name: 'Open terminal' });
    await expect(entry).toHaveCount(available ? 1 : 0);
    expect(await page.evaluate(() => (window as any).terminalOpens)).toBe(0);
    if (available) {
      await entry.click();
      await expect.poll(() => page.evaluate(() => (window as any).terminalOpens)).toBe(1);
    }
  });
}
