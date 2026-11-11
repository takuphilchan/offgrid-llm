// Native, isolated qualification. No installer registration or application files
// are changed: the compiled fixture writes one marker only after the guard allows it.
import assert from 'node:assert/strict';
import {spawn, spawnSync} from 'node:child_process';
import {mkdtemp, writeFile, copyFile, access} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';

assert.equal(process.platform, 'win32', 'Native guard qualification requires Windows');
const [compiler, plugins] = process.argv.slice(2);
assert.ok(compiler && plugins, 'Pass makensis.exe and the x86-unicode plugin directory');
const root=resolve(import.meta.dirname, '../..');
const guard=join(root, 'apps/desktop/assets/installer-process-guard.nsh');
const include=join(root, 'apps/desktop/node_modules/app-builder-lib/templates/nsis/include/nsProcess.nsh');
const directory=await mkdtemp(join(tmpdir(), 'offgrid-installer-guard-'));
const results=[];
const quote=value=>{
  assert.ok(!/["\r\n$]/.test(value), 'Unsupported NSIS fixture path');
  return `"${value}"`;
};
async function runCase(name, processName, code, expected) {
  const executable=join(directory, `${name}.exe`), marker=join(directory, `${name}.marker`);
  const source=join(directory, `${name}.nsi`);
  const enumeration=code===null
    ? `!addplugindir /x86-unicode ${quote(resolve(plugins))}\n!include ${quote(include)}`
    : `!define nsProcess::FindProcess \`!insertmacro FixtureFindProcess\`\n!macro FixtureFindProcess NAME RESULT\nStrCpy \${RESULT} ${code}\n!macroend`;
  await writeFile(source, `Unicode true\nRequestExecutionLevel user\nSilentInstall silent\nOutFile ${quote(executable)}\n${enumeration}\n!include ${quote(guard)}\nSection\n!insertmacro OffGridRequireProcessClosed ${quote(processName)}\nFileOpen $0 ${quote(marker)} w\nFileWrite $0 "replacement permitted"\nFileClose $0\nSetErrorLevel 0\nSectionEnd\n`);
  const compiled=spawnSync(compiler, ['/V2', source], {encoding:'utf8', windowsHide:true, timeout:30000});
  assert.equal(compiled.status, 0, compiled.stdout+compiled.stderr+String(compiled.error||''));
  const start=Date.now();
  const executed=spawnSync(executable, ['/S'], {encoding:'utf8', windowsHide:true, timeout:15000});
  assert.equal(executed.status, expected, `${name}: ${executed.stderr} ${executed.error||''}`);
  const written=await access(marker).then(()=>true,()=>false);
  assert.equal(written, expected===0, `${name}: replacement marker`);
  results.push({name, expected, exitCode:executed.status, markerWritten:written, elapsedMs:Date.now()-start});
}

for(const code of [603,0,601,602,604,605,606,607,608,9999]) {
  await runCase(`enumeration-${code}`, 'offgrid.exe', code, code===603?0:code===0?2:3);
}
const fixtureName=`OffGrid-Guard-Fixture-${process.pid}.exe`;
await runCase('native-absent', fixtureName, null, 0);
const fixture=join(directory, fixtureName);
await copyFile(join(process.env.SystemRoot, 'System32/ping.exe'), fixture);
const child=spawn(fixture, ['-t','127.0.0.1'], {stdio:'ignore', windowsHide:true});
const exited=new Promise((resolve,reject)=>{child.once('error',reject);child.once('exit',resolve);});
try {
  await delay(500);
  assert.equal(child.exitCode, null, 'Owned fixture must be running');
  assert.ok(child.pid);
  await runCase('native-running', fixtureName, null, 2);
  assert.equal(child.exitCode, null, 'Guard must preserve the running process');
  process.kill(child.pid, 0);
} finally {
  if(child.exitCode===null) child.kill(); // Only this test's exact child handle.
  await exited;
}
await runCase('native-after-exit', fixtureName, null, 0);
await writeFile(join(directory,'result.json'), JSON.stringify({passed:true, guard, results, scope:'Disposable NSIS executables, marker files and one owned ping child; no installation or registry changes.'},null,2));
console.log(`Passed ${results.length} native guard cases. Evidence: ${join(directory,'result.json')}`);
