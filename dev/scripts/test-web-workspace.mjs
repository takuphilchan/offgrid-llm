// Real service + built renderer on an ephemeral port and disposable data.
// Never run browser integration tests against a developer's active workspace.
import { mkdtemp, mkdir, writeFile } from 'node:fs/promises';
import { createWriteStream } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createRequire } from 'node:module';
import { setTimeout as delay } from 'node:timers/promises';
const root = resolve(import.meta.dirname, '../..');
const require = createRequire(join(root, 'web/app/package.json'));
const { availablePort } = createRequire(join(root, 'desktop/package.json'))('./runtime.js');
if (!process.argv[2]) throw new Error('Pass the native OffGrid executable');
const evidence = await mkdtemp(join(tmpdir(), 'offgrid-web-qualification-'));
const port = await availablePort();
for (const dir of ['models', 'data', 'bin']) await mkdir(join(evidence, dir));
await writeFile(join(evidence, 'config.yaml'), '{}');
const env = Object.fromEntries(Object.entries(process.env).filter(([name]) => !name.startsWith('OFFGRID_')));
Object.assign(env, { OFFGRID_HOST: '127.0.0.1', OFFGRID_PORT: String(port), OFFGRID_MODELS_DIR: join(evidence, 'models'),
  OFFGRID_DATA_DIR: join(evidence, 'data'), OFFGRID_BIN_DIR: join(evidence, 'bin'), OFFGRID_CONFIG: join(evidence, 'config.yaml'),
  OFFGRID_UI_DIR: join(root, 'web/dist'), OFFGRID_REQUIRE_AUTH: 'false', OFFGRID_ENABLE_P2P: 'false' });
const log = createWriteStream(join(evidence, 'service.log'));
const service = spawn(resolve(process.argv[2]), ['serve'], { cwd: evidence, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
let spawnError; service.once('error', error => { spawnError = error; });
const stopped = new Promise(resolve => service.once('exit', resolve));
console.log(`Isolated web qualification: ${evidence}`);
try {
  const url = `http://127.0.0.1:${port}`;
  let ready = false;
  for (let attempt = 0; attempt < 100; attempt++) {
    if (spawnError) throw spawnError;
    if (service.exitCode !== null) throw new Error('Fixture service exited; inspect service.log');
    try { ready = (await fetch(`${url}/api/v2/system`, { signal: AbortSignal.timeout(500) })).ok; } catch {}
    if (ready) break;
    await delay(100);
  }
  if (!ready) throw new Error('Fixture service did not become ready');
  const tests = spawn(process.execPath, [require.resolve('@playwright/test/cli'), 'test', ...process.argv.slice(3)], {
    cwd: join(root, 'web/app'), env: { ...process.env, OFFGRID_E2E_URL: url }, windowsHide: true, stdio: 'inherit'
  });
  const [code] = await once(tests, 'exit');
  process.exitCode = code ?? 1;
} finally {
  if (service.exitCode === null && !spawnError) {
    service.kill('SIGTERM');
    const timer = setTimeout(() => service.kill('SIGKILL'), 5000); timer.unref();
    await stopped; clearTimeout(timer);
  }
  log.end();
}
