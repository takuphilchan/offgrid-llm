import {expect, test, type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';
import {mkdirSync} from 'node:fs';
import {resolve} from 'node:path';
import {conversationInputText} from '../src/i18n/conversation-input';
import {readerOptions} from './helpers/reader-options';

async function openFolderChoices(page:Page){
  await page.locator('.composer-attachments-menu > button').click();
  await page.locator('.chat-execution-workspace > button').click();
}

for (const stale of [false, true]) test(`document handoff preserves draft without writing to a canonical conversation${stale ? ' after a stale-client conflict' : ''}`, async ({page}) => {
  const f = await fixture(page);
  if (!stale) f.session.canonical_id = f.conversation.id;
  f.session.messages.push({role: 'user', content: 'Keep this original conversation'});
  await page.route('**/api/v2/system', route => route.fulfill({json: {product: 'offgrid', workspace_id: 'craft', api_version: 2, capabilities: ['conversation-inputs-v1', 'conversation-input-admission-v1', 'session-files-v1']}}));
  const created: any[] = [], uploads: string[] = [];
  await page.addInitScript(() => {
    const fetch = window.fetch.bind(window);
    window.fetch = async (input, init) => {
      if (String(input).endsWith('/generate')) (window as any).handoffPayload = JSON.parse(String(init?.body));
      return fetch(input, init);
    };
  });
  await page.route('**/v1/sessions', route => {
    if (route.request().method() === 'GET') return route.fulfill({json: {sessions: [f.session, ...created]}});
    const body = route.request().postDataJSON();
    const session = {...body, messages: [], created_at: f.session.created_at};
    created.push(session); return route.fulfill({json: session});
  });
  await page.route('**/v1/sessions/*/attachments', route => {
    if (route.request().method() === 'GET') return route.fulfill({json: {attachments: []}});
    const name = decodeURIComponent(new URL(route.request().url()).pathname.split('/')[3]); uploads.push(name);
    if (name === f.session.name) return route.fulfill({status: 409, json: {error_code: 'canonical_conversation_required', error: {message: 'legacy writes cannot start another executor'}}});
    return route.fulfill({json: {attachment: {id: 'file-' + 'a'.repeat(32), name: 'policy.txt', media_type: 'text/plain', bytes: 20, sha256: 'b'.repeat(64), revision: 'c'.repeat(64), status: 'ready'}}});
  });
  if (stale) await page.setViewportSize({width: 390, height: 844});
  await page.goto('/ui/#/chat');
  const composer = page.locator('.composer textarea');
  await expect(composer).toBeEnabled(); await composer.fill('Review this policy');
  await page.locator('input[data-attachment=files]').setInputFiles({name: 'policy.txt', mimeType: 'text/plain', buffer: Buffer.from('Policy requirements.')});
  const handoff = page.locator('.file-conversation-handoff');
  await expect(handoff).toContainText(conversationInputText.en);
  await expect(handoff).toContainText('policy.txt');
  await expect(page.locator('.composer .file-conversation-handoff')).toHaveCount(0);
  expect((await page.locator('.composer').boundingBox())!.height).toBeLessThan(200);
  await expect(page.getByRole('button', {name: 'Send', exact: true})).toBeDisabled();
  await composer.press('Enter');
  await expect(composer).toHaveValue('Review this policy');
  await expect(page.locator('.composer')).not.toContainText('legacy writes');
  expect(created).toHaveLength(0); expect(uploads).toHaveLength(stale ? 1 : 0); expect(f.commands).toHaveLength(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({path: resolve('../../build/ui-craft/canonical-document-handoff', stale ? 'narrow.png' : 'desktop.png')});
  await handoff.getByRole('button', {name: 'New chat', exact: true}).click();
  await expect(page.locator('.composer .file-attachments')).toContainText('policy.txt');
  await expect(composer).toHaveValue('Review this policy');
  await expect(handoff).toHaveCount(0);
  await expect(page.locator('.message.user')).toHaveCount(0);
  expect(created).toHaveLength(1); expect(uploads.at(-1)).toBe(created[0].name);
  expect(f.session.messages).toEqual([{role: 'user', content: 'Keep this original conversation'}]);
  expect(f.commands).toHaveLength(0); expect(f.submissions).toHaveLength(0);
  await page.getByRole('button', {name: 'Send', exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as any).handoffPayload?.attachments?.length)).toBe(1);
  expect(f.commands).toHaveLength(0); expect(f.submissions).toHaveLength(0);
});

test('cancelling a canonical document handoff keeps the draft and performs no writes', async ({page}) => {
  const f = await fixture(page); f.session.canonical_id = f.conversation.id;
  await page.route('**/api/v2/system', route => route.fulfill({json: {product: 'offgrid', workspace_id: 'craft', capabilities: ['conversation-input-admission-v1', 'session-files-v1']}}));
  const writes: string[] = [];
  page.on('request', request => {if (request.method() !== 'GET') writes.push(request.url());});
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep my question');
  await page.locator('input[data-attachment=files]').setInputFiles({name: 'policy.txt', mimeType: 'text/plain', buffer: Buffer.from('Policy requirements.')});
  await page.locator('.file-conversation-handoff').getByRole('button', {name: 'Cancel', exact: true}).click();
  await expect(page.locator('.file-conversation-handoff')).toHaveCount(0);
  await expect(page.locator('.composer textarea')).toHaveValue('Keep my question');
  await expect(page.locator('.composer textarea')).toBeFocused();
  expect(writes).toHaveLength(0); expect(f.commands).toHaveLength(0);
});

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

const defaultInputCapabilities = ['conversation-inputs-v1', 'conversation-input-admission-v1', 'conversation-input-options-v1', 'conversation-source-inputs-v1', 'session-files-v1'];

test('default canonical admission preserves response options across a lost receipt and reload', async ({page}) => {
  const f = await fixture(page, true);
  await page.route('**/api/v2/system', route => route.fulfill({json: {product: 'offgrid', workspace_id: 'craft', capabilities: defaultInputCapabilities}}));
  await page.addInitScript(() => localStorage.setItem('offgrid.chat.preferences:local', JSON.stringify({profile: 'extended', maxTokens: 256})));
  let legacyWrites = 0;
  page.on('request', request => {if (/\/(generate|agent-turn)$/.test(new URL(request.url()).pathname)) legacyWrites++;});
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Keep my response settings');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(1);
  expect(f.submissions[0].payload).toMatchObject({content: 'Keep my response settings', response_profile: 'extended', max_tokens: 256});
  expect(f.conversation.preferences.profile_id ?? '').toBe('');
  await expect(page.locator('.composer-submit')).toHaveAttribute('data-generating', 'false');
  await page.reload();
  await expect(page.locator('.composer textarea')).toHaveValue('Keep my response settings');
  await page.locator('.composer-submit').click();
  await expect.poll(() => f.submissions.length).toBe(2);
  expect(f.submissions[1]).toEqual(f.submissions[0]);
  expect(f.inputs).toHaveLength(1);
  expect(legacyWrites).toBe(0);
});

for (const optionsSupported of [true, false]) test(`canonical document-only input requires request-format support: ${optionsSupported}`, async ({page}) => {
  const f = await fixture(page); f.session.canonical_id = f.conversation.id;
  await page.route('**/api/v2/system', route => route.fulfill({json: {product: 'offgrid', workspace_id: 'craft', capabilities: defaultInputCapabilities.filter(capability => optionsSupported || capability !== 'conversation-input-options-v1')}}));
  const attachment = {id: 'file-' + 'a'.repeat(32), name: 'policy.txt', media_type: 'text/plain', bytes: 24, sha256: 'b'.repeat(64), revision: 'c'.repeat(64), status: 'ready'};
  await page.route('**/v1/sessions/*/attachments', route => route.fulfill({json: route.request().method() === 'GET' ? {attachments: []} : {attachment}}));
  await page.goto('/ui/#/chat');
  await expect(page.locator('.composer textarea')).toBeEnabled();
  await page.locator('input[data-attachment=files]').setInputFiles({name: 'policy.txt', mimeType: 'text/plain', buffer: Buffer.from('Shipment 12345 is exempt.')});
  await expect(page.locator('.composer .file-attachments')).toContainText('policy.txt');
  if (!optionsSupported) {
    await expect(page.getByRole('button', {name: 'Send', exact: true})).toBeDisabled();
    expect(f.submissions).toHaveLength(0);
    return;
  }
  await page.getByRole('button', {name: 'Send', exact: true}).click();
  await expect.poll(() => f.submissions.length).toBe(1);
  expect(f.submissions[0].payload).toMatchObject({content: '', source_session: f.session.name, attachments: [{id: attachment.id, revision: attachment.revision}], response_profile: 'interactive'});
  expect(f.submissions[0].payload.intent).not.toBe('new_work');
});

test('disabled default admission retains the draft without invoking legacy generation', async ({page}) => {
  const f = await fixture(page);
  await page.route('**/api/v2/system', route => route.fulfill({json: {product: 'offgrid', workspace_id: 'craft', capabilities: defaultInputCapabilities.filter(capability => capability !== 'conversation-input-admission-v1')}}));
  let rejected = 0, legacyWrites = 0;
  await page.route('**/api/v2/conversations', route => {rejected++; return route.fulfill({status: 503, json: {error: {message: 'New conversation admission is disabled'}}});});
  page.on('request', request => {if (/\/(generate|agent-turn)$/.test(new URL(request.url()).pathname)) legacyWrites++;});
  await page.goto('/ui/#/chat');
  await page.locator('.composer textarea').fill('Retain my message');
  await page.locator('.composer-submit').click();
  await expect.poll(() => rejected).toBe(1);
  await expect(page.locator('.composer textarea')).toHaveValue('Retain my message');
  await expect(page.locator('.composer-submit')).toHaveAttribute('data-generating', 'false');
  expect(legacyWrites).toBe(0); expect(f.submissions).toHaveLength(0);
});

for(const mode of ['live','buffered','restricted','revoked','reconnected','denied'] as const) test(`canonical response display observes saved work: ${mode}`,async({page})=>{
 const f=await fixture(page);f.session.canonical_id=f.conversation.id;
 f.inputs.push({id:'live-input',sequence:1,conversation_id:f.conversation.id,conversation_revision:1,request_id:'live-request',input:{content:'Explain this idea'},state:'accepted',run_id:'live-run',run_status:'running',accepted_at:new Date().toISOString()});
 const snapshot={run_id:'live-run',status:'running',steps:[],output:'',event_cursor:'1',response_preview_allowed:mode!=='restricted',progress:{phase:'generating',preview:'A provisional explanation',iteration:1}};
 let denied=false;
 await page.route('**/api/v2/jobs/live-run',route=>denied?route.fulfill({status:403,json:{error:{message:'Access removed'}}}):route.fulfill({json:snapshot}));
 await page.addInitScript(({snapshot,mode})=>{
  localStorage.setItem('offgrid.chat.preferences:local',JSON.stringify({profile:'interactive',maxTokens:1024,streamResponses:mode!=='buffered'}));
  const original=window.fetch.bind(window);
  window.fetch=async(input,init)=>{
   if(!String(input).endsWith('/live-run/events'))return original(input,init);
   (window as any).liveConnections=((window as any).liveConnections??0)+1;
   return new Response(new ReadableStream({start(controller){
    const emit=(value:any)=>controller.enqueue(new TextEncoder().encode(`data: ${JSON.stringify(value)}\n\n`));
    emit(snapshot);(window as any).emitLiveRun=emit;(window as any).disconnectLiveRun=()=>controller.close();
    init?.signal?.addEventListener('abort',()=>controller.error(new DOMException('Aborted','AbortError')),{once:true});
   }}),{headers:{'Content-Type':'text/event-stream'}});
  };
 },{snapshot,mode});
 await page.goto('/ui/#/chat');await expect(page.locator('.canonical-response-status')).toBeVisible();
 if(mode==='live'||mode==='revoked'||mode==='reconnected'||mode==='denied') {
  await expect(page.locator('.canonical-live-response')).toContainText('A provisional explanation');
  await expect.poll(()=>page.evaluate(()=>(window as any).liveConnections)).toBe(1);
  if(mode==='reconnected'||mode==='denied') {
   snapshot.event_cursor='2';snapshot.progress.preview='Recovered provisional explanation';denied=mode==='denied';
   await page.evaluate(()=>(window as any).disconnectLiveRun());
   if(denied)await expect(page.locator('.canonical-live-response')).toHaveCount(0);
   else {await expect.poll(()=>page.evaluate(()=>(window as any).liveConnections)).toBe(2);await expect(page.locator('.canonical-live-response')).toContainText('Recovered provisional explanation');}
  } else if(mode==='revoked') {
   await page.evaluate(snapshot=>(window as any).emitLiveRun({...snapshot,event_cursor:'2',response_preview_allowed:false}),snapshot);
   await expect(page.locator('.canonical-live-response')).toHaveCount(0);
  } else {
   await page.evaluate(snapshot=>(window as any).emitLiveRun({...snapshot,event_cursor:'2',status:'completed',output:'Saved final answer'}),snapshot);
   await expect(page.locator('.canonical-live-response')).toHaveCount(0);
  }
 } else {
  if(mode==='restricted')await expect.poll(()=>page.evaluate(()=>(window as any).liveConnections)).toBe(1);
  await expect(page.locator('.canonical-live-response')).toHaveCount(0);
 }
 expect(f.commands).toHaveLength(0);expect(f.submissions).toHaveLength(0);
});

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
  const activity = page.locator('.canonical-response-status');
  await expect(page.locator('.composer').getByRole('button', {name: 'Stop', exact: true})).toHaveCount(1);
  await expect(activity.locator('.canonical-response-summary')).toContainText('20');
  const box = await composer.boundingBox();
  expect(box!.y).toBeGreaterThanOrEqual(0);
  expect(box!.y + box!.height).toBeLessThanOrEqual(700);
  expect((await activity.boundingBox())!.height).toBeLessThan(80);
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
  await page.locator('.composer').getByRole('button', {name: 'Stop', exact: true}).click();
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
  await openFolderChoices(page);
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
  await openFolderChoices(page);
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
  await openFolderChoices(page);
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


test('canonical source capability admits a pinned document without a new conversation',async({page})=>{
 const f=await fixture(page);f.session.canonical_id=f.conversation.id;
 await page.route('**/api/v2/system',route=>route.fulfill({json:{product:'offgrid',workspace_id:'craft',api_version:2,capabilities:['conversation-inputs-v1','conversation-input-admission-v1','conversation-source-inputs-v1','session-files-v1']}}));
 const attachment={id:'file-'+'a'.repeat(32),name:'policy.txt',media_type:'text/plain',bytes:40,sha256:'b'.repeat(64),revision:'c'.repeat(64),status:'ready'};
 await page.route('**/v1/sessions/*/attachments',route=>route.fulfill({json:route.request().method()==='GET'?{attachments:[]}:{attachment}}));
 await page.goto('/ui/#/chat');
 const composer=page.locator('.composer textarea');await expect(composer).toBeEnabled();await composer.fill('Which exception applies?');
 await page.locator('input[data-attachment=files]').setInputFiles({name:'policy.txt',mimeType:'text/plain',buffer:Buffer.from('Shipment 12345 is exempt.')});
 await expect(page.locator('.file-conversation-handoff')).toHaveCount(0);
 await expect(page.locator('.composer .file-attachments')).toContainText('policy.txt');
 await page.getByRole('button',{name:'Send',exact:true}).click();
 await expect.poll(()=>f.submissions.length).toBe(1);
 expect(f.submissions[0].payload.attachments).toEqual([{id:attachment.id,revision:attachment.revision}]);
 expect(f.submissions[0].payload.source_session).toBe(f.session.name);
 expect(f.commands.filter(c=>c.kind==='import_conversation')).toHaveLength(0);
 await expect(page.locator('.canonical-response-summary')).toBeVisible();
 await expect(page.locator('.canonical-response-summary')).not.toContainText('Which exception applies?');
 await expect(page.locator('.composer-submit')).toHaveAttribute('aria-label','Stop');
});


test('page selection is explicit and survives reader navigation before submission',async({page})=>{
 const f=await fixture(page);f.session.canonical_id=f.conversation.id;
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['conversation-inputs-v1','conversation-input-admission-v1','conversation-source-inputs-v1','session-files-v1']}}));
 const file={id:'file-'+'a'.repeat(32),name:'policy.pdf',sha256:'b'.repeat(64),revision:'c'.repeat(64),media_type:'application/pdf',bytes:100,status:'ready'};
 await page.route('**/v1/sessions/*/attachments',r=>r.fulfill({json:r.request().method()==='GET'?{attachments:[file]}:{attachment:file}}));
 await page.route('**/attachments/*/preview*',r=>new URL(r.request().url()).searchParams.has('page')?r.fulfill({status:503,json:{error:{message:'Fixture raster unavailable'}}}):r.fulfill({json:{name:file.name,sha256:file.sha256,format:'pdf',pages:2,state:'complete',units:[1,2].map(index=>({kind:'page',index,text:'Evidence on page '+index,state:'ready',source_sha256:file.sha256,extractor_revision:'fixture'}))}}));
 await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Explain the selected page');
 await page.locator('input[data-attachment=files]').setInputFiles({name:file.name,mimeType:'application/pdf',buffer:Buffer.from('%PDF-fixture')});
 await page.getByRole('button',{name:'Open document',exact:true}).click();
 const reader=page.locator('.document-reader');await reader.getByRole('spinbutton').fill('2');await reader.getByRole('spinbutton').press('Enter');
 await reader.getByRole('button',{name:'Ask about this page',exact:true}).click();
 await expect(page.locator('.composer-source-scope')).toContainText('Page 2');
 await reader.getByRole('spinbutton').fill('1');await reader.getByRole('spinbutton').press('Enter');
 await page.locator('.composer').getByRole('button',{name:'Send',exact:true}).click();
 await expect.poll(()=>f.submissions.length).toBe(1);
 expect(f.submissions[0].payload.attachments).toEqual([{id:file.id,revision:file.revision,page:2}]);
});

