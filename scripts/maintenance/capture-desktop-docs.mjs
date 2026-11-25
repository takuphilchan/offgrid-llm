// Hidden native terminal capture with a disposable profile and local API fixtures.
// Runs only the packaged CLI's read-only --help command, never model inference.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {once} from 'node:events';
import {mkdtemp, mkdir, readFile, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve, join, dirname, extname, sep} from 'node:path';
import {createRequire} from 'node:module';

const root = resolve(import.meta.dirname, '../..');
assert.ok(process.argv[2], 'Pass the packaged desktop executable');
const binary = resolve(process.argv[2]), dist = join(dirname(binary), 'resources/ui');
const load = createRequire(join(root, 'apps/web/package.json'));
const {_electron: electron, expect} = load('@playwright/test');
const {fingerprintUI} = load(join(root, 'apps/desktop/compatibility.cjs'));
const hash = fingerprintUI(await readFile(join(dist, 'index.html')));
let metadata;
try { metadata = await readFile(join(dirname(binary), '../candidate.json'), 'utf8'); }
catch (error) {
  if (error.code !== 'ENOENT') throw error;
  metadata = await readFile(join(dirname(binary), '../preview.json'), 'utf8');
}
const packageInfo = JSON.parse(metadata);
const version = packageInfo.version;
const profile = await mkdtemp(join(tmpdir(), 'offgrid-documentation-'));
const folder = join(profile, 'OffGrid Demo'), output = join(root, 'build/documentation-captures');
await mkdir(folder); await mkdir(output, {recursive: true});
await writeFile(join(profile, 'desktop-presentation.json'), JSON.stringify({locale: 'en', theme: 'dark'}));
const identity = {product: 'offgrid', version, api_version: 2, workspace_id: 'documentation-terminal',
  ui_build_id: hash, desktop_bridge: {status: 'ready', schema_version: 1, protocol: 1, ui_build_id: hash},
  capabilities: ['sessions-v1', 'chat-streaming-v1', 'durable-agent-runs-v1']};
const bodies = {
  '/health': {status: 'healthy'}, '/api/v2/system': identity,
  '/v1/system/config': {version, require_auth: false}, '/v1/users/me': {authenticated: false, auth_required: false},
  '/v1/models': {data: []}, '/v1/sessions': {sessions: []}, '/v1/rag/status': {enabled: false},
  '/api/v2/agent-profiles': {profiles: []}, '/api/v2/models': {models: []},
  '/api/v2/models/operations': {operations: []}, '/v1/audio/status': {profiles: []},
  '/api/v2/jobs': [], '/v1/agents/tasks': [], '/api/v2/computer/status': {available: false},
};
const writes = [];
const server = createServer(async (request, response) => {
  try {
    const pathname = new URL(request.url, 'http://localhost').pathname;
    if (request.method !== 'GET') { writes.push(pathname); response.writeHead(405).end(); return; }
    if (pathname.startsWith('/ui/')) {
      const file = resolve(dist, decodeURIComponent(pathname.slice(4)) || 'index.html');
      if (!file.startsWith(dist + sep)) { response.writeHead(403).end(); return; }
      const types = {'.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css',
        '.json': 'application/json', '.woff2': 'font/woff2', '.svg': 'image/svg+xml'};
      response.writeHead(200, {'Content-Type': types[extname(file)] || 'application/octet-stream'}).end(await readFile(file));
      return;
    }
    response.writeHead(200, {'Content-Type': 'application/json'}).end(JSON.stringify(bodies[pathname] ?? {}));
  } catch { response.writeHead(404).end(); }
});
server.listen(0, '127.0.0.1'); await once(server, 'listening');
const env = {...process.env, OFFGRID_DESKTOP_HOME: profile, OFFGRID_DESKTOP_TEST_HIDDEN: '1',
  OFFGRID_PORT: String(server.address().port), NO_COLOR: '', FORCE_COLOR: '1'};
