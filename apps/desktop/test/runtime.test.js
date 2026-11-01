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

// The production reconnect timer is intentionally unreferenced so it cannot
// prevent Quit. Unlike Electron, this isolated Node test has no window loop;
// hold a bounded test-owned deadline while observing the background event.
function waitForState(runtime, expected) {
  return new Promise((resolve, reject) => {
    const finish = error => {
      clearTimeout(timer);
      runtime.off('status', listener);
      if (error) reject(error); else resolve();
    };
    const listener = state => { if (state.state === expected) finish(); };
    const timer = setTimeout(() => finish(new Error(`Expected runtime state: ${expected}`)), 2000);
    runtime.on('status', listener);
  });
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

test('child exit is visible; shutdown cannot spawn after a pending probe', { timeout: 3000 }, async () => {
  let signalSpawn;
  const spawned = new Promise(resolve => { signalSpawn = resolve; });
  const one = fixture(async () => ({ state: 'offline' }), {
    timeoutMs: 1000,
    spawn: () => {
      queueMicrotask(signalSpawn);
      return one.child;
    }
  });
  const pending = one.runtime.connect();
  await spawned;
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

test('probe context remains owned after spawn and on retry', async () => {
  const modes = [];
  const {runtime} = fixture(async (...args) => { modes.push(args[5]); return {state:modes.length === 1 ? 'offline' : 'ready'}; });
  await runtime.connect(); await runtime.connect(); await runtime.stop();
  assert.deepEqual(modes, ['external','owned','owned']);
});

test('explicit reconnect reassesses identity and revokes old authority before readiness', async () => {
  let workspaceID = 'first'; let bridgeProtocol = 1; let invalid = false;
  const revoked = [];
  const {runtime,calls} = fixture(async () => invalid ? {state:'incompatible'} : {state:'ready',workspaceID,uiBuildID:'a',bridgeProtocol}, {
    onIdentityChange: async () => { assert.equal(runtime.state.state, 'checking'); revoked.push(workspaceID); }
  });
  await runtime.connect(); await runtime.connect(); assert.deepEqual(revoked, []);
  workspaceID = 'second'; await runtime.connect();
  bridgeProtocol = 2; await runtime.connect();
  invalid = true; await runtime.connect();
  assert.deepEqual(revoked, ['second','second','second']);
  assert.equal(runtime.state.state, 'incompatible');
  assert.equal(calls.spawn.length, 0);
});

test('uncertain local revocation cannot accept a replacement workspace', async () => {
  let workspaceID = 'first';
  const {runtime} = fixture(async () => ({state:'ready',workspaceID}), {onIdentityChange:async()=>{throw Error('stop uncertain');}});
  await runtime.connect(); workspaceID = 'second';
  assert.equal((await runtime.connect()).state, 'error');
});

test('a disconnected external workspace never silently becomes a bundled workspace on retry', async () => {
  let state = 'ready'; let revoked = 0;
  const {runtime,calls} = fixture(async()=>({state,workspaceID:'external'}), {onIdentityChange:async()=>{revoked++;}});
  await runtime.connect(); state = 'offline';
  assert.equal((await runtime.connect()).state, 'unavailable');
  assert.equal((await runtime.connect()).state, 'unavailable');
  assert.equal(calls.spawn.length, 0);
  assert.equal(revoked, 1);
  await runtime.stop();
});

test('external ownership survives quit and relaunch while its endpoint is offline', async () => {
  const remembered = new Set();
  const attachments = {load:async url=>remembered.has(url),save:async url=>{remembered.add(url);}};
  const first = fixture(async()=>({state:'ready',workspaceID:'external'}), {attachments});
  await first.runtime.connect(); await first.runtime.stop();
  let state = 'offline';
  const second = fixture(async()=>({state,workspaceID:'external'}), {attachments});
  assert.equal((await second.runtime.connect()).state, 'unavailable');
  assert.equal((await second.runtime.connect()).state, 'unavailable');
  assert.deepEqual(second.calls, {spawn:[],mkdir:[],kill:[]});
  state = 'ready';
  assert.equal((await second.runtime.connect()).state, 'ready');
  assert.equal(second.calls.spawn.length, 0);
  await second.runtime.stop();
});

test('unreadable ownership and failed persistence never authorize fallback or readiness', async () => {
  const broken = fixture(async()=>({state:'offline'}), {attachments:{load:async()=>{throw Error('corrupt');}}});
  assert.equal((await broken.runtime.connect()).state, 'error');
  assert.equal(broken.calls.spawn.length, 0);
  const failed = fixture(async()=>({state:'ready'}), {attachments:{load:async()=>false,save:async()=>{throw Error('full disk');}}});
  assert.equal((await failed.runtime.connect()).state, 'error');
  assert.equal(failed.calls.spawn.length, 0);
});

test('explicit isolated recovery does not overwrite the remembered external endpoint', async () => {
  let probes = 0; const saved = [];
  const {runtime,calls} = fixture(async url=>({state:url.endsWith(':34567') && ++probes > 1 ? 'ready' : 'offline'}), {
    attachments:{load:async()=>true,save:async url=>saved.push(url)}
  });
  assert.equal((await runtime.connect()).state, 'unavailable');
  assert.equal((await runtime.connect(true)).state, 'ready');
  assert.equal(calls.spawn.length, 1);
  assert.equal(runtime.state.workspaceMode, 'isolated');
  assert.deepEqual(saved, []);
  await runtime.stop();
});

for (const initial of ['offline', 'unavailable']) test(`remembered service recovers from ${initial} during startup without a child`, async () => {
  let probes = 0;
  const {runtime,calls} = fixture(async()=>({state:++probes < 3 ? initial : 'ready',workspaceID:'original'}), {
    attachments:{load:async()=>true,save:async()=>{}}
  });
  const states=[]; runtime.on('status', state=>states.push(state));
  assert.equal((await runtime.connect()).state, 'ready');
  assert.ok(states.some(state=>state.state === 'checking' && state.autoReconnect));
  assert.equal(runtime.snapshot().autoReconnect,false);
  assert.equal(calls.spawn.length,0);
  await runtime.stop();
});

test('late external startup reconnects automatically after grace period without overlapping probes', {timeout:2000}, async () => {
  let ready=false, active=0, maximum=0;
  const {runtime,calls}=fixture(async()=>{
    maximum=Math.max(maximum,++active);
    await new Promise(resolve=>setTimeout(resolve,2)); active--;
    return {state:ready?'ready':'offline',workspaceID:'original'};
  }, {reconnectMs:5,attachments:{load:async()=>true,save:async()=>{}}});
  assert.equal((await runtime.connect()).autoReconnect,true);
  ready=true;
  await waitForState(runtime, 'ready');
  assert.equal(maximum,1); assert.equal(calls.spawn.length,0);
  await runtime.stop();
});

test('manual retry joins pending recovery and cancels its previously scheduled timer', async () => {
  let ready=false,probes=0;
  const {runtime,calls}=fixture(async()=>{probes++;return {state:ready?'ready':'offline'};}, {
    reconnectMs:10,attachments:{load:async()=>true,save:async()=>{}}
  });
  await runtime.connect(); ready=true;
  const retry=runtime.connect(); assert.equal(runtime.connect(),retry);
  assert.equal((await retry).state,'ready');
  const count=probes; await new Promise(resolve=>setTimeout(resolve,25));
  assert.equal(probes,count); assert.equal(calls.spawn.length,0);
  await runtime.stop();
});

test('automatic retry stops on incompatible identity and quit cancels queued checks', async () => {
  let state='offline', probes=0;
  const {runtime}=fixture(async()=>{probes++;return {state};}, {reconnectMs:5,attachments:{load:async()=>true}});
  await runtime.connect(); state='incompatible';
  await waitForState(runtime, 'incompatible');
  const before=probes; await new Promise(resolve=>setTimeout(resolve,20)); assert.equal(probes,before);
  state='offline'; await runtime.connect(); await runtime.stop();
  const stopped=probes; await new Promise(resolve=>setTimeout(resolve,20)); assert.equal(probes,stopped);
});

test('quit cancels initial external waiting and never spawns or accepts a service', async () => {
  const {runtime,calls}=fixture(async()=>({state:'offline'}),{timeoutMs:1000,attachments:{load:async()=>true}});
  const waiting=new Promise(resolve=>runtime.on('status',state=>{if(state.autoReconnect)resolve();}));
  const pending=runtime.connect(); await waiting; await runtime.stop(); await pending;
  assert.notEqual(runtime.snapshot().state,'ready'); assert.equal(calls.spawn.length,0);
});

test('separate workspace cancels scheduled reconnection to external endpoint', async () => {
  const urls=[];
  const {runtime,calls}=fixture(async(url,...args)=>{
    urls.push(url); return {state:args[4]==='owned'?'ready':'offline'};
  },{reconnectMs:10,attachments:{load:async()=>true}});
  await runtime.connect(); assert.equal((await runtime.connect(true)).state,'ready');
  const count=urls.length; await new Promise(resolve=>setTimeout(resolve,25));
  assert.equal(urls.length,count); assert.equal(calls.spawn.length,1);
  await runtime.stop();
});

test('shutdown during ownership loading cannot spawn a child', async () => {
  let release;
  const {runtime,calls} = fixture(async()=>({state:'offline'}), {attachments:{load:()=>new Promise(resolve=>{release=resolve;})}});
  const connecting = runtime.connect(); const stopping = runtime.stop(); release(false);
  await Promise.all([connecting,stopping]); assert.equal(calls.spawn.length,0);
});

test('first-use offline installation still starts its own workspace without recording external ownership', async () => {
  let probes = 0; const saved = [];
  const {runtime,calls} = fixture(async()=>({state:++probes === 1 ? 'offline' : 'ready'}), {
    attachments:{load:async()=>false,save:async url=>saved.push(url)}
  });
  assert.equal((await runtime.connect()).state, 'ready');
  assert.equal(calls.spawn.length,1); assert.deepEqual(saved,[]);
  assert.equal(runtime.snapshot().managedByDesktop,true);
  await runtime.stop();
});
