import assert from 'node:assert/strict';
import { test } from 'node:test';
import { developerLayoutIssues, requiredDeveloperPathIssues } from './check-developer-layout.mjs';

test('old developer and Python source locations fail with a location', () => {
  const issues = developerLayoutIssues([
    ['dev/scripts/test-web-workspace.mjs', 'fixture'],
    ['python/offgrid/client.py', 'fixture'],
    ['README.md', 'node dev/scripts/check-docs.mjs\npython -m pytest python/tests/test_client.py'],
  ]);
  assert.ok(issues.some(issue => issue.includes('dev/scripts/test-web-workspace.mjs: tracked')));
  assert.ok(issues.some(issue => issue.includes('python/offgrid/client.py: tracked')));
  assert.ok(issues.some(issue => issue.includes('README.md:1: obsolete dev')));
  assert.ok(issues.some(issue => issue.includes('README.md:2: obsolete python')));
});

test('current locations and historical evidence are allowed', () => {
  assert.deepEqual(developerLayoutIssues([
    ['README.md', 'node scripts/test/test-web-workspace.mjs\npython -m pytest sdk/python/tests/test_client.py'],
    ['docs/releases/old.md', 'dev/scripts/old.mjs'],
    ['openspec/changes/archive/old/tasks.md', 'python/offgrid/client.py'],
    ['docs/advanced/product-reliability-plan.md', 'dev/scripts/recorded.mjs'],
    ['docs/quality/evidence/old-record.md', 'dev/scripts/recorded.mjs'],
  ]), []);
});

test('Windows path separators and batch or service consumers are checked', () => {
  const issues = developerLayoutIssues([
    ['apps/desktop/build.bat', 'node dev\\scripts\\check-docs.mjs'],
    ['installers/systemd/offgrid.service', 'ExecStart=python\\offgrid\\client.py'],
  ]);
  assert.ok(issues.some(issue => issue.includes('build.bat:1: obsolete dev')));
  assert.ok(issues.some(issue => issue.includes('offgrid.service:1: obsolete python')));
  assert.deepEqual(developerLayoutIssues([
    ['apps/desktop/build.bat', 'py sdk\\python\\tests\\test_client.py'],
  ]), []);
});

test('relocated worker, SDK and native test context paths are required', () => {
  const current = [
    'docker/test/native-contracts.Dockerfile',
    'tools/model-package-metadata/main.go',
    'sdk/python/pyproject.toml',
    'runtimes/python/audio_service.py',
    'runtimes/python/provider_worker.py',
  ];
  assert.deepEqual(requiredDeveloperPathIssues(current, 'dev/\npython/\n'), []);
  assert.ok(requiredDeveloperPathIssues(current.slice(1)).some(issue =>
    issue.includes('native-contracts.Dockerfile: required relocated source is missing')));
  assert.ok(requiredDeveloperPathIssues(current, 'docker/test/\n').some(issue =>
    issue.includes('excluded by .dockerignore')));
});
