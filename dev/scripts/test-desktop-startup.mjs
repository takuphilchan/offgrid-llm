// Exercise the real packaged Electron main/preload/renderer boundary against
// an isolated HTTP fixture and the bundled Go service, never a user's instance.
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { mkdtemp, readFile, mkdir, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import { createRequire } from 'node:module';
import { createHash } from 'node:crypto';
import { performance } from 'node:perf_hooks';
import { setTimeout as delay } from 'node:timers/promises';

const root = resolve(import.meta.dirname, '../..');
const require = createRequire(join(root, 'web/app/package.json'));
const { _electron: electron } = require('playwright');
const binary = resolve(process.argv[2] || '');
assert.ok(process.argv[2], 'Pass the packaged desktop executable');
const version = JSON.parse(await readFile(join(root, 'desktop/package.json'), 'utf8')).version;
const uiBuild = createHash('sha256').update((await readFile(join(root, 'web/dist/index.html'), 'utf8')).replace(/\r\n/g, '\n')).digest('hex');
const evidence = await mkdtemp(join(tmpdir(), 'offgrid-desktop-startup-'));
let responseVersion = `${version}-older-fixture`;
let requests = 0;
let hang = false;
const server = createServer((request, response) => {
  if (request.url === '/api/v2/system') {
    requests++;
    if (hang) return;
    response.setHeader('Content-Type', 'application/json');
    response.end(JSON.stringify({ product: 'offgrid', version: responseVersion, api_version: 2, ui_build_id: uiBuild,
      capabilities: ['sessions-v1', 'chat-streaming-v1', 'durable-agent-runs-v1'] }));
  } else { response.setHeader('Content-Type', 'text/html'); response.end('<!doctype html><title>Fixture workspace</title><h1>Fixture workspace</h1>'); }
});
server.listen(0, '127.0.0.1'); await once(server, 'listening');
const url = `http://127.0.0.1:${server.address().port}`;
const launch = async profile => {
  await mkdir(profile, { recursive: true });
  const env = { ...process.env, OFFGRID_PORT: String(server.address().port), OFFGRID_DESKTOP_HOME: profile, OFFGRID_DESKTOP_TEST_HIDDEN: '1' };
  delete env.ELECTRON_RUN_AS_NODE;
  return electron.launch({ executablePath: binary, env, timeout: 30000 });
};
let app;
async function capture(filename) {
  // Hidden native windows may not have a compositor frame on the first call.
  // Keep them hidden; retry only the transient capture error, not app assertions.
  await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].webContents.setBackgroundThrottling(false));
  for (let attempt = 0; attempt < 10; attempt++) {
    try {
      const png = await app.evaluate(async ({ BrowserWindow }) => (await BrowserWindow.getAllWindows()[0].webContents.capturePage(undefined, { stayHidden: true, stayAwake: true })).toPNG().toString('base64'));
      await writeFile(join(evidence, filename), Buffer.from(png, 'base64'));
      return;
    } catch (error) {
      if (!String(error).includes('UnknownVizError') || attempt === 9) throw error;
      await delay(200);
    }
  }
}
try {
  const profile = join(evidence, 'isolated-profile');
  await mkdir(profile);
  const start = performance.now();
  app = await launch(profile);
  const page = await app.firstWindow();
  await page.locator('#recovery:not([hidden])').waitFor();
  const recoveryMs = Math.round(performance.now() - start);
  assert.equal(await page.locator('#service-version').innerText(), responseVersion);
  assert.equal(await page.locator('#desktop-version').innerText(), version);
  assert.equal(await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows().length), 1);
  assert.equal(requests, 1, 'One startup controller, no duplicate renderer probes');
  const status = await page.evaluate(() => window.electron.getBackendInfo());
  assert.equal(status.managedByDesktop, false);
  await capture('version-recovery.png');
  await page.locator('#separate').click();
  await page.locator('#local').click();
  await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/ui\//, { timeout: 40000 });
  const separateURL = new URL(page.url()).origin;
  assert.notEqual(separateURL, url);
  const ready = await page.evaluate(() => window.electron.getBackendInfo());
  assert.equal(ready.state, 'ready');
  assert.equal(ready.managedByDesktop, true);
  assert.equal(ready.workspaceMode, 'isolated');
  const paths = await page.evaluate(() => window.electron.getPaths());
  assert.equal(paths.data, join(profile, 'desktop-workspace/data'));
  assert.equal(await page.evaluate(async () => { try { await window.electron.startDesktopWorkspace(); return 'allowed'; } catch { return 'denied'; } }), 'denied', 'Web UI cannot invoke startup-only process controls');
  await app.close(); app = null;
  assert.equal((await fetch(`${url}/api/v2/system`)).status, 200, 'Quitting never stops the external fixture');
  assert.equal(JSON.parse(await readFile(join(profile, 'desktop-connection.json'), 'utf8')).mode, 'isolated');
  // The explicitly chosen workspace survives application restart.
  app = await launch(profile);
  const resumed = await app.firstWindow();
  await resumed.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/ui\//, { timeout: 40000 });
  assert.equal((await resumed.evaluate(() => window.electron.getPaths())).data, paths.data);
  // Changing the native preference must not interrupt a running workspace.
  await app.evaluate(({ Menu }) => Menu.getApplicationMenu().getMenuItemById('connection-default').click());
  for (let i = 0; i < 100; i++) {
    if (JSON.parse(await readFile(join(profile, 'desktop-connection.json'), 'utf8')).mode === 'default') break;
    await delay(20);
  }
  assert.equal(JSON.parse(await readFile(join(profile, 'desktop-connection.json'), 'utf8')).mode, 'default');
  assert.equal((await resumed.evaluate(() => window.electron.getBackendInfo())).workspaceMode, 'isolated');
  await app.close(); app = null;
  // A matching external service attaches without a native child, and remains
  // alive after Electron quits. It does not adopt the isolated profile's data.
  responseVersion = version;
  app = await launch(profile);
  const external = await app.firstWindow();
  await external.waitForURL(url + '/ui/');
  assert.equal((await external.evaluate(() => window.electron.getBackendInfo())).managedByDesktop, false);
  await app.close(); app = null;
  assert.equal((await fetch(`${url}/api/v2/system`)).status, 200);
  // A hung port still gets an interactive recovery window, not a blank screen,
  // duplicate startup loop or an unapproved replacement process.
  hang = true;
  app = await launch(join(evidence, 'timeout-profile'));
  const timeout = await app.firstWindow();
  await timeout.locator('#recovery:not([hidden])').waitFor();
  assert.match(await timeout.locator('#status-text').innerText(), /timed out/);
  assert.equal((await timeout.evaluate(() => window.electron.getBackendInfo())).managedByDesktop, false);
  // Reduced motion, dark mode, keyboard recovery, and minimum window size.
  await timeout.emulateMedia({ colorScheme: 'dark', reducedMotion: 'reduce' });
  await app.evaluate(({ BrowserWindow }) => BrowserWindow.getAllWindows()[0].setSize(760, 560));
  assert.equal(await timeout.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  await timeout.locator('#separate > summary').focus();
  await timeout.keyboard.press('Enter');
  assert.equal(await timeout.locator('#separate').getAttribute('open'), '');
  assert.equal(await timeout.locator('#indicator').evaluate(element => getComputedStyle(element).animationName), 'none');
  await capture('timeout-recovery-dark.png');
  hang = false;
  await timeout.locator('#retry').focus();
  await timeout.keyboard.press('Enter');
  await timeout.waitForURL(url + '/ui/');
  await app.close(); app = null;
  console.log(JSON.stringify({ passed: true, platform: process.platform, recoveryMs, evidence }, null, 2));
} finally {
  if (app) await app.close();
  server.closeAllConnections();
  server.close();
}
