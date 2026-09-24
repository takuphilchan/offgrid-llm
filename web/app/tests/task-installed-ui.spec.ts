import {test, expect} from '@playwright/test';

// Read-only post-deployment check. Never submit or alter work in a live workspace.
test('installed task workspace loads existing history without legacy setup clutter', async ({page}, testInfo) => {
  test.skip(process.env.OFFGRID_TASK_INSTALLED_UI !== '1', 'Explicit deployment check only');
  const base = new URL(process.env.OFFGRID_E2E_URL!);
  expect(base.hostname).toBe('127.0.0.1');
  expect(base.port).toBe('11611');
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  const response = await page.request.get('/api/v2/jobs');
  expect(response.ok()).toBe(true);
  const jobs = await response.json();
  const inspectable = jobs.find((job: {status: string}) => job.status === 'completed') ?? jobs[0];
  await page.goto('/ui/#/agents/new');
  await expect(page.getByRole('heading', {name: 'What would you like done?'})).toBeVisible();
  await expect(page.getByRole('button', {name: 'Start task', exact: true})).toBeDisabled();
  await expect(page.getByText('Pair browser', {exact: true})).toHaveCount(0);
  await expect(page.locator('.task-list ul')).toHaveCSS('row-gap', '12px');
  await expect(page.locator('.task-list ul')).toHaveCSS('padding-inline-end', '12px');
  if (jobs.length) await expect(page.locator('.task-list li').first()).toHaveCSS('column-gap', '8px');
  await page.screenshot({path: testInfo.outputPath('installed-task-desktop.png'), fullPage: true});
  await page.setViewportSize({width: 390, height: 844});
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({path: testInfo.outputPath('installed-task-mobile.png'), fullPage: true});
  // Live history may legitimately have been cleared by its owner. Do not create
  // a synthetic task simply to satisfy a deployment check.
  if (inspectable) {
    await page.goto(`/ui/#/agents/task/${inspectable.id}`);
    await expect(page.getByRole('heading', {name: inspectable.prompt, exact: true})).toBeVisible();
    await expect(page.locator('.task-state')).toBeVisible();
  }
  expect(errors).toEqual([]);
});

test('installed history controls and management panels work without mutating live data', async ({page}, testInfo) => {
  test.skip(process.env.OFFGRID_TASK_INSTALLED_UI !== '1', 'Explicit deployment check only');
  const base = new URL(process.env.OFFGRID_E2E_URL!);
  expect(base.origin).toBe('http://127.0.0.1:11611');
  const mutations: string[] = [], errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.route(/\/(?:api\/v2|v1)\//, async route => {
    if (!['GET', 'HEAD'].includes(route.request().method())) {
      mutations.push(route.request().method());
      return route.abort(); // A live smoke test must never alter user history.
    }
    return route.continue();
  });
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  const jobs = await (await page.request.get('/api/v2/jobs')).json();
  const activity = await (await page.request.get('/v1/runs')).json();
  expect(activity.runs.some((run: {id: string}) => run.id === 'computer-system')).toBe(false);
  for (const run of activity.runs) {
    const job = jobs.find((job: {id: string}) => job.id === run.id);
    if (job) expect(run.deletable).toBe(job.deletable);
  }
  await page.goto('/ui/#/agents/new');
  const removable = page.locator('.task-list .task-delete:not([disabled])').first();
  await expect(page.getByRole('heading', {name:'What would you like done?'})).toBeVisible();
  if (await removable.count()) {
    await removable.click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.getByRole('dialog').getByRole('button', {name:'Cancel', exact:true}).click();
  }
  await page.getByRole('link', {name:'Available tools', exact:true}).click();
  await expect(page.locator('.tool-list article').first()).toBeVisible();
  await page.screenshot({path:testInfo.outputPath('installed-tools.png'), fullPage:true});
  await page.getByRole('link', {name:'Connections', exact:true}).click();
  await expect(page.locator('.connector-panel form')).toBeVisible();
  const mcp = await (await page.request.get('/v1/agents/mcp')).json();
  await expect(page.locator('.connector-remove')).toHaveCount(mcp.servers.length);
  if (mcp.servers.length) {
    await page.locator('.connector-remove').first().click();
    await expect(page.getByRole('dialog')).toContainText('Task history is kept');
    await page.getByRole('dialog').getByRole('button', {name:'Cancel', exact:true}).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(page.locator('.connector-remove')).toHaveCount(mcp.servers.length);
  }
  await page.screenshot({path:testInfo.outputPath('installed-connections.png'), fullPage:true});
  await page.goto('/ui/#/activity');
  const clear = page.getByRole('button', {name:'Clear removable tasks', exact:true});
  await expect(clear).toBeVisible();
  if (activity.runs.some((run: {deletable: boolean}) => run.deletable)) {
    await expect(clear).toBeEnabled();
    await clear.click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.getByRole('dialog').getByRole('button', {name:'Cancel', exact:true}).click();
  } else {
    await expect(clear).toBeDisabled();
  }
  await page.screenshot({path:testInfo.outputPath('installed-activity.png'), fullPage:true});
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(mutations).toEqual([]);
  expect(errors).toEqual([]);
});