for(const capable of [true,false])test('Library block scope pins evidence and preserves draft '+capable,async({page})=>{
 const f=await fixture(page);f.session.canonical_id=f.conversation.id;
 await page.route('**/api/v2/system',r=>r.fulfill({json:{product:'offgrid',workspace_id:'craft',capabilities:['conversation-inputs-v1','conversation-input-admission-v1','conversation-source-inputs-v1','scoped-knowledge-chat-v1',...(capable?['conversation-source-scopes-v1']:[])]}}));
 const doc={id:'policy',name:'Policy.txt',size:40,chunk_count:1,index_status:'ready',source_retained:true,original_retained:false,extraction_revision:'extraction-original'};
 await page.route('**/v1/documents',r=>r.fulfill({json:{documents:[doc]}}));
 await page.route('**/v1/documents/source?*',r=>r.fulfill({json:{document:doc,content:'Selected exception.',truncated:false}}));
 await page.route('**/v1/documents/structure?*',r=>r.fulfill({json:{document_id:'policy',revision:'extraction-original',offset:0,total_blocks:1,next_offset:null,can_correct:false,units:[{id:'section-1',kind:'section',index:1}],blocks:[{id:'exception',unit_id:'section-1',kind:'paragraph',text:'Selected exception.',text_sha256:'a'.repeat(64),editable:false,truncated:false,metadata_omitted:false}],relationships:[]}}));
 await page.goto('/ui/#/chat');await page.locator('.composer textarea').fill('Explain this exception');
 await page.evaluate(()=>location.hash='#/knowledge');
 const {workflow}=await import('../src/i18n/workflow'),{documentReview}=await import('../src/i18n/document-review');
 await page.getByRole('button',{name:workflow.en.viewSource,exact:true}).click();
 const reader=page.locator('.document-source-dialog');await reader.getByRole('button',{name:documentReview('en').title,exact:true}).click();
 const select=reader.locator('[data-block-id="exception"]').getByRole('button',{name:'Ask about this selection',exact:true});
 if(!capable){await expect(select).toHaveCount(0);expect(f.submissions).toHaveLength(0);return;}
 await select.click();await expect(page.locator('.composer textarea')).toHaveValue('Explain this exception');await expect(page.locator('.composer-source-scope')).toContainText('Selected exception.');
 await page.reload();await expect(page.locator('.composer textarea')).toHaveValue('Explain this exception');await expect(page.locator('.composer-source-scope')).toContainText('Selected exception.');
 expect(f.submissions).toHaveLength(0);await page.getByRole('button',{name:'Send',exact:true}).click();await expect.poll(()=>f.submissions.length).toBe(1);
 expect(f.submissions[0].payload.knowledge_selections).toEqual([{document_id:'policy',revision:'extraction-original',block_ids:['exception']}]);expect(f.submissions[0].payload.knowledge_documents).toBeUndefined();expect(f.submissions[0].payload.use_knowledge).toBeUndefined();
});
