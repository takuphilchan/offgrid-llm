import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import { createHash } from 'node:crypto';
import { buildKit, verifyKit } from './create-usb-package.mjs';
import { inventory, safePath } from './offline-kit-files.mjs';

const hash = data => createHash('sha256').update(data).digest('hex');
async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'offgrid-kit-test-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const source = path.join(root, 'source'), output = path.join(root, 'kit');
  async function write(name, value) {
    const data = typeof value === 'string' ? value : JSON.stringify(value);
    const file = path.join(source, name);
    await fs.mkdir(path.dirname(file), { recursive: true }); await fs.writeFile(file, data);
    return { size: Buffer.byteLength(data), sha256: hash(data) };
  }
  await write('application/OffGrid LLM Desktop.exe', 'fixture, not executable');
  await write('application/resources/app.asar', 'fixture, not a desktop');
  const engine = await write('application/resources/bin/llama-server.exe', 'engine fixture');
  await write('application/resources/bin/runtime-dependencies.json', { schema_version: 1, platform: 'windows-amd64-cpu', files: [{ name: 'llama-server.exe', sha256: engine.sha256, dependencies: ['kernel32.dll'] }] });
  const gguf = await write('models/model.gguf', 'GGUF storage fixture, not inference');
  await write('models/manifest.json', { models: [{ file_name: 'model.gguf', ...gguf }] });
  for (const architecture of ['whisper', 'piper']) {
    const artifacts = [];
    for (const role of ['weights', 'license', ...(architecture === 'piper' ? ['config'] : [])]) {
      const file = `${role}.fixture`;
      artifacts.push({ path: file, role, ...await write(`speech/${architecture}/${file}`, `fixture ${role}`) });
    }
    await write(`speech/${architecture}/manifest.json`, { schema_version: 1, architecture, license: 'fixture', languages: ['en'], artifacts });
  }
  const options = { source, output, version: '0.4.33' };
  // Fixtures cannot pass actual native desktop gates. Only unit orchestration
  // injects this verifier; the CLI always uses the real packaging checks.
  const checks = { verifyDesktop: async () => {} };
  return { root, source, output, write, options, checks };
}

test('copies verified content, detects damaged/missing/extra files and refuses overwrite', async t => {
  const f = await fixture(t);
  const manifest = await buildKit(f.options, f.checks);
  assert.match(manifest.qualification, /unqualified/);
  assert.equal((await verifyKit(f.output)).files.length, manifest.files.length);
  await assert.rejects(buildKit(f.options, f.checks), /already exists/);
  await fs.writeFile(path.join(f.output, 'models/model.gguf'), 'damaged');
  await assert.rejects(verifyKit(f.output), /readback failed/);
  await fs.copyFile(path.join(f.source, 'models/model.gguf'), path.join(f.output, 'models/model.gguf'));
  await fs.writeFile(path.join(f.output, 'extra'), 'extra');
  await assert.rejects(verifyKit(f.output), /readback failed/);
  await fs.unlink(path.join(f.output, 'extra'));
  await fs.unlink(path.join(f.output, 'models/model.gguf'));
  await assert.rejects(verifyKit(f.output), /readback failed/);
});

test('source changes after preflight fail before a completion manifest is written', async t => {
  const f = await fixture(t);
  await assert.rejects(buildKit(f.options, { verifyDesktop: async () => f.write('models/model.gguf', 'changed') }), /Copy verification failed/);
  await assert.rejects(fs.stat(path.join(f.output, 'kit.json')), { code: 'ENOENT' });
});

test('missing inference and speech dependencies fail without publishing', async t => {
  const f = await fixture(t);
  await fs.unlink(path.join(f.source, 'application/resources/bin/llama-server.exe'));
  await assert.rejects(buildKit(f.options, f.checks), /Missing required runtime/);
  await assert.rejects(fs.stat(f.output), { code: 'ENOENT' });
  await f.write('application/resources/bin/llama-server.exe', 'engine fixture');
  await fs.unlink(path.join(f.source, 'speech/piper/config.fixture'));
  await assert.rejects(buildKit(f.options, f.checks), /Artifact verification failed/);
});

test('rejects incomplete DLL closure, budget overflow and nested output', async t => {
  const f = await fixture(t);
  await assert.rejects(buildKit({ ...f.options, maxBytes: 1 }, f.checks), /budget/);
  await assert.rejects(buildKit({ ...f.options, output: path.join(f.source, 'nested') }, f.checks), /outside the source/);
  await f.write('application/resources/bin/runtime-dependencies.json', { schema_version: 1, platform: 'windows-amd64-cpu', files: [{ name: 'llama-server.exe', sha256: hash('engine fixture'), dependencies: ['vcomp140.dll'] }] });
  await assert.rejects(buildKit(f.options, f.checks), /dependency closure/);
});

test('real desktop verification cannot be bypassed by fixture bytes', async t => {
  const f = await fixture(t);
  await assert.rejects(buildKit(f.options));
  await assert.rejects(fs.stat(f.output), { code: 'ENOENT' });
});

test('rejects unsafe names, undeclared models and linked input', async t => {
  for (const value of ['../file', '/file', 'C:/file', 'models\\file', 'CON.txt', 'a/../b', 'a./b']) assert.throws(() => safePath(value));
  const f = await fixture(t);
  await f.write('models/undeclared.gguf', 'extra');
  await assert.rejects(buildKit(f.options, f.checks), /Undeclared/);
  await fs.symlink(f.source, path.join(f.root, 'linked'), process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(inventory(path.join(f.root, 'linked')), /links/);
});
