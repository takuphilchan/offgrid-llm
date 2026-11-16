import { expect, test } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { craftFixture } from './helpers/craft-fixture';

// Real UI with explicitly staged task receipts. This is a presentation capture,
// not evidence that a model completed the task or that commands ran on a host.
test('capture documentation light agent workspace', async ({ page }) => {
  test.skip(!process.env.OFFGRID_DOC_CAPTURES, 'Set OFFGRID_DOC_CAPTURES=1 to refresh documentation images.');
  await page.setViewportSize({ width: 1440, height: 900 });
  await craftFixture(page, 'en', 'light');
  const id = 'run-' + 'd'.repeat(32);
  const model = 'Qwen2.5-7B-Instruct-Q4_K_M';
  const step = (tool: string, args: object, result: object, text: string) => ({
    type: 'tool_result', tool_name: tool, tool_args: JSON.stringify(args),
    tool_result: JSON.stringify({ receipt: { state: 'complete', result }, text }),
  });
  const run = {
    id, run_id: id, prompt: 'Build a local project dashboard from my brief', model,
    status: 'completed', created_at: '2026-11-16T09:00:00Z',
    output: '## Your project dashboard is ready\n\nCreated a local dashboard with a task list, status filters and upcoming deadlines.\n\n| File | Purpose |\n| --- | --- |\n| `index.html` | Dashboard layout and navigation |\n| `styles.css` | Responsive light and dark styles |\n| `app.js` | Task filters and status updates |\n\n**Check:** `node --check app.js` finished with exit code 0.\n\nOpen `project-dashboard/index.html` to review it. No external services or dependencies are required.',
    profile: { id: 'project-assistant', name: 'Project assistant', revision: 1 },
    execution_workspace: { reference: { host: 'This computer' }, root: 'Demo:/project-dashboard', backend: 'local' },
    steps: [
      step('workspace_read', { path: 'brief.md' }, {}, 'Read the project requirements and existing folder structure.'),
      step('workspace_write', { path: 'index.html, styles.css, app.js' }, {}, 'Created the dashboard layout, styles and task filters.'),
      step('workspace_process_start', { program: 'node', arguments: ['--check', 'app.js'] }, { state: 'exited', exit_code: 0 }, 'JavaScript syntax check finished without errors.'),
    ],
    artifacts: [], instructions: [], pending_input: null, pending_approval: null, can_steer: false,
  };
  await page.route('**/api/v2/jobs', r => r.fulfill({ json: [run] }));
  await page.route(`**/api/v2/jobs/${id}`, r => r.fulfill({ json: run }));
  await page.route('**/v1/models', r => r.fulfill({ json: { data: [{ id: model, type: 'chat', size: 4.4 * 1024 ** 3 }] } }));
  const turn = { id: 'request-demo', prompt: run.prompt, model, status: run.status, output: run.output, job_id: id, job_admission: 'linked', agent: { id: 'project-assistant', revision: 1 } };
  const session = { name: 'Build a project dashboard', model_id: model, updated_at: run.created_at,
    jobs: [{ turn_id: turn.id, job_id: id }],
    messages: [
      { role: 'user', content: 'Build a local project dashboard from my brief. Create the files and check the JavaScript.', turn_id: turn.id, job_id: id },
      { role: 'assistant', content: run.output, turn_id: turn.id, job_id: id },
    ],
  };
  await page.route('**/v1/sessions', r => r.fulfill({ json: { sessions: [session,
    { ...session, name: 'Prepare the workshop folder', messages: [], jobs: [] },
    { ...session, name: 'Review the release checklist', messages: [], jobs: [] },
  ] } }));
  await page.route('**/v1/sessions/Build%20a%20project%20dashboard', r => r.fulfill({ json: session }));
  await page.route('**/v1/sessions/*/turn', r => r.fulfill({ json: { turn } }));
  await page.goto('/ui/#/chat');
  await page.locator('.history-open').filter({ hasText: session.name }).click();
  await expect(page.getByRole('heading', { name: 'Your project dashboard is ready', exact: true })).toBeVisible();
  await expect(page.locator('.work-status')).toHaveText('Completed');
  await expect(page.locator('.conversation-work-details > summary')).toContainText('3');
  await page.evaluate(() => document.fonts.ready);
  const output = resolve('../../build/documentation-captures');
  mkdirSync(output, { recursive: true });
  await page.screenshot({ path: resolve(output, 'agent-workspace-light.png'), animations: 'disabled' });
});
