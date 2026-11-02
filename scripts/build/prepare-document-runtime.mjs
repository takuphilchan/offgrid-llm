// Explicit maintainer build step, never invoked by a user's document upload.
import { createHash } from 'node:crypto';
import { gunzipSync } from 'node:zlib';
import { readFileSync, writeFileSync, mkdirSync, lstatSync, renameSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
export const lock = JSON.parse(readFileSync(resolve(root, 'runtimes/pdfium/engine.lock.json'), 'utf8'));
const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');

export function unpackRuntime(archive) {
  if (archive.length > 16 * 1024 * 1024 || sha256(archive) !== lock.archive_sha256) throw new Error('PDF archive digest mismatch');
  const tar = gunzipSync(archive, { maxOutputLength: 64 * 1024 * 1024 });
  const entries = new Map();
  for (let offset = 0; offset + 512 <= tar.length;) {
    const header = tar.subarray(offset, offset + 512);
    if (header.every(value => value === 0)) break;
    const name = header.subarray(0, 100).toString('utf8').replace(/\0.*$/s, '');
    const size = Number.parseInt(header.subarray(124, 136).toString('ascii').replace(/\0.*$/s, '').trim(), 8);
    const checksum = Number.parseInt(header.subarray(148, 156).toString('ascii').trim(), 8);
    const sum = header.reduce((total, value, index) => total + (index >= 148 && index < 156 ? 32 : value), 0);
    if (sum !== checksum || !Number.isSafeInteger(size) || size < 0 || offset + 512 + size > tar.length || name.startsWith('/') || name.split('/').includes('..') || name.includes('\\') || entries.has(name)) throw new Error('Invalid pinned PDF archive');
    if (header[156] === 0 || header[156] === 48) entries.set(name, tar.subarray(offset + 512, offset + 512 + size));
    else if (header[156] !== 53) throw new Error('Unexpected PDF archive entry type');
    offset += 512 + Math.ceil(size / 512) * 512;
  }
  const wasm = entries.get('lib/pdfium.wasm');
  if (!wasm || sha256(wasm) !== lock.wasm_sha256) throw new Error('PDF module digest mismatch');
  const version = Object.fromEntries((entries.get('VERSION')?.toString().trim() ?? '').split(/\r?\n/).map(line => line.split('=')));
  if (['MAJOR', 'MINOR', 'BUILD', 'PATCH'].map(key => version[key]).join('.') !== lock.version) throw new Error('PDF version mismatch');
  const args = entries.get('args.gn')?.toString() ?? '';
  if (!args.includes('pdf_enable_v8 = false') || !args.includes('pdf_enable_xfa = false')) throw new Error('PDF scripting must be disabled');
  const notices = [...entries].filter(([name]) => name === 'LICENSE' || name.startsWith('licenses/')).sort(([a], [b]) => a.localeCompare(b));
  if (!entries.has('licenses/pdfium.txt') || notices.length < 2) throw new Error('PDF redistribution notices missing');
  const notice = Buffer.concat([Buffer.from(`OffGrid PDF engine ${lock.version}\n${lock.release}\nArchive SHA-256: ${lock.archive_sha256}\n\n`), ...notices.flatMap(([name, bytes]) => [Buffer.from(`\n--- ${name} ---\n`), bytes, Buffer.from('\n')])]);
  if (sha256(notice) !== lock.notices_sha256) throw new Error('PDF notice digest mismatch');
  return { wasm, notice };
}

export async function prepare(args) {
  if (args.length && (args.length !== 2 || args[0] !== '--archive')) throw new Error('Usage: node scripts/build/prepare-document-runtime.mjs [--archive PATH]');
  let archive;
  if (args.length) archive = readFileSync(resolve(args[1]));
  else {
    const response = await fetch(lock.archive_url, { signal: AbortSignal.timeout(60_000) });
    if (!response.ok) throw new Error(`PDF archive download failed: HTTP ${response.status}`);
    const parts = []; let size = 0;
    for await (const part of response.body) { size += part.length; if (size > 16 * 1024 * 1024) throw new Error('PDF archive exceeds build limit'); parts.push(part); }
    archive = Buffer.concat(parts);
  }
  const assets = resolve(root, 'runtimes/pdfium/assets');
  const { wasm, notice } = unpackRuntime(archive);
  mkdirSync(assets, { recursive: true });
  if (!lstatSync(assets).isDirectory() || lstatSync(assets).isSymbolicLink()) throw new Error('Runtime asset directory must be local');
  for (const [name, data] of [['pdfium.wasm', wasm], ['NOTICE.txt', notice]]) {
    const temporary = resolve(assets, `${name}.${process.pid}.tmp`);
    writeFileSync(temporary, data, { flag: 'wx' });
    renameSync(temporary, resolve(assets, name));
  }
  console.log(`Prepared pinned PDFium ${lock.version}, module SHA-256 ${lock.wasm_sha256}, with redistribution notices.`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) prepare(process.argv.slice(2)).catch(error => { console.error(error.message); process.exitCode = 1; });
