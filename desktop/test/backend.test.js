const { test } = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const { once } = require('node:events');
const { assessIdentity, inspectBackend, isTrustedPage, isTrustedSender, fingerprintUI } = require('../backend');

const hash = 'a'.repeat(64);
const identity = { product: 'offgrid', version: '0.4.3', api_version: 2, revision: 'abc', ui_build_id: hash, capabilities: ['sessions-v1', 'chat-streaming-v1', 'durable-agent-runs-v1'], desktop_bridge: {status:'ready',schema_version:1,protocol:1,ui_build_id:hash} };

test('identity requires product, API, capabilities, and renderer-bound bridge contract', () => {
  assert.equal(assessIdentity(identity, '0.4.3', hash).state, 'ready');
  assert.equal(assessIdentity(identity, '0.5.0', 'b'.repeat(64)).state, 'ready');
  for (const altered of [null, {}, { ...identity, product: 'other' }, { ...identity, api_version: 1 }, { ...identity, capabilities: [] }, { ...identity, ui_build_id: 'b'.repeat(64) }, {...identity,desktop_bridge:null}, {...identity,desktop_bridge:{...identity.desktop_bridge,protocol:2}}, {...identity,desktop_bridge:{...identity.desktop_bridge,schema_version:2}}]) {
    assert.equal(assessIdentity(altered, '0.4.3', hash).state, 'incompatible');
  }
});

test('owned child must match package; external compatibility cannot mask a bad installation', () => {
  assert.equal(assessIdentity(identity, identity.version, hash, 'owned').state, 'ready');
  for (const [version, ui] of [['other',hash], [identity.version,'b'.repeat(64)], [identity.version,null]]) {
    assert.equal(assessIdentity(identity, version, ui, 'owned').code, 'bundle_inconsistent');
  }
});

test('reviewed legacy tuple survives a new UI fingerprint and cannot override invalid metadata', () => {
  const entry = require('../legacy-compatibility.json').entries[0];
  for (const version of entry.versions) {
    const old = {...identity, version, ui_build_id:entry.ui_build_id}; delete old.desktop_bridge;
    assert.equal(assessIdentity(old, 'future-desktop', hash).compatibilityBasis, 'reviewed-legacy');
    assert.equal(assessIdentity({...old,desktop_bridge:{status:'missing'}}, 'future-desktop', hash).state, 'ready');
    assert.equal(assessIdentity({...old,version:'0.4.13'}, 'future-desktop', hash).code, 'service_legacy_unreviewed');
    assert.equal(assessIdentity({...old,ui_build_id:hash}, 'future-desktop', hash).state, 'incompatible');
    for (const desktop_bridge of [null,{}, {status:'invalid'}, {status:'missing',protocol:2}, {status:'ready',schema_version:1,protocol:2,ui_build_id:entry.ui_build_id}]) {
      assert.equal(assessIdentity({...old,desktop_bridge}, 'future-desktop', hash).state, 'incompatible');
    }
    assert.equal(assessIdentity(old, version, entry.ui_build_id, 'owned').code, 'bundle_inconsistent');
  }
});

test('UI identity is stable across Windows and Unix checkout line endings', () => {
  const unix = '<html>\n<script src="assets/current.js"></script>\n</html>\n';
  assert.equal(fingerprintUI(Buffer.from(unix)), fingerprintUI(Buffer.from(unix.replace(/\n/g, '\r\n'))));
  assert.notEqual(fingerprintUI(Buffer.from(unix)), fingerprintUI(Buffer.from(unix.replace('current.js', 'stale.js'))));
});

test('probe distinguishes a current backend from occupied incompatible ports', async t => {
  let status = 200;
  let payload = JSON.stringify(identity);
  const server = http.createServer((request, response) => {
    assert.equal(request.url, '/api/v2/system');
    response.writeHead(status, { 'Content-Type': 'application/json' });
    response.end(payload);
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => server.close());
  const base = `http://127.0.0.1:${server.address().port}`;
  assert.equal((await inspectBackend(base, '0.4.3', hash)).state, 'ready');
  status = 404;
  assert.equal((await inspectBackend(base, '0.4.3', hash)).state, 'incompatible');
  status = 200; payload = '<html>old service</html>';
  assert.equal((await inspectBackend(base, '0.4.3', hash)).state, 'incompatible');
  payload = 'x'.repeat(16385);
  assert.equal((await inspectBackend(base, '0.4.3', hash)).state, 'incompatible');
});

test('an occupied unresponsive port is not treated as free', async t => {
  const server = http.createServer(() => {});
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  assert.equal((await inspectBackend(`http://127.0.0.1:${server.address().port}`, '0.4.3', hash, 30)).state, 'unavailable');
});

test('incompatible contract includes actual version; cancellation closes a pending handshake', async t => {
  let pending = false;
  const server = http.createServer((_request, response) => {
    if (!pending) response.end(JSON.stringify({ ...identity, version: '0.4.3-history-dev', desktop_bridge:{status:'invalid'} }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  t.after(() => { server.closeAllConnections(); server.close(); });
  const base = `http://127.0.0.1:${server.address().port}`;
  const result = await inspectBackend(base, '0.4.4', hash);
  assert.equal(result.state, 'incompatible');
  assert.equal(result.version, '0.4.3-history-dev');
  assert.equal(result.canOpenBrowser, true);
  pending = true;
  const abort = new AbortController();
  const check = inspectBackend(base, '0.4.4', hash, 10000, abort.signal);
  abort.abort();
  assert.match((await check).reason, /cancelled/);
});

test('navigation and IPC use exact origin and the main frame', () => {
  const origin = 'http://127.0.0.1:11611';
  const loading = 'file:///app/loading.html';
  assert.equal(isTrustedPage(`${origin}/ui/#/settings`, origin, loading), true);
  assert.equal(isTrustedPage(loading, origin, loading), true);
  for (const url of [`${origin}@evil.example/ui/`, `${origin}.evil.example/ui/`, 'http://127.0.0.1:11612/ui/', `${origin}/v1/documents`, 'file:///private/data', 'file:///app/loading.html?query', 'javascript:alert(1)']) {
    assert.equal(isTrustedPage(url, origin, loading), false, url);
  }
  const mainFrame = { url: `${origin}/ui/` };
  const contents = { mainFrame };
  assert.equal(isTrustedSender({ sender: contents, senderFrame: mainFrame }, contents, origin, loading), true);
  assert.equal(isTrustedSender({ sender: contents, senderFrame: { url: mainFrame.url } }, contents, origin, loading), false);
  assert.equal(isTrustedSender({ sender: {}, senderFrame: mainFrame }, contents, origin, loading), false);
});
