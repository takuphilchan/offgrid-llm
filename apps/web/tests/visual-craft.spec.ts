import { craftFixture } from './helpers/craft-fixture';
import { expect, test } from '@playwright/test';
import { mkdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// Opt-in evidence matrix. All API traffic is intercepted; no installed state,
// model inference, native input or external network is used.
const phase = process.env.OFFGRID_VISUAL_PHASE;
const prototype = phase === 'gradient-prototype' || phase === 'materials-prototype'
  ? readFileSync(resolve(`../../build/ui-craft/${phase}.css`), 'utf8') : '';
test.describe.configure({ mode: 'parallel' });
for(const theme of ['light','dark']) for(const locale of ['en','ar']) for(const width of [390,768,1024,1440]) {
  test(`visual craft ${theme} ${locale} ${width}`,async({page})=>{
    test.skip(!phase,'Set OFFGRID_VISUAL_PHASE=before or after for the evidence matrix.');
    test.setTimeout(120_000);
    await page.setViewportSize({width,height:900});
    const state=await craftFixture(page,locale,theme);
    const dir=resolve('../../build/ui-craft',phase!,`${theme}-${locale}-${width}`); mkdirSync(dir,{recursive:true});
    const capture=async(name:string)=>{
      if (prototype) await page.addStyleTag({content: prototype});
      await page.evaluate(()=>document.fonts.ready);
      await expect(page.locator('.page-failure')).toHaveCount(0);
      await expect.poll(()=>page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
      // Floating UI belongs to the viewport, not an expanded full-page capture.
      // Wait for its entrance before recording evidence of its actual contents.
      const overlay = name === 'popover' ? page.locator('.workspace-options-panel')
        : name === 'palette' ? page.locator('dialog.command-palette')
        : name === 'onboarding' ? page.locator('.onboarding') : null;
      if (overlay) await expect(overlay).toHaveCSS('opacity', '1');
      await page.screenshot({path:resolve(dir,`${name}.png`),fullPage:!overlay,animations:'disabled'});
    };
    for(const route of ['agents','activity','knowledge','models','settings']) {
      await page.goto(`/ui/#/${route}`);
      await expect(page.locator('.page-content')).toBeVisible();
      if(route==='agents') await expect(page.locator('.profile-list-item')).toHaveCount(1);
      if(route==='activity') await expect(page.locator('.task-list-item')).toHaveCount(1);
      if(route==='knowledge') await expect(page.locator('.knowledge-documents article')).toHaveCount(1);
      if(route==='models') await expect(page.locator('.installed-model')).toHaveCount(1);
      await capture(route);
      if(route==='agents') {
        await page.locator('.profile-library-header > button').click(); await expect(page.locator('.profile-editor')).toBeVisible(); await capture('agent-editor');
        await page.goto('/ui/#/agents/tools'); await expect(page.locator('.agent-management-page')).toBeVisible(); await capture('tools');
        await page.goto('/ui/#/agents/connections'); await expect(page.locator('.connector-panel')).toBeVisible(); await capture('connections');
      }
      if(route==='models') { await page.locator('#model-view-discover').click(); await expect(page.locator('.catalog-card')).toHaveCount(1); await capture('models-discover'); }
    }
    for(const mode of ['empty','error','loading']) {
      state.mode=mode; await page.goto('/ui/#/knowledge'); await page.reload();
      await expect(page.locator('.page-content')).toBeVisible();
      await expect(page.locator(mode==='empty'?'.workspace-empty':mode==='error'?'[role="alert"]':'.skeleton-card').first()).toBeVisible();
      await capture(mode);
    }
    state.mode='ready'; await page.goto('/ui/#/chat');
    if(width<1024) await page.locator('.navigation-trigger').click();
    await page.locator('.history-open').first().click();
    await expect(page.locator('.code-block')).toHaveCount(1); await capture('chat-code');
    await page.locator('.composer textarea').fill('Continue the briefing.'); await page.locator('.composer-actions > button').click();
    await expect(page.locator('.streaming-response')).toBeVisible(); await capture('chat-stream');
    // Reload ends only the synthetic stream, never an installed job.
    await page.goto('/ui/#/settings'); await page.reload();
    await page.locator('.command-trigger').click(); await expect(page.locator('dialog.command-palette')).toBeVisible(); await capture('palette');
    await page.keyboard.press('Escape');
    if(width<1024) await page.locator('.navigation-trigger').click();
    await page.locator('.workspace-options > button').click(); await expect(page.locator('.workspace-options-panel')).toBeVisible(); await capture('popover');
    await page.keyboard.press('Escape');
    if(width<1024) await page.keyboard.press('Escape');
    await page.locator('.settings-group .settings-panel button.secondary-button').first().click();
    await expect(page.locator('.onboarding')).toBeVisible(); await capture('onboarding');
    expect(state.writes.every(path=>path.startsWith('/v1/sessions'))).toBe(true);
  });
}
