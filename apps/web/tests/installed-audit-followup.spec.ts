import {test, expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

test('empty library distinguishes conversation attachments without a disabled question action',async({page})=>{
 const state=await craftFixture(page,'en','light');state.mode='empty';
 await page.goto('/ui/#/knowledge');
 await expect(page.getByText('Files attached in Chat stay with that conversation', {exact:false})).toBeVisible();
 await expect(page.getByRole('button',{name:'Ask using Knowledge',exact:true})).toHaveCount(0);
 expect(state.writes).toEqual([]);
});

for (const theme of ['light','dark']) test(`model choice and cancelled transfer history stay distinct: ${theme}`,async({page})=>{
 const state=await craftFixture(page,'en',theme);
 await page.route('**/v1/models/download/progress',r=>r.fulfill({json:{'old.gguf':{file_name:'old.gguf',status:'cancelled',percent:3.6,bytes_done:36,bytes_total:1000,speed:0,started_at:1}}}));
 await page.goto('/ui/#/models');
 await expect(page.locator('.installed-model').getByRole('button',{name:'Selected',exact:true})).toHaveAttribute('aria-pressed','true');
 const transfers=page.locator('.model-transfers');
 await expect(transfers).not.toHaveAttribute('open','');
 await expect(transfers.locator('.download-state')).not.toBeVisible();
 await transfers.locator('summary').click();
 await expect(transfers.locator('.download-state')).toBeVisible();
 expect(state.writes).toEqual([]);
 await page.screenshot({path:test.info().outputPath(`models-${theme}.png`)});
});

test('attention opens before history, filters preserve work and header uses the same destination',async({page})=>{
 const state=await craftFixture(page,'en','dark');
 const jobs=[['a','Completed answer','completed'],['b','Approve the draft','waiting_for_approval'],['c','Research in progress','running']].map(([letter,prompt,status])=>({id:'run-'+letter.repeat(32),prompt,status,created_at:'2026-11-05T00:00:00Z',deletable:status==='completed'}));
 await page.route('**/api/v2/jobs',r=>r.fulfill({json:jobs}));
 await page.goto('/ui/#/activity');
 const filters=page.locator('.work-state-filters');
 await expect(filters.getByRole('button',{name:'Needs attention 1',exact:true})).toHaveAttribute('aria-pressed','true');
 await expect(page.locator('.task-list')).toContainText('Approve the draft');
 await expect(page.locator('.task-list')).not.toContainText('Completed answer');
 await filters.getByRole('button',{name:'History 1',exact:true}).click();
 await expect(page.locator('.task-list')).toContainText('Completed answer');
 await expect(page.locator('.task-list')).not.toContainText('Approve the draft');
 await page.reload();
 await expect(filters.getByRole('button',{name:'History 1',exact:true})).toHaveAttribute('aria-pressed','true');
 await page.getByRole('button',{name:'Known work',exact:true}).click();
 const panel=page.getByRole('region',{name:'Known work'});
 await panel.getByRole('link',{name:'Activity',exact:true}).click();
 await expect(page).toHaveURL(/#\/activity\/attention$/);
 await expect(filters.getByRole('button',{name:'Needs attention 1',exact:true})).toHaveAttribute('aria-pressed','true');
 await expect(panel).not.toBeVisible();
 await page.setViewportSize({width:390,height:844});
 await expect(filters.getByRole('button',{name:'All work 3',exact:true})).toBeVisible();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth <= innerWidth+1)).toBe(true);
 expect(state.writes).toEqual([]);
 await page.screenshot({path:test.info().outputPath('activity-narrow.png')});
});
