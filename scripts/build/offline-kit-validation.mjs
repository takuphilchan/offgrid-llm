import path from 'node:path';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { readJSON, checkRecord, safePath } from './offline-kit-files.mjs';
const require = createRequire(import.meta.url);

// Reuse the actual desktop packaging gates. This does not execute the app,
// fetch assets, install anything or certify a clean-machine first response.
export async function verifyDesktop(root, version) {
  const resources = path.join(root, 'application/resources');
  require('../../apps/desktop/presentation-pack.cjs').verify(resources);
  require('../../apps/desktop/service-pack.cjs').verifyBundledServices(resources, 'win32', version);
  await require('../../apps/desktop/speech-pack.cjs').verify(path.join(resources, 'speech-runtime'), 'win32', 'x64');
  await require('../../apps/desktop/document-pack.cjs').verify(path.join(resources, 'bin'), 'win32', 'x64');
  await require('../../apps/desktop/terminal-pack.cjs').verify(path.join(resources, 'terminal-runtime'), version, 'win32', 'x64');
  await require('../../apps/desktop/execution-pack.cjs').verify(path.join(resources, 'execution-runtime'), version, 'win32', 'x64');
  require('../../apps/desktop/compatibility.cjs').verifyUI(path.join(resources, 'ui'));
  await require('../../apps/computer/pack.cjs').verifyPack(path.join(resources, 'computer'), 'win32', 'x64');
  // Reuse the strict service manifest contract; prohibit dependency/toolchain
  // fetching even on a maintainer machine whose Go cache is incomplete.
  execFileSync('go', ['run', './tools/offline-kit-check', path.join(root, 'speech')], {
    cwd: fileURLToPath(new URL('../..', import.meta.url)), windowsHide: true,
    timeout: 120000, maxBuffer: 1024 * 1024,
    env: { ...process.env, GOPROXY: 'off', GOSUMDB: 'off', GOTOOLCHAIN: 'local' },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

export async function verifyContent(root, files) {
  const allowed = new Set(['application', 'models', 'speech', 'documents', 'licenses']);
  for (const file of files) if (!allowed.has(file.path.split('/')[0])) throw Error(`Unexpected kit content: ${file.path}`);
  for (const name of ['application/OffGrid LLM Desktop.exe', 'application/resources/app.asar', 'application/resources/bin/llama-server.exe']) {
    if (!files.some(f => f.path === name && f.size > 0)) throw Error(`Missing required runtime: ${name}`);
  }
  const runtimePrefix = 'application/resources/bin/';
  const runtime = await readJSON(root, `${runtimePrefix}runtime-dependencies.json`);
  if (runtime.schema_version !== 1 || runtime.platform !== 'windows-amd64-cpu' || !Array.isArray(runtime.files) || !runtime.files.some(f => f.name === 'llama-server.exe')) throw Error('Missing complete Windows CPU runtime manifest');
  const runtimeNames = new Set();
  for (const entry of runtime.files) {
    if (safePath(entry.name).includes('/') || runtimeNames.has(entry.name.toLowerCase())) throw Error('Invalid runtime dependency name');
    runtimeNames.add(entry.name.toLowerCase());
    const file = files.find(f => f.path === runtimePrefix + entry.name);
    checkRecord(files, runtimePrefix, { path: entry.name, size: file?.size, sha256: entry.sha256 });
  }
  const systemLibraries = new Set('kernel32.dll kernelbase.dll ntdll.dll advapi32.dll shell32.dll shlwapi.dll user32.dll userenv.dll gdi32.dll ole32.dll oleaut32.dll ws2_32.dll bcrypt.dll crypt32.dll secur32.dll rpcrt4.dll msvcrt.dll ucrtbase.dll version.dll winmm.dll psapi.dll normaliz.dll'.split(' '));
  for (const entry of runtime.files) {
    if (!Array.isArray(entry.dependencies) || !entry.dependencies.length) throw Error('Missing runtime dependency closure');
    for (const name of entry.dependencies) {
      if (typeof name !== 'string' || (!systemLibraries.has(name.toLowerCase()) && !/^(api|ext)-ms-win-[a-z0-9-]+\.dll$/i.test(name) && !runtimeNames.has(name.toLowerCase()))) throw Error('Missing runtime dependency closure');
    }
  }
  for (const file of files.filter(f => f.path.startsWith(runtimePrefix) && /\.(exe|dll)$/i.test(f.path))) {
    // Other OffGrid document/service executables are independently checked above.
    if (/\.dll$/i.test(file.path) && !runtimeNames.has(path.basename(file.path).toLowerCase())) throw Error('Unlisted inference dependency');
  }
  const models = await readJSON(root, 'models/manifest.json');
  if (!Array.isArray(models.models) || !models.models.length) throw Error('Kit needs at least one explicitly selected GGUF model');
  const names = new Set();
  for (const model of models.models) {
    const name = safePath(model.file_name);
    if (name.includes('/') || !name.endsWith('.gguf') || names.has(name.toLowerCase())) throw Error('Invalid or duplicate model filename');
    names.add(name.toLowerCase());
    checkRecord(files, 'models/', { path: name, size: model.size, sha256: model.sha256 });
  }
  for (const file of files.filter(f => f.path.startsWith('models/') && f.path !== 'models/manifest.json')) {
    if (!names.has(file.path.slice('models/'.length).toLowerCase())) throw Error('Undeclared GGUF model content');
  }
  const speech = files.filter(f => /^speech\/[^/]+\/manifest.json$/.test(f.path));
  const architectures = new Set();
  for (const entry of speech) {
    const manifest = await readJSON(root, entry.path), prefix = entry.path.slice(0, -'manifest.json'.length);
    if (manifest.schema_version !== 1 || !['whisper', 'piper'].includes(manifest.architecture) || !Array.isArray(manifest.artifacts)) throw Error('Starter-kit speech profile currently requires Whisper/Piper packages');
    const roles = new Set(), declared = new Set();
    for (const artifact of manifest.artifacts) {
      checkRecord(files, prefix, artifact);
      if (declared.has(artifact.path.toLowerCase())) throw Error('Duplicate speech artifact');
      declared.add(artifact.path.toLowerCase()); roles.add(artifact.role);
    }
    if (!roles.has('weights') || !roles.has('license') || !manifest.license || !manifest.languages?.length || (manifest.architecture === 'piper' && !roles.has('config'))) throw Error('Incomplete speech model package');
    for (const file of files.filter(f => f.path.startsWith(prefix) && f.path !== entry.path)) {
      if (!declared.has(file.path.slice(prefix.length).toLowerCase())) throw Error('Undeclared speech artifact');
    }
    architectures.add(manifest.architecture);
  }
  if (!architectures.has('whisper') || !architectures.has('piper')) throw Error('Kit needs verified recognition and spoken-response model packages');
  for (const file of files.filter(f => f.path.startsWith('speech/'))) {
    if (!speech.some(entry => file.path.startsWith(entry.path.slice(0, -'manifest.json'.length)))) throw Error('Undeclared speech package content');
  }
  return { models: models.models.map(m => m.file_name), speech_packages: speech.map(f => f.path), qualification: 'unqualified: clean-machine inference and language review required' };
}
