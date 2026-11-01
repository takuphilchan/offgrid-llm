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

for (const recovery of [
  { status: 'failed', resumable: false, hint: 'This task did not finish.', resume: false },
  { status: 'interrupted', resumable: true, hint: 'Review the last activity before choosing Resume.', resume: true },
  { status: 'interrupted', resumable: false, hint: 'This saved task has no resumable checkpoint.', resume: false },
  { status: 'cancelled', resumable: false, hint: 'This task was cancelled.', resume: false },
]) test(`saved ${recovery.status} work explains the next step (resumable: ${recovery.resumable}) without a misleading Refresh`, async ({ page }) => {
  const state = await fixture(page);
  Object.assign(state.run, { status: recovery.status, resumable: recovery.resumable, error: 'Original service reason' });
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.work-recovery-guidance')).toContainText(recovery.hint);
  await expect(page.locator('.work-recovery-guidance')).toContainText('Original service reason');
  await expect(page.locator('.work-recovery-guidance')).toContainText('Saved answers and files remain available');
  await expect(page.locator('.work-inspector').getByRole('button', { name: 'Refresh', exact: true })).toHaveCount(0);
  await expect(page.locator('.work-inspector').getByRole('button', { name: 'Resume', exact: true })).toHaveCount(recovery.resume ? 1 : 0);
  await expect(page.getByRole('heading', { name: 'Generated summary' })).toBeVisible();
  expect(state.writes).toEqual([]);
});

