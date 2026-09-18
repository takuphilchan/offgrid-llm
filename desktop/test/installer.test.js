const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

test('Finish only records intent; launch happens after the wizard closes', () => {
  const source = fs.readFileSync(path.join(__dirname, '../assets/installer.nsh'), 'utf8');
  const request = source.match(/Function OffGridRequestLaunch([\s\S]*?)FunctionEnd/)[1];
  assert.match(request, /StrCpy \$OffGridLaunchRequested "yes"/);
  assert.doesNotMatch(request, /Exec|Sleep|Wait|Shell/);
  const closed = source.match(/Function \.onGUIEnd([\s\S]*?)FunctionEnd/)[1];
  assert.match(closed, /\$OffGridLaunchRequested == "yes"/);
  assert.match(closed, /\$\{IfNot\} \$\{UAC_IsAdmin\}/);
  assert.match(closed, /StdUtils.ExecShellAsUser/);
});

test('running-app handling cannot force-kill processes or invoke PowerShell', () => {
  const source = fs.readFileSync(path.join(__dirname, '../assets/installer.nsh'), 'utf8');
  const check = source.match(/!macro customCheckAppRunning([\s\S]*?)!macroend/)[1];
  assert.doesNotMatch(check, /KillProcess|CloseProcess|taskkill|Stop-Process|nsExec/);
  assert.match(check, /--offgrid-quit-for-install/);
  assert.match(check, /\$\{Silent\}/);
  assert.match(check, /SetErrorLevel 2/);
});

test('installer keeps native DPI support and exposes installation details', () => {
  const source = fs.readFileSync(path.join(__dirname, '../assets/installer.nsh'), 'utf8');
  assert.match(source, /ManifestDPIAware true/);
  assert.match(source, /SetFont "Segoe UI" 9/);
  assert.match(source, /IsHighContrastModeActive/);
  assert.match(source, /!macro customCheckAppRunning[\s\S]*?SetDetailsPrint listonly/);
  assert.match(source, /Function \.onInstFailed[\s\S]*?DetailPrint/);
  for (const [name, width, height] of [['installer-header.bmp', 600, 228], ['installer-sidebar.bmp', 656, 1256]]) {
    const bitmap = fs.readFileSync(path.join(__dirname, '../assets', name));
    assert.equal(bitmap.readInt32LE(18), width);
    assert.equal(bitmap.readInt32LE(22), height);
  }
});
