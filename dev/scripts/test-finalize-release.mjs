import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const version = 'v0.3.1';
const packageVersion = version.slice(1);
const script = readFileSync(join(repo, '.github/scripts/finalize-release.sh'), 'utf8');
const names = [
  `offgrid-${version}-linux-amd64-cpu-avx2.tar.gz`,
  `offgrid-${version}-linux-amd64-cpu-avx512.tar.gz`,
  `offgrid-${version}-linux-amd64-vulkan-avx2.tar.gz`,
  `offgrid-${version}-linux-amd64-vulkan-avx512.tar.gz`,
  `offgrid-${version}-linux-arm64-cpu-neon.tar.gz`,
  `offgrid-${version}-darwin-amd64-cpu-avx2.tar.gz`,
  `offgrid-${version}-darwin-arm64-metal-apple-silicon.tar.gz`,
  `offgrid-${version}-windows-amd64-cpu-avx2.zip`,
  `OffGrid.LLM.Desktop-${packageVersion}-amd64.deb`,
  `OffGrid.LLM.Desktop-${packageVersion}-x86_64.AppImage`,
  `OffGrid.LLM.Desktop-${packageVersion}-arm64.zip`,
  `OffGrid.LLM.Desktop-${packageVersion}-x64.zip`,
  `OffGrid.LLM.Desktop-${packageVersion}-Portable.exe`,
  `OffGrid.LLM.Desktop-Setup-${packageVersion}.exe`,
];
const digest = 'a'.repeat(64);

function run(assetNames, { initialAssetNames = assetNames, attempts = '1' } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-release-finalize-'));
  try {
    mkdirSync(join(root, 'docs/releases'), { recursive: true });
    copyFileSync(
      join(repo, `docs/releases/release-notes-${version}.md`),
      join(root, `docs/releases/release-notes-${version}.md`),
    );
    const rows = assetNames.map((name) => `${name}\tuploaded\t1\tsha256:${digest}`);
    const initialRows = initialAssetNames.map((name) => `${name}\tuploaded\t1\tsha256:${digest}`);
    const rowArgs = rows.map((row) => `'${row}'`).join(' ');
    const initialRowArgs = initialRows.map((row) => `'${row}'`).join(' ');
    const mock = `
gh() {
  if [[ "$1 $2" == "release view" ]]; then
    if [[ "$*" == *"@tsv"* ]]; then
      api_count_file='.mock-gh-api-count'
      api_count=$(cat "$api_count_file" 2>/dev/null || echo 0)
      api_count=$((api_count + 1))
      printf '%s' "$api_count" > "$api_count_file"
      if [[ "$api_count" -eq 1 ]]; then
        printf '%s\\n' ${initialRowArgs}
      else
        printf '%s\\n' ${rowArgs}
      fi
    elif [[ "$*" == *"--json assets"* ]]; then
      printf '%s\\n' 'checksums-v0.3.1.sha256'
    fi
  elif [[ "$1 $2" == "release upload" ]]; then
    test -s "$4" || return 1
    echo MOCK_UPLOAD
  elif [[ "$1 $2" == "release edit" ]]; then
    echo MOCK_EDIT
  else
    return 1
  fi
}
set -- '${version}' 'example/offgrid-llm'
`;
    const result = spawnSync('bash', ['-s'], {
      cwd: root,
      input: mock + script,
      encoding: 'utf8',
      env: {
        ...process.env,
        OFFGRID_FINALIZE_ATTEMPTS: attempts,
        OFFGRID_FINALIZE_RETRY_SECONDS: '0',
      },
    });
    const checksumPath = join(root, `checksums-${version}.sha256`);
    const checksums = result.status === 0 ? readFileSync(checksumPath, 'utf8') : '';
    return { ...result, checksums };
  } finally {
    if (dirname(realpathSync(root)) !== realpathSync(tmpdir())) {
      throw new Error(`Refusing to remove unexpected test directory: ${root}`);
    }
    rmSync(root, { recursive: true, force: true });
  }
}

const complete = run(names);
assert.equal(complete.status, 0, complete.stdout + complete.stderr);
assert.match(complete.stdout, /MOCK_UPLOAD/);
assert.match(complete.stdout, /MOCK_EDIT/);
assert.equal(complete.checksums.trim().split('\n').length, names.length);
for (const name of names) {
  assert.ok(complete.checksums.includes(`${digest}  ${name}\n`), `Missing checksum: ${name}`);
}

const incomplete = run(names.slice(1));
assert.notEqual(incomplete.status, 0, 'A partial release must not publish');
assert.match(incomplete.stdout, /Missing required release asset/);
assert.doesNotMatch(incomplete.stdout, /MOCK_UPLOAD|MOCK_EDIT/);

const eventuallyComplete = run(names, { initialAssetNames: names.slice(1), attempts: '2' });
assert.equal(eventuallyComplete.status, 0, eventuallyComplete.stdout + eventuallyComplete.stderr);
assert.match(eventuallyComplete.stdout, /not fully indexed yet/);
assert.match(eventuallyComplete.stdout, /MOCK_UPLOAD/);
console.log('Release finalization accepts 14 verified assets, retries indexing, and rejects a partial release');
