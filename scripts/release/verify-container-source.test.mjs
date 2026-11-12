import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { test } from 'node:test';
import { verifyContainerSource, ContainerIdentityMismatch } from './verify-container-source.mjs';

const revision = 'a'.repeat(40);
const hash = bytes => 'sha256:' + createHash('sha256').update(bytes).digest('hex');
function fixture({ tag = '0.4.30', source = revision, architectures = ['amd64', 'arm64'], corrupt = false, unavailable = false } = {}) {
  const payloads = new Map();
  function store(value) {
    const bytes = Buffer.from(JSON.stringify(value));
    const digest = hash(bytes);
    payloads.set(digest, bytes);
    return digest;
  }
  const manifests = architectures.map(architecture => {
    const config = store({ os: 'linux', architecture, config: { Labels: {
      'org.opencontainers.image.version': tag, 'org.opencontainers.image.revision': source
    } } });
    const digest = store({ config: { digest: config } });
    if (corrupt) payloads.set(config, Buffer.from('{}'));
    return { digest, platform: { os: 'linux', architecture } };
  });
  payloads.set(tag, Buffer.from(JSON.stringify({ manifests })));
  return async (url, options) => {
    if (url.startsWith('https://auth.docker.io/token?')) return Response.json({ token: 'public-pull-fixture' });
    assert.ok(url.startsWith('https://registry-1.docker.io/v2/example/offgrid/'));
    assert.equal(options.headers.Authorization, 'Bearer public-pull-fixture');
    if (unavailable) return new Response('', { status: 503 });
    const reference = url.split('/').at(-1);
    assert.ok(payloads.has(reference), `Unexpected registry reference: ${reference}`);
    return new Response(payloads.get(reference));
  };
}
const options = { repository: 'example/offgrid', tag: '0.4.30', revision, architectures: ['amd64', 'arm64'] };
test('validates both CPU configurations through digest-bound manifests', async () => {
  const result = await verifyContainerSource({ ...options, fetchImpl: fixture() });
  assert.deepEqual(result.images.map(image => image.architecture), ['amd64', 'arm64']);
});
test('accepts the GPU variant version while requiring the application revision', async () => {
  const result = await verifyContainerSource({ ...options, tag: '0.4.30-gpu', architectures: ['amd64'],
    fetchImpl: fixture({ tag: '0.4.30-gpu', architectures: ['amd64'] }) });
  assert.equal(result.revision, revision);
});
test('rejects the automation revision as application identity', async () => {
  await assert.rejects(verifyContainerSource({ ...options, fetchImpl: fixture({ source: 'b'.repeat(40) }) }), ContainerIdentityMismatch);
});
test('rejects a tag missing a required architecture', async () => {
  await assert.rejects(verifyContainerSource({ ...options, fetchImpl: fixture({ architectures: ['amd64'] }) }), ContainerIdentityMismatch);
});
test('digest corruption is a fatal integrity error, not permission to rebuild', async () => {
  await assert.rejects(verifyContainerSource({ ...options, fetchImpl: fixture({ corrupt: true }) }), error => {
    assert.ok(!(error instanceof ContainerIdentityMismatch));
    assert.match(error.message, /referenced digest/);
    return true;
  });
});
test('registry unavailability is a fatal lookup error, not permission to rebuild', async () => {
  await assert.rejects(verifyContainerSource({ ...options, fetchImpl: fixture({ unavailable: true }) }), error => {
    assert.ok(!(error instanceof ContainerIdentityMismatch));
    assert.match(error.message, /Registry manifests request failed/);
    return true;
  });
});
