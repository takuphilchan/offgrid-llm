const { test } = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { DesktopRuntime } = require('../runtime');

function fixture(inspect, extra = {}) {
  const calls = { spawn: [], mkdir: [], kill: [] };
  const child = new EventEmitter();
  child.stdout = child.stderr = { resume() {} };
  child.kill = signal => { calls.kill.push(signal); queueMicrotask(() => child.emit('exit', 0)); };
  const runtime = new DesktopRuntime({
    url: 'http://127.0.0.1:11611', version: '0.4.4', uiBuildID: 'a'.repeat(64), binary: '/app/offgrid', uiDir: '/app/ui',
    workspace: { models: '/original/models', data: '/original/data' },
    isolatedWorkspace: { models: '/separate/models', data: '/separate/data' },
    timeoutMs: 35, retryMs: 2, availablePort: async () => 34567, inspect,
    fs: { access: async () => {}, mkdir: async p => calls.mkdir.push(p) },
    spawn: (...args) => { calls.spawn.push(args); return child; }, ...extra
  });
  return { runtime, calls, child };
}

test('an incompatible external service is never stopped, modified, or replaced', async () => {
  const result = { state: 'incompatible', version: '0.4.3-history-dev', canOpenBrowser: true };
  const { runtime, calls } = fixture(async () => result);
  assert.equal((await runtime.connect()).state, 'incompatible');
  assert.equal(runtime.snapshot().version, result.version);
  await runtime.stop();
  assert.deepEqual(calls, { spawn: [], mkdir: [], kill: [] });
});

test('concurrent retries share one probe; passive status reads do not probe', async () => {
  let probes = 0;
  let release;
  const { runtime, calls } = fixture(() => { probes++; return new Promise(resolve => { release = resolve; }); });
  const first = runtime.connect();
  assert.equal(runtime.connect(), first);
  for (let i = 0; i < 20; i++) runtime.snapshot();
  assert.equal(probes, 1);
  release({ state: 'ready', version: '0.4.4' });
  await first;
  await runtime.stop();
  assert.equal(calls.spawn.length, 0);
});

test('explicit separate workspace uses another port, isolated data and a hidden loopback child', async () => {
  let probes = 0;
  const { runtime, calls } = fixture(async () => ++probes === 1 ? { state: 'offline' } : { state: 'ready' });
  runtime.state.state = 'incompatible';
  assert.equal((await runtime.connect(true)).state, 'ready');
  assert.equal(runtime.url, 'http://127.0.0.1:34567');
  assert.equal(calls.spawn.length, 1);
  const options = calls.spawn[0][2];
  assert.equal(options.windowsHide, true);
  assert.equal(options.env.OFFGRID_HOST, '127.0.0.1');
  assert.equal(options.env.OFFGRID_DATA_DIR, '/separate/data');
  assert.deepEqual(calls.mkdir, ['/separate/models', '/separate/data']);
  await runtime.stop();
  assert.deepEqual(calls.kill, ['SIGTERM']);
});

test('timeout stays recoverable and retry never spawns a duplicate child', async () => {
  let ready = false;
  const { runtime, calls } = fixture(async () => ({ state: ready ? 'ready' : 'offline' }));
  assert.equal((await runtime.connect()).state, 'unavailable');
  ready = true;
  assert.equal((await runtime.connect()).state, 'ready');
  assert.equal(calls.spawn.length, 1);
  await runtime.stop();
});

test('an occupied selected port cannot adopt an unrelated workspace', async () => {
  const { runtime, calls } = fixture(async () => ({ state: 'ready' }));
  const status = await runtime.connect(true);
  assert.equal(status.state, 'unavailable');
  assert.match(status.reason, /occupied/);
  assert.deepEqual(calls, { spawn: [], mkdir: [], kill: [] });
});

test('missing runtime immediately provides repair guidance without spawning', async () => {
  const { runtime, calls } = fixture(async () => ({ state: 'offline' }), { fs: { access: async () => { throw new Error('ENOENT'); } } });
  const status = await runtime.connect();
  assert.equal(status.state, 'error');
  assert.match(status.reason, /Repair or reinstall/);
  assert.equal(calls.spawn.length, 0);
});

test('child exit is visible; shutdown cannot spawn after a pending probe', async () => {
  const one = fixture(async () => ({ state: 'offline' }));
  const pending = one.runtime.connect();
  await new Promise(resolve => setTimeout(resolve, 10));
  one.child.emit('exit', 1);
  assert.equal((await pending).state, 'error');
  let release;
  const two = fixture(() => new Promise(resolve => { release = resolve; }));
  two.runtime.connect();
  const stop = two.runtime.stop();
  release({ state: 'offline' });
  await stop;
  assert.equal(two.calls.spawn.length, 0);
});