test('expired desktop access explains reconnect without offering a conflicting Resume', async ({ page }) => {
  const state = await fixture(page);
  Object.assign(state.run, { status: 'interrupted', resumable: true, computer_session_expired: true, last_access: { id: 'input', mode: 'desktop' } });
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.locator('.work-recovery-guidance')).toContainText('Desktop access ended.');
  await expect(page.locator('.work-inspector').getByRole('button', { name: 'Reconnect access' })).toBeVisible();
  await expect(page.locator('.work-inspector').getByRole('button', { name: 'Resume', exact: true })).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('specialists expose their own status, dependencies and draft without claiming the parent is complete', async ({ page }) => {
  const state = await fixture(page), childA = 'run-' + 'b'.repeat(32), childB = 'run-' + 'c'.repeat(32);
  Object.assign(state.run, { status: 'interrupted', children: [
    { id: childA, spec: { key: 'research', goal: 'Find supporting sources', depends_on: [], tools: [] } },
    { id: childB, spec: { key: 'write', goal: 'Draft the briefing', depends_on: ['research'], tools: [] } },
  ] });
  await page.route(`**/api/v2/jobs/${childA}`, route => route.fulfill({ json: { ...state.run, run_id: childA, parent_id: id, status: 'completed', output: 'Research findings', children: [], artifacts: [] } }));
  await page.route(`**/api/v2/jobs/${childB}`, route => route.fulfill({ json: { ...state.run, run_id: childB, parent_id: id, status: 'failed', error: 'The draft needs more evidence.', output: 'Partial specialist draft', children: [], artifacts: [] } }));
  await page.goto(`/ui/#/agents/task/${id}`);
  const rows = page.locator('.specialist-row');
  await expect(rows.nth(0)).toContainText('Completed');
  await expect(rows.nth(1)).toContainText('Failed');
  await expect(page.getByText('Partial specialist draft', { exact: true })).not.toBeVisible();
  await rows.nth(1).locator('summary').click();
  await expect(rows.nth(1)).toContainText('Depends on: Find supporting sources');
  await expect(page.getByText('Partial specialist draft', { exact: true })).toBeVisible();
  await expect(page.locator('.work-status')).toHaveText('Interrupted');
  await expect(rows.getByRole('button', { name: 'Resume', exact: true })).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('conflicting and blocked branches remain attributable without completing or restarting the parent', async ({ page }) => {
  const state = await fixture(page);
  const branches = [
    { key: 'source-a', goal: 'Inspect first source', status: 'completed', output: 'Source A reports twelve participants.', depends_on: [] },
    { key: 'source-b', goal: 'Inspect second source', status: 'completed', output: 'Source B reports fifteen participants.', depends_on: [] },
    { key: 'verify', goal: 'Verify disputed count', status: 'failed', output: '', error: 'The primary record is unavailable.', depends_on: ['source-a', 'source-b'] },
    { key: 'draft', goal: 'Draft verified result', status: 'cancelled', output: '', error: 'Cancelled after the verification prerequisite failed.', depends_on: ['verify'] },
  ];
  Object.assign(state.run, { status: 'failed', output: 'The counts conflict. No verified conclusion is available.', artifacts: [],
    children: branches.map((branch, i) => ({ id: 'run-' + String(i + 1).repeat(32), spec: { key: branch.key, goal: branch.goal, depends_on: branch.depends_on, tools: [] } })) });
  for (const [i, branch] of branches.entries()) {
    const childId = state.run.children[i].id;
    await page.route(`**/api/v2/jobs/${childId}`, route => route.fulfill({ json: { ...state.run, ...branch, run_id: childId, parent_id: id, children: [] } }));
  }
  await page.goto(`/ui/#/agents/task/${id}`);
  const rows = page.locator('.specialist-row');
  for (let i = 0; i < branches.length; i++) await rows.nth(i).locator('summary').click();
  await expect(rows.nth(0)).toContainText('Source A reports twelve participants.');
  await expect(rows.nth(1)).toContainText('Source B reports fifteen participants.');
  await expect(rows.nth(2)).toContainText('The primary record is unavailable.');
  await expect(rows.nth(3)).toContainText('Depends on: Verify disputed count');
  await expect(rows.nth(3)).toContainText('Cancelled after the verification prerequisite failed.');
  await expect(page.locator('.work-status')).toHaveText('Failed');
  await expect(page.getByText('The counts conflict. No verified conclusion is available.', { exact: true })).toBeVisible();
  await expect(rows.getByRole('button', { name: /Resume|Retry|Approve|Stop/ })).toHaveCount(0);
  expect(state.writes).toEqual([]);
});

test('specialist reads are bounded and unrelated or revoked child output is not displayed', async ({ page }) => {
  const state = await fixture(page), reads: string[] = [];
  state.run.children = Array.from({ length: 6 }, (_, i) => ({ id: 'run-' + String(i + 1).repeat(32), spec: { key: `child-${i}`, goal: `Specialist ${i + 1}`, depends_on: [], tools: [] } }));
  await page.route(/\/api\/v2\/jobs\/run-[1-6]{32}$/, route => {
    const child = new URL(route.request().url()).pathname.split('/').pop()!; reads.push(child);
    if (child.endsWith('2')) return route.fulfill({ status: 403, json: { error: 'Access revoked' } });
    return route.fulfill({ json: { ...state.run, run_id: child, parent_id: 'different-parent', output: 'Unrelated private draft', children: [] } });
  });
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect.poll(() => reads.length).toBe(4);
  await expect(page.locator('.specialist-row')).toHaveCount(6);
  await page.locator('.specialist-row').nth(0).locator('summary').click();
  await expect(page.locator('body')).not.toContainText('Unrelated private draft');
  await page.locator('.specialist-row').nth(1).locator('summary').click();
  await expect(page.getByText('Access revoked', { exact: true })).toBeVisible();
  await page.locator('.specialist-row').last().locator('summary').click();
  await expect(page.locator('.specialist-row').last()).toContainText('Open this subtask to inspect its current state.');
  expect(reads.length).toBe(4);
  expect(state.writes).toEqual([]);
});

test('saved answer and files precede collapsed logs without turning generation into verification', async ({ page }) => {
  const state = await fixture(page); await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('heading', { name: 'Generated summary' })).toBeVisible();
  await expect(page.locator('.work-inspector')).toContainText('Generated content is not independent verification');
  await expect(page.locator('.task-artifact')).toContainText('File integrity checked');
  await expect(page.locator('.task-artifacts')).toContainText('not factual correctness');
  await expect(page.getByText('Private execution detail', { exact: true })).not.toBeVisible();
  expect(await page.locator('.task-result').evaluate(node => !!(node.compareDocumentPosition(document.querySelector('.conversation-work-details')!) & Node.DOCUMENT_POSITION_FOLLOWING))).toBe(true);
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
  await expect(page.getByRole('heading', { name: 'Generated summary' })).toBeVisible();
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

test('artifact preview checks bytes in context, closes and rechecks revoked access', async ({ page }) => {
  const state = await fixture(page);
  let reads = 0, downloads = 0;
  page.on('request', request => { if (new URL(request.url()).pathname.endsWith('/artifact')) reads++; });
  page.on('download', () => downloads++);
  await page.goto(`/ui/#/agents/task/${id}`);
  expect(reads).toBe(0);
  await page.getByRole('button', { name: 'Preview: summary.csv', exact: true }).click();
  const preview = page.getByRole('region', { name: 'Preview: summary.csv' });
  await expect(preview.locator('pre')).toHaveText(content);
  await expect(page.locator('.task-artifacts').getByText('The recorded check covers stored bytes and format, not factual correctness.', { exact: true })).toHaveCount(1);
  await page.getByRole('button', { name: 'Close preview: summary.csv' }).click();
  await expect(preview).toHaveCount(0);
  state.artifactStatus = 403;
  await page.getByRole('button', { name: 'Preview: summary.csv', exact: true }).click();
  await expect(preview.getByRole('alert')).toContainText('Preview unavailable');
  await expect(preview.locator('pre')).toHaveCount(0);
  expect(reads).toBe(2); expect(downloads).toBe(0); expect(state.writes).toEqual([]);
});

test('preview rejects mismatched content and metadata above its limit', async ({ page }) => {
  const state = await fixture(page); state.artifactBody = 'untrusted replacement';
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('button', { name: 'Preview: summary.csv', exact: true }).click();
  await expect(page.locator('.artifact-preview').getByRole('alert')).toBeVisible();
  await expect(page.locator('.artifact-preview pre')).toHaveCount(0);
  state.run.artifacts[0] = { ...artifact, bytes: 65537 };
  await page.reload();
  await expect(page.getByRole('button', { name: 'Preview: summary.csv', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Download: summary.csv' })).toBeVisible();
});

test('closing a pending preview discards its late content and reopening requires another read', async ({ page }) => {
  await fixture(page);
  let release!: () => void, reads = 0;
  const pending = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/artifact?*', async route => { reads++; if (reads === 1) await pending; await route.fulfill({ contentType: 'application/octet-stream', body: content }); });
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('button', { name: 'Preview: summary.csv', exact: true }).click();
  await expect.poll(() => reads).toBe(1);
  await page.getByRole('button', { name: 'Close preview: summary.csv', exact: true }).click();
  release();
  await expect(page.locator('.artifact-preview-content')).toHaveCount(0);
  await page.getByRole('button', { name: 'Preview: summary.csv', exact: true }).click();
  await expect(page.locator('.artifact-preview-content pre')).toContainText(content.trim());
  expect(reads).toBe(2);
});

test('Markdown preview renders Unicode and blocks active content and remote images', async ({ page }) => {
  const state = await fixture(page);
  const body = '# Café 日本語\n\n<script>window.artifactExecuted=true</script>\n\n![private](https://artifact.invalid/image)\n\n[unsafe](javascript:alert(1))';
  state.run.artifacts = [{ ...artifact, name: 'report.md', format: 'markdown', bytes: Buffer.byteLength(body), sha256: createHash('sha256').update(body).digest('hex') }];
  state.artifactBody = body;
  const remote: string[] = []; page.on('request', request => { if (request.url().includes('artifact.invalid')) remote.push(request.url()); });
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('button', { name: 'Preview: report.md' }).click();
  await expect(page.locator('.artifact-preview').getByRole('heading', { name: 'Café 日本語' })).toBeVisible();
  await expect(page.locator('.artifact-preview script, .artifact-preview img, .artifact-preview iframe')).toHaveCount(0);
  await expect(page.locator('.artifact-preview a[href^="javascript:"]')).toHaveCount(0);
  expect(await page.evaluate(() => (window as any).artifactExecuted)).toBeUndefined(); expect(remote).toEqual([]);
});

for (const bytes of [Buffer.from([0xc3, 0x28]), Buffer.from([0x61, 0, 0x62])]) test(`preview rejects invalid text ${bytes.toString('hex')}`, async ({ page }) => {
  const state = await fixture(page);
  state.run.artifacts = [{ ...artifact, name: 'invalid.txt', format: 'txt', bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }];
  await page.route('**/artifact?*', r => r.fulfill({ contentType: 'application/octet-stream', body: bytes }));
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('button', { name: 'Preview: invalid.txt' }).click();
  await expect(page.locator('.artifact-preview').getByRole('alert')).toBeVisible();
  await expect(page.locator('.artifact-preview pre')).toHaveCount(0);
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
    await expect(page.getByRole('heading', { name: 'Generated summary' })).toBeVisible();
    const name = page.locator('.artifact-name');
    await expect(name).toHaveText(state.run.artifacts[0].name);
    expect(await name.evaluate(el => el.getBoundingClientRect().height / parseFloat(getComputedStyle(el).lineHeight))).toBeLessThanOrEqual(2.1);
    await page.getByText('Full filename', { exact: true }).click();
    await expect(page.locator('.artifact-name-details p')).toBeVisible();
    await expect(page.locator('.artifact-name-details p')).toHaveText(state.run.artifacts[0].name);
    await page.getByText('Full filename', { exact: true }).click();
    await page.getByRole('button', { name: /^Preview:/ }).click();
    await expect(page.locator('.artifact-preview-content pre')).toContainText(content.trim());
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
    await page.screenshot({ path: info.outputPath(`result-${theme}.png`), fullPage: true, animations: 'disabled' });
  }
});

test('unsupported continuation links to Chat without resubmitting or losing the old result', async ({ page }) => {
  const state = await fixture(page); await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('region', { name: 'Next step' })).toContainText('does not change or rerun');
  await page.getByRole('region',{name:'Next step'}).getByRole('link', { name: 'New chat',exact:true }).click();
  await expect(page).toHaveURL(/#\/chat$/);
  await expect(page.locator('.composer textarea')).toHaveValue('');
  expect(state.writes).toEqual([]);
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByRole('button', { name: 'Start a new task from this' })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Generated summary' })).toBeVisible();
});

test('cancelled empty work renders no zero and offers a compact new-chat action', async ({page}, info) => {
  const state=await fixture(page);
  Object.assign(state.run,{status:'cancelled',output:'',artifacts:[],steps:[]});
  await page.goto(`/ui/#/agents/task/${id}`);
  const recovery=page.locator('.work-recovery-guidance');
  await expect(recovery).toContainText('This task was cancelled.');
  expect(await recovery.evaluate(el=>Array.from(el.childNodes).filter(n=>n.nodeType===Node.TEXT_NODE).map(n=>n.textContent?.trim()).filter(Boolean))).toEqual([]);
  const continuation=page.getByRole('region',{name:'Next step'}), link=continuation.getByRole('link',{name:'New chat',exact:true});
  await expect(link).toHaveAttribute('href','#/chat/new');
  expect((await link.boundingBox())!.width).toBeLessThan(200);
  await continuation.scrollIntoViewIfNeeded();
  await page.screenshot({path:info.outputPath('cancelled-task.png')});
  await link.click(); await expect(page.locator('.composer textarea')).toHaveValue('');
  expect(state.writes).toEqual([]);
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
