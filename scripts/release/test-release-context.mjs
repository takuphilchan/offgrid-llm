import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';

const script = resolve('.github/scripts/resolve-release-context.sh');

function resolveContext(eventName, ref, requestedVersion = '') {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-release-context-'));
  const output = join(root, 'github-output');
  try {
    const result = spawnSync('bash', [script, eventName, ref, requestedVersion], {
      encoding: 'utf8',
      env: { ...process.env, GITHUB_OUTPUT: output },
    });
    const values = result.status === 0
      ? Object.fromEntries(readFileSync(output, 'utf8').trim().split('\n').map((line) => line.split('=')))
      : {};
    return { ...result, values };
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

for (const testCase of [
  ['push', 'refs/tags/v0.4.2', '', 'v0.4.2'],
  ['push', 'refs/heads/release-build/v0.4.2', '', 'v0.4.2'],
  ['workflow_dispatch', 'refs/heads/main', 'v0.4.2', 'v0.4.2'],
]) {
  const [eventName, ref, requestedVersion, expected] = testCase;
  const result = resolveContext(eventName, ref, requestedVersion);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.deepEqual(result.values, { version: expected, source_ref: expected });
}

for (const testCase of [
  ['push', 'refs/heads/main', ''],
  ['workflow_dispatch', 'refs/heads/main', '0.4.2'],
  ['push', 'refs/heads/release-build/not-a-version', ''],
]) {
  const result = resolveContext(...testCase);
  assert.notEqual(result.status, 0, 'Invalid release context must be rejected');
}

console.log('Release context resolves immutable tags and rejects unsupported triggers');
