import { expect, test, type Page } from '@playwright/test';

async function fixture(page: Page) {
  const started = new Date().toISOString();
  let preview = 'First live response';
  let status = 'running';
  let output = '';
  let steps: { id: string; type: string; content: string }[] = [];
  let approval: object | null = null;
  const snapshot = () => ({ run_id: 'layout-run', status, output, steps, pending_approval: approval,
    started_at: started, resumable: false, progress: { phase: 'generating', iteration: 1, preview, updated_at: started } });
  await page.addInitScript(() => {
    localStorage.setItem('offgrid.locale', 'en');
    localStorage.setItem('offgrid.onboarding.complete', 'true');
  });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route('**/api/v2/system', r => r.fulfill({ json: { product: 'offgrid', version: 'test', api_version: 2, capabilities: [] } }));
  await page.route('**/v1/**', async route => {
    const path = new URL(route.request().url()).pathname;
    let json: unknown = {};
    if (path === '/v1/users/me') json = { authenticated: false, user: null };
    else if (path === '/v1/models') json = { data: [{ id: `model-${'long-name-'.repeat(30)}`, type: 'chat' }] };
    else if (path === '/v1/agents/run' || path === '/v1/agents/tasks/layout-run') json = snapshot();
    else if (path.endsWith('/events')) {
      await new Promise(resolve => setTimeout(resolve, 100));
      return route.fulfill({ contentType: 'text/event-stream', body: `data: ${JSON.stringify(snapshot())}\n\n` });
    } else if (path.endsWith('/cancel')) { status = 'cancelled'; approval = null; json = snapshot(); }
    else if (path === '/v1/agents/tasks') json = [];
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
      approval = { id: 'approval', tool: 'write_artifact', canonical_arguments: JSON.stringify({ body: 'review this carefully '.repeat(2000) }, null, 2), expires_at: new Date(Date.now() + 60_000).toISOString() };
    }
  };
}

async function start(page: Page) {
  await page.goto('/ui/#/agents');
  await page.getByLabel('Task', { exact: true }).fill('Produce a detailed report');
  await page.getByRole('button', { name: 'Run task', exact: true }).click();
  await expect(page.locator('.agent-live-preview')).toContainText('First live response');
}

async function geometry(page: Page) {
  return page.evaluate(() => {
    const rect = (selector: string) => {
      const { x, y, width, height } = document.querySelector(selector)!.getBoundingClientRect();
      return { x, y: y + window.scrollY, width, height };
    };
    const pane = document.querySelector('.agent-result-body')!;
    return { form: rect('.task-card'), button: rect('.agent-task-actions'), result: rect('.result-card'),
      paneHeight: pane.clientHeight, contentHeight: pane.scrollHeight,
      overflow: document.documentElement.scrollWidth - window.innerWidth };
  });
}

test('task-first agent workspace leaves desktop room for output and visible Run controls', async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await fixture(page); await page.goto('/ui/#/agents');
  await expect(page.getByRole('button', { name: 'Run task', exact: true })).toBeVisible();
  const task = await page.locator('.agent-task-editor').boundingBox();
  const options = await page.locator('.agent-task-options').boundingBox();
  const form = await page.locator('.task-card').boundingBox();
  const result = await page.locator('.result-card').boundingBox();
  const metrics = await page.locator('.agent-metrics').boundingBox();
  const run = await page.locator('.agent-task-actions button').boundingBox();
  expect(task!.y + task!.height).toBeLessThanOrEqual(options!.y);
  expect(result!.width).toBeGreaterThan(form!.width * 1.4);
  expect(Math.abs(result!.height - form!.height)).toBeLessThan(2);
  expect(metrics!.height).toBeLessThan(100);
  expect(run!.y + run!.height).toBeLessThan(900);
  await page.screenshot({ path: info.outputPath('agent-workspace.png') });
});

