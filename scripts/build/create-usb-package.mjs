import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { inventory, readJSON, copyInventory } from './offline-kit-files.mjs';
import { verifyContent, verifyDesktop } from './offline-kit-validation.mjs';

const readme = `OFFGRID OFFLINE STARTER KIT - WINDOWS X64

This kit contains a prepared application, a CPU inference runtime and selected
model packages. File verification is not a language-quality or performance test.

Copy the entire application folder to a NEW folder on the computer's internal
SSD, then open OffGrid LLM Desktop.exe. Keep existing application folders until
the new build is verified. Do not copy a kit over an existing workspace.

In Models, use advanced local import for the GGUF files in models and the
complete package folders in speech. Import preserves conflicting installed files;
new model versions need distinct identities. Select recognition and speech models
independently in Voice settings. Check the connected workspace before importing.

The app retains its normal user-data location. This USB is distribution media,
not a portable personal workspace. Do not copy credentials or private histories
back onto the master. No automatic installation, model loading or service restart
is performed by this kit. OCR languages and optional third-party runtimes require
separate offline preparation; they are not promised by this baseline profile.

Maintainer verification: node scripts/build/create-usb-package.mjs --verify KIT
after every copy. Compare kit.json's SHA-256 against your independently retained
master digest. Co-located hashes detect corruption, not publisher authenticity.
`;

export async function verifyKit(root) {
  const manifest = await readJSON(root, 'kit.json');
  if (manifest.schema_version !== 1 || manifest.platform !== 'windows-x64' || !Array.isArray(manifest.files)) throw Error('Invalid kit manifest');
  const actual = (await inventory(root)).filter(f => f.path !== 'kit.json');
  if (JSON.stringify(actual) !== JSON.stringify(manifest.files)) throw Error('Kit readback failed: files, sizes or digests differ');
  return manifest;
}

export async function buildKit({ source, output, version, maxBytes = 40_000_000_000 }, checks = { verifyDesktop }) {
  if (!source || !output || !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(version ?? '') || !Number.isSafeInteger(maxBytes) || maxBytes < 1) throw Error('Provide source, fresh output, version and a positive byte budget');
  source = await fs.realpath(source);
  const parent = await fs.realpath(path.dirname(path.resolve(output)));
  output = path.join(parent, path.basename(path.resolve(output)));
  const relative = path.relative(source, output);
  if (!relative || (!relative.startsWith('..' + path.sep) && relative !== '..' && !path.isAbsolute(relative))) throw Error('Output must be outside the source');
  try { await fs.lstat(output); throw Error('Output already exists; choose a fresh directory'); } catch (err) { if (err.code !== 'ENOENT') throw err; }
  const files = await inventory(source);
  const bytes = files.reduce((sum, file) => sum + file.size, 0);
  if (bytes + 4 * 1024 * 1024 > maxBytes) throw Error('Kit exceeds byte budget including manifest reserve');
  const disk = await fs.statfs(parent);
  if (disk.bavail * disk.bsize < bytes + 4 * 1024 * 1024) throw Error('Insufficient destination space');
  const content = await verifyContent(source, files);
  await checks.verifyDesktop(source, version);
  // Reserve a new directory exclusively. Failure leaves a diagnosable partial
  // folder without kit.json; never erase or merge an older master.
  await fs.mkdir(output);
  await copyInventory(source, output, files);
  await fs.writeFile(path.join(output, 'READ-ME.txt'), readme, { flag: 'wx' });
  const copied = await inventory(output);
  const manifest = { schema_version: 1, platform: 'windows-x64', version, ...content, files: copied };
  const serialized = JSON.stringify(manifest, null, 2) + '\n';
  if (Buffer.byteLength(serialized) > 4 * 1024 * 1024 || copied.reduce((n, f) => n + f.size, 0) + Buffer.byteLength(serialized) > maxBytes) throw Error('Kit inventory exceeds budget');
  await fs.writeFile(path.join(output, 'kit.json'), serialized, { flag: 'wx' });
  await verifyKit(output);
  return manifest;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const args = process.argv.slice(2);
    if (args.length === 2 && args[0] === '--verify') {
      const result = await verifyKit(args[1]);
      console.log(`Verified ${result.files.length} files. ${result.qualification}`);
    } else {
      const opts = {};
      for (let i = 0; i < args.length; i += 2) {
        const key = { '--source': 'source', '--output': 'output', '--version': 'version', '--max-bytes': 'maxBytes' }[args[i]];
        if (!key || !args[i + 1] || key in opts) throw Error('Usage: node create-usb-package.mjs --source PREPARED --output NEW_KIT --version VERSION [--max-bytes BYTES] | --verify KIT');
        opts[key] = key === 'maxBytes' ? Number(args[i + 1]) : args[i + 1];
      }
      const result = await buildKit(opts);
      console.log(`Prepared ${result.files.length} verified files. ${result.qualification}`);
    }
  } catch (err) { console.error(err.message); process.exitCode = 1; }
}
