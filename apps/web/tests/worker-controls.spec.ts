import {expect, test, type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

async function fixture(page: Page, completed = false, loseAck = false) {
  await craftFixture(page, 'en', 'dark');
  const worker: any = {id: 'worker-review', revision: 3, work_id: 'work-root', root_run_id: 'run-root', current_run_id: 'run-child', run_ids: ['run-old', 'run-child'], assignment: {goal: 'Review sources', expected_result: 'Citations', tools: [], depends_on: [], instruction_version: 1, model_calls: 4}, assignment_version: 2, status: completed ? 'cancelled' : 'running', stopped: completed, closed: false};
  const spec = {key: 'review', goal: 'Review sources', depends_on: [], tools: []};
  const root: any = {run_id: 'run-root', task_id: 'run-root', work_id: 'work-root', prompt: 'Research this', model: 'fixture', status: 'waiting_for_children', output: '', steps: [], event_cursor: '1', children: [{id: 'run-old', worker_id: worker.id, invalidated: true, status: 'completed', spec}, {id: 'run-child', worker_id: worker.id, status: worker.status, spec}], can_steer: true};
  const session = {name: 'Demo briefing', model_id: 'fixture', messages: [{role: 'user', content: root.prompt}], turn: {id: 'turn-root', job_id: root.run_id, status: root.status, prompt: root.prompt, job_admission: 'linked'}};
  const commands: any[] = [], mail: any[] = [], accepted = new Set<string>();
  let lost = false;
  await page.route('**/v1/sessions', route => route.fulfill({json: {sessions: [session]}}));
  await page.route('**/v1/sessions/Demo%20briefing', route => route.fulfill({json: session}));
  await page.route('**/v1/sessions/Demo%20briefing/turn', route => route.fulfill({json: {turn: session.turn, job: root}}));
  await page.route('**/api/v2/jobs/**', route => {
    const path = new URL(route.request().url()).pathname;
    const id = path.split('/')[4];
    const run = id === 'run-root' ? root : {run_id: id, task_id: id, parent_id: root.run_id, status: worker.status, output: '', steps: [], event_cursor: String(worker.revision)};
    return route.fulfill(path.endsWith('/events') ? {contentType: 'text/event-stream', body: `data: ${JSON.stringify({...run, type: 'snapshot'})}\n\n`} : {json: run});
  });
  await page.route('**/api/v2/workers/**', async route => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() === 'GET') return route.fulfill({json: path.endsWith('/messages') ? {items: mail, has_more: false, next_cursor: mail.length} : worker});
    const command = route.request().postDataJSON(); commands.push(command);
    if (accepted.has(command.request_id)) return route.fulfill({json: {request_id: command.request_id, target_id: worker.id, revision: worker.revision, event_cursor: worker.revision, worker, message: mail[0]}});
    if (command.kind === 'message' || command.kind === 'answer') {
      if (!mail.length) {mail.push({id: command.request_id, worker_id: worker.id, run_id: worker.current_run_id, content: command.payload.content, state: 'accepted', instruction_version: 1, accepted_at: new Date().toISOString()}); worker.revision++;}
      if (command.kind === 'answer') {
        if (command.payload.question_id !== worker.question.id) return route.fulfill({status: 409, json: {error: {message: 'Question changed'}}});
        worker.question.state = 'answered'; worker.status = 'pending';
      }
    } else if (command.kind === 'continue') {
      worker.current_run_id = 'run-followup'; worker.run_ids.push(worker.current_run_id); worker.stopped = false; worker.status = 'pending'; worker.revision++;
      root.children.push({id: worker.current_run_id, worker_id: worker.id, status: worker.status, spec}); root.event_cursor = '2';
    } else if (command.kind === 'stop') {worker.stopped = true; worker.revision++;}
    else throw new Error('Unexpected mutation');
    accepted.add(command.request_id);
    if (loseAck && !lost) {lost = true; return route.abort('failed');}
    return route.fulfill({status: 202, json: {request_id: command.request_id, target_id: worker.id, revision: worker.revision, event_cursor: worker.revision, worker, message: mail[0]}});
  });
  return {worker, root, commands, mail};
}