delete env.ELECTRON_RUN_AS_NODE;
let app;
try {
  app = await electron.launch({executablePath: binary, args: [], cwd: dirname(binary), env, timeout: 30000});
  app.context().setDefaultTimeout(20000);
  const page = await app.firstWindow(); await page.waitForURL(/\/ui\//);
  assert.equal(await app.evaluate(({BrowserWindow}) => BrowserWindow.getAllWindows()[0].isVisible()), false);
  await app.evaluate(({BrowserWindow, dialog}, folder) => {
    const win = BrowserWindow.getAllWindows()[0];
    win.show = () => {}; win.focus = () => {};
    win.setContentSize(1440, 640); win.webContents.setZoomFactor(1);
    win.webContents.setBackgroundThrottling(false);
    dialog.showOpenDialog = async () => ({canceled: false, filePaths: [folder]});
  }, folder);
  await page.evaluate(() => {
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.setItem('offgrid.locale', 'en'); localStorage.setItem('offgrid.theme', 'dark');
  });
  await page.reload();
  await expect(page.locator('.composer textarea')).toBeVisible();
  await app.evaluate(({Menu}) => {
    const action = Menu.getApplicationMenu().items.flatMap(item => item.submenu?.items ?? [])
      .find(item => item.accelerator === 'CommandOrControl+`');
    if (!action) throw Error('Missing native terminal action');
    action.click();
  });
  await expect.poll(() => app.context().pages().some(p => p.url().endsWith('/terminal.html'))).toBe(true);
  const terminal = app.context().pages().find(p => p.url().endsWith('/terminal.html'));
  await expect(terminal.locator('#start')).toBeEnabled();
  // The initial native theme arrives asynchronously. Capture its settled surface,
  // not the short transition from the browser's default button color.
  await expect.poll(() => terminal.locator('#start').evaluate(el => {
    const style = getComputedStyle(el), probe = document.createElement('span');
    probe.style.color = style.getPropertyValue('--canvas').trim() || style.getPropertyValue('--bg').trim();
    document.body.append(probe);
    const ready = style.backgroundColor === getComputedStyle(probe).color;
    probe.remove(); return ready;
  })).toBe(true);
  await capture('terminal-welcome.png');
  await terminal.locator('#start').evaluate(button => button.click());
  await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText('PS ');
  // Use the terminal's ordinary trusted IPC input path; no OS input or focus.
  const send = text => terminal.evaluate(text => request('input', {id: current, text}), text);
  await send("New-PSDrive -Name Demo -PSProvider FileSystem -Root (Get-Location).Path | Out-Null; Set-Location Demo:\\; Clear-Host\r");
  await expect.poll(() => terminal.evaluate(() => {
    const term = tabs.get(current).term;
    return term.buffer.active.getLine(term.buffer.active.baseY + term.buffer.active.cursorY)?.translateToString(true) || '';
  })).toBe('PS Demo:\\> ');
  await send('offgrid --help\r');
  await expect(terminal.locator('.pane:not([hidden]) .xterm-accessibility-tree')).toContainText(`OffGrid ${version}`);
  await expect.poll(() => terminal.evaluate(() => tabs.get(current).commands.commands.at(-1)?.finished)).toBe(true);
  async function capture(name) {
    await terminal.evaluate(() => document.fonts.ready);
    // Warm hidden compositor frames, then retain its settled native capture.
    for (let frame = 0; frame < 2; frame++) {
      const png = await app.evaluate(async ({webContents}, url) => {
        const view = webContents.getAllWebContents().find(view => view.getURL() === url);
        view.setBackgroundThrottling(false);
        return (await view.capturePage(undefined, {stayHidden: true, stayAwake: true})).toPNG().toString('base64');
      }, terminal.url());
      if (frame) await writeFile(join(output, name), Buffer.from(png, 'base64'));
    }
  }
  await capture('desktop-terminal.png');
  await app.evaluate(({BrowserWindow}) => BrowserWindow.getAllWindows()[0].setContentSize(1440, 940));
  await expect.poll(() => terminal.evaluate(() => innerHeight)).toBeGreaterThan(800);
  await terminal.locator('#commands').evaluate(button => button.click());
  await expect(terminal.locator('#context-state')).toContainText('exit 0');
  await capture('terminal-command-context.png');
  assert.deepEqual(writes, [], 'Documentation capture must not submit service work');
  assert.equal(await app.evaluate(({BrowserWindow}) => BrowserWindow.getAllWindows()[0].isVisible()), false);
  await writeFile(join(output, 'desktop-provenance.json'), JSON.stringify({version, ui_build_id: hash,
    profile, hidden: true, command: 'offgrid --help', service: 'local HTTP fixture', modelInference: false}, null, 2));
  console.log(JSON.stringify({status: 'captured', version, output}));
} finally {
  if (app) await app.close();
  server.close(); server.closeAllConnections();
}
