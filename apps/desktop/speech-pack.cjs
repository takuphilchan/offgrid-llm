// Build-time native speech assets. No model downloads or end-user build tools.
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { execFile } = require('node:child_process');
const lock = require('../../runtimes/speech/engine.lock.json');
const repo = path.resolve(__dirname, '../..');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const run = (file, args, options = {}) => new Promise((resolve, reject) => execFile(file, args, { windowsHide: true, timeout: 600000, maxBuffer: 16 * 1024 * 1024, ...options }, (err, stdout, stderr) => err ? reject(Error(`${file} failed: ${stderr || err.message}`)) : resolve(stdout)));

function target(platform, arch) {
  const value = lock.targets[`${platform}-${arch}`];
  if (!value) throw Error('Unsupported speech runtime target');
  // The pinned upstream macOS Piper archives omit libespeak-ng.1.dylib.
  // Shipping their executable would make a broken voice runtime look ready.
  return { ...value, suffix: platform === 'win32' ? '.exe' : '', piperBundled: platform !== 'darwin' };
}

function archiveExtractor(platform, systemRoot = process.env.SystemRoot) {
  return platform === 'win32' ? path.win32.join(systemRoot || 'C:\\Windows', 'System32', 'tar.exe') : 'tar';
}

async function files(root, relative = '') {
  const result = [];
  for (const entry of await fs.readdir(path.join(root, relative), { withFileTypes: true })) {
    const name = path.posix.join(relative, entry.name);
    if (entry.isSymbolicLink()) throw Error('Linked speech payload');
    if (entry.isDirectory()) result.push(...await files(root, name));
    else if (entry.isFile()) result.push({ path: name, sha256: hash(await fs.readFile(path.join(root, name))) });
    else throw Error('Invalid speech payload');
  }
  return result.sort((a, b) => a.path.localeCompare(b.path));
}

async function verify(root, platform, arch) {
  const expected = target(platform, arch);
  const pack = JSON.parse(await fs.readFile(path.join(root, 'speech-pack.json'), 'utf8'));
  if (pack.schema_version !== 1 || pack.platform !== platform || pack.arch !== arch || pack.whisper_revision !== lock.whisper_revision || pack.piper_sha256 !== (expected.piperBundled ? expected.sha256 : null)) throw Error('Speech runtime identity mismatch');
  const actual = (await files(root)).filter(file => file.path !== 'speech-pack.json');
  if (JSON.stringify(actual) !== JSON.stringify(pack.files)) throw Error('Speech payload digest mismatch');
  const required = [`whisper/whisper-cli${expected.suffix}`, 'licenses/whisper.txt', 'licenses/piper.txt', 'licenses/espeak.txt', 'NOTICE.txt'];
  if (expected.piperBundled) required.push(`piper/piper${expected.suffix}`);
  for (const name of required) {
    if (!actual.some(file => file.path === name)) throw Error(`Missing speech runtime file: ${name}`);
  }
  if (expected.piperBundled && !actual.some(file => file.path.startsWith('piper/espeak-ng-data/'))) throw Error('Missing Piper language data');
  if (!expected.piperBundled && actual.some(file => file.path.startsWith('piper/'))) throw Error('Unqualified Piper runtime in speech payload');
  return pack;
}

async function download(file, url, digest) {
  try { if (hash(await fs.readFile(file)) === digest) return; } catch (err) { if (err.code !== 'ENOENT') throw err; }
  const partial = `${file}.partial`;
  await run('curl', ['--fail', '--location', '--retry', '3', '--output', partial, url]);
  if (hash(await fs.readFile(partial)) !== digest) throw Error('Speech download digest mismatch');
  await fs.rename(partial, file);
}

async function cmakePath() {
  if (process.platform !== 'win32') return 'cmake';
  const finder = path.join(process.env['ProgramFiles(x86)'] || 'C:/Program Files (x86)', 'Microsoft Visual Studio/Installer/vswhere.exe');
  const found = (await run(finder, ['-latest', '-products', '*', '-find', '**/cmake.exe'])).trim().split(/\r?\n/)[0];
  if (!found) throw Error('CMake is required on the Windows build machine');
  return found;
}