test('saved worker output pages retain final citations without dispatch', async ({page}) => {
  const f = await fixture(page, true);
  f.worker.result_reference = 'result:run-child:fixture';
  const reads: number[] = [];
  const text = 'Retained evidence. Final citation [42].';
  await page.route('**/api/v2/results/**', route => {
    const offset = Number(new URL(route.request().url()).searchParams.get('offset'));
    reads.push(offset);
    const end = offset === 0 ? 19 : text.length;
    return route.fulfill({json: {id: f.worker.result_reference, run_id: 'run-child', status: 'cancelled', instruction_version: 1, control_epoch: 1, independently_verified: false, evidence_steps: [], content: text.slice(offset, end), offset, next_offset: end, total_bytes: text.length, complete: end === text.length}});
  });
  await page.goto('/ui/#/chat');
  await page.locator('.specialist-row > details > summary').click();
  const panel = page.locator('.worker-controls');
  await panel.getByText('Result', {exact: true}).click();
  await expect(panel.locator('.saved-result-content')).toContainText('Retained evidence.');
  await panel.getByRole('button', {name: 'Show more', exact: true}).click();
  await expect(panel.locator('.saved-result-content')).toHaveText(text);
  expect(reads).toEqual([0, 19]);
  expect(f.commands).toHaveLength(0);
});

test('work resource accounting is inspectable without dispatch or exposing it in the conversation', async ({page}) => {
  const f = await fixture(page);
  f.root.resource_usage = {model_call_limit: 30, model_calls: 2, child_call_allocations: 4, input: {window: 4096, charged_tokens: 4196, measured_tokens: 100, unmeasured_calls: 1}, charged_output_tokens: 120, measured_output_tokens: 20, charged_inference_ns: 400, measured_inference_ns: 200, context_preparations: 2, measured_context_preparation_ns: 42};
  await page.setViewportSize({width: 390, height: 844});
  await page.goto('/ui/#/chat');
  const usage = page.locator('.task-resource-usage');
  await expect(usage).toHaveCount(1);
  await expect(usage).toBeHidden();
  for (const detail of await page.locator('details').filter({has: usage}).all()) {
    if (await detail.getAttribute('open') === null) await detail.locator(':scope > summary').click();
  }
  await expect(usage).toContainText('"charged_tokens": 4196');
  await expect(usage).toContainText('"measured_tokens": 100');
  await expect(usage).toBeVisible();
  expect((await usage.boundingBox())!.width).toBeLessThanOrEqual(390);
  expect(f.commands).toHaveLength(0);
});

test('worker question answers survive reload with the original question and command identity', async ({page}) => {
  const f = await fixture(page, false, true);
  f.worker.status = 'waiting_for_input';
  f.worker.question = {id: 'question-one', worker_id: f.worker.id, run_id: f.worker.current_run_id, content: 'Should I include offline runtimes?', instruction_version: 1, assignment_version: 2, state: 'pending', requested_at: new Date().toISOString()};
  await page.goto('/ui/#/chat');
  await page.locator('.specialist-row summary').click();
  const panel = page.locator('.worker-controls');
  await expect(panel.locator('.worker-question')).toHaveText('Should I include offline runtimes?');
  await panel.getByRole('textbox').fill('Include offline runtimes');
  expect(f.commands).toHaveLength(0);
  await panel.getByRole('button', {name: 'Send instruction', exact: true}).click();
  await expect(panel.getByRole('alert')).toBeVisible();
  expect(f.commands[0]).toMatchObject({kind: 'answer', expected_revision: 3, payload: {question_id: 'question-one', content: 'Include offline runtimes'}});
  await page.reload();
  await page.locator('.specialist-row summary').click();
  await expect(panel.getByRole('button', {name: 'Try again', exact: true})).toBeVisible();
  expect(f.commands).toHaveLength(1);
  await panel.getByRole('button', {name: 'Try again', exact: true}).click();
  await expect.poll(() => f.commands.length).toBe(2);
  expect(f.commands[1]).toEqual(f.commands[0]);
  expect(f.worker.run_ids).toEqual(['run-old', 'run-child']);
  expect(f.mail).toHaveLength(1);
  await expect(panel.getByRole('textbox')).toHaveValue('');
});

test('primary clarification shows a question and accepts an answer without host setup', async ({page}) => {
  const f = await fixture(page);
  f.root.status = 'waiting_for_input';
  f.root.pending_input = {id: 'question-one', kind: 'worker_clarification', mode: 'clarification', target: 'Should this comparison include offline runtimes?'};
  f.worker.status = 'waiting_for_input';
  f.worker.question = {id: 'question-one', worker_id: f.worker.id, run_id: f.worker.current_run_id, content: 'Should I include offline runtimes?', instruction_version: 1, assignment_version: 2, state: 'pending', requested_at: new Date().toISOString()};
  await page.goto('/ui/#/chat');
  await expect(page.getByText('Should this comparison include offline runtimes?', {exact: true})).toBeVisible();
  await page.locator('.specialist-row summary').click();
  const panel = page.locator('.worker-controls');
  await panel.getByRole('textbox').fill('Include offline runtimes');
  await panel.getByRole('button', {name: 'Send instruction', exact: true}).click();
  await expect.poll(() => f.commands.length).toBe(1);
  expect(f.commands[0]).toMatchObject({kind: 'answer', payload: {question_id: 'question-one', content: 'Include offline runtimes'}});
  expect(f.worker.run_ids).toEqual(['run-old', 'run-child']);
});

