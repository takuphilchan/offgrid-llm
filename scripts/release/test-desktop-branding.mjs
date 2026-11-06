import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

const read = path => readFileSync(new URL(`../../${path}`, import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const pkg = JSON.parse(read('apps/desktop/package.json'));
assert.equal(pkg.name, 'offgrid-llm-desktop');
assert.equal(pkg.build.productName, 'OffGrid');
assert.equal(pkg.build.appId, 'com.offgrid.llm.desktop');
assert.equal(pkg.build.win.artifactName, 'OffGrid-${version}-windows-${arch}-setup.${ext}');
assert.equal(pkg.build.portable.artifactName, 'OffGrid-${version}-windows-${arch}-portable.${ext}');
assert.equal(pkg.build.mac.artifactName, 'OffGrid-${version}-macos-${arch}.${ext}');
assert.equal(pkg.build.linux.artifactName, 'OffGrid-${version}-linux-${arch}.${ext}');
const cases = [
  ['0.4.26', 'windows', 'x64', '-setup.exe', 'OffGrid.LLM.Desktop-Setup-0.4.26.exe'],
  ['0.4.27', 'windows', 'x64', '-setup.exe', 'OffGrid-0.4.27-windows-x64-setup.exe'],
  ['0.4.26', 'macos', 'arm64', '.zip', 'OffGrid.LLM.Desktop-0.4.26-arm64.zip'],
  ['0.4.27', 'macos', 'arm64', '.zip', 'OffGrid-0.4.27-macos-arm64.zip'],
  ['0.4.27', 'macos', 'x64', '.zip', 'OffGrid-0.4.27-macos-x64.zip'],
  ['0.4.27', 'linux', 'amd64', '.deb', 'OffGrid-0.4.27-linux-amd64.deb'],
  ['0.4.26', 'linux', 'x86_64', '.AppImage', 'OffGrid.LLM.Desktop-0.4.26-x86_64.AppImage'],
  ['v0.4.27-rc.1', 'linux', 'x86_64', '.AppImage', 'OffGrid-0.4.27-rc.1-linux-x86_64.AppImage'],
  ['1.0.0', 'windows', 'x64', '-setup.exe', 'OffGrid-1.0.0-windows-x64-setup.exe'],
];
for (const path of ['install.sh', 'installers/desktop.sh']) {
  const source = read(path);
  const helper = source.match(/desktop_asset_name\(\) \{[\s\S]*?\n\}/)?.[0];
  assert.ok(helper, `${path}: standalone installer naming helper`);
  for (const [version, os, arch, kind, expected] of cases) {
    const result = spawnSync('bash', ['-s'], { encoding: 'utf8', input: `${helper}\ndesktop_asset_name '${version}' '${os}' '${arch}' '${kind}'\n` });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, expected, path);
  }
}
const ps = read('installers/desktop.ps1');
assert.ok(ps.includes("$coreVersion -ge [version]'0.4.27'"));
assert.ok(ps.includes('OffGrid-${VERSION}-windows-x64-setup.exe'));
assert.ok(ps.includes('OffGrid.LLM.Desktop-Setup-${VERSION}.exe'));
assert.ok(!read('installers/desktop.sh').includes('hdiutil attach'));
console.log('Desktop identity and current/historical download names passed');
