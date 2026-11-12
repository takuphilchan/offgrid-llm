import {createRequire} from 'node:module';
const require=createRequire(new URL('../../apps/web/package.json',import.meta.url));
const {expect}=require('@playwright/test');

// These isolated fixtures start with an empty profile. Wait for workspace
// binding before dismissing setup, and verify that dismissal survives restart.
export async function dismissWorkspaceSetup(page,workspace) {
  const identity=JSON.stringify(['local',workspace]);
  await expect.poll(()=>page.evaluate(()=>localStorage.getItem('offgrid.onboarding.legacyOwner'))).toBe(identity);
  const key=`offgrid.onboarding.workspace:${identity}`;
  const state=await page.evaluate(key=>localStorage.getItem(key),key);
  const dialog=page.locator('dialog.onboarding');
  if(state!=='working'&&state!=='complete') {
    await expect(dialog).toBeVisible();
    await dialog.locator('.onboarding-close').click();
    await expect.poll(()=>page.evaluate(key=>localStorage.getItem(key),key)).toBe('working');
  }
  await expect(dialog).toHaveCount(0);
}
