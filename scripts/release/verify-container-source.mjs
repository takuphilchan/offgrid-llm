import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { pathToFileURL } from 'node:url';

export class ContainerIdentityMismatch extends Error {}
const digestOf = bytes => 'sha256:' + createHash('sha256').update(bytes).digest('hex');
const accept = 'application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json';

export async function verifyContainerSource({ repository, tag, revision, architectures, fetchImpl = fetch }) {
  assert.match(repository, /^[a-z0-9][a-z0-9._-]*\/[a-z0-9][a-z0-9._-]*$/);
  assert.match(tag, /^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$/);
  assert.match(revision, /^[a-f0-9]{40}$/);
  assert.ok(architectures.length > 0 && architectures.every(value => ['amd64', 'arm64'].includes(value)));
  const auth = await fetchImpl(`https://auth.docker.io/token?service=registry.docker.io&scope=repository:${repository}:pull`, { signal: AbortSignal.timeout(30000) });
  assert.equal(auth.status, 200, 'Anonymous registry authentication failed');
  const { token } = await auth.json();
  assert.ok(typeof token === 'string' && token.length > 0, 'Registry did not grant public pull access');
  async function read(kind, reference, expected) {
    if (expected) assert.match(reference, /^sha256:[a-f0-9]{64}$/);
    const response = await fetchImpl(`https://registry-1.docker.io/v2/${repository}/${kind}/${reference}`, {
      headers: { Authorization: `Bearer ${token}`, Accept: accept }, signal: AbortSignal.timeout(30000)
    });
    assert.equal(response.status, 200, `Registry ${kind} request failed`);
    const bytes = Buffer.from(await response.arrayBuffer());
    const digest = digestOf(bytes);
    if (expected) assert.equal(digest, expected, 'Registry bytes do not match the referenced digest');
    const advertised = response.headers.get('docker-content-digest');
    if (advertised) assert.equal(digest, advertised, 'Registry response digest mismatch');
    return { digest, value: JSON.parse(bytes.toString()) };
  }
  const index = await read('manifests', tag);
  const images = [];
  for (const architecture of architectures) {
    const matches = (index.value.manifests || []).filter(item => item.platform?.os === 'linux' && item.platform?.architecture === architecture);
    if (matches.length !== 1) throw new ContainerIdentityMismatch(`Expected one Linux ${architecture} image for ${tag}`);
    const descriptor = matches[0];
    const manifest = await read('manifests', descriptor.digest, descriptor.digest);
    const config = await read('blobs', manifest.value.config.digest, manifest.value.config.digest);
    const labels = config.value.config?.Labels || {};
    if (config.value.os !== 'linux' || config.value.architecture !== architecture ||
        labels['org.opencontainers.image.version'] !== tag || labels['org.opencontainers.image.revision'] !== revision) {
      throw new ContainerIdentityMismatch(`${tag} Linux ${architecture} does not identify the expected version and application revision ${revision}`);
    }
    images.push({ architecture, manifest: descriptor.digest, config: config.digest });
  }
  return { repository, tag, revision, index: index.digest, images };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [repository, tag, revision, platforms = ''] = process.argv.slice(2);
  try {
    const result = await verifyContainerSource({ repository, tag, revision, architectures: platforms.split(',') });
    console.log(`Verified ${result.repository}:${result.tag} (${result.images.map(image => image.architecture).join(', ')}) at application revision ${result.revision}`);
  } catch (error) {
    // Never print registry credentials, request headers or signed blob URLs.
    console.error(error.message);
    process.exitCode = error instanceof ContainerIdentityMismatch ? 2 : 1;
  }
}
