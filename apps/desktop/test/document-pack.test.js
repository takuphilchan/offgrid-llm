const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const { createHash } = require('node:crypto');
const { target, verify } = require('../document-pack.cjs');
const lock = require('../../../runtimes/pdfium/engine.lock.json');

test('desktop copies only parser deliverables, never staged test programs or private language data', () => {
  const resource = require('../package.json').build.extraResources.find(item => item.from.includes('document-runtime'));
  assert.equal(resource.to, 'bin');
  assert.equal(resource.filter.length, 8);
  for (const name of resource.filter) {
    assert.match(name, /^(offgrid-(document|ocr)-worker(\.exe)?|(document|ocr)-(NOTICE\.txt|pack\.json))$/);
  }
});

test('document targets are explicit and architecture-bound', () => {
  assert.deepEqual(target('win32', 'x64'), { goos: 'windows', goarch: 'amd64', file: 'offgrid-document-worker.exe' });
  assert.equal(target('darwin', 'arm64').goarch, 'arm64');
  assert.throws(() => target('web', 'x64'));
  assert.throws(() => target('linux', '../x64'));
});

test('copied document package rejects identity drift and binary tampering', async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'offgrid-document-pack-'));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const file = 'offgrid-document-worker.exe', binary = Buffer.from('package verification fixture');
  const pack = { schema_version: 1, platform: 'windows', architecture: 'amd64', file, engine_sha256: lock.wasm_sha256, sha256: createHash('sha256').update(binary).digest('hex') };
  const manifest = path.join(root, 'document-pack.json');
  await fs.writeFile(manifest, JSON.stringify(pack)); await fs.writeFile(path.join(root, file), binary);
  await assert.rejects(verify(root, 'darwin', 'arm64'), /identity mismatch/);
  await fs.writeFile(path.join(root, file), 'modified');
  await assert.rejects(verify(root, 'win32', 'x64'), /digest mismatch/);
  await fs.writeFile(path.join(root, file), binary);
  await fs.writeFile(path.join(root, 'document-NOTICE.txt'), 'missing notices');
  await assert.rejects(verify(root, 'win32', 'x64'), /notices mismatch/);
  let notices;
  try { notices = await fs.readFile(path.resolve(__dirname, '../../../runtimes/pdfium/assets/NOTICE.txt')); }
  catch { t.diagnostic('Positive packaging fixture requires prepared pinned assets; rejection checks ran.'); return; }
  await fs.writeFile(path.join(root, 'document-NOTICE.txt'), notices);
  await assert.rejects(verify(root, 'win32', 'x64'), /ENOENT/, 'OCR must not be silently omitted');
  const ocrLock = require('../../../runtimes/ocr/engine.lock.json');
  let ocrNotices;
  try { ocrNotices = await fs.readFile(path.resolve(__dirname, '../../../runtimes/ocr/worker/assets/NOTICE.txt')); }
  catch { t.diagnostic('Positive OCR package verification requires prepared pinned assets; missing-worker rejection ran.'); return; }
  const ocrFile = 'offgrid-ocr-worker.exe';
  const ocrPack = { ...pack, file: ocrFile, engine_sha256: ocrLock.wasm_sha256 };
  await fs.writeFile(path.join(root, 'ocr-pack.json'), JSON.stringify(ocrPack));
  await fs.writeFile(path.join(root, ocrFile), binary);
  await fs.writeFile(path.join(root, 'ocr-NOTICE.txt'), ocrNotices);
  assert.deepEqual(await verify(root, 'win32', 'x64'), pack);
  await fs.writeFile(path.join(root, ocrFile), 'modified');
  await assert.rejects(verify(root, 'win32', 'x64'), /digest mismatch/);
  await fs.writeFile(path.join(root, ocrFile), binary);
  await fs.writeFile(path.join(root, 'ocr-NOTICE.txt'), 'modified');
  await assert.rejects(verify(root, 'win32', 'x64'), /notices mismatch/);
});
