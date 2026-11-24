import { expect, test } from '@playwright/test';
import { createHash } from 'node:crypto';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { InstalledTypedModel, Model } from '../src/api/client';

test('isolated service and renderer import, verify and remove ONNX metadata without runtime activation', async ({ page, request }, testInfo) => {
  test.skip(!process.env.OFFGRID_E2E_URL, 'Requires test-web-workspace.mjs isolated service, never an installed workspace.');
  const directory = await mkdtemp(join(tmpdir(), 'offgrid-onnx-service-package-'));
  const id = `onnx-fixture-${Date.now()}`, revision = 'fixture-1', selector = `embedding:${id}@${revision}`;
  const artifacts = {
    'model.onnx': { role: 'weights', bytes: Buffer.from('Synthetic package data; not an ONNX graph or weights.') },
    'tokenizer.json': { role: 'tokenizer', bytes: Buffer.from('{}') },
    'embedding.json': { role: 'config', bytes: Buffer.from(JSON.stringify({ schema_version: 1, family: 'bert-symmetric', inputs: ['input_ids', 'attention_mask'], output: 'hidden', pooling: 'mean', normalize_l2: true, dimensions: 3, max_tokens: 128, pad_token: '[PAD]', pad_token_id: 0 })) }
  };
  const manifest = {
    schema_version: 1, id, revision, name: 'Isolated ONNX metadata fixture', architecture: 'onnx-embedding', model_family: 'bert-symmetric',
    runtime: { adapter: 'onnx-embedding', revision: 'bert-symmetric-v1' }, capabilities: ['embeddings'], modalities: ['text'], tool_calling: 'unsupported',
    languages: ['en'], sample_rates: [], license: 'CC0-1.0', engine: 'onnx', format: 'onnx', backend: 'cpu', context_length: 128,
    runner_revision: 'worker-v1', adapter_revision: 'bert-symmetric-v1',
    artifacts: Object.entries(artifacts).map(([path, { role, bytes }]) => ({ path, role, size: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex'), license: 'CC0-1.0' }))
  };
  try {
    await writeFile(join(directory, 'manifest.json'), JSON.stringify(manifest));
    for (const [path, artifact] of Object.entries(artifacts)) await writeFile(join(directory, path), artifact.bytes);
    await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
    await page.goto('/ui/#/models');
    await page.locator('.model-category-filter').selectOption({ label: 'Embeddings' });
    await page.getByText('Advanced · offline import', { exact: true }).click();
    await page.getByLabel('Choose a package folder').setInputFiles(directory);
    await expect(page.getByRole('button', { name: 'Import package', exact: true })).toBeEnabled();
    await page.getByRole('button', { name: 'Import package', exact: true }).click();
    const card = page.locator('.package-card').filter({ hasText: manifest.name });
    await expect(card).toBeVisible();
    await expect(card).toContainText('Qualified: No');
    const typedResponse = await request.get('/api/v2/models?category=embeddings');
    expect(typedResponse.ok()).toBe(true);
    const typed = ((await typedResponse.json()).models as InstalledTypedModel[]).filter(item => item.id === id);
    expect(typed).toHaveLength(1);
    expect(typed[0]).toMatchObject({ kind: 'package', engine: 'onnx', category: 'embeddings', readiness: { installed: true, runtime_ready: false, qualified: false } });
    const publicResponse = await request.get('/v1/models');
    expect(publicResponse.ok()).toBe(true);
    const publicModel = ((await publicResponse.json()).data as Model[]).find(item => item.id === selector);
    expect(publicModel).toMatchObject({ type: 'embedding', engine: 'onnx', capability_status: { embeddings: 'installed_unqualified' } });
    const knowledge = await request.get('/v1/rag/status');
    expect(knowledge.ok()).toBe(true);
    expect((await knowledge.json()).enabled).toBe(false);
    await expect(page.getByRole('button', { name: `Model actions: ${selector}`, exact: true })).toHaveCount(0);
    await page.screenshot({ path: testInfo.outputPath('onnx-actual-service-library.png'), fullPage: true });
    await card.getByRole('button', { name: 'Verify', exact: true }).click();
    await expect(card.getByRole('button', { name: 'Delete', exact: true })).toBeEnabled();
    await card.getByRole('button', { name: 'Delete', exact: true }).click();
    await page.getByRole('dialog').locator('.danger-button').click();
    await expect(card).toHaveCount(0);
    const after = await request.get('/v1/models');
    expect(((await after.json()).data as Model[]).some(item => item.id === selector)).toBe(false);
  } finally {
    await request.post(`/api/v2/models/packages/${id}/${revision}/remove`, { data: {} });
    await rm(directory, { recursive: true, force: true });
  }
});
