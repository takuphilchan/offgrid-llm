import { expect, test } from '@playwright/test';
import { catalogDisplay, legacyDisplay, packageCatalogDisplay, packageDisplay } from '../src/features/models/model-display';
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
