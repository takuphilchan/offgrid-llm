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
/** Only technical repository/file labels are shortened; custom display names stay intact. */
export function speechModelName(name:string, architecture?:string):string {
  if (!/[\/\\]|\.(?:onnx|bin|gguf)\b/i.test(name)) return name;
  const file=(name.split('·').at(-1)?.trim() || name).split(/[\/\\]/).at(-1)!;
  const family=/whisper/i.test(`${architecture} ${name}`) ? 'Whisper' : /piper/i.test(`${architecture} ${name}`) ? 'Piper' : /kokoro/i.test(`${architecture} ${name}`) ? 'Kokoro' : '';
  const variant=file.replace(/\.(?:onnx|bin|gguf)$/i,'').replace(/^ggml[-_]/i,'').replace(/[_-]+/g,' ').trim();
  return [family,variant.toLowerCase()==='model' ? '' : variant].filter(Boolean).join(' · ') || name;
}
/** Compact labels are presentation only. Full names and exact IDs stay in the picker. */
export function compactModelName(id: string, limit = 28): string {
  const name = readableModelName(id.split(/[\\/]/).at(-1) || id)
    .replace(/^(?:NVIDIA|Meta)\s+/i, '')
    .replace(/\b(?:instruct|gguf|UD)\b/gi, '').replace(/\s+/g, ' ').trim();
  if ([...name].length <= limit) return name;
  const words = name.split(' '), size = words.find(word => /^(?:E?\d+(?:\.\d+)?[BM])$/i.test(word));
  const family = words.filter(word => word !== size);
  const selected: string[] = [];
  for (const word of family) {
    if ([...[...selected, word, ...(size ? [size] : [])].join(' ')].length > limit) break;
    selected.push(word);
  }
  return selected.length ? [...selected, ...(size ? [size] : [])].join(' ') : '';
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
