import { createHash } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
const id = 'run-' + 'a'.repeat(32);
const content = 'name,total\nExample,12\n', sha256 = createHash('sha256').update(content).digest('hex');
const artifact = { name: 'summary.csv', format: 'csv', bytes: Buffer.byteLength(content), sha256, verified: true, check: 'stored_bytes_sha256_and_format' };

async function fixture(page: Page) {
  const state = { run: { run_id: id, prompt: 'Prepare an example summary', status: 'completed', output: '## Generated summary\n\nThe total is twelve.', model: 'model', steps: [{ id: 'step-1', tool_name: 'create_artifact', type: 'tool', tool_result: 'Private execution detail' }], artifacts: [artifact], can_steer: false, instructions: [] } as any, writes: [] as { path: string; data: any }[], artifactStatus: 200, artifactBody: content, loseAck: false, publishInstruction: true };
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', r => r.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(api\/v2|v1)\//, r => {
    const path = new URL(r.request().url()).pathname;
    if (r.request().method() !== 'GET') {
      const data = r.request().postDataJSON(); state.writes.push({ path, data });
      if (path.endsWith('/steer')) {
        if (state.publishInstruction && !state.run.instructions.some((v: any) => v.request_id === data.request_id)) state.run.instructions.push({ request_id: data.request_id, text: data.instruction });
        if (state.loseAck) return r.fulfill({ status: 503, json: { error: 'Acknowledgment lost. Check task status.' } });
        return r.fulfill({ json: state.run });
      }
    }
    if (path.endsWith('/artifact')) return r.fulfill({ status: state.artifactStatus, contentType: 'application/octet-stream', body: state.artifactBody });
    const data: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: 'results', capabilities: ['task-first-agents-v2'] },
      '/v1/users/me': { authenticated: true, auth_required: true, user: { id: 'alice', username: 'alice', role: 'admin' } },
      '/v1/models': { data: [{ id: 'model', type: 'chat' }] }, '/v1/catalog': { models: [] }, '/v1/sessions': { sessions: [] },
      '/api/v2/jobs': [{ id, prompt: state.run.prompt, status: state.run.status, created_at: '2026-10-08T00:00:00Z', deletable: true }],
      [`/api/v2/jobs/${id}`]: state.run,
    };
    return r.fulfill({ json: data[path] ?? {} });
  });
  return state;
}