test('worker activity groups follow-up runs and retries mailbox delivery with the exact identity', async ({page}) => {
  const f = await fixture(page, false, true);
  await page.goto('/ui/#/chat');
  await expect(page.locator('.specialist-row')).toHaveCount(1);
  await page.locator('.specialist-row summary').click();
  const panel = page.locator('.worker-controls');
  await expect(panel.getByRole('textbox')).toBeVisible();
  expect(f.commands).toHaveLength(0);
  await panel.getByRole('textbox').fill('Retain final citations');
  await panel.getByRole('button', {name: 'Send instruction', exact: true}).click();
  await expect(panel.getByRole('alert')).toBeVisible();
  await expect(panel.getByRole('textbox')).toHaveValue('Retain final citations');
  await panel.getByRole('button', {name: 'Send instruction', exact: true}).click();
  await expect.poll(() => f.commands.length).toBe(2);
  expect(f.commands[1]).toEqual(f.commands[0]);
  expect(f.mail).toHaveLength(1);
  await expect(panel.getByRole('textbox')).toHaveValue('');
  await expect(panel.getByRole('status')).toContainText('Instruction queued');
  f.mail[0].state = 'applied';
  await expect(panel.getByRole('status')).toContainText('Instruction applied');
  await panel.getByRole('button', {name: 'Stop', exact: true}).click();
  await expect.poll(() => f.worker.stopped).toBe(true);
  expect(f.root.status).toBe('waiting_for_children');
  expect(f.commands[2].target_id).toBe('worker-review');
  await page.setViewportSize({width: 390, height: 800});
  await panel.getByText('Activity details', {exact: true}).click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('stopped workers are only continued explicitly and recover a lost continuation receipt after reload', async ({page}) => {
  const f = await fixture(page, true, true);
  await page.goto('/ui/#/chat');
  await page.locator('.specialist-row summary').click();
  const panel = page.locator('.worker-controls');
  await expect(panel.getByRole('button', {name: 'Resume', exact: true})).toBeDisabled();
  await panel.getByRole('textbox').fill('Expand the remaining questions');
  await page.reload();
  await page.locator('.specialist-row summary').click();
  await expect(panel.getByRole('textbox')).toHaveValue('Expand the remaining questions');
  expect(f.commands).toHaveLength(0);
  await panel.getByRole('button', {name: 'Resume', exact: true}).click();
  await expect.poll(() => f.commands.length).toBe(1);
  expect(f.commands[0]).toMatchObject({kind: 'continue', target_id: 'worker-review', payload: {content: 'Expand the remaining questions'}});
  expect(f.worker.run_ids).toEqual(['run-old', 'run-child', 'run-followup']);
  await expect(panel.getByRole('alert')).toBeVisible();
  await page.reload();
  await page.locator('.specialist-row > details > summary').click();
  await expect(panel.getByRole('button', {name: 'Try again', exact: true})).toBeVisible();
  expect(f.commands).toHaveLength(1);
  await panel.getByRole('button', {name: 'Try again', exact: true}).click();
  await expect.poll(() => f.commands.length).toBe(2);
  expect(f.commands[1]).toEqual(f.commands[0]);
  expect(f.worker.run_ids).toHaveLength(3);
  await expect(page.locator('.specialist-row')).toHaveCount(1);
  await expect(panel.getByRole('textbox')).toHaveValue('');
});

test('completed parent work offers an explicit retained-worker follow-up without changing its recorded outcome', async ({page}) => {
  const f = await fixture(page, true);
  f.root.status = 'completed'; f.root.output = 'Previous completed outcome';
  f.worker.status = 'completed'; f.worker.stopped = false;
  await page.goto('/ui/#/chat');
  await page.locator('.specialist-row > details > summary').click();
  const panel = page.locator('.worker-controls');
  await expect(panel.getByRole('textbox')).toBeVisible();
  expect(f.commands).toHaveLength(0);
  await panel.getByRole('textbox').fill('Check the remaining assumption');
  await panel.getByRole('button', {name: 'Resume', exact: true}).click();
  await expect.poll(() => f.commands.length).toBe(1);
  expect(f.commands[0].target_id).toBe('worker-review');
  expect(f.root.output).toBe('Previous completed outcome');
});
