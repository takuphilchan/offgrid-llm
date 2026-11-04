const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const { target, verify, files } = require('../speech-pack.cjs');
const lock = require('../../../runtimes/speech/engine.lock.json');

test('speech packaging binds supported architectures and includes the full runtime directory', () => {
  assert.equal(target('win32', 'x64').suffix, '.exe');
  assert.equal(target('darwin', 'arm64').archive, 'piper_macos_aarch64.tar.gz');
  assert.equal(target('darwin', 'arm64').piperBundled, false);
  assert.throws(() => target('win32', 'arm64'), /Unsupported/);
  assert.throws(() => target('linux', '../x64'), /Unsupported/);
  const resource = require('../package.json').build.extraResources.find(item => item.to === 'speech-runtime');
  assert.equal(resource.from, '../../build/speech-runtime/${os}-${arch}');
});

test('macOS speech payload advertises recognition without an incomplete Piper runtime', async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'offgrid-mac-speech-pack-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  for (const file of ['whisper/whisper-cli', 'licenses/whisper.txt', 'licenses/piper.txt', 'licenses/espeak.txt', 'NOTICE.txt']) {
    await fs.mkdir(path.dirname(path.join(root, file)), { recursive: true });
    await fs.writeFile(path.join(root, file), 'fixture');
  }
  const pack = { schema_version: 1, platform: 'darwin', arch: 'arm64', whisper_revision: lock.whisper_revision, piper_sha256: null, files: await files(root) };
  await fs.writeFile(path.join(root, 'speech-pack.json'), JSON.stringify(pack));
  await verify(root, 'darwin', 'arm64');
  await fs.mkdir(path.join(root, 'piper'));
  await fs.writeFile(path.join(root, 'piper/piper'), 'incomplete');
  pack.files = (await files(root)).filter(file => file.path !== 'speech-pack.json');
  await fs.writeFile(path.join(root, 'speech-pack.json'), JSON.stringify(pack));
  await assert.rejects(verify(root, 'darwin', 'arm64'), /Unqualified Piper runtime/);
});

test('copied speech payload rejects drift, corruption, omitted assets and extra code', async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'offgrid-speech-pack-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const required = ['whisper/whisper-cli.exe', 'piper/piper.exe', 'piper/espeak-ng-data/en', 'licenses/whisper.txt', 'licenses/piper.txt', 'licenses/espeak.txt', 'NOTICE.txt'];
  for (const file of required) { await fs.mkdir(path.dirname(path.join(root, file)), { recursive: true }); await fs.writeFile(path.join(root, file), 'fixture'); }
  const pack = { schema_version: 1, platform: 'win32', arch: 'x64', whisper_revision: lock.whisper_revision, piper_sha256: target('win32', 'x64').sha256, files: await files(root) };
  const manifest = path.join(root, 'speech-pack.json');
  await fs.writeFile(manifest, JSON.stringify(pack));
  await verify(root, 'win32', 'x64');
  await assert.rejects(verify(root, 'darwin', 'x64'), /identity mismatch/);
  await fs.writeFile(path.join(root, 'piper/piper.exe'), 'corrupted');
  await assert.rejects(verify(root, 'win32', 'x64'), /digest mismatch/);
  await fs.writeFile(path.join(root, 'piper/piper.exe'), 'fixture');
  await fs.writeFile(path.join(root, 'unexpected.exe'), 'extra');
  await assert.rejects(verify(root, 'win32', 'x64'), /digest mismatch/);
  await fs.unlink(path.join(root, 'unexpected.exe'));
  await fs.unlink(path.join(root, 'licenses/espeak.txt'));
  // Even a re-hashed manifest cannot omit mandatory runtime components.
  pack.files = (await files(root)).filter(file => file.path !== 'speech-pack.json');
  await fs.writeFile(manifest, JSON.stringify(pack));
  await assert.rejects(verify(root, 'win32', 'x64'), /Missing speech runtime file/);
});
