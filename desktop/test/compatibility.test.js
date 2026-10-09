const { test } = require('node:test');
const assert = require('node:assert/strict');
const { mkdtempSync, writeFileSync, readFileSync, rmSync } = require('node:fs');
const { join } = require('node:path');
const { tmpdir } = require('node:os');
const { BRIDGE_PROTOCOL, createManifest, parseManifest, verifyUI } = require('../compatibility.cjs');

test('renderer contract generation is deterministic and normalizes checkout newlines', () => {
  const index = '<html>\n<script src="assets/a.js"></script></html>';
  const value = createManifest(index);
  assert.deepEqual(value, createManifest(index.replaceAll('\n', '\r\n')));
  assert.deepEqual(parseManifest(JSON.stringify(value), index), value);
  for (const bad of ['{}', 'null', '{', JSON.stringify({...value, schema_version: 2}), JSON.stringify({...value, protocol: 0}), JSON.stringify({...value, protocol: 1.5}), JSON.stringify({...value, ui_build_id: 'a'.repeat(64)}), ' '.repeat(4097)]) {
    assert.throws(() => parseManifest(bad, index));
  }
});

test('copied package must contain a current manifest, not staging-only metadata', t => {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-contract-'));
  t.after(() => rmSync(root, {recursive:true, force:true}));
  writeFileSync(join(root, 'index.html'), 'new renderer');
  assert.throws(() => verifyUI(root));
  writeFileSync(join(root, 'desktop-compatibility.json'), JSON.stringify(createManifest('old renderer')));
  assert.throws(() => verifyUI(root));
  writeFileSync(join(root, 'desktop-compatibility.json'), JSON.stringify(createManifest('new renderer')));
  assert.equal(verifyUI(root).protocol, 1);
});

test('preload and package validation retain the declared bridge contract', () => {
  const preload = readFileSync(join(__dirname, '../preload.js'), 'utf8');
  assert.match(preload, new RegExp(`bridgeProtocol: ${BRIDGE_PROTOCOL},`));
  const packageJSON = require('../package.json');
  for (const file of ['compatibility.cjs','legacy-compatibility.json']) assert.ok(packageJSON.build.files.includes(file));
  const hook = readFileSync(join(__dirname, '../verify-computer.cjs'), 'utf8');
  assert.match(hook, /verifyUI\(path.join\(context.packager.getResourcesDir\(context.appOutDir\),'ui'\)\)/);
  const local = readFileSync(join(__dirname, '../../docker/Dockerfile.local-runtime'), 'utf8');
  assert.match(local, /verifyUI\('\/ui'\)/);
  assert.match(local, /COPY --from=checked-ui/);
});
