import {expect,test} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

for(const theme of ['light','dark']) test(`inactive uncertain work archives and restores without execution in ${theme}`,async({page})=>{
  await craftFixture(page,'en',theme);
  const calls:string[]=[];
  let archived=false;
  const id='run-'+ 'a'.repeat(32);
  await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'craft',capabilities:['task-first-agents-v2','work-summary-v1']}}));
  await page.route('**/api/v2/work?*',r=>{
    const bucket=new URL(r.request().url()).searchParams.get('bucket');
    const visible=archived?bucket==='archived':bucket!=='archived'&&bucket!=='history'&&bucket!=='running';
    return r.fulfill({json:{items:visible?[{id,title:'Old disposable test',status:'uncertain',created_at:'2026-10-26T00:00:00Z',deletable:false,archivable:true,archived_at:archived?'2026-11-06T00:00:00Z':undefined,protection:'unknown_effects'}]:[],counts:{all:archived?0:1,attention:archived?0:1,running:0,history:0,archived:archived?1:0},removable_count:0,has_more:false,scope:'actor_jobs'}});
  });
  await page.route(`**/api/v2/jobs/${id}/*`,r=>{
    const action=r.request().url().split('/').at(-1)!;
    calls.push(`${r.request().method()} ${action}`);
    if(!['archive','restore'].includes(action))return r.fulfill({status:500,json:{error:{message:'Unexpected execution'}}});
    archived=action==='archive';
    return r.fulfill({status:202,json:{id,status:'uncertain',prompt:'Old disposable test',created_at:'2026-10-26T00:00:00Z',archived_at:archived?'2026-11-06T00:00:00Z':null}});
  });
  await page.goto('/ui/#/activity/attention');
  await expect(page.locator('.task-history-row')).toHaveCount(1);
  await page.locator('.task-history-row .history-item-menu').click();
  await page.getByRole('button',{name:'Archive from Activity',exact:true}).click();
  const dialog=page.getByRole('dialog');
  await expect(dialog).toContainText('Old disposable test');
  await dialog.getByRole('button',{name:'Archive from Activity',exact:true}).click();
  await expect(page.locator('.task-history-row')).toHaveCount(0);
  await expect(page.getByRole('button',{name:'Needs attention 0',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Archived 1',exact:true}).click();
  await expect(page.locator('.task-history-row')).toContainText('Outcome unknown');
  await page.screenshot({path:`../../build/activity-archived-${theme}-20261106.png`});
  await page.reload();
  await expect(page.locator('.task-history-row')).toHaveCount(1);
  await page.locator('.task-history-row .history-item-menu').click();
  await page.getByRole('button',{name:'Restore to Activity',exact:true}).click();
  await dialog.getByRole('button',{name:'Restore to Activity',exact:true}).click();
  await expect(page.locator('.task-history-row')).toHaveCount(0);
  await page.getByRole('button',{name:'Needs attention 1',exact:true}).click();
  await expect(page.locator('.task-history-row')).toContainText('Outcome unknown');
  expect(calls).toEqual(['POST archive','POST restore']);
});

for (const width of [390, 1440]) for (const theme of ['light', 'dark']) test(`archive-only Activity has a truthful, compact empty view at ${width} ${theme}`, async ({page}) => {
  const state=await craftFixture(page,'en',theme);
  await page.setViewportSize({width,height:900});
  await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',api_version:2,workspace_id:'craft',capabilities:['task-first-agents-v2','work-summary-v1']}}));
  await page.route('**/api/v2/work?*',r=>{
    const url=new URL(r.request().url());
    return r.fulfill({json:{items:url.searchParams.get('bucket')==='archived'&&!url.searchParams.get('query')?[{id:'run-'+'c'.repeat(32),title:'Saved test evidence',status:'uncertain',created_at:'2026-10-26T00:00:00Z',archived_at:'2026-11-06T00:00:00Z',deletable:false,archivable:true}]:[],counts:{all:0,attention:0,running:0,history:0,archived:7},removable_count:0,has_more:false,scope:'actor_jobs'}});
  });
  await page.goto('/ui/#/activity/attention');
  const history=page.locator('.work-history');
  await expect(history.getByText('No work needs attention',{exact:true})).toBeVisible();
  await expect(history.getByText('No agent tasks have run yet.',{exact:true})).toHaveCount(0);
  await expect(history.getByText('Earlier work is available in Archived.')).toBeVisible();
  const filters=history.locator('.work-state-filters');
  await expect(filters.locator('[aria-pressed=true]')).toHaveCount(1);
  await expect(filters.getByRole('button',{name:'Running 0',exact:true})).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
  const tabs=page.locator('.activity-page > .model-library-tabs');
  await expect(tabs.getByRole('button',{name:'Work',exact:true})).toHaveCSS('border-bottom-width','2px');
  await expect(tabs.getByRole('button',{name:'Work',exact:true})).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
  await expect(history.getByRole('button',{name:'Review removable history'})).toBeHidden();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:test.info().outputPath('activity-empty.png'),fullPage:true});
  await history.locator('.work-history-empty').getByRole('link',{name:'Archived 7',exact:true}).click();
  await expect(history.locator('.task-history-row')).toContainText('Saved test evidence');
  await filters.getByRole('button',{name:'Running 0',exact:true}).click();
  await expect(history.getByText('Nothing is running',{exact:true})).toBeVisible();
  expect(state.writes).toEqual([]);
});
