// Build-time parser staging. Never downloads a runtime during document import.
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { spawn } = require('node:child_process');
const runtimes = [
  { name: 'document', module: 'runtimes/pdfium', assets: 'runtimes/pdfium/assets', wasm: 'pdfium.wasm', lock: require('../../runtimes/pdfium/engine.lock.json') },
  { name: 'ocr', module: 'runtimes/ocr/worker', assets: 'runtimes/ocr/worker/assets', wasm: 'ocr.wasm', lock: require('../../runtimes/ocr/engine.lock.json') },
];
const digest = bytes => createHash('sha256').update(bytes).digest('hex');

function target(platform, arch, name = 'document') {
  const goos = { win32: 'windows', darwin: 'darwin', linux: 'linux' }[platform];
  const goarch = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!goos || !goarch || !runtimes.some(runtime => runtime.name === name)) throw Error('Unsupported document worker target');
  return { goos, goarch, file: `offgrid-${name}-worker${goos === 'windows' ? '.exe' : ''}` };
}

async function checkedFile(file, max = 128 * 1024 * 1024) {
  const stat = await fs.lstat(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size < 1 || stat.size > max) throw Error('Invalid document runtime file');
  return fs.readFile(file);
}

async function verifyRuntime(root, platform, arch, runtime) {
  const { name, lock } = runtime;
  const expected = target(platform, arch, name);
  const pack = JSON.parse(await checkedFile(path.join(root, `${name}-pack.json`), 4096));
  if (pack.schema_version !== 1 || pack.platform !== expected.goos || pack.architecture !== expected.goarch || pack.file !== expected.file || pack.engine_sha256 !== lock.wasm_sha256 || !/^[a-f0-9]{64}$/.test(pack.sha256)) throw Error('Document runtime identity mismatch');
  if (digest(await checkedFile(path.join(root, expected.file))) !== pack.sha256) throw Error('Document worker digest mismatch');
  if (digest(await checkedFile(path.join(root, `${name}-NOTICE.txt`))) !== lock.notices_sha256) throw Error('Document runtime notices mismatch');
  return pack;
}

async function verify(root, platform, arch) {
  const packs = [];
  for (const runtime of runtimes) packs.push(await verifyRuntime(root, platform, arch, runtime));
  return packs[0]; // Preserve the PDF pack caller contract; both workers are required.
}

async function prepare(context) {
  const platform = context.electronPlatformName, arch = require('builder-util').Arch[context.arch];
  const repo = path.resolve(__dirname, '../..');
  const directory = path.join(repo, 'build/document-runtime', `${{ win32: 'win', darwin: 'mac', linux: 'linux' }[platform]}-${arch}`);
  // Validate every input before replacing a staged worker.
  const prepared = [];
  for (const runtime of runtimes) {
    const assets = path.join(repo, runtime.assets);
    let wasm, notices;
    try { wasm = await checkedFile(path.join(assets, runtime.wasm)); notices = await checkedFile(path.join(assets, 'NOTICE.txt')); }
    catch { throw Error(`Prepare pinned ${runtime.name} assets before desktop packaging; see runtimes/${runtime.name === 'ocr' ? 'ocr' : 'pdfium'}/README.md`); }
    if (digest(wasm) !== runtime.lock.wasm_sha256 || digest(notices) !== runtime.lock.notices_sha256) throw Error('Pinned document asset digest mismatch');
    prepared.push({ ...runtime, notices });
  }
  await fs.mkdir(directory, { recursive: true });
  for (const runtime of prepared) {
    const expected = target(platform, arch, runtime.name);
    await new Promise((resolve, reject) => {
      const child = spawn('go', ['-C', runtime.module, 'build', '-mod=readonly', '-trimpath', '-ldflags=-s -w', '-o', path.join(directory, expected.file), '.'], { cwd: repo, env: { ...process.env, GOOS: expected.goos, GOARCH: expected.goarch, CGO_ENABLED: '0' }, stdio: 'inherit', shell: false });
      child.once('error', reject); child.once('exit', code => code === 0 ? resolve() : reject(Error('Document worker build failed')));
    });
    await fs.writeFile(path.join(directory, `${runtime.name}-NOTICE.txt`), runtime.notices);
    const pack = { schema_version: 1, platform: expected.goos, architecture: expected.goarch, file: expected.file, engine_sha256: runtime.lock.wasm_sha256, sha256: digest(await checkedFile(path.join(directory, expected.file))) };
    await fs.writeFile(path.join(directory, `${runtime.name}-pack.json`), JSON.stringify(pack, null, 2) + '\n');
  }
  await verify(directory, platform, arch);
}

module.exports = { target, verify, prepare };
