import type { CatalogModel, InstalledTypedModel, Model, ModelCategory, TypedCatalogModel } from '../../api/client';
import { supportsChat } from '../../api/model-capabilities';

export type ModelDisplay = {
  id: string; name: string; category?: ModelCategory; source?: string; revision?: string;
  bytes?: number; maximumBytes?: number; sizeMeaning: 'installed' | 'download';
  runtime?: 'unchecked' | 'loaded' | 'ready' | 'unavailable'; quant?: string; license?: string;
  engine?: string; format?: string; backend?: string;
};
const bytes = (value: number | undefined) => Number.isSafeInteger(value) && value! > 0 ? value : undefined;
/** Presentation only; callers retain the exact identifier for selection/provenance. */
export function readableModelName(id: string): string {
  return id.replace(/\.gguf$/i, '').replace(/[-_][a-f0-9]{12,64}$/i, '')
    .replace(/[-_](?:I?Q\d(?:_[A-Z0-9]+)*|BF16|F16|F32)$/i, '')
    .replace(/[-_]+/g, ' ').trim() || id;
}
export function legacyDisplay(model: Model, catalog?: CatalogModel): ModelDisplay {
  return { id: model.id, name: catalog?.name || readableModelName(model.id), category: model.type === 'embedding' ? 'embeddings' : supportsChat(model) ? 'language' : undefined,
    // The legacy inventory has no immutable provenance. A matching filename is
    // not evidence that this file came from the curated repository.
    bytes: bytes(model.size), sizeMeaning: 'installed', runtime: model.runtime_ready === false ? 'unavailable' : model.loaded ? 'loaded' : model.runtime_ready === true ? 'ready' : 'unchecked', quant: catalog?.quant,
    engine: model.engine ?? 'llama_cpp', format: model.format, backend: model.backend };
}
export function packageDisplay(model: InstalledTypedModel): ModelDisplay {
  const artifacts = model.package?.manifest?.artifacts;
  const total = artifacts?.length && artifacts.every(a => bytes(a.size) !== undefined) ? artifacts.reduce((n, a) => n + a.size, 0) : undefined;
  return { id: model.id, revision: model.revision, name: model.name || model.id, category: model.category, source: model.provenance.repository,
    engine: model.engine, format: model.format, backend: model.backend,
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
