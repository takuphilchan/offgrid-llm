// Opt-in integration check: real model + service + Chromium, owned fixture only.
// This is NOT a general-purpose unattended companion or a platform qualification.
import assert from 'node:assert/strict';
import { setTimeout as delay } from 'node:timers/promises';
import { BrowserDriver } from '../browser.mjs';
import { startDemo, DEMO_ORIGIN } from '../demo.mjs';

const [model, consent, scenario] = process.argv.slice(2);
if (!model || consent !== '--approve-owned-fixture-only' || (scenario && scenario !== '--revisions')) {
  console.error('Usage: node test/live-demo.mjs MODEL --approve-owned-fixture-only [--revisions]');
  process.exit(2);
}
const service = 'http://127.0.0.1:11611';
const titles = scenario === '--revisions' ? ['Zimbabwe research — preliminary', 'Zimbabwe research — final (2026)'] : ['OffGrid test'];
const title = titles.at(-1);
const expected = `Draft saved: ${title}`;
let driver, demo, token, session, run, transport, stopped = false, transportError;
const dispatched = new Set(), approved = new Set(), actions = [];
async function request(path, body, companion = false, timeout = 20000) {
  const response = await fetch(service + path, {
    method: body === undefined ? 'GET' : 'POST', redirect: 'error',
    headers: { 'Content-Type': 'application/json', ...(companion && token ? { Authorization: `Bearer ${token}` } : {}) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    signal: AbortSignal.timeout(timeout),
  });
  if (!response.ok) throw Error(`${path}: HTTP ${response.status}`);
  return response.json();
}
const companion = (path, body = {}) => request(`/api/v2/computer/companion/${path}`, body, true);

// Check both before approval and before execution. Only the fixture's exact
// changes are eligible; navigation and every other mutation are refused.
async function validateAction(kind, args) {
  if (kind === 'browser_observe') { assert.deepEqual(args, {}); return; }
  if (kind === 'browser_verify') {
    assert.ok(titles.some(value => args.text === `Draft saved: ${value}`), 'Unrelated verification refused');
    assert.deepEqual(args, { text: args.text });
    assert.equal(await driver.page.locator('#status').innerText(), args.text);
    return;
  }
  assert.ok(['browser_fill', 'browser_click'].includes(kind), 'Unexpected tool refused');
  const changes = actions.filter(action => ['browser_fill','browser_click'].includes(action)).length;
  assert.ok(changes < titles.length * 2, 'Extra mutation refused');
  assert.equal(kind, changes % 2 === 0 ? 'browser_fill' : 'browser_click', 'Wrong action order refused');
  const nextTitle = titles[Math.floor(changes / 2)];
  assert.equal(args.observation_id, driver.observation, 'Stale approval refused');
  const handle = driver.elements.get(args.element);
  assert.ok(handle, 'Unknown element refused');
  const target = await handle.evaluate(el => ({ id: el.id, tag: el.tagName }));
  if (kind === 'browser_fill') {
    assert.deepEqual(target, { id: 'report', tag: 'INPUT' });
    assert.deepEqual(args, { observation_id: driver.observation, element: args.element, text: nextTitle });
  } else {
    assert.deepEqual(target, { id: 'save', tag: 'BUTTON' });
    assert.deepEqual(args, { observation_id: driver.observation, element: args.element });
    assert.equal(await driver.page.locator('#report').inputValue(), nextTitle);
  }
}

try {
  assert.equal((await request('/api/v2/computer/sessions')).sessions.length, 0, 'Do not disturb an active browser session');
  const check = await request('/api/v2/computer/model-check', { model }, false, 120000);
  console.log(JSON.stringify({ stage: 'model-check', ...check }));
  assert.equal(check.passed, true, 'Model check failed; no browser was paired');
  demo = await startDemo();
  driver = await BrowserDriver.open(demo.origin, { headless: true, testLoopback: true });
  const { code } = await request('/api/v2/computer/pairing', {});
  const paired = await companion('pair', { code, origin: DEMO_ORIGIN, protocol_version: 1 });
  token = paired.token; session = paired.session.id;
  transport = (async () => {
    while (!stopped) {
      await companion('heartbeat');
      const { action } = await companion('poll');
      if (action) {
        assert.ok(!dispatched.has(action.id), 'Duplicate dispatch refused');
        dispatched.add(action.id);
        await validateAction(action.kind, action.arguments);
        const result = await driver.execute(action.kind, action.arguments);
        actions.push(action.kind);
        console.log(JSON.stringify({ stage: 'action', kind: action.kind, verified: result.verified }));
        await companion('reply', { id: action.id, result: JSON.stringify(result) });
      }
      await delay(250);
    }
  })().catch(error => { transportError = error; });
  run = await request('/v1/agents/run', {
    model, computer_session: session, style: 'react', async: true, max_iterations: 20,
    prompt: scenario === '--revisions'
      ? titles.map(value => `Inspect the page. Set Report title to exactly ${value}. Inspect again, click Save draft, then verify the page says Draft saved: ${value}.`).join(' Next: ') + ' Do these steps in order. Do not navigate elsewhere.'
      : `Set Report title to ${title} and save the draft.`,
  }, false, 120000);
  console.log(JSON.stringify({ stage: 'submitted', run_id: run.run_id }));
  const deadline = Date.now() + 7 * 60 * 1000;
  while (Date.now() < deadline) {
    if (transportError) throw transportError;
    run = await request(`/v1/agents/tasks/${run.run_id}`);
    if (run.status === 'waiting_for_approval') {
      const approval = run.pending_approval;
      assert.equal(approval.run_id, run.run_id);
      assert.ok(!approved.has(approval.id), 'Duplicate approval refused');
      assert.ok(approved.size < titles.length * 2, 'More than the fixture changes requested');
      assert.ok(['browser_fill', 'browser_click'].includes(approval.tool));
      await validateAction(approval.tool, approval.arguments);
      approved.add(approval.id);
      console.log(JSON.stringify({ stage: 'approve-owned-fixture', kind: approval.tool }));
      await request(`/v1/agents/tasks/${run.run_id}/approve`, { approval_id: approval.id, async: true });
    } else if (!['pending', 'running'].includes(run.status)) break;
    await delay(500);
  }
  assert.equal(run.status, 'completed', run.error || 'Task did not complete within budget');
  assert.equal(approved.size, titles.length * 2);
  assert.equal(await driver.page.locator('#report').inputValue(), title);
  assert.equal(await driver.page.locator('#status').innerText(), expected);
  assert.equal(actions.at(-1), 'browser_verify');
  console.log(JSON.stringify({ passed: true, model, run_id: run.run_id, approvals: approved.size, actions,
    verification: 'Independent DOM confirms the temporary draft; no file or external submission.' }));
} catch (error) {
  console.error(JSON.stringify({ passed: false, run_id: run?.run_id, error: error.message }));
  process.exitCode = 1;
} finally {
  stopped = true;
  await transport;
  if (run?.run_id && !['completed', 'failed', 'cancelled'].includes(run.status)) {
    await request(`/v1/agents/tasks/${run.run_id}/cancel`, {}).catch(() => {});
  }
  if (session) {
    const sessions = await request('/api/v2/computer/sessions').catch(() => ({ sessions: [] }));
    if (sessions.sessions.some(item => item.id === session)) await request('/api/v2/computer/stop', {}).catch(() => {});
  }
  await driver?.close();
  await demo?.close();
}
