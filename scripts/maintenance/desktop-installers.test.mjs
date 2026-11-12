import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdtempSync, writeFileSync, rmSync, existsSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';

const unix = readFileSync('installers/desktop.sh', 'utf8');
const windows = readFileSync('installers/desktop.ps1', 'utf8');
const bash = process.platform === 'win32' ? 'C:/Program Files/Git/bin/bash.exe' : 'bash';
const ps = process.platform === 'win32' ? 'powershell.exe' : 'pwsh';
const run = (command, args) => {
  const env = { ...process.env };
  // CI launches Node from pwsh. Windows PowerShell must discover its own
  // built-in modules instead of inheriting PowerShell 7's PSModulePath.
  if (process.platform === 'win32' && command === ps) {
    for (const key of Object.keys(env)) if (key.toLowerCase() === 'psmodulepath') delete env[key];
  }
  const result = spawnSync(command, args, { encoding: 'utf8', windowsHide: true, env });
  assert.equal(result.status, 0, result.error?.message || result.stdout + result.stderr);
};

test('Unix verifies exact manifest entries, corruption, and process ownership without installation', () => {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-script-test-'));
  try {
    const payload = 'fixture payload';
    writeFileSync(join(root, 'payload'), payload);
    const digest = createHash('sha256').update(payload).digest('hex');
    for (const [name, content] of Object.entries({ valid: `${digest}  asset.zip\n`, binary: `${digest} *asset.zip\r\n`, missing: `${digest}  other.zip\n`, duplicate: `${digest}  asset.zip\n${digest}  asset.zip\n`, corrupt: `${'0'.repeat(64)}  asset.zip\n`, malformed: 'invalid  asset.zip\n' })) {
      writeFileSync(join(root, name), content);
    }
    const code = unix.slice(0, unix.indexOf('\nprint_banner\n')) + `
cd "$(dirname "$0")"
verify_release_checksum payload asset.zip valid
verify_release_checksum payload asset.zip binary
for manifest in missing duplicate corrupt malformed nonexistent; do
  if verify_release_checksum payload asset.zip "$manifest"; then exit 9; fi
done
pgrep() { return 0; }
if assert_cli_stopped; then exit 10; fi
pgrep() { return 1; }
assert_cli_stopped
pgrep() { return 2; }
if assert_cli_stopped; then exit 11; fi
`;
    writeFileSync(join(root, 'test.sh'), code);
    run(bash, [join(root, 'test.sh').replaceAll('\\', '/')]);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('Windows verifies manifests, preserves full payload, and selects user or machine scope', { skip: process.platform !== 'win32' && !process.env.OFFGRID_TEST_PWSH }, () => {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-script-test-'));
  try {
    const script = `
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile('${resolve('installers/desktop.ps1').replaceAll("'", "''")}', [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$ast.FindAll({param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst]}, $false) | ForEach-Object { Invoke-Expression $_.Extent.Text }
Set-Location -LiteralPath $PSScriptRoot
Set-Content payload 'fixture'
$hash = (Get-FileHash payload).Hash
Set-Content manifest "$hash  asset.zip"
Assert-ReleaseChecksum payload asset.zip manifest
Set-Content manifest "$hash *asset.zip"
Assert-ReleaseChecksum payload asset.zip manifest
foreach ($content in @("$hash  other.zip", "$hash  asset.zip\n$hash  asset.zip", ('0' * 64 + '  asset.zip'), 'invalid  asset.zip')) {
  Set-Content manifest $content
  $failed = $false
  try { Assert-ReleaseChecksum payload asset.zip manifest } catch { $failed = $true }
  if (-not $failed) { throw 'Expected checksum failure' }
}
$failed = $false
try { Assert-ReleaseChecksum payload asset.zip absent } catch { $failed = $true }
if (-not $failed) { throw 'Expected missing manifest failure' }
$user = Get-CliInstallTarget $false
$machine = Get-CliInstallTarget $true
if ($user.Scope -ne 'User' -or -not $user.Directory.StartsWith($env:LOCALAPPDATA)) { throw 'Wrong user target' }
if ($machine.Scope -ne 'Machine' -or -not $machine.Directory.StartsWith($env:ProgramFiles)) { throw 'Wrong machine target' }
New-Item -ItemType Directory -Path source, source/web/ui, source/audio/piper -Force | Out-Null
foreach ($file in @('offgrid.exe', 'llama-server.exe', 'ggml.dll', 'runtime-dependencies.json', 'web/ui/index.html', 'audio/piper/data')) { Set-Content "source/$file" $file }
Copy-CliPayload (Join-Path $PSScriptRoot source) (Join-Path $PSScriptRoot destination)
foreach ($file in Get-ChildItem source -Recurse -File) {
  $relative = $file.FullName.Substring((Join-Path $PSScriptRoot source).Length + 1)
  if ((Get-FileHash $file.FullName).Hash -ne (Get-FileHash "destination/$relative").Hash) { throw 'Payload differs' }
}
Remove-Item source/llama-server.exe
$failed = $false
try { Copy-CliPayload (Join-Path $PSScriptRoot source) (Join-Path $PSScriptRoot incomplete) } catch { $failed = $true }
if (-not $failed -or (Test-Path incomplete)) { throw 'Incomplete bundle accepted' }
function Get-Process { return @() }
function Get-CimInstance { return [pscustomobject]@{ Architecture = 9 } }
Add-Type 'public static class OffGridInstallerCpu { public static bool IsProcessorFeaturePresent(uint feature) { return true; } }'
Assert-CliStopped
function Get-Process { return @{ ProcessName = 'llama-server' } }
$failed = $false
try { Assert-CliStopped } catch { $failed = $true }
if (-not $failed) { throw 'Active runtime accepted' }
function Get-Process { throw 'Process inspection unavailable' }
$failed = $false
try { Assert-CliStopped } catch { $failed = $true }
if (-not $failed) { throw 'Process inspection failure accepted' }
`;
    writeFileSync(join(root, 'test.ps1'), script);
    run(ps, ['-NoProfile', '-File', join(root, 'test.ps1')]);
    assert.ok(existsSync(join(root, 'destination/web/ui/index.html')));
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('each payload is verified before extraction, execution or installation', () => {
  assert.ok(windows.indexOf('Assert-ReleaseChecksum $CLI_ZIP') < windows.indexOf('Expand-Archive -Path'));
  assert.ok(windows.indexOf('Assert-ReleaseChecksum $INSTALLER_PATH') < windows.indexOf('Start-Process -FilePath'));
  assert.match(windows, /if \(\$setup.ExitCode -ne 0\) \{ throw/);
  assert.doesNotMatch(unix, /pkill/);
  assert.match(unix, /verify_release_checksum "\$\{TMP_DIR\}\/\$\{CLI_BUNDLE\}"/);
  assert.equal((unix.match(/verify_release_checksum "\$\{TMP_DIR\}\/OffGrid-LLM-Desktop.AppImage"/g) || []).length, 2);
  assert.match(unix, /verify_release_checksum "\$\{TMP_DIR\}\/offgrid-desktop.deb"/);
  assert.match(unix, /verify_release_checksum "\$\{TMP_DIR\}\/offgrid-desktop.zip"/);
});

test('Windows entry point rejects corruption and reports setup failure', { skip: process.platform !== 'win32' }, () => {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-script-test-'));
  try {
    for (const choice of ['1', '2', 'setup-failure']) {
      const code = `
$ErrorActionPreference = 'Stop'
$env:TEMP = $PSScriptRoot
function Read-Host { return '${choice === 'setup-failure' ? '2' : choice}' }
function Get-Process { return @() }
function Invoke-RestMethod { return @{ tag_name = 'v0.4.28' } }
function Invoke-WebRequest {
  param($Uri, $OutFile, [switch]$UseBasicParsing)
  if ($Uri.EndsWith('.sha256')) {
    Set-Content -LiteralPath $OutFile -Value @(('0' * 64 + '  offgrid-v0.4.28-windows-amd64-cpu-avx2.zip'), ('0' * 64 + '  OffGrid-0.4.28-windows-x64-setup.exe'))
  } else {
    Set-Content -LiteralPath $OutFile 'fixture payload'
    ${choice === 'setup-failure' ? `Set-Content -LiteralPath (Join-Path (Split-Path $OutFile) 'checksums.sha256') ((Get-FileHash $OutFile).Hash + '  OffGrid-0.4.28-windows-x64-setup.exe')` : ''}
  }
}
function Expand-Archive { throw 'UNSAFE extraction reached' }
function Start-Process { ${choice === 'setup-failure' ? 'return @{ ExitCode = 17 }' : "throw 'UNSAFE execution reached'"} }
try {
  & '${resolve('installers/desktop.ps1').replaceAll("'", "''")}'
  throw 'Expected checksum rejection'
} catch {
  if ($_.Exception.Message -notmatch '${choice === 'setup-failure' ? 'Desktop installer exited with code 17' : 'Checksum mismatch'}') { throw }
}
`;
      writeFileSync(join(root, 'entry.ps1'), code);
      run(ps, ['-NoProfile', '-File', join(root, 'entry.ps1')]);
    }
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('Unix entry point rejects corrupt downloads before unpacking or copying', () => {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-script-test-'));
  try {
    for (const [os, choice] of [['Linux', '1'], ['Linux', '2'], ['Darwin', '2']]) {
      const mocks = `
uname() { if [ "$1" = -s ]; then echo ${os}; else echo x86_64; fi; }
awk() { case "$*" in *'/proc/cpuinfo'*) echo 'sse4_2 avx avx2 f16c fma bmi2';; *) command awk "$@";; esac; }
pgrep() { return 1; }
curl() {
  case "$2" in
    */releases/latest) echo '{"tag_name": "v0.4.28"}';;
    *.sha256)
      for asset in offgrid-v0.4.28-linux-amd64-cpu-avx2.tar.gz OffGrid-0.4.28-linux-amd64.deb OffGrid-0.4.28-linux-x86_64.AppImage OffGrid-0.4.28-macos-x64.zip; do
        printf '%064d  %s\\n' 0 "$asset"
      done > "$4";;
    *) printf 'corrupt payload' > "$4";;
  esac
}
tar() { echo 'UNSAFE extraction reached'; exit 99; }
ditto() { echo 'UNSAFE extraction reached'; exit 99; }
cp() { echo 'UNSAFE copy reached'; exit 99; }
sudo() { echo 'UNSAFE privilege command reached'; exit 99; }
dpkg() { echo 'UNSAFE install reached'; exit 99; }
`;
      writeFileSync(join(root, 'entry.sh'), mocks + unix);
      const result = spawnSync(bash, [join(root, 'entry.sh').replaceAll('\\', '/')], { input: `${choice}\n`, encoding: 'utf8', windowsHide: true });
      assert.equal(result.status, 1, result.error?.message || result.stdout + result.stderr);
      assert.match(result.stderr, /Checksum mismatch/);
      assert.doesNotMatch(result.stdout + result.stderr, /UNSAFE|Installation complete/);
    }
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test('hardware selection matches release assets and refuses unsupported CLI hosts before download', () => {
  const rootScript = readFileSync('install.sh', 'utf8');
  const definitions = rootScript.slice(0, rootScript.lastIndexOf('\nmain "$@"'));
  const featureFunctions = source => source.slice(source.indexOf('cpu_features_from_flags() {'), source.indexOf('\ndetect_gpu_variant() {') > 0 ? source.indexOf('\ndetect_gpu_variant() {') : source.indexOf('\nprint_banner\n')).trim();
  assert.equal(featureFunctions(rootScript), featureFunctions(unix), 'standalone Unix hardware probes must stay aligned');
  const fixture = mkdtempSync(join(tmpdir(), 'offgrid-hardware-'));
  try {
    const code = definitions + `
test "$(cpu_features_from_flags 'sse4_2 avx avx2 f16c fma bmi2')" = avx2
test "$(cpu_features_from_flags 'SSE4.2 AVX1.0 AVX2.0 F16C FMA BMI2')" = avx2
test "$(cpu_features_from_flags 'sse4_2 avx avx2 f16c fma bmi2 avx512f avx512dq avx512bw avx512vl')" = avx512
test "$(cpu_features_from_flags 'sse4_2 avx avx2 f16c fma bmi2 avx512f')" = avx2
test "$(cpu_features_from_flags 'sse4_2')" = basic
test "$(cpu_features_from_flags 'sse4_2 avx avx2 f16c bmi2')" = basic
uname(){ if [ "$1" = -s ]; then echo "$fixture_os"; else echo "$fixture_arch"; fi; }
fixture_os=Darwin; fixture_arch=arm64
test "$(detect_cpu_features)" = apple-silicon
fixture_os=Linux; fixture_arch=aarch64
test "$(detect_cpu_features)" = neon
download_with_progress(){ echo "REQUEST $1"; return 1; }
download_cli_bundle darwin arm64 v0.4.28 avx2 "$1" cpu || true
download_cli_bundle linux arm64 v0.4.28 basic "$1" cpu || true
echo UNSUPPORTED
if download_cli_bundle linux amd64 v0.4.28 basic "$1" cpu; then exit 90; fi
if download_cli_bundle windows arm64 v0.4.28 avx2 "$1" cpu; then exit 91; fi
download_desktop_app linux arm64 v0.4.28 "$1"
`;
    const result = spawnSync(bash, ['-s', '--', fixture.replaceAll('\\', '/')], { input: code, encoding: 'utf8', windowsHide: true });
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.match(result.stdout, /darwin-arm64-metal-apple-silicon\.tar\.gz/);
    assert.match(result.stdout, /linux-arm64-cpu-neon\.tar\.gz/);
    assert.doesNotMatch(result.stdout, /arm64-cpu-avx|arm64-cpu-basic|arm64.AppImage/);
    assert.doesNotMatch(result.stdout.split('UNSUPPORTED')[1], /REQUEST/);
    // Desktop-only ARM64 must fail before touching the CLI or network.
    const armEntry = 'uname(){ if [ "$1" = -s ]; then echo Linux; else echo aarch64; fi; }\ncurl(){ echo UNSAFE; exit 99; }\n' + unix;
    const refused = spawnSync(bash, ['-c', armEntry], { input: '3\n', encoding: 'utf8', windowsHide: true });
    assert.equal(refused.status, 1);
    assert.match(refused.stderr, /CLI\/web support only/);
    assert.doesNotMatch(refused.stdout, /UNSAFE/);
  } finally { rmSync(fixture, { recursive: true, force: true }); }
});

test('Windows hardware preflight rejects ARM64 and missing AVX2', { skip: process.platform !== 'win32' }, () => {
  const script = `
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile('${resolve('installers/desktop.ps1').replaceAll("'", "''")}', [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$ast.FindAll({param($n) $n -is [System.Management.Automation.Language.FunctionDefinitionAst]}, $false) | ForEach-Object { Invoke-Expression $_.Extent.Text }
Add-Type 'public static class OffGridInstallerCpu { public static bool Available = false; public static bool IsProcessorFeaturePresent(uint feature) { return Available; } }'
function Get-CimInstance { return [pscustomobject]@{ Architecture = 12 } }
$refused = $false
try { Get-NativeArchitecture } catch { $refused = $_.Exception.Message -match 'native x64 only' }
if (-not $refused) { throw 'ARM64 accepted' }
function Get-CimInstance { return [pscustomobject]@{ Architecture = 9 } }
if ((Get-NativeArchitecture) -ne 'amd64') { throw 'x64 rejected' }
$refused = $false
try { Assert-CliHardware } catch { $refused = $_.Exception.Message -match 'requires AVX2' }
if (-not $refused) { throw 'Unsupported CPU accepted' }
[OffGridInstallerCpu]::Available = $true
Assert-CliHardware
`;
  run(ps, ['-NoProfile', '-Command', script]);
});

test('root speech install uses verified payload only and propagates copy failure', () => {
  const source = readFileSync('install.sh', 'utf8');
  const definitions = source.slice(0, source.lastIndexOf('\nmain "$@"'));
  const fixture = mkdtempSync(join(tmpdir(), 'offgrid-audio-install-'));
  try {
    const code = definitions + `
export HOME="$1/home"
download_with_progress(){ echo 'UNSAFE download'; exit 99; }
install_audio "$1/missing"
mkdir -p "$1/payload/audio/whisper"
echo fixture > "$1/payload/audio/whisper/worker"
install_audio "$1/payload"
cmp "$1/payload/audio/whisper/worker" "$HOME/.offgrid-llm/audio/whisper/worker"
cp(){ return 1; }
if install_audio "$1/payload"; then echo 'UNSAFE success'; exit 98; fi
`;
    const result = spawnSync(bash, ['-s', '--', fixture.replaceAll('\\', '/')], { input: code, encoding: 'utf8', windowsHide: true });
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.doesNotMatch(result.stdout + result.stderr, /UNSAFE/);
    assert.doesNotMatch(source, /download_piper/);
  } finally { rmSync(fixture, { recursive: true, force: true }); }
});

test('generated archive installer preserves runtimes and refuses active processes', () => {
  const workflow = readFileSync('.github/workflows/release-unified.yml', 'utf8');
  const match = workflow.match(/cat > "\$BUNDLE_NAME\/install\.sh" << 'EOF'\r?\n([\s\S]*?)\r?\n          EOF/);
  assert.ok(match, 'generated archive installer exists');
  const installer = match[1].split(/\r?\n/).map(line => line.replace(/^          /, '')).join('\n');
  const root = mkdtempSync(join(tmpdir(), 'offgrid-script-test-'));
  try {
    for (const name of ['offgrid', 'llama-server', 'offgrid-document-worker', 'document-NOTICE.txt', 'offgrid-ocr-worker', 'ocr-NOTICE.txt', 'execution-runtime/execution-pack.json', 'audio/whisper/whisper', 'audio/piper/piper', 'web/ui/index.html']) {
      mkdirSync(join(root, name, '..'), { recursive: true });
      writeFileSync(join(root, name), `fixture ${name}`);
    }
    const prelude = `
fixture_root="$(cd "$(dirname "$0")" && pwd)"
export HOME="$fixture_root/home"
pgrep(){ return "$PROCESS_STATUS"; }
sudo(){
  local operation="$1"; shift
  local args=() arg
  for arg in "$@"; do
    case "$arg" in
      /usr/local/*) arg="$fixture_root/installed\${arg#/usr/local}" ;;
      /var/lib/offgrid*) arg="$fixture_root/legacy-ui\${arg#/var/lib/offgrid}" ;;
      "$fixture_root"/*) ;;
      /*) echo "Unexpected absolute fixture path: $arg" >&2; return 98 ;;
    esac
    args+=("$arg")
  done
  command "$operation" "\${args[@]}"
}
`;
    writeFileSync(join(root, 'install.sh'), prelude + installer);
    for (const status of ['0', '2']) {
      const result = spawnSync(bash, [join(root, 'install.sh').replaceAll('\\', '/')], { encoding: 'utf8', windowsHide: true, env: { ...process.env, PROCESS_STATUS: status } });
      assert.equal(result.status, 1, result.stdout + result.stderr);
      assert.equal(existsSync(join(root, 'installed')), false, 'active/error state changed installation');
    }
    mkdirSync(join(root, 'installed/bin'), { recursive: true });
    const result = spawnSync(bash, [join(root, 'install.sh').replaceAll('\\', '/')], { encoding: 'utf8', windowsHide: true, env: { ...process.env, PROCESS_STATUS: '1' } });
    assert.equal(result.status, 0, result.stdout + result.stderr);
    for (const name of ['execution-runtime/execution-pack.json', 'audio/whisper/whisper', 'audio/piper/piper', 'web/ui/index.html']) {
      assert.equal(readFileSync(join(root, 'installed/bin', name), 'utf8'), `fixture ${name}`);
    }
  } finally { rmSync(root, { recursive: true, force: true }); }
});
