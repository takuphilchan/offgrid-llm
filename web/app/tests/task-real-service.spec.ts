import {test, expect} from '@playwright/test';

// Explicit opt-in against isolated data; never run this on a user's workspace.
test.skip(process.env.OFFGRID_TASK_REAL_SERVICE !== '1', 'Needs isolated packaged service and completed test-task-jobs.mjs fixtures');
test('packaged UI opens real durable results, child jobs and verified downloads', async ({page},testInfo)=>{
  const base=new URL(process.env.OFFGRID_E2E_URL!);
  expect(base.hostname).toBe('127.0.0.1');expect(base.port).toBe('11613');
  await page.addInitScript(()=>{localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.onboarding.complete','true');});
  const tasks=await (await page.request.get('/api/v2/jobs')).json();
  const completed=tasks.filter((task:any)=>task.status==='completed'&&!task.parent_id);
  let artifactRun:any,graphRun:any;
  for(const task of completed){const run=await(await page.request.get(`/api/v2/jobs/${task.id}`)).json();if(run.artifacts?.length)artifactRun=run;if(run.children?.length)graphRun=run;}
  expect(artifactRun).toBeTruthy();expect(graphRun).toBeTruthy();
  await page.goto(`/ui/#/agents/task/${artifactRun.run_id}`);
  const link=page.getByRole('link',{name:'comparison.csv',exact:true});await expect(link).toBeVisible();
  const downloading=page.waitForEvent('download');await link.click();const download=await downloading;
  expect(download.suggestedFilename()).toBe('comparison.csv');
  expect((await page.request.get(await link.getAttribute('href')!)).status()).toBe(200);
  await page.screenshot({path:testInfo.outputPath('real-artifact-desktop.png'),fullPage:true});
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.screenshot({path:testInfo.outputPath('real-artifact-mobile.png'),fullPage:true});
  await page.goto(`/ui/#/agents/task/${graphRun.run_id}`);
  await expect(page.getByRole('heading',{name:'Subtasks',exact:true})).toBeVisible();
  const child=page.locator('.task-children a').first();const href=await child.getAttribute('href');await child.click();
  await expect(page).toHaveURL(url => url.hash === href);
  await expect(page.getByRole('link',{name:'Work plan',exact:true})).toHaveAttribute('href',`#/agents/task/${graphRun.run_id}`);
});
