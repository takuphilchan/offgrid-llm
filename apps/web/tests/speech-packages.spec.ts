import { expect, test } from '@playwright/test';
import { createHash } from 'node:crypto';
import { mkdtemp, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { supportsChat } from '../src/api/model-capabilities';

test('positive model capabilities preserve legacy and vision chat, excluding speech', () => {
  for (const model of [{}, { type: 'llm' }, { type: 'chat' }, { type: 'vlm', capabilities: ['chat'] }]) expect(supportsChat(model)).toBe(true);
  for (const model of [{ type: 'embedding' }, { type: 'asr', capabilities: ['chat'] }, { type: 'tts' }, { type: 'unknown' }, { type: 'llm', capabilities: ['transcription'] }]) expect(supportsChat(model)).toBe(false);
});

test('real service imports, verifies and removes a package without making it a chat model', async ({ page, request }, testInfo) => {
  test.skip(!process.env.OFFGRID_E2E_URL, 'Requires the isolated service wrapper, never an installed workspace.');
  const directory = await mkdtemp(join(tmpdir(), 'offgrid-speech-model-fixture-'));
  const bytes = Buffer.from('Synthetic data; not an inference model.');
  const id = `speech-fixture-${Date.now()}`;
  const manifest = { schema_version: 1, id, revision: 'fixture-1', name: 'Synthetic speech fixture', architecture: 'whisper', runtime: { adapter: 'whisper.cpp', revision: 'fixture-1' }, capabilities: ['transcription'], languages: ['en'], sample_rates: [16000], license: 'CC0-1.0', artifacts: [{ path: 'weights.bin', role: 'weights', size: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex'), license: 'CC0-1.0' }] };
  await writeFile(join(directory, 'manifest.json'), JSON.stringify(manifest));
  await writeFile(join(directory, 'weights.bin'), bytes);
  await page.addInitScript(() => { localStorage.setItem('offgrid.onboarding.complete', 'true'); localStorage.setItem('offgrid.locale', 'en'); });
  try {
    await page.goto('/ui/#/models');
    await page.getByRole('button', { name: 'Speech recognition', exact: true }).click();
    await page.getByText('Advanced · offline import', { exact: true }).click();
    const section = page.locator('details').filter({ has: page.getByText('Advanced · offline import', { exact: true }) });
    await section.getByLabel('Choose a package folder').setInputFiles(directory);
    await expect(section.getByText(/Synthetic speech fixture.*fixture-1/)).toBeVisible();
    await section.getByRole('button', { name: 'Import package', exact: true }).click();
    const card = page.getByRole('region', { name: 'Installed', exact: true }).locator('article').filter({ hasText: id });
    await expect(card.getByText('Integrity checked', { exact: true })).toBeVisible();
    await expect(card.getByText('Install the whisper.cpp speech runtime for this model.', { exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath('speech-package-inventory.png'), fullPage: true });
    const legacy = await request.get('/v1/models');
    expect((await legacy.json()).data.some((model: { id: string }) => model.id === id)).toBe(false);
    await card.getByRole('button', { name: 'Verify', exact: true }).click();
    await expect(card.getByRole('button', { name: 'Delete', exact: true })).toBeEnabled();
    await card.getByRole('button', { name: 'Delete', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText(/Original imports and user documents are kept/)).toBeVisible();
    await dialog.locator('.danger-button').click();
    await expect(card).toHaveCount(0);
    await page.setViewportSize({ width: 390, height: 844 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  } finally {
    await request.post(`/api/v2/models/packages/${id}/fixture-1/remove`, { data: {} });
  }
});
