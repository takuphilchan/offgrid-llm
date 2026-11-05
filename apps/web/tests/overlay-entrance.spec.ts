import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';

for (const theme of ['light', 'dark']) for (const kind of ['setup', 'palette', 'menu']) {
  test(`overlay entrance keeps text opaque throughout: ${kind} ${theme}`, async ({ page }) => {
    const fixture = await craftFixture(page, 'en', theme);
    if (kind === 'setup') {
      await page.addInitScript(() => localStorage.removeItem('offgrid.onboarding.complete'));
      await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [] } }));
    }
    await page.goto('/ui/#/chat');
    if (kind === 'palette') await page.locator('.command-trigger').click();
    if (kind === 'menu') await page.getByRole('button', { name: 'Workspace options', exact: true }).click();
    const panel = page.locator(kind === 'setup' ? 'dialog.onboarding' : kind === 'palette' ? 'dialog.command-palette' : '.workspace-options-panel');
    await expect(panel).toBeVisible();
    const frames = await panel.evaluate(element => {
      const animation = element.getAnimations().find(item => item instanceof CSSAnimation);
      if (!animation) throw Error('Expected the actual overlay entrance animation');
      animation.pause();
      const duration = Number(animation.effect!.getComputedTiming().duration);
      return [0, .5, 1].map(fraction => {
        animation.currentTime = duration * fraction;
        return getComputedStyle(element).opacity;
      });
    });
    expect(frames).toEqual(['1', '1', '1']);
    expect(fixture.writes).toEqual([]);
    await page.screenshot({ path: test.info().outputPath(`${kind}-${theme}.png`), fullPage: true });
  });
}
