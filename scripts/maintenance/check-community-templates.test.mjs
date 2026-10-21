import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const issueNames = ['bug_report', 'feature_request', 'documentation', 'model_runtime'];

for (const name of issueNames) {
  test(`${name} issue template has GitHub metadata and actionable sections`, () => {
    const content = readFileSync(resolve(root, `.github/ISSUE_TEMPLATE/${name}.md`), 'utf8');
    assert.match(content, /^---\n(?:[^\n]+\n)+---\n/);
    for (const key of ['name', 'about', 'title', 'labels', 'assignees']) {
      assert.match(content, new RegExp(`^${key}:\\s*\\S.*$`, 'm'));
    }
    assert.match(content, /^## .+/m);
    assert.doesNotMatch(content, /paste (?:your )?(?:password|token)/i);
  });
}

test('pull request template covers verification, compatibility, state, security, and docs', () => {
  const content = readFileSync(resolve(root, '.github/pull_request_template.md'), 'utf8');
  for (const heading of ['Verification', 'Compatibility and state', 'Security and trust boundaries', 'Documentation and release scope']) {
    assert.ok(content.includes(`## ${heading}`), heading);
  }
  assert.match(content, /- \[ \] Relevant Go/);
  assert.match(content, /migration, rollback, or recovery/);
});