for (const width of [390, 768, 1024, 1440]) {
  test(`agent panels contain large results without moving the Run task button at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 });
    const state = await fixture(page);
    await start(page);
    const initial = await geometry(page);
    state.complete();
    await expect(page.locator('.agent-result-body')).toContainText('Completed report');
    await expect(page.locator('.agent-steps article')).toHaveCount(100);
    await expect(page.locator('.agent-steps')).not.toHaveAttribute('open', '');
    const final = await geometry(page);
    expect(final.form).toEqual(initial.form);
    expect(final.button).toEqual(initial.button);
    expect(final.result).toEqual(initial.result);
    expect(final.contentHeight).toBeGreaterThan(final.paneHeight * 5);
    expect(final.overflow).toBeLessThanOrEqual(1);
    if (width < 1100) expect(final.result.y).toBeGreaterThanOrEqual(final.form.y + final.form.height);
    else expect(final.result.y).toBe(final.form.y);
    // The output is a labelled, keyboard-scrollable region, not a second page.
    const pane = page.getByRole('region', { name: 'Result', exact: true });
    await pane.focus();
    await page.keyboard.press('End');
    await expect.poll(() => pane.evaluate(e => e.scrollTop)).toBeGreaterThan(0);
    await page.locator('.agent-steps summary').click();
    await expect(page.locator('.agent-steps')).toHaveAttribute('open', '');
    await page.screenshot({ path: test.info().outputPath(`agents-${width}.png`), fullPage: true });
  });
}

test('live output follows within its pane but leaves readers and controls in place', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  await page.setViewportSize({ width: 1440, height: 1000 });
  const state = await fixture(page);
  await start(page);
  const pane = page.locator('.agent-result-body');
  const gap = () => pane.evaluate(e => e.scrollHeight - e.clientHeight - e.scrollTop);
  const initial = await geometry(page);
  const pageScroll = await page.evaluate(() => window.scrollY);
  const long = 'Growing output 日本語\n'.repeat(120);
  state.addSteps();
  state.grow(`${long}First tail`);
  await expect(page.locator('.agent-live-preview')).toContainText('First tail');
  await expect.poll(gap).toBeLessThan(2);
  expect(await page.locator('.agent-live-preview').evaluate(e => e === e.parentElement!.lastElementChild)).toBe(true);
  expect(await page.evaluate(() => window.scrollY)).toBe(pageScroll);
  // Explicit reading position survives the next update; no focus stealing.
  await pane.focus();
  await page.keyboard.press('Home');
  await expect.poll(() => pane.evaluate(e => e.scrollTop)).toBe(0);
  state.grow(`${long.repeat(2)}Second tail`);
  await expect(page.locator('.agent-live-preview')).toContainText('Second tail');
  expect(await pane.evaluate(e => e.scrollTop)).toBe(0);
  await expect(pane).toBeFocused();
  await page.keyboard.press('End');
  await expect.poll(gap).toBeLessThan(2);
  state.grow(`${long.repeat(3)}Third tail`);
  await expect(page.locator('.agent-live-preview')).toContainText('Third tail');
  await expect.poll(gap).toBeLessThan(2);
  expect((await geometry(page)).button).toEqual(initial.button);
  await expect(page.locator('.agent-result-header').getByRole('button', { name: 'Cancel', exact: true })).toBeInViewport();
  await expect(page.getByRole('checkbox', { name: 'Live response preview' })).toBeInViewport();
  // Only the body scrolls, not a nested preview box.
  expect(await page.locator('.agent-live-preview pre').evaluate(e => e.scrollHeight - e.clientHeight)).toBeLessThanOrEqual(1);
  await page.screenshot({ path: test.info().outputPath('agent-live.png'), fullPage: true });
});

for (const locale of ['ar', 'de']) {
  test(`agent panels stay contained after switching to ${locale}`, async ({ page }) => {
    await page.setViewportSize({ width: 1100, height: 800 });
    const state = await fixture(page);
    await start(page);
    await page.locator('.locale-picker select').selectOption(locale);
    await expect(page.locator('html')).toHaveAttribute('dir', locale === 'ar' ? 'rtl' : 'ltr');
    const initial = await geometry(page);
    state.complete();
    await expect(page.locator('.agent-result-body')).toContainText('Completed report');
    const final = await geometry(page);
    expect(final.button).toEqual(initial.button);
    expect(final.overflow).toBeLessThanOrEqual(1);
    expect(final.result.width).toBe(initial.result.width);
    await page.screenshot({ path: test.info().outputPath(`agents-${locale}.png`), fullPage: true });
  });
}

test('approval transitions reset the output pane and keep cancellation reachable', async ({ page }) => {
  const state = await fixture(page);
  await start(page);
  state.grow('Prior output\n'.repeat(100));
  await expect(page.locator('.agent-live-preview')).toContainText('Prior output');
  state.requestApproval();
  await expect(page.getByRole('alertdialog')).toBeVisible();
  const pane = page.locator('.agent-result-body');
  expect(await pane.evaluate(e => e.scrollTop)).toBe(0);
  expect(await page.locator('.approval-card pre').evaluate(e => e.scrollHeight - e.clientHeight)).toBeLessThanOrEqual(1);
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.locator('.agent-result-header')).toContainText('Cancelled');
});