test('saved answer and files precede collapsed logs without turning generation into verification', async ({ page }) => {
  const state = await fixture(page); await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('heading', { name: 'Saved answer' })).toBeVisible();
  await expect(page.locator('.task-result')).toContainText('Generated content is not independent verification');
  await expect(page.locator('.task-artifact')).toContainText('File integrity checked');
  await expect(page.locator('.task-artifacts')).toContainText('not factual correctness');
  await expect(page.getByText('Private execution detail', { exact: true })).not.toBeVisible();
  expect(await page.locator('.task-result').evaluate(node => !!(node.compareDocumentPosition(document.querySelector('.task-activity')!) & Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);
  const downloading = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download: summary.csv' }).click();
  expect((await downloading).suggestedFilename()).toBe('summary.csv');
  expect(state.writes).toEqual([]);
});

for (const status of [403, 404, 503]) test(`artifact ${status} remains an in-context error without replacing the saved answer`, async ({ page }) => {
  const state = await fixture(page); state.artifactStatus = status;
  await page.goto(`/ui/#/agents/task/${id}`);
  let downloads = 0; page.on('download', () => downloads++);
  await page.getByRole('button', { name: 'Download: summary.csv' }).click();
  await expect(page.locator('.task-artifact').getByRole('alert')).toContainText('File unavailable');
  await expect(page.getByRole('heading', { name: 'Saved answer' })).toBeVisible();
  expect(downloads).toBe(0); expect(state.writes).toEqual([]);
});

test('incorrect artifact bytes cannot be downloaded as verified output', async ({ page }) => {
  const state = await fixture(page); state.artifactBody = 'different bytes';
  await page.goto(`/ui/#/agents/task/${id}`);
  let downloads = 0; page.on('download', () => downloads++);
  await page.getByRole('button', { name: 'Download: summary.csv' }).click();
  await expect(page.locator('.task-artifact').getByRole('alert')).toContainText('File unavailable');
  expect(downloads).toBe(0);
});

test('unknown formats have a download fallback, never an executable preview', async ({ page }) => {
  const state = await fixture(page);
  const body = '<script>window.executedArtifact=true</script>';
  state.run.artifacts = [{ ...artifact, name: 'document.html', format: 'html', verified: false, sha256: createHash('sha256').update(body).digest('hex'), bytes: Buffer.byteLength(body) }];
  state.artifactBody = body;
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.task-artifact')).toContainText('Not verified');
  await expect(page.locator('.task-artifact')).toContainText('No inline preview');
  await expect(page.locator('.task-result iframe, .task-result script')).toHaveCount(0);
  const downloading = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download: document.html' }).click();
  expect((await downloading).suggestedFilename()).toBe('document.html');
  expect(await page.evaluate(() => (window as any).executedArtifact)).toBeUndefined();
});

test('results and long file names fit a narrow screen in both themes', async ({ page }, info) => {
  const state = await fixture(page); state.run.artifacts[0] = { ...artifact, name: 'VeryLongUnbrokenFilename'.repeat(8) + '.csv' };
  await page.setViewportSize({ width: 320, height: 640 });
  for (const theme of ['light', 'dark']) {
    await page.goto(`/ui/#/agents/task/${id}`); await page.evaluate(theme => localStorage.setItem('offgrid.theme', theme), theme); await page.reload();
    await expect(page.getByRole('heading', { name: 'Saved answer' })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
    await page.screenshot({ path: info.outputPath(`result-${theme}.png`), fullPage: true });
  }
});

test('unsupported continuation creates only an explicit new draft and preserves the old result', async ({ page }) => {
  const state = await fixture(page); await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('region', { name: 'Next step' })).toContainText('does not change or rerun');
  await page.getByRole('button', { name: 'Start a new task from this' }).click();
  await expect(page).toHaveURL(/#\/agents\/new$/);
  await expect(page.getByRole('textbox', { name: 'Task', exact: true })).toHaveValue(state.run.prompt);
  expect(state.writes).toEqual([]);
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('button', { name: 'Start a new task from this' })).toBeDisabled();
  await expect(page.getByRole('heading', { name: 'Saved answer' })).toBeVisible();
});

test('lost steering acknowledgment reconciles the accepted instruction without another submission', async ({ page }) => {
  const state = await fixture(page); state.run.status = 'interrupted'; state.run.can_steer = true; state.loseAck = true;
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('textbox', { name: 'Update instruction' }).fill('Keep the exact numbers.');
  await page.getByRole('button', { name: 'Save instruction' }).click();
  await expect(page.getByText('Instruction saved. Resume when ready.')).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Update instruction' })).toHaveValue('');
  await page.reload();
  expect(state.writes).toHaveLength(1);
  expect(state.writes[0].path).toBe(`/api/v2/jobs/${id}/steer`);
});

test('uncertain steering retry retains its ID across reload', async ({ page }) => {
  const state = await fixture(page); state.run.status = 'interrupted'; state.run.can_steer = true; state.loseAck = true; state.publishInstruction = false;
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('textbox', { name: 'Update instruction' }).fill('Keep the exact numbers.');
  await page.getByRole('button', { name: 'Save instruction' }).click();
  await expect(page.locator('.task-steering').getByRole('alert')).toContainText('Acknowledgment lost');
  await page.reload(); state.loseAck = false; state.publishInstruction = true;
  await expect(page.getByRole('textbox', { name: 'Update instruction' })).toHaveValue('Keep the exact numbers.');
  await page.getByRole('button', { name: 'Save instruction' }).click();
  await expect(page.getByText('Instruction saved. Resume when ready.')).toBeVisible();
  expect(state.writes).toHaveLength(2);
  expect(state.writes[0].data).toEqual(state.writes[1].data);
  expect(state.run.instructions).toHaveLength(1);
});

test('a late accepted instruction does not erase a newer correction', async ({ page }) => {
  const state = await fixture(page); state.run.status = 'interrupted'; state.run.can_steer = true;
  let release: (() => Promise<void>) | undefined;
  await page.route(`**/api/v2/jobs/${id}/steer`, r => new Promise<void>(resolve => { release = async () => { await r.fulfill({ json: state.run }); resolve(); }; }));
  await page.goto(`/ui/#/agents/task/${id}`);
  const editor = page.getByRole('textbox', { name: 'Update instruction' });
  await editor.fill('First correction.'); await page.getByRole('button', { name: 'Save instruction' }).click();
  await expect.poll(() => !!release).toBeTruthy(); await editor.fill('Newer correction.'); await release!();
  await expect(editor).toHaveValue('Newer correction.');
  await expect(page.locator('.task-steering')).toHaveCount(1);
});

test('capability changes hide unsupported steering but retain the correction draft', async ({ page }) => {
  const state = await fixture(page); state.run.status = 'interrupted'; state.run.can_steer = true;
  await page.clock.install(); await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('textbox', { name: 'Update instruction' }).fill('Preserve this correction.');
  state.run.can_steer = false; await page.clock.fastForward(2600);
  await expect(page.getByRole('textbox', { name: 'Update instruction' })).toHaveCount(0);
  state.run.can_steer = true; await page.clock.fastForward(2600);
  await expect(page.getByRole('textbox', { name: 'Update instruction' })).toHaveValue('Preserve this correction.');
  expect(state.writes).toEqual([]);
});
