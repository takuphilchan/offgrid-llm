import {expect, test, type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {conversationInputText} from '../src/i18n/conversation-input';

test('canonical conversation visual evidence', async ({page}) => {
  test.skip(!process.env.OFFGRID_VISUAL_PHASE, 'Opt-in screenshot evidence');
  const f = await fixture(page);
  f.session.canonical_id = f.conversation.id;
  await page.route('**/api/v2/projects/default?revision=1', route => route.fulfill({json: {id: 'default', revision: 1, name: 'Private default project', preferences: {}}}));
  const dir = resolve('../../build/ui-craft', process.env.OFFGRID_VISUAL_PHASE!, 'canonical');
  mkdirSync(dir, {recursive: true});
  await page.goto('/ui/#/chat');
  for (const width of [1440, 390]) {
    await page.setViewportSize({width, height: 900});
    await expect(page.locator('.composer textarea')).toBeVisible();
    await page.screenshot({path: resolve(dir, `chat-${width}.png`)});
    await page.locator('.composer-context-menu > button').click();
    await page.locator('.conversation-settings > summary').click();
    await expect(page.locator('.conversation-settings')).toContainText('Private default project');
    await page.screenshot({path: resolve(dir, `settings-${width}.png`)});
    await page.locator('.conversation-settings > summary').click();
    await page.keyboard.press('Escape');
  }
  expect(f.commands).toHaveLength(0);
});

async function fixture(page: Page, loseAcknowledgment = false) {
  await craftFixture(page, 'en', 'dark');
  const model = 'Local-Research-7B-Instruct-Q4_K_M-long-model-name';
  const conversation: any = {id: 'conversation-fixture', revision: 1, project_id: 'default', title: 'Demo briefing', created_at: '2026-11-13T00:00:00Z', configuration_known: true, preferences: {model, restrictions: {computer: false, tools: []}}};
  const session: any = {name: 'Demo briefing', model_id: model, created_at: conversation.created_at, updated_at: conversation.created_at, messages: []};
  const inputs: any[] = [], commands: any[] = [], submissions: any[] = [];
  let lost = false;
  await page.route('**/api/v2/system', route => route.fulfill({json: {product: 'offgrid', workspace_id: 'craft', api_version: 2, capabilities: ['conversation-inputs-v1', 'conversation-input-admission-v1', 'agent-profiles-v1', 'agent-conversations-v1', 'native-execution-preview-v1']}}));
  await page.route('**/v1/sessions', route => route.fulfill({json: {sessions: [session]}}));
  await page.route('**/v1/sessions/Demo%20briefing', route => route.fulfill({json: session}));
  await page.route('**/api/v2/conversations**', async route => {
    const path = new URL(route.request().url()).pathname;
    if (route.request().method() === 'GET') {
      if (path.endsWith('/inputs')) return route.fulfill({json: {items: inputs, next_cursor: inputs.length, has_more: false}});
      if (path.endsWith('/configuration')) return route.fulfill({json: {...conversation.preferences, provenance: {}, resources: [], folder: conversation.folder, project_id: conversation.project_id, project_revision: 1}});
      return route.fulfill({json: conversation});
    }
    const command = route.request().postDataJSON(); commands.push(command);
    if (command.kind === 'import_conversation') {
      session.canonical_id = conversation.id; session.control_revision = conversation.revision;
      return route.fulfill({status: 202, json: {request_id: command.request_id, target_id: conversation.id, revision: conversation.revision, event_cursor: 1, conversation}});
    }
    if (command.kind === 'configure_conversation') {
      conversation.preferences = command.payload.preferences; conversation.revision++;
      return route.fulfill({status: 202, json: {request_id: command.request_id, target_id: conversation.id, revision: conversation.revision, event_cursor: 2, conversation}});
    }
    if (command.kind === 'use_project_folder' || command.kind === 'clear_project_folder') {
      conversation.folder = command.payload.folder ? {session: command.payload.folder.session, available: true, reason: 'Connected'} : undefined;
      if (conversation.folder) conversation.project_id = 'folder-project';
      conversation.revision++;
      return route.fulfill({status: 202, json: {request_id: command.request_id, target_id: conversation.id, revision: conversation.revision, event_cursor: 2, conversation}});
    }
    if (command.kind === 'submit_input') {
      submissions.push(command);
      let input = inputs.find(input => input.request_id === command.request_id);
      if (!input) {
        conversation.revision++;
        input = {id: `input-${inputs.length}`, sequence: inputs.length + 1, conversation_id: conversation.id, conversation_revision: conversation.revision, request_id: command.request_id, input: command.payload, state: 'accepted', accepted_at: new Date().toISOString()};
        inputs.push(input);
        session.messages.push({role: 'user', content: command.payload.content, turn_id: input.id, timestamp: input.accepted_at});
        if (command.payload.intent === 'answer') {
          input.state = 'responded'; input.incorporation = 'worker_answer'; input.applied_at = input.accepted_at;
          delete conversation.pending_question;
        }
      }
      if (loseAcknowledgment && !lost) {lost = true; return route.abort('failed');}
      return route.fulfill({status: 202, json: {request_id: command.request_id, target_id: conversation.id, revision: conversation.revision, event_cursor: 3, input}});
    }
    if (command.kind === 'stop') {
      const input = inputs.find(input => input.id === command.target_id);
      input.state = 'stopping'; input.stopped_at = new Date().toISOString(); conversation.revision++;
      return route.fulfill({status: 202, json: {request_id: command.request_id, target_id: conversation.id, revision: conversation.revision, event_cursor: 4, input}});
    }
    return route.fulfill({status: 400, json: {error: {message: 'Unexpected fixture command'}}});
  });
  return {inputs, commands, submissions, conversation, session};
}

test('conversation settings inspect pinned project and configuration without commands', async ({page}) => {
  const f = await fixture(page);
  f.session.canonical_id = f.conversation.id;
  await page.route('**/api/v2/projects/default?revision=1', route => route.fulfill({json: {id: 'default', revision: 1, name: 'Private default project', default: true, preferences: {restrictions: {}}}}));
  await page.goto('/ui/#/chat');
  await page.locator('.composer-context-menu > button').click();
  await page.locator('.conversation-settings summary').click();
  const settings = page.locator('.conversation-settings');
  await expect(settings).toContainText('Private default project');
  await expect(settings.locator('pre')).not.toBeVisible();
  await settings.locator('.conversation-configuration-details > summary').click();
  await expect(settings.locator('pre')).toContainText('"computer": false');
  expect(f.commands).toHaveLength(0);
  await page.setViewportSize({width: 390, height: 844});
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await settings.getByRole('button', {name: 'Refresh', exact: true}).click();
  await expect(settings).toContainText('Private default project');
  expect(f.commands).toHaveLength(0);
});

test('queued inputs keep the narrow composer visible and controls reachable', async ({page}) => {
  const f = await fixture(page);
  f.session.canonical_id = f.conversation.id;
  for (let index = 0; index < 20; index++) f.inputs.push({id: `queued-${index}`, sequence: index + 1, input: {content: `Pending instruction ${index}`}, state: 'accepted'});
  await page.setViewportSize({width: 390, height: 700});
  await page.goto('/ui/#/chat');
  const composer = page.locator('.composer');
  const activity = page.locator('.canonical-input-controls > .conversation-work');
  await expect(activity.getByRole('button', {name: 'Stop', exact: true})).toHaveCount(20);
  const box = await composer.boundingBox();
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.y + box!.height).toBeLessThanOrEqual(700);
  expect(await activity.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(f.commands).toHaveLength(0);
});

test('default conversation accepts follow-ups while inference waits and reload only observes', async ({page}) => {
  test.setTimeout(60_000); // Includes two cold fixture navigations on local Windows CI.
  const f = await fixture(page);
  await page.goto('/ui/#/chat', {waitUntil: 'domcontentloaded'});
  const composer = page.locator('.composer textarea');
  await composer.fill('Research the options');
  await page.getByRole('button', {name: 'Send', exact: true}).click();
  await expect.poll(() => f.inputs.length).toBe(1);
  await expect(composer).toHaveValue('');
  await composer.fill('Keep the original constraints');
  await page.getByRole('button', {name: 'Send', exact: true}).click();
  await expect.poll(() => f.inputs.length).toBe(2);
  expect(f.conversation.preferences.profile_id ?? '').toBe('');
  expect(f.conversation.preferences.restrictions).toEqual({computer: false, tools: []});
  expect(f.commands.filter(command => command.kind === 'import_conversation')).toHaveLength(1);
  const writes = f.commands.length;
  await page.reload({waitUntil: 'domcontentloaded'});
  await expect(composer).toBeEnabled();
  await expect(page.locator('.message.user')).toHaveCount(2);
  expect(f.commands).toHaveLength(writes);
  await page.getByRole('region', {name: 'Agent activity'}).getByRole('button', {name: 'Stop', exact: true}).first().click();
  await expect.poll(() => f.inputs[0].state).toBe('stopping');
  expect(f.inputs[1].state).toBe('accepted');
  expect(f.commands.filter(command => command.kind === 'stop')).toHaveLength(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

for (const loseAck of [false, true]) test(`composer answers a saved clarification without a model${loseAck ? ' and recovers after reload' : ''}`, async ({page}) => {
  const f = await fixture(page, loseAck);
  f.session.canonical_id = f.conversation.id;
  f.session.model_id = f.conversation.preferences.model = 'unavailable-model';
  f.conversation.focused_work_id = 'work-one';
  f.conversation.pending_question = {id: 'question-one', work_id: 'work-one', run_id: 'run-one', content: 'Should we include offline runtimes?'};
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer-question')).toHaveText('Should we include offline runtimes?');
  const composer = page.locator('.composer textarea'), submit = page.locator('.composer-submit');
  await composer.fill('Yes, include offline runtimes.');
  await expect(submit).toBeEnabled();
  await submit.click();
  await expect.poll(() => f.submissions.length).toBe(1);
  expect(f.submissions[0].payload).toMatchObject({intent: 'answer', question_id: 'question-one', focused_work_id: 'work-one', content: 'Yes, include offline runtimes.'});
  if (loseAck) {
    await expect(submit).toHaveAttribute('data-generating', 'false');
    f.conversation.pending_question = {id: 'question-two', work_id: 'work-one', run_id: 'run-one', content: 'A newer question'};
    await page.reload({waitUntil: 'domcontentloaded'});
    await expect(composer).toHaveValue('Yes, include offline runtimes.');
    await expect(submit).toBeEnabled();
    await submit.click();
    await expect.poll(() => f.submissions.length).toBe(2);
    expect(f.submissions[1]).toEqual(f.submissions[0]);
  }
  await expect(composer).toHaveValue('');
  expect(f.inputs).toHaveLength(1);
  expect(f.commands.every(command => command.kind === 'submit_input')).toBe(true);
});

test('lost acknowledgment retains the exact input envelope for an explicit retry', async ({page}) => {
  const f = await fixture(page, true);
  await page.goto('/ui/#/chat');
  const composer = page.locator('.composer textarea');
  await composer.fill('One request despite transport failure');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(1);
  await expect(composer).toHaveValue('One request despite transport failure');
  await expect(page.locator('.composer-submit')).toHaveAttribute('data-generating', 'false');
  const writes = f.commands.length;
  // Another client changes shared settings after the first input was accepted.
  f.conversation.preferences.delegation = 'off';
  f.conversation.preferences.model = 'another-qualified-model';
  f.conversation.revision++;
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(2);
  expect(f.submissions[1]).toEqual(f.submissions[0]);
  expect(f.inputs).toHaveLength(1);
  expect(f.commands.slice(writes).map(command => command.kind)).toEqual(['submit_input']);
  expect(f.conversation.preferences.model).toBe('another-qualified-model');
  await expect(composer).toHaveValue('');
});

for (const intent of ['message', 'new_work']) test(`pending clarification permits explicit ${intent} without answering it`, async ({page}) => {
  const f = await fixture(page);
  f.session.canonical_id = f.conversation.id;
  f.conversation.focused_work_id = 'work-one';
  f.conversation.pending_question = {id: 'question-one', work_id: 'work-one', run_id: 'run-one', content: 'Include offline runtimes?'};
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer-question')).toHaveText('Include offline runtimes?');
  await page.locator('.composer-context-menu > button').click();
  await page.getByRole('combobox', {name: 'Send as'}).selectOption(intent);
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill(intent === 'message' ? 'What does offline mean?' : 'Start a separate comparison.');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.inputs.length).toBe(1);
  expect(f.submissions[0].payload.intent).toBe(intent);
  expect(f.submissions[0].payload.question_id).toBeUndefined();
  expect(f.conversation.pending_question.id).toBe('question-one');
  await expect(page.locator('.composer-mode-override')).toHaveCount(0);
});

test('explicit new-task retry keeps its intent after reload', async ({page}) => {
  const f = await fixture(page, true);
  f.session.canonical_id = f.conversation.id;
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('combobox', {name: 'Send as'})).not.toBeVisible();
  await page.locator('.composer-context-menu > button').click();
  await page.getByRole('combobox', {name: 'Send as'}).selectOption('new_work');
  await page.keyboard.press('Escape');
  await expect(page.locator('.composer-mode-override')).toContainText('New task');
  await page.locator('.composer textarea').fill('A separate task.');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(1);
  await expect(page.locator('.composer-submit')).toHaveAttribute('data-generating', 'false');
  await page.reload();
  await expect(page.locator('.composer textarea')).toHaveValue('A separate task.');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(2);
  expect(f.submissions[1]).toEqual(f.submissions[0]);
  expect(f.submissions[1].payload.intent).toBe('new_work');
});

test('optional specialization preserves the explicitly selected conversation model on first admission', async ({page}) => {
  const f = await fixture(page);
  const selected = f.session.model_id;
  const profile = {id: 'writer', revision: 7, settings: {name: 'Writer', description: 'Write with evidence', model: 'different-profile-model', instructions: 'Write clearly.', max_iterations: 10, tools: [], knowledge: [], computer: false}};
  await page.route('**/api/v2/agent-profiles', route => route.fulfill({json: {profiles: [profile]}}));
  await page.goto('/ui/#/chat');
  await page.locator('.composer-context-menu > button').click();
  await page.getByRole('button', {name: 'Agent', exact: true}).click();
  await page.getByRole('radio', {name: /Writer Write with evidence/}).check();
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill('Keep my selected model');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.inputs.length).toBe(1);
  expect(f.conversation.preferences).toMatchObject({model: selected, profile_id: 'writer', profile_revision: 7, restrictions: {computer: false, tools: []}});
  expect(f.inputs[0].input.model).toBe(selected);
});

test('focused background work does not force every message into legacy steering', async ({page}) => {
  const f = await fixture(page);
  f.session.canonical_id = f.conversation.id;
  f.conversation.focused_work_id = 'work-fixture';
  const run = {run_id: 'run-background', task_id: 'run-background', prompt: 'Research the options', status: 'waiting_for_children', steps: [], output: '', event_cursor: '1', can_steer: false};
  f.session.turn = {id: 'work-input', job_id: run.run_id, prompt: run.prompt, status: run.status, job_admission: 'linked'};
  await page.route('**/api/v2/jobs/run-background**', route => route.fulfill(new URL(route.request().url()).pathname.endsWith('/events') ? {contentType: 'text/event-stream', body: `data: ${JSON.stringify({...run,type:'snapshot'})}\n\n`} : {json: run}));
  await page.goto('/ui/#/chat');
  await expect(page.locator('.conversation-work')).toBeVisible();
  await page.locator('.composer textarea').fill('What does that term mean?');
  await expect(page.locator('.composer-submit')).toBeEnabled();
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.inputs.length).toBe(1);
  expect(f.inputs[0].input.content).toBe('What does that term mean?');
  expect(f.commands.some(command => command.kind === 'create_work' || command.action === 'steer')).toBe(false);
  expect(f.conversation.focused_work_id).toBe('work-fixture');
});

