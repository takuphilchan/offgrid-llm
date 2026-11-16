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
    if (kind === 'menu') await page.locator('.model-select-trigger').click();
    const panel = page.locator(kind === 'setup' ? '.workspace-setup' : kind === 'palette' ? 'dialog.command-palette' : '.model-select-panel');
    await expect(panel).toBeVisible();
    if(kind === 'setup') {
      await expect(page.locator('dialog:modal')).toHaveCount(0);
      await expect(panel).toHaveCSS('opacity','1');
      expect(fixture.writes).toEqual([]);
      return;
    }
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
    const backgroundAlpha = await panel.evaluate(element => {
      const canvas = document.createElement('canvas'); canvas.width=canvas.height=1;
      const context=canvas.getContext('2d')!;
      context.fillStyle=getComputedStyle(element).backgroundColor; context.fillRect(0,0,1,1);
      return context.getImageData(0,0,1,1).data[3];
    });
    expect(backgroundAlpha).toBe(255);
    expect(fixture.writes).toEqual([]);
    await page.screenshot({ path: test.info().outputPath(`${kind}-${theme}.png`), fullPage: true });
  });
}
