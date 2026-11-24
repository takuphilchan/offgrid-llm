const { test } = require('node:test');
const assert = require('node:assert/strict');
const { verifyServiceVersion } = require('../service-pack.cjs');

test('Darwin service packaging rejects the nonfunctional no-cgo authority fallback', () => {
  const metadata = '\tbuild\t-ldflags="-X main.Version=0.4.33"\n\tbuild\tGOOS=darwin\n';
  for (const suffix of ['', '\tbuild\tCGO_ENABLED=0\n']) {
    assert.throws(() => verifyServiceVersion('offgrid-arm64', '0.4.33', () => metadata + suffix), /native ACL inspector/);
  }
  verifyServiceVersion('offgrid-arm64', '0.4.33', () => metadata + '\tbuild\tCGO_ENABLED=1\n');
});

test('packaging rejects development binaries even when a nearby VERSION could match', () => {
  assert.throws(() => verifyServiceVersion('offgrid.exe', '0.4.26', () => '\tbuild\t-ldflags="-X internal/server.BuildRevision=preview"'), /must embed/);
});

test('packaging requires the matching embedded service version, including cross-built binaries', () => {
  for (const value of ['0.4.26', 'v0.4.26']) {
    verifyServiceVersion('offgrid-arm64', '0.4.26', () => `\tbuild\t-ldflags="-s -w -X main.Version=${value} -X main.BuildTime=now"`);
  }
  for (const value of ['43.7.9', 'dev', '0.4.25']) {
    assert.throws(() => verifyServiceVersion('offgrid.exe', '0.4.26', () => `\tbuild\t-ldflags="-X main.Version=${value}"`), /must embed/);
  }
  assert.throws(() => verifyServiceVersion('offgrid.exe', '0.4.26', () => '\tbuild\t-ldflags="-X main.Version=0.4.26 -X main.Version=dev"'), /must embed/);
});
