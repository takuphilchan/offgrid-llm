import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { unpackRuntime, lock } from './prepare-document-runtime.mjs';

test('unknown and oversized archives cannot prepare runtime assets', () => {
  assert.throws(() => unpackRuntime(Buffer.from('not the approved archive')), /digest mismatch/);
  assert.throws(() => unpackRuntime(Buffer.alloc(16 * 1024 * 1024 + 1)), /digest mismatch/);
});

test('the pinned archive retains exact module and all redistribution notices', { skip: !process.env.OFFGRID_TEST_DOCUMENT_ARCHIVE }, () => {
  const archive = readFileSync(process.env.OFFGRID_TEST_DOCUMENT_ARCHIVE);
  const { wasm, notice } = unpackRuntime(archive);
  const digest = value => createHash('sha256').update(value).digest('hex');
  assert.equal(digest(wasm), lock.wasm_sha256);
  assert.equal(digest(notice), lock.notices_sha256);
  for (const name of ['pdfium.txt', 'libopenjpeg.txt', 'abseil.txt', 'lcms.txt', 'agg23.txt', 'libjpeg_turbo.md', 'zlib.txt', 'freetype.txt', 'icu.txt', 'simdutf.txt', 'fast_float.txt', 'libpng.txt']) assert.match(notice.toString(), new RegExp(`licenses/${name.replace('.', '\\.')}`));
  const corrupt = Buffer.from(archive); corrupt[corrupt.length - 1] ^= 1;
  assert.throws(() => unpackRuntime(corrupt), /digest mismatch/);
});
