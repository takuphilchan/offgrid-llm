import { expect, test } from '@playwright/test';
import { compactModelName, catalogDisplay, legacyDisplay, packageCatalogDisplay, packageDisplay } from '../src/features/models/model-display';
import type { CatalogModel, InstalledTypedModel, TypedCatalogModel } from '../src/api/client';

test('display adapters distinguish unknown sizes, runtime state and proven source identity', () => {
  const catalog = { id: 'local', name: 'Readable name', repo: 'publisher/model', type: 'llm', size_bytes: 10, license: 'MIT' } as CatalogModel;
  expect(legacyDisplay({ id: 'local', size: 0 }, catalog)).toMatchObject({ name: 'Readable name', bytes: undefined, runtime: 'unchecked', sizeMeaning: 'installed' });
  expect(legacyDisplay({ id: 'local' }, catalog).source).toBeUndefined();
  expect(legacyDisplay({ id: 'x', type: 'asr', capabilities: ['transcription'] }).category).toBeUndefined();
  expect(legacyDisplay({ id: 'loaded', loaded: true, size: 12 }).runtime).toBe('loaded');
  expect(catalogDisplay(catalog)).toMatchObject({ source: 'publisher/model', bytes: 10, sizeMeaning: 'download' });
  const item = { id: 'tts', name: 'Speech', category: 'speech_generation', revision: 'r1', provenance: { repository: 'owner/voice' }, package: { manifest: { license: 'MIT', artifacts: [{ size: 10 }, { size: 20 }] } } } as InstalledTypedModel;
  expect(packageDisplay(item)).toMatchObject({ bytes: 30, source: 'owner/voice', category: 'speech_generation', revision: 'r1' });
  item.package!.manifest!.artifacts[1].size = 0;
  expect(packageDisplay(item).bytes).toBeUndefined();
  const entry = { ...item, variants: [{ size_bytes: 10, provenance: { repository: 'owner/voice' } }, { size_bytes: 20, provenance: { repository: 'owner/voice' } }] } as unknown as TypedCatalogModel;
  expect(packageCatalogDisplay(entry)).toMatchObject({ bytes: 10, maximumBytes: 20, sizeMeaning: 'download' });
  entry.variants[1].size_bytes = 0;
  expect(packageCatalogDisplay(entry).bytes).toBeUndefined();
});

test('compact names keep family and size while the exact ID stays unchanged',()=>{
 const id='NVIDIA-Nemotron-3.5-Bolt-E4B-UD-Q4_K_XL-47d072be609f';
 expect(compactModelName(id)).toBe('Nemotron 3.5 Bolt E4B');
 expect(compactModelName(id,14)).toBe('Nemotron E4B');
 expect(compactModelName('publisher/Qwen2.5-7B-Instruct-Q4_K_M.gguf')).toBe('Qwen2.5 7B');
 expect(compactModelName('Meta-Llama-3.1-8B-Instruct-Q4_K_L-f1e3673a9589')).toBe('Llama 3.1 8B');
 expect(compactModelName('unbreakablemodelidentifierthatisverylong',14)).toBe('');
 expect(id).toBe('NVIDIA-Nemotron-3.5-Bolt-E4B-UD-Q4_K_XL-47d072be609f');
});
