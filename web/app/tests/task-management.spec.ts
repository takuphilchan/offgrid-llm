import {test, expect, type Page} from '@playwright/test';

const done = 'run-' + 'd'.repeat(32), active = 'run-' + 'e'.repeat(32);
async function fixture(page: Page, prompt = 'Finished report') {
  let removed = false;
  const deleted: string[] = [];
  await page.addInitScript(() => {localStorage.setItem('offgrid.locale','en');localStorage.setItem('offgrid.onboarding.complete','true');});
  await page.route(/\/(?:api\/v2|v1)\//, async route => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() === 'DELETE') {
      expect(path).toBe(`/api/v2/jobs/${done}`); deleted.push(path); removed = true;
      return route.fulfill({json:{deleted:true}});
    }
    const tasks = [
      ...(!removed ? [{id:done,prompt,status:'completed',deletable:true,created_at:'2026-09-24T00:00:00Z'}] : []),
      {id:active,prompt:'Current task',status:'running',deletable:false,created_at:'2026-09-24T00:01:00Z'}
    ];
    const data: Record<string,unknown> = {
      '/api/v2/system':{version:'test',api_version:2,workspace_id:'test-workspace',capabilities:['task-first-agents-v2']},
      '/v1/system/config':{require_auth:false}, '/v1/users/me':{user:null,guest:true,authenticated:false},
      '/v1/models':{data:[{id:'model',type:'chat'}]}, '/v1/sessions':{sessions:[]}, '/v1/catalog':{models:[]},
      '/api/v2/jobs':tasks, '/v1/agents/tasks':tasks,
      [`/api/v2/jobs/${done}`]:{run_id:done,prompt,status:'completed',model:'model',output:'Finished.',steps:[],deletable:true},
      '/v1/agents/tools':{tools:[{name:'calculator',description:'Calculate a mathematical expression',enabled:true,source:'builtin',capability:{risk:'low'}}],enabled_count:1},
      '/v1/mcp/servers':{servers:[]}, '/v1/integrations':{integrations:[]},
      '/api/v2/computer/status':{available:false},
      '/v1/stats':{server:{version:'test',uptime:'1m'}},
      '/v1/runs':{runs:tasks.map(task => ({...task,data:{prompt:task.prompt},event_count:1,updated_at:task.created_at,started_at:task.created_at}))},
    };
    return route.fulfill({json:data[path] ?? {}});
  });
  return deleted;
}

test('task deletion is visible beside history and confirmed without affecting active work', async ({page}) => {
  const deleted=await fixture(page);await page.goto('/ui/#/agents/new');
  await expect(page.getByRole('button',{name:'Delete task: Current task',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'Delete task: Finished report',exact:true}).click();
  expect(deleted).toHaveLength(0);
  await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(page.getByRole('button',{name:'Delete task: Finished report',exact:true})).toHaveCount(0);
  await expect(page.getByRole('button',{name:'Delete task: Current task',exact:true})).toBeVisible();
  expect(deleted).toHaveLength(1);
});

for (const layout of [
  {name:'desktop-light', width:1280, theme:'light', locale:'en'},
  {name:'desktop-dark', width:1280, theme:'dark', locale:'en'},
  {name:'mobile', width:390, theme:'light', locale:'en'},
  {name:'rtl', width:1280, theme:'dark', locale:'ar'},
]) {
  test(`history cards have spacing between actions, rows and scrollbar: ${layout.name}`, async ({page},info) => {
    await page.setViewportSize({width:layout.width,height:900});
    await fixture(page, 'Use the Cloudflare Docs MCP tools to find how to connect a custom domain');
    // Apply presentation on every document before the app initializes.
    await page.addInitScript(({theme,locale})=>{
      localStorage.setItem('offgrid.theme',theme);localStorage.setItem('offgrid.locale',locale);
    },layout);
    await page.goto(`/ui/#/agents/task/${done}`);
    await expect(page.locator('.task-list-item[aria-current="true"]')).toBeVisible();
    const geometry = await page.locator('.task-list ul').evaluate(list => {
      const rows = [...list.querySelectorAll('li')];
      const card = rows[0].querySelector('.task-list-item')!.getBoundingClientRect();
      const remove = rows[0].querySelector('.task-delete')!.getBoundingClientRect();
      const first = rows[0].getBoundingClientRect(), next = rows[1].getBoundingClientRect();
      const style = getComputedStyle(list);
      return {
        actionGap: Math.max(remove.left-card.right,card.left-remove.right),
        rowGap: style.display === 'flex' ? Math.max(next.left-first.right,first.left-next.right) : next.top-first.bottom,
        inset:parseFloat(style.paddingInlineEnd),
        overflowsPage:document.documentElement.scrollWidth>innerWidth,
      };
    });
    expect(geometry.actionGap).toBeGreaterThanOrEqual(8);
    expect(geometry.rowGap).toBeGreaterThanOrEqual(12);
    expect(geometry.inset).toBeGreaterThanOrEqual(12);
    expect(geometry.overflowsPage).toBe(false);
    await page.locator('.task-list').screenshot({path:info.outputPath(`history-${layout.name}.png`)});
  });
}

test('Activity clears removable task records through the shared deletion API', async ({page}) => {
  const deleted=await fixture(page);await page.goto('/ui/#/activity');
  await page.getByRole('button',{name:'Clear removable tasks',exact:true}).click();
  await expect(page.getByRole('dialog')).toContainText('Finished report');
  await expect(page.getByRole('dialog')).not.toContainText('Current task');
  await page.getByRole('dialog').getByRole('button',{name:'Confirm delete',exact:true}).click();
  await expect(page.getByRole('button',{name:'Delete task: Finished report',exact:true})).toHaveCount(0);
  await expect(page.getByRole('button',{name:'Delete task: Current task',exact:true})).toBeDisabled();
  expect(deleted).toHaveLength(1);
});

test('management navigation stays at the top and settings remain responsive', async ({page},info) => {
  await fixture(page);await page.goto('/ui/#/agents/new');
  const nav=page.locator('.task-management');
  await expect(nav.getByRole('link',{name:'Work',exact:true})).toHaveAttribute('aria-current','page');
  expect((await nav.boundingBox())!.y).toBeLessThan((await page.getByRole('heading',{name:'What would you like done?'}).boundingBox())!.y);
  await nav.getByRole('link',{name:'Available tools',exact:true}).click();
  await expect(page.getByRole('checkbox',{name:'calculator',exact:true})).toBeVisible();
  await expect(page.locator('.agent-metrics')).toHaveCount(0);
  await page.screenshot({path:info.outputPath('tools.png'),fullPage:true});
  await nav.getByRole('link',{name:'Connections',exact:true}).click();
  await page.setViewportSize({width:390,height:844});
  await expect(page.locator('.connector-panel form')).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({path:info.outputPath('connections-mobile.png'),fullPage:true});
});