test('approved folder persists without a profile and reload restores it without a command', async ({page}) => {
  const f = await fixture(page);
  f.conversation.preferences.restrictions = {computer: false};
  const resource = {project_context: true, reference: {host: 'project-host', session: 'folder-session', grant: 'local-grant', revision: 1}, backend: 'local', root: '/approved/project', directory: '.', permissions: {read: true, write: false, processes: false}};
  await page.route('**/api/v2/execution/workspaces', route => route.fulfill({json: {protocol: 1, workspaces: [resource]}}));
  await page.goto('/ui/#/chat');
  const selector = page.locator('.chat-execution-workspace');
  await selector.locator('button').first().click();
  await page.getByRole('radio', {name: /project-host/}).check();
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill('Use this project');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.inputs.length).toBe(1);
  expect(f.commands.find(command => command.kind === 'use_project_folder').payload.folder).toEqual(resource.reference);
  expect(f.conversation.project_id).toBe('folder-project');
  expect(f.conversation.preferences.profile_id ?? '').toBe('');
  const writes = f.commands.length;
  await page.reload();
  await expect(selector).toContainText('project-host');
  expect(f.commands).toHaveLength(writes);
  await selector.locator('button').first().click();
  await page.getByRole('radio', {name: 'No host folder', exact: true}).check();
  await page.keyboard.press('Escape');
  await page.locator('.composer textarea').fill('Continue without this folder');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.inputs.length).toBe(2);
  expect(f.commands.filter(command => command.kind === 'clear_project_folder')).toHaveLength(1);
  expect(f.conversation.project_id).toBe('folder-project');
});

