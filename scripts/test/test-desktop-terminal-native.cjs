// Explicit isolated Windows native-module smoke check. This does NOT qualify
// the Electron dock, interactive input, or descendant process cleanup.
if (!process.argv.includes('--isolated-native-test')) throw Error('Pass --isolated-native-test to launch a disposable native shell.');
if (process.platform !== 'win32') throw Error('Windows qualification only; other platforms remain unverified.');
const fs = require('node:fs'), path = require('node:path'), os = require('node:os');
const pty = require('../../apps/desktop/node_modules/node-pty');
const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'offgrid-pty-smoke-'));
const shell = path.join(process.env.SystemRoot, 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe');
const marker = 'OFFGRID_NATIVE_PTY_READY';
let output = '', finished = false;
const child = pty.spawn(shell, ['-NoLogo', '-NoProfile', '-NonInteractive', '-Command', `[Console]::WriteLine('${marker}'); exit 0`], {
  name: 'xterm-256color', cols: 90, rows: 24, cwd, useConpty: true, useConptyDll: true,
  env: { SystemRoot: process.env.SystemRoot, PATH: process.env.PATH, TEMP: cwd, TMP: cwd, USERPROFILE: cwd }
});
const timer = setTimeout(() => { if (!finished) { process.exitCode = 1; console.error('Native PTY timeout'); child.kill(); } }, 15000);
child.onData(text => { output += text; if (output.length > 65536) { process.exitCode = 1; child.kill(); } });
child.onExit(({ exitCode }) => {
  finished = true; clearTimeout(timer);
  const passed = exitCode === 0 && output.includes(marker);
  console.log(JSON.stringify({ passed, platform: process.platform, node: process.version, nodePty: '1.1.0', conptyDll: true, exitCode, isolatedDirectory: cwd }));
  if (!passed) process.exitCode = 1;
});
