// Real service + built renderer on an ephemeral port and disposable data.
// Never run browser integration tests against a developer's active workspace.
import { mkdtemp, mkdir, writeFile, readFile, unlink } from 'node:fs/promises';
import { createWriteStream } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { createRequire } from 'node:module';
import { setTimeout as delay } from 'node:timers/promises';
import { privateFixtureDirectory } from './private-fixture-directory.mjs';
const root = resolve(import.meta.dirname, '../..');
const require = createRequire(join(root, 'apps/web/package.json'));
const { availablePort } = createRequire(join(root, 'apps/desktop/package.json'))('./runtime.js');
if (!process.argv[2]) throw new Error('Pass the native OffGrid executable');
const modelFixtures = process.argv.includes('--model-fixtures');
const endpointFixtures = process.argv.includes('--endpoint-fixtures');
const endpointOwnerFixtures = process.argv.includes('--endpoint-owner-fixtures');
if (endpointFixtures && endpointOwnerFixtures) throw Error('Choose one endpoint fixture mode');
const evidence = endpointOwnerFixtures ? await privateFixtureDirectory() : await mkdtemp(join(tmpdir(), 'offgrid-web-qualification-'));
const port = await availablePort();
for (const dir of ['models', 'data', 'bin']) await mkdir(join(evidence, dir));
await writeFile(join(evidence, 'config.yaml'), '{}');
const env = Object.fromEntries(Object.entries(process.env).filter(([name]) => !name.startsWith('OFFGRID_')));
Object.assign(env, { OFFGRID_HOST: '127.0.0.1', OFFGRID_PORT: String(port), OFFGRID_MODELS_DIR: join(evidence, 'models'),
  OFFGRID_DATA_DIR: join(evidence, 'data'), OFFGRID_BIN_DIR: join(evidence, 'bin'), OFFGRID_CONFIG: join(evidence, 'config.yaml'),
  OFFGRID_UI_DIR: join(root, 'apps/web/dist'), OFFGRID_REQUIRE_AUTH: 'false', OFFGRID_ENABLE_P2P: 'false' });
if (endpointFixtures) {
  const entry = { id: 'isolated-endpoint', revision: 'r1', provider: 'ollama', url: 'https://fixture.example.test', allowed_ips: ['192.0.2.9'],
    actors: ['local-admin'], credential_env: 'OFFGRID_ISOLATED_UNAVAILABLE_CREDENTIAL', model: 'fixture-model', model_digest: `sha256:${'a'.repeat(64)}`,
    format: 'gguf', runtime_version: 'fixture-1', context_length: 8192, deployment_attested: true };
  env.OFFGRID_INFERENCE_ENDPOINTS_FILE = join(evidence, 'endpoints.json');
  await writeFile(env.OFFGRID_INFERENCE_ENDPOINTS_FILE, JSON.stringify({ version: 1, endpoints: [entry] }));
}
if (endpointOwnerFixtures) {
  env.OFFGRID_INFERENCE_ENDPOINT_OWNER_POLICY_FILE = join(evidence, 'owner-policy.json');
  await writeFile(env.OFFGRID_INFERENCE_ENDPOINT_OWNER_POLICY_FILE, JSON.stringify({ version: 1, credentials: [{ id: 'fixture-credential', environment: 'OFFGRID_ISOLATED_UNAVAILABLE_CREDENTIAL', origins: ['https://fixture.example.test'] }], managed_templates: [] }));
}
const log = createWriteStream(join(evidence, 'service.log'));
let service, stopped, spawnError, closing = false, restarting = false;
function start() {
  spawnError = undefined;
  service = spawn(resolve(process.argv[2]), ['serve'], { cwd: evidence, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
  service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
  service.once('error', error => { spawnError = error; });
  stopped = new Promise(resolve => service.once('exit', resolve));
}
start();
// Test-only restart control lives in this disposable directory. It can only
// terminate the wrapper's owned child, never an installed service or container.
const restartTimer = modelFixtures || endpointFixtures || endpointOwnerFixtures ? setInterval(async () => {
  if (closing || restarting) return;
  restarting = true;
  try {
    const token = await readFile(join(evidence, 'restart-request'), 'utf8').catch(error => { if (error.code === 'ENOENT') return ''; throw error; });
    if (!token) return;
    await unlink(join(evidence, 'restart-request'));
    service.kill('SIGKILL'); await stopped; start();
    let ready = false;
    for (let i = 0; i < 100; i++) {
      try { ready = (await fetch(`http://127.0.0.1:${port}/api/v2/system`, { signal: AbortSignal.timeout(500) })).ok; } catch {}
      if (ready) break;
      await delay(100);
    }
    if (!ready) throw new Error('Fixture restart failed');
    await writeFile(join(evidence, 'restart-ack'), token);
  } catch (error) { console.error(error); process.exitCode = 1; }
  finally { restarting = false; }
}, 100) : null;
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
  const tests = spawn(process.execPath, [require.resolve('@playwright/test/cli'), 'test', ...process.argv.slice(3).filter(arg => !['--model-fixtures', '--endpoint-fixtures', '--endpoint-owner-fixtures'].includes(arg))], {
    cwd: join(root, 'apps/web'), env: { ...process.env, OFFGRID_E2E_URL: url, ...(modelFixtures ? { OFFGRID_E2E_MODEL_FIXTURES: '1', OFFGRID_E2E_RESTART_DIR: evidence } : {}), ...(endpointFixtures ? { OFFGRID_E2E_ENDPOINT_FIXTURES: evidence, OFFGRID_E2E_RESTART_DIR: evidence } : {}), ...(endpointOwnerFixtures ? { OFFGRID_E2E_ENDPOINT_OWNER_FIXTURES: evidence, OFFGRID_E2E_RESTART_DIR: evidence } : {}) }, windowsHide: true, stdio: 'inherit'
  });
  const [code] = await once(tests, 'exit');
  process.exitCode = code ?? 1;
} finally {
  closing = true;
  if (restartTimer) clearInterval(restartTimer);
  while (restarting) await delay(50);
  if (service.exitCode === null && !spawnError) {
    service.kill('SIGTERM');
    const timer = setTimeout(() => service.kill('SIGKILL'), 5000); timer.unref();
    await stopped; clearTimeout(timer);
  }
  log.end();
}
