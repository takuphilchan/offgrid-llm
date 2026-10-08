import type { CatalogModel, InstalledTypedModel, Model, ModelCategory, TypedCatalogModel } from '../../api/client';
import { supportsChat } from '../../api/model-capabilities';

export type ModelDisplay = {
  id: string; name: string; category?: ModelCategory; source?: string; revision?: string;
  bytes?: number; maximumBytes?: number; sizeMeaning: 'installed' | 'download';
  runtime?: 'unchecked' | 'loaded'; quant?: string; license?: string;
};
const bytes = (value: number | undefined) => Number.isSafeInteger(value) && value! > 0 ? value : undefined;
export function legacyDisplay(model: Model, catalog?: CatalogModel): ModelDisplay {
  return { id: model.id, name: catalog?.name || model.id.replace(/\.gguf$/i, '').replaceAll('_', ' '), category: model.type === 'embedding' ? 'embeddings' : supportsChat(model) ? 'language' : undefined,
    // The legacy inventory has no immutable provenance. A matching filename is
    // not evidence that this file came from the curated repository.
    bytes: bytes(model.size), sizeMeaning: 'installed', runtime: model.loaded ? 'loaded' : 'unchecked', quant: catalog?.quant };
}
export function packageDisplay(model: InstalledTypedModel): ModelDisplay {
  const artifacts = model.package?.manifest?.artifacts;
  const total = artifacts?.length && artifacts.every(a => bytes(a.size) !== undefined) ? artifacts.reduce((n, a) => n + a.size, 0) : undefined;
  return { id: model.id, revision: model.revision, name: model.name || model.id, category: model.category, source: model.provenance.repository,
    bytes: bytes(total), sizeMeaning: 'installed', license: model.package?.manifest?.license };
}
export function catalogDisplay(model: CatalogModel): ModelDisplay {
  return { id: model.id, name: model.name, category: model.type === 'embedding' ? 'embeddings' : supportsChat(model) ? 'language' : undefined,
    source: model.repo, bytes: bytes(model.size_bytes), sizeMeaning: 'download', quant: model.quant, license: model.license };
}
export function packageCatalogDisplay(model: TypedCatalogModel): ModelDisplay {
  const sizes = model.variants.map(v => bytes(v.size_bytes));
  const known = sizes.length > 0 && sizes.every(v => v !== undefined);
  const repositories = new Set(model.variants.map(v => v.provenance.repository).filter(Boolean));
  return { id: model.id, name: model.name, category: model.category, source: repositories.size === 1 && model.variants.every(v => !!v.provenance.repository) ? [...repositories][0] : undefined,
    bytes: known ? Math.min(...sizes as number[]) : undefined, maximumBytes: known ? Math.max(...sizes as number[]) : undefined,
    sizeMeaning: 'download', license: model.license };
}
