import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const started = new Date().toISOString();
  let preview = 'First live response';
  let status = 'running';
  let output = '';
  let steps: { id: string; type: string; content: string }[] = [];
  let approval: object | null = null;
  const snapshot = () => ({ run_id: 'run-cccccccccccccccccccccccccccccccc', status, output, steps, pending_approval: approval,
    started_at: started, resumable: false, progress: { phase: 'generating', iteration: 1, preview, updated_at: started } });
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2, capabilities: ['task-first-agents-v2'] } }));
  await page.route(/\/(?:v1\/|api\/v2\/jobs(?:\/|$))/, async route => {
    const path = new URL(route.request().url()).pathname;
    let json: unknown = {};
    if (path === '/v1/users/me') json = { authenticated: false, user: null };
    else if (path === '/v1/models') json = { data: [{ id: `model-${'long-name-'.repeat(30)}`, type: 'chat' }] };
    else if ((path === '/api/v2/jobs' && route.request().method() === 'POST') || path === '/api/v2/jobs/run-cccccccccccccccccccccccccccccccc') json = snapshot();
    else if (path.endsWith('/events')) {
      await new Promise(resolve => setTimeout(resolve, 100));
      return route.fulfill({ contentType: 'text/event-stream', body: `data: ${JSON.stringify(snapshot())}\n\n` });
    } else if (path.endsWith('/cancel')) { status = 'cancelled'; approval = null; json = snapshot(); }
    else if (path === '/api/v2/jobs') json = [];
    else if (path === '/v1/agents/tools') json = { tools: [], enabled_count: 0 };
    else if (path === '/v1/agents/mcp') json = { servers: [] };
    else if (path === '/v1/integrations') json = { integrations: [] };
    else if (path === '/v1/sessions') json = { sessions: [] };
    await route.fulfill({ json });
  });
  return {
    grow: (value: string) => { preview = value; },
    addSteps: () => { steps = [{ id: 'prior-step', type: 'observation', content: 'Earlier step\n'.repeat(80) }]; },
    complete: () => {
      status = 'completed'; output = `Completed report\n${'Result 日本語 /path/'.repeat(3000)}`;
      steps = Array.from({ length: 100 }, (_, id) => ({ id: String(id), type: 'observation', content: `Step ${id} ${'long-unbroken-text'.repeat(50)}` }));
    },
    requestApproval: () => {
      status = 'waiting_for_approval';
      approval = { id: 'approval', tool: 'write_artifact', arguments: {}, canonical_arguments: JSON.stringify({ body: 'review this carefully '.repeat(2000) }, null, 2), expires_at: new Date(Date.now() + 60_000).toISOString() };
    }
  };
}

test('current composer has no legacy setup wall and keeps Start visible', async ({ page }, info) => {
  await fixture(page); await page.setViewportSize({width:1440,height:900}); await page.goto('/ui/#/agents/new');
  await expect(page.getByRole('button',{name:'Start task',exact:true})).toBeInViewport();
  await expect(page.locator('.task-card, .result-card, .computer-mode-picker')).toHaveCount(0);
  await expect(page.locator('.task-composer')).toBeVisible();
  await page.screenshot({path:info.outputPath('current-composer.png')});
});

for (const width of [390, 768, 1024, 1440]) {
  test(`current task contains long results and exposes technical steps at ${width}px`, async ({page}) => {
    const state=await fixture(page);state.complete();await page.setViewportSize({width,height:900});
    await page.goto('/ui/#/agents/task/run-cccccccccccccccccccccccccccccccc');
    await expect(page.locator('.task-result')).toContainText('Completed report');
    await expect(page.locator('.task-activity')).not.toHaveAttribute('open','');
    await expect(page.locator('.task-activity article')).toHaveCount(100);
    expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
    await page.locator('.task-activity > summary').click();
    await expect(page.locator('.task-activity')).toHaveAttribute('open','');
    expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
    await page.locator('.task-detail header').getByRole('button',{name:'Copy result',exact:true}).focus();
    await expect(page.locator('.task-detail header').getByRole('button',{name:'Copy result',exact:true})).toBeInViewport();
    await page.screenshot({path:test.info().outputPath(`current-result-${width}.png`)});
  });
}

for (const locale of ['ar','de']) {
  test(`current results remain contained in ${locale}`,async({page})=>{
    const state=await fixture(page);state.complete();await page.setViewportSize({width:1100,height:800});
    await page.addInitScript(locale=>localStorage.setItem('offgrid.locale',locale),locale);
    await page.goto('/ui/#/agents/task/run-cccccccccccccccccccccccccccccccc');
    await expect(page.locator('.task-result')).toContainText('Completed report');
    await expect(page.locator('html')).toHaveAttribute('dir',locale==='ar'?'rtl':'ltr');
    expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)).toBeLessThanOrEqual(1);
  });
}

test('large approval payload stays behind details with Stop reachable',async({page})=>{
  const state=await fixture(page);state.requestApproval();
  await page.goto('/ui/#/agents/task/run-cccccccccccccccccccccccccccccccc');
  await expect(page.locator('.approval-card')).toBeVisible();
  // Unknown tool effects retain exact arguments, not an inferred safe summary.
  await expect(page.locator('.approval-card pre')).toContainText('review this carefully');
  await page.getByRole('button',{name:'Stop',exact:true}).click();
  await expect(page.locator('.task-state')).toContainText('Cancelled');
});
