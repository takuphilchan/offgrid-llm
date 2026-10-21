import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const script = readFileSync(join(repo, '.github/scripts/verify-release-containers.sh'), 'utf8');

const cpuComplete = JSON.stringify({
  images: [
    { os: 'linux', architecture: 'amd64' },
    { os: 'linux', architecture: 'arm64' },
  ],
});
const cpuPartial = JSON.stringify({ images: [{ os: 'linux', architecture: 'amd64' }] });
const gpuComplete = JSON.stringify({ images: [{ os: 'linux', architecture: 'amd64' }] });

function run({ initialCpu = cpuComplete, cpu = cpuComplete, gpu = gpuComplete, attempts = '1' } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-container-verify-'));
  const mock = `
curl() {
  url="\${*: -1}"
  if [[ "\${url}" == *'-gpu/' ]]; then
    printf '%s' '${gpu}'
    return
  fi
  count_file='.mock-curl-count'
  mock_attempt=$(cat "\${count_file}" 2>/dev/null || echo 0)
  mock_attempt=$((mock_attempt + 1))
  printf '%s' "\${mock_attempt}" > "\${count_file}"
  if [[ "\${mock_attempt}" -eq 1 ]]; then
    printf '%s' '${initialCpu}'
  else
    printf '%s' '${cpu}'
  fi
}
set -- 'v0.4.2'
`;
  try {
    return spawnSync('bash', ['-s'], {
      cwd: root,
      input: mock + script,
      encoding: 'utf8',
      env: {
        ...process.env,
        OFFGRID_CONTAINER_VERIFY_ATTEMPTS: attempts,
        OFFGRID_CONTAINER_VERIFY_RETRY_SECONDS: '0',
      },
    });
  } finally {
    if (dirname(realpathSync(root)) !== realpathSync(tmpdir())) {
      throw new Error(`Refusing to remove unexpected test directory: ${root}`);
    }
    rmSync(root, { recursive: true, force: true });
  }
}

const complete = run();
assert.equal(complete.status, 0, complete.stdout + complete.stderr);
assert.match(complete.stdout, /Verified .*0\.4\.2.*AMD64 and ARM64/);
assert.match(complete.stdout, /Verified .*0\.4\.2-gpu.*AMD64/);

const partial = run({ initialCpu: cpuPartial, cpu: cpuPartial });
assert.notEqual(partial.status, 0, 'A single-platform CPU tag must not pass');
assert.match(partial.stdout, /cpu=false/);

const eventuallyComplete = run({ initialCpu: cpuPartial, attempts: '2' });
assert.equal(eventuallyComplete.status, 0, eventuallyComplete.stdout + eventuallyComplete.stderr);
assert.match(eventuallyComplete.stdout, /not fully indexed/);

console.log('Container release verification checks both CPU platforms, GPU, and registry indexing retries');
