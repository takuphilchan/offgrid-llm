import { expect, test, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { packageMatchesCategory } from '../src/features/models/package-import';
import type { ModelPackageManifest } from '../src/api/client';

test('package import categories separate ONNX, native language and speech layouts', () => {
  for (const [architecture, capability, category] of [
    ['onnx-embedding', 'embeddings', 'embeddings'], ['mlx-text', 'chat', 'language'],
    ['transformers-text', 'chat', 'language'], ['whisper', 'transcription', 'speech_recognition'],
    ['piper', 'speech_synthesis', 'speech_generation']
  ] as const) {
    const manifest = { architecture, capabilities: [capability] } as ModelPackageManifest;
    for (const candidate of ['language', 'embeddings', 'speech_recognition', 'speech_generation'] as const) {
      expect(packageMatchesCategory(manifest, candidate)).toBe(candidate === category);
    }
  }
});

async function fixture(page: Page, role = 'admin', initiallyInstalled = false) {
  const state = { installed: initiallyInstalled, mutations: [] as string[] };
  const pkg = {
    id: 'offline-embedding', revision: 'v1', name: 'Offline ONNX embedding', kind: 'package', category: 'embeddings',
    engine: 'onnx', format: 'onnx', backend: 'cpu', provenance: { kind: 'local_untrusted' },
    readiness: { installed: true, integrity: 'checked', runtime_ready: false, smoke_tested: false, qualified: false },
    package: { in_use: 0, installed: true, integrity: 'checked', runtime_compatible: false, smoke_tested: false, qualified: false,
      manifest: { context_length: 128, artifacts: [{ size: 100 }], runtime: { adapter: 'onnx-embedding', revision: 'bert-symmetric-v1' }, license: 'MIT' } }
  };
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  await page.route('**/health', route => route.fulfill({ json: { status: 'healthy' } }));
  await page.route(/\/(?:v1|api\/v2)\//, route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname;
    if (request.method() !== 'GET') {
      state.mutations.push(path);
      if (path === '/api/v2/models/packages') state.installed = true;
      if (path.endsWith('/remove')) state.installed = false;
    }
    const bodies: Record<string, unknown> = {
      '/api/v2/system': { product: 'offgrid', api_version: 2, workspace_id: 'onnx-library-fixture', capabilities: [] },
      '/v1/system/config': { require_auth: true }, '/v1/users/me': { authenticated: true, auth_required: true, user: { id: 'fixture-user', username: 'fixture-user', role } },
      '/v1/models': { data: [{ id: 'fixture-chat', type: 'llm' }, { id: 'existing-gguf', type: 'embedding', engine: 'llama_cpp' }, ...(state.installed ? [{ id: 'embedding:offline-embedding@v1', type: 'embedding', engine: 'onnx', format: 'onnx', capability_status: { embeddings: 'installed_unqualified' } }] : [])] },
      '/v1/catalog': { models: [] }, '/v1/models/download/progress': {}, '/api/v2/jobs': [], '/v1/sessions': { sessions: [] },
      '/api/v2/models': { models: state.installed && url.searchParams.get('category') === 'embeddings' ? [pkg] : [] },
      '/api/v2/models/operations': { operations: [] }, '/api/v2/models/packages': { packages: state.installed ? [pkg.package] : [] },
    };
    return route.fulfill({ json: bodies[path] ?? {} });
  });
  return state;
}

test('first ONNX package imports and uses package verification/removal while GGUF remains separate', async ({ page }, testInfo) => {
  const state = await fixture(page);
  const directory = await mkdtemp(join(tmpdir(), 'offgrid-onnx-ui-fixture-'));
  const bytes = Buffer.from('Metadata-only fixture; no ONNX runtime or weights.');
  const manifest = { schema_version: 1, id: 'offline-embedding', revision: 'v1', name: 'Offline ONNX embedding', architecture: 'onnx-embedding', capabilities: ['embeddings'], artifacts: [{ path: 'model.onnx', size: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') }] };
  try {
    await writeFile(join(directory, 'manifest.json'), JSON.stringify(manifest));
    await writeFile(join(directory, 'model.onnx'), bytes);
    await page.goto('/ui/#/models');
    await page.locator('.model-category-filter').selectOption({ label: 'Embeddings' });
    await page.getByText('Advanced · offline import', { exact: true }).click();
    await page.getByLabel('Choose a package folder').setInputFiles(directory);
    const importButton = page.getByRole('button', { name: 'Import package', exact: true });
    await expect(importButton).toBeEnabled();
    await importButton.click();
    const card = page.locator('.package-card').filter({ hasText: 'Offline ONNX embedding' });
    await expect(card).toBeVisible();
    await expect(card).toContainText('Qualified: No');
    await expect(card.getByRole('button', { name: /select/i })).toHaveCount(0);
    await page.screenshot({ path: testInfo.outputPath('onnx-package-library.png'), fullPage: true });
    await expect(page.getByRole('button', { name: 'Model actions: embedding:offline-embedding@v1', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Model actions: existing-gguf', exact: true })).toBeVisible();
    await card.getByRole('button', { name: 'Verify', exact: true }).click();
    await expect(card.getByRole('button', { name: 'Delete', exact: true })).toBeEnabled();
    await card.getByRole('button', { name: 'Delete', exact: true }).click();
    await page.getByRole('dialog').locator('.danger-button').click();
    await expect(card).toHaveCount(0);
    expect(state.mutations).toEqual(['/api/v2/models/packages', '/api/v2/models/packages/offline-embedding/v1/verify', '/api/v2/models/packages/offline-embedding/v1/remove']);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test('ONNX package administration remains unavailable to non-administrators', async ({ page }) => {
  const state = await fixture(page, 'user', true);
  await page.goto('/ui/#/models');
  await page.locator('.model-category-filter').selectOption({ label: 'Embeddings' });
  const card = page.locator('.package-card').filter({ hasText: 'Offline ONNX embedding' });
  await expect(card.getByRole('button', { name: 'Verify', exact: true })).toBeDisabled();
  await expect(card.getByRole('button', { name: 'Delete', exact: true })).toBeDisabled();
  await page.getByText('Advanced · offline import', { exact: true }).click();
  await expect(page.getByLabel('Choose a package folder')).toHaveCount(0);
  expect(state.mutations).toEqual([]);
});