async function prepare(context) {
  const platform = context.electronPlatformName, arch = require('builder-util').Arch[context.arch];
  const expected = target(platform, arch);
  if (platform !== process.platform) throw Error('Speech runtimes require a native platform build');
  const cache = path.join(repo, 'build/speech-source');
  const out = path.join(repo, 'build/speech-runtime', `${{ win32: 'win', darwin: 'mac', linux: 'linux' }[platform]}-${arch}`);
  await fs.mkdir(cache, { recursive: true });
  await download(path.join(cache, 'whisper.tar.gz'), `https://codeload.github.com/ggml-org/whisper.cpp/tar.gz/${lock.whisper_revision}`, lock.whisper_sha256);
  if (expected.piperBundled) await download(path.join(cache, expected.archive), `https://github.com/rhasspy/piper/releases/download/${lock.piper_revision}/${expected.archive}`, expected.sha256);
  // Only digest-pinned upstream archives are extracted, in a fresh owned folder.
  const work = await fs.mkdtemp(path.join(cache, 'prepare-'));
  // Windows' built-in BSD tar handles both the pinned ZIP and tar.gz archives.
  // Git Bash may put GNU tar first, which cannot read the Piper ZIP.
  const extractor = archiveExtractor(platform);
  // Relative arguments also avoid GNU tar interpreting drive letters as hosts.
  const workName = path.basename(work);
  await run(extractor, ['-xf', 'whisper.tar.gz', '-C', workName], { cwd: cache });
  if (expected.piperBundled) await run(extractor, ['-xf', expected.archive, '-C', workName], { cwd: cache });
  const source = path.join(work, `whisper.cpp-${lock.whisper_revision}`);
  const build = path.join(cache, `whisper-${platform}-${arch}`);
  const cmake = await cmakePath();
  const flags = ['-DCMAKE_BUILD_TYPE=Release', '-DBUILD_SHARED_LIBS=OFF', '-DGGML_NATIVE=OFF', '-DGGML_OPENMP=OFF', '-DGGML_AVX=OFF', '-DGGML_AVX2=OFF', '-DGGML_FMA=OFF', '-DGGML_F16C=OFF', '-DGGML_BMI2=OFF', '-DGGML_METAL=OFF', '-DWHISPER_BUILD_TESTS=OFF'];
  if (platform === 'win32') flags.push('-A', 'x64', '-DCMAKE_MSVC_RUNTIME_LIBRARY=MultiThreaded');
  if (platform === 'darwin') flags.push(`-DCMAKE_OSX_ARCHITECTURES=${arch === 'x64' ? 'x86_64' : 'arm64'}`);
  // Stable source path allows CMake cache reuse; preserve earlier build inputs.
  const stable = path.join(cache, `source-${lock.whisper_revision}`);
  try { await fs.rename(source, stable); } catch (err) { if (!['EEXIST', 'ENOTEMPTY', 'EPERM'].includes(err.code)) throw err; await fs.access(path.join(stable, 'CMakeLists.txt')); }
  await run(cmake, ['-S', stable, '-B', build, ...flags]);
  await run(cmake, ['--build', build, '--config', 'Release', '--parallel', '2', '--target', 'whisper-cli']);
  const staged = path.join(work, 'payload');
  await fs.mkdir(path.join(staged, 'whisper'), { recursive: true });
  if (expected.piperBundled) await fs.cp(path.join(work, 'piper'), path.join(staged, 'piper'), { recursive: true, dereference: true });
  await fs.copyFile(path.join(build, 'bin', ...(platform === 'win32' ? ['Release'] : []), `whisper-cli${expected.suffix}`), path.join(staged, 'whisper', `whisper-cli${expected.suffix}`));
  await fs.cp(path.join(repo, 'runtimes/speech/licenses'), path.join(staged, 'licenses'), { recursive: true });
  await fs.copyFile(path.join(stable, 'LICENSE'), path.join(staged, 'licenses/whisper.txt'));
  await fs.copyFile(path.join(repo, 'runtimes/speech/NOTICE.txt'), path.join(staged, 'NOTICE.txt'));
  // Help probes run with a system-only PATH, not developer-installed libraries.
  const env = { ...process.env, PATH: platform === 'win32' ? path.join(process.env.SystemRoot, 'System32') : '/usr/bin:/bin' };
  for (const [adapter, program] of [['whisper', 'whisper-cli'], ...(expected.piperBundled ? [['piper', 'piper']] : [])]) await run(path.join(staged, adapter, program + expected.suffix), ['--help'], { env, cwd: work, timeout: 15000 });
  const pack = { schema_version: 1, platform, arch, whisper_revision: lock.whisper_revision, piper_sha256: expected.piperBundled ? expected.sha256 : null, files: await files(staged) };
  await fs.writeFile(path.join(staged, 'speech-pack.json'), JSON.stringify(pack, null, 2) + '\n');
  await verify(staged, platform, arch);
  await fs.mkdir(path.dirname(out), { recursive: true });
  try { await fs.rename(out, `${out}-previous-${Date.now()}`); } catch (err) { if (err.code !== 'ENOENT') throw err; }
  // Windows scanners may retain a handle after the executable probe. Copy the
  // verified payload; the final verifier rejects incomplete or extra files.
  await fs.cp(staged, out, { recursive: true, force: false, errorOnExist: true });
  await verify(out, platform, arch);
  return out;
}

module.exports = { prepare, verify, target, files, archiveExtractor };
