import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { applicationLayoutIssues, checkApplicationLayout } from './check-application-layout.mjs';

test('rejects tracked old source and obsolete references in maintained consumers', () => {
  const issues = applicationLayoutIssues([
    ['web/app/package.json', '{}'],
    ['.github/workflows/ci.yml', 'working-directory: web/app\npath: desktop/dist/*'],
    ['docs/advanced/BUILDING.md', '[desktop](../../desktop/README.md)'],
    ['scripts/build-all.sh', 'cc computer/native/linux/fixture.c'],
    ['README.md', 'cd desktop\nnpm test --prefix computer'],
    ['docs/advanced/workspace-ui.md', 'cd apps/web\ncd ../../desktop'],
  ]);
  assert.ok(issues.some(issue => issue.includes('tracked source remains')));
  assert.ok(issues.some(issue => issue.includes('obsolete web/app')));
  assert.ok(issues.some(issue => issue.includes('obsolete desktop/')));
  assert.ok(issues.some(issue => issue.includes('obsolete computer/')));
  assert.ok(issues.some(issue => issue.includes('obsolete application command')));
  assert.ok(issues.some(issue => issue.includes('obsolete old desktop root navigation')));
});

test('accepts new rooted paths, valid sibling imports, and immutable records', () => {
  assert.deepEqual(applicationLayoutIssues([
    ['apps/web/vite.config.ts', "import compatibility from '../desktop/compatibility.cjs'"],
    ['apps/desktop/main.js', "require('../computer/pack.cjs'); const ui = '../web/dist'"],
    ['apps/computer/test/native-pack.test.mjs', "new URL('../../desktop/prepare-computer.cjs', import.meta.url)"],
    ['docs/advanced/BUILDING.md', '`apps/web/dist` and `apps/desktop/package.json`'],
    ['internal/server/browser_handlers.go', '"/api/v2/computer/companion/pair"'],
    ['docs/releases/release-notes-v0.4.0.md', 'web/app desktop/package.json'],
    ['openspec/changes/old/design.md', 'web/app desktop/package.json'],
    ['docs/advanced/product-reliability-plan.md', 'web/app desktop/package.json'],
    ['docs/quality/evidence/old-record.md', 'web/app desktop/package.json'],
  ]), []);
});

test('an indexed removal does not hide a new source with stale references', () => {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-layout-check-'));
  try {
    execFileSync('git', ['init', '--quiet'], { cwd: root });
    mkdirSync(join(root, 'cmd', 'offgrid'), { recursive: true });
    const former = join(root, 'cmd', 'offgrid', 'model_package_commands.go');
    writeFileSync(former, 'package main\n');
    execFileSync('git', ['add', 'cmd/offgrid/model_package_commands.go'], { cwd: root });
    rmSync(former);
    mkdirSync(join(root, 'internal', 'cli'), { recursive: true });
    writeFileSync(join(root, 'internal', 'cli', 'model_package_commands.go'), 'package cli\n// web/app is obsolete\n');

    assert.ok(checkApplicationLayout(root).some(issue => issue.includes('obsolete web/app')));
    execFileSync('git', ['add', '-A'], { cwd: root });
    assert.ok(checkApplicationLayout(root).some(issue => issue.includes('obsolete web/app')));
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