test('folder context is not mutated again when retrying an accepted input', async ({page}) => {
  const f = await fixture(page, true);
  f.session.canonical_id = f.conversation.id;
  f.conversation.preferences.restrictions = {computer: false};
  f.conversation.folder = {session: 'folder-session', available: true, reason: 'Connected'};
  const resource = {project_context: true, reference: {host: 'project-host', session: 'folder-session', grant: 'local-grant', revision: 1}, backend: 'local', root: '/approved/project', directory: '.', permissions: {read: true, write: false, processes: false}};
  await page.route('**/api/v2/execution/workspaces', route => route.fulfill({json: {protocol: 1, workspaces: [resource]}}));
  await page.goto('/ui/#/chat');
  await expect(page.locator('.chat-execution-workspace')).toContainText('project-host');
  await page.locator('.composer textarea').fill('Use the same accepted work');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(1);
  await expect(page.locator('.composer-submit')).toHaveAttribute('data-generating', 'false');
  // A choice made after the unknown outcome belongs to the next new input.
  // Observing the old receipt must neither apply nor discard that choice.
  await page.locator('.chat-execution-workspace button').first().click();
  await page.getByRole('radio', {name: 'No host folder', exact: true}).check();
  await page.keyboard.press('Escape');
  const writes = f.commands.length;
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(2);
  expect(f.submissions[1]).toEqual(f.submissions[0]);
  expect(f.commands.slice(writes).map(command => command.kind)).toEqual(['submit_input']);
  await page.locator('.composer textarea').fill('Now clear the selected folder');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.inputs.length).toBe(2);
  expect(f.commands.filter(command => command.kind === 'clear_project_folder')).toHaveLength(1);
});
