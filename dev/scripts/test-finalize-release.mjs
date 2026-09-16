import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
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

const asset = (name) => ({ name, state: 'uploaded', size: 1, digest: `sha256:${digest}` });

function run(assetNames, {
  initialAssets = assetNames.map(asset),
  assets = assetNames.map(asset),
  attempts = '1',
  apiFailure = false,
} = {}) {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-release-finalize-'));
  try {
    mkdirSync(join(root, 'docs/releases'), { recursive: true });
    copyFileSync(
      join(repo, `docs/releases/release-notes-${version}.md`),
      join(root, `docs/releases/release-notes-${version}.md`),
    );
    writeFileSync(join(root, 'initial-assets.json'), JSON.stringify(initialAssets));
    writeFileSync(join(root, 'assets.json'), JSON.stringify(assets));
    const mock = `
gh() {
  local filter='' arg previous=''
  for arg in "$@"; do
    if [[ "$previous" == '--jq' ]]; then filter="$arg"; fi
    previous="$arg"
  done
  if [[ "$1 $2" == "release view" ]]; then
    if [[ "$*" == *"--json databaseId"* ]]; then
      printf '%s' '{"databaseId":123,"isDraft":true}' | jq -r "$filter"
    elif [[ "$*" == *"--json assets"* && -f .mock-published ]]; then
      printf '%s' '{"assets":[{"name":"checksums-v0.3.1.sha256"}]}' | jq -r "$filter"
    else
      echo 'Unexpected release query' >&2
      return 1
    fi
  elif [[ "$1" == 'api' ]]; then
    if [[ "$2" != 'repos/example/offgrid-llm/releases/123/assets?per_page=100' || "$*" != *'--paginate'* ]]; then
      echo 'Draft assets must be requested by release ID with pagination' >&2
      return 1
    fi
    if [[ "${apiFailure}" == true ]]; then
      echo 'gh: Forbidden (HTTP 403)' >&2
      return 1
    fi
    api_count_file='.mock-gh-api-count'
    api_count=$(cat "$api_count_file" 2>/dev/null || echo 0)
    api_count=$((api_count + 1))
    printf '%s' "$api_count" > "$api_count_file"
    if [[ "$api_count" -eq 1 ]]; then
      jq -r "$filter" initial-assets.json
    else
      jq -r "$filter" assets.json
    fi
  elif [[ "$1 $2" == "release upload" ]]; then
    test -s "$4" || return 1
    echo MOCK_UPLOAD
  elif [[ "$1 $2" == "release edit" ]]; then
    touch .mock-published
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

const eventuallyComplete = run(names, { initialAssets: names.slice(1).map(asset), attempts: '2' });
assert.equal(eventuallyComplete.status, 0, eventuallyComplete.stdout + eventuallyComplete.stderr);
assert.match(eventuallyComplete.stdout, /not fully indexed yet/);
assert.match(eventuallyComplete.stdout, /MOCK_UPLOAD/);
const pendingDigests = names.map((name) => ({ ...asset(name), digest: null }));
const eventuallyIndexed = run(names, { initialAssets: pendingDigests, attempts: '2' });
assert.equal(eventuallyIndexed.status, 0, eventuallyIndexed.stdout + eventuallyIndexed.stderr);
assert.match(eventuallyIndexed.stdout, /digest=pending/);

for (const initialAssets of [[], pendingDigests, names.map((name) => ({ ...asset(name), size: 0 }))]) {
  const invalid = run(names, { initialAssets });
  assert.notEqual(invalid.status, 0, 'Unverified assets must not publish');
  assert.doesNotMatch(invalid.stdout, /MOCK_UPLOAD|MOCK_EDIT/);
}
const denied = run(names, { apiFailure: true });
assert.notEqual(denied.status, 0);
assert.match(denied.stderr, /Forbidden/);
assert.doesNotMatch(denied.stdout, /MOCK_UPLOAD|MOCK_EDIT/);
console.log('Draft finalization uses release IDs, evaluates real JSON filters, retries digests, and rejects incomplete or inaccessible assets');
