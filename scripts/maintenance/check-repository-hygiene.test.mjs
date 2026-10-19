import test from 'node:test';
import assert from 'node:assert/strict';

import { hygieneIssues } from './check-repository-hygiene.mjs';

test('reports tracked binaries, caches, test output, and Python distributions by path', () => {
  assert.deepEqual(hygieneIssues([
    'bin/offgrid',
    'apps\\desktop\\dist\\setup.exe',
    '.gocache/cache.bin',
    'apps/web/test-results/report.json',
    'python/offgrid.egg-info/PKG-INFO',
  ], []), [
    'tracked generated output: bin/offgrid',
    'tracked generated output: apps/desktop/dist/setup.exe',
    'tracked generated output: .gocache/cache.bin',
    'tracked generated output: apps/web/test-results/report.json',
    'tracked generated output: python/offgrid.egg-info/PKG-INFO',
  ]);
});

test('reports Go packages discovered in build and temporary work directories', () => {
  assert.deepEqual(hygieneIssues([], [
    'github.com/example/offgrid/build/generated',
    'github.com/example/offgrid/_work/diagnostic',
  ]), [
    'Go package in output directory: github.com/example/offgrid/build/generated',
    'Go package in output directory: github.com/example/offgrid/_work/diagnostic',
  ]);
});

test('allows source, tests, and ignored local output absent from the index', () => {
  assert.deepEqual(hygieneIssues([
    'cmd/offgrid/main.go',
    'internal/models/package_store_test.go',
    'scripts/maintenance/check-repository-hygiene.mjs',
    'scripts/build/source-install-build.sh',
    'docker/Dockerfile',
  ], ['github.com/example/offgrid/internal/models']), []);
});
