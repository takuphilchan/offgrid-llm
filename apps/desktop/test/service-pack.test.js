const { test } = require('node:test');
const assert = require('node:assert/strict');
const { verifyServiceVersion } = require('../service-pack.cjs');

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
