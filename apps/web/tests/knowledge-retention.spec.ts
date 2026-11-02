import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { documentRetention } from '../src/i18n/document-retention';
import { workspaceManagement } from '../src/i18n/workspace-management';

for (const locale of ['en', 'fr', 'de', 'es', 'ar', 'sw', 'sn', 'nd', 'zu'] as const) {
  test(`retention needs an explicit save and reload after failure in ${locale}`, async ({ page }) => {
    await craftFixture(page, locale, 'light');
    await page.route('**/api/v2/system', route => route.fulfill({json:{product:'offgrid',version:'fixture',workspace_id:'craft',api_version:2,capabilities:['task-first-agents-v2','agent-profiles-v1','knowledge-retention-v1']}}));
    let reads=0, writes=0;
    await page.route('**/v1/documents/retention?**', route => {
      if(route.request().method()==='GET'){reads++;return route.fulfill({json:{version:1,revision:reads,delete_after:0,legal_hold:false}});}
      writes++;
      expect(route.request().postDataJSON()).toEqual({version:1,revision:reads,delete_after:0,legal_hold:true});
      return route.fulfill({status:409,json:{error:{message:'Policy changed. Reload before saving.'}}});
    });
    await page.setViewportSize({width:390,height:844});
    await page.goto('/ui/#/knowledge');
    await page.getByRole('button',{name:workspaceManagement(locale).manageDocument,exact:false}).click();
    const copy=documentRetention(locale);
    await page.getByRole('button',{name:copy.title,exact:true}).click();
    const dialog=page.getByRole('dialog',{name:copy.title});
    await expect(dialog.getByRole('checkbox',{name:copy.keep,exact:true})).toBeChecked();
    expect(writes).toBe(0);
    const initialReads=reads;
    await dialog.getByRole('checkbox',{name:copy.hold,exact:true}).check();
    await dialog.getByRole('button',{name:copy.save,exact:true}).click();
    await expect(dialog.getByRole('alert')).toContainText('Policy changed');
    await expect(dialog.getByRole('button',{name:copy.save,exact:true})).toBeDisabled();
    expect(writes).toBe(1);
    await dialog.getByRole('button',{name:copy.reload,exact:true}).click();
    await expect(dialog.getByRole('button',{name:copy.save,exact:true})).toBeEnabled();
    await expect(dialog.getByRole('checkbox',{name:copy.hold,exact:true})).not.toBeChecked();
    expect(reads).toBe(initialReads+1);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
    await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);
  });
}
