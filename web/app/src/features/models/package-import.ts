import type { ModelPackageManifest } from '../../api/client';

export async function preparePackageImport(files: File[]): Promise<{ manifest: ModelPackageManifest; body: FormData }> {
  const entries = new Map<string, File>();
  for (const file of files) {
    const relative = file.webkitRelativePath ? file.webkitRelativePath.split('/').slice(1).join('/') : file.name;
    if (entries.has(relative)) throw new Error('duplicate_file');
    entries.set(relative, file);
  }
  const manifestFile = entries.get('manifest.json');
  if (!manifestFile || manifestFile.size > 1024 ** 2) throw new Error('missing_manifest');
  const manifest = JSON.parse(await manifestFile.text()) as ModelPackageManifest;
  if (manifest.schema_version !== 1 || !manifest.id || !manifest.revision || !Array.isArray(manifest.artifacts) || manifest.artifacts.length === 0 || manifest.artifacts.length > 2048 || entries.size !== manifest.artifacts.length + 1) throw new Error('invalid_manifest');
  const body = new FormData();
  body.append('manifest', manifestFile);
  const seen = new Set<string>(['manifest.json']);
  let total = 0;
  for (const artifact of manifest.artifacts) {
    const file = entries.get(artifact.path);
    if (!file || seen.has(artifact.path) || !Number.isSafeInteger(artifact.size) || artifact.size <= 0 || file.size !== artifact.size) throw new Error('invalid_artifacts');
    seen.add(artifact.path); total += file.size;
    body.append(artifact.path, file);
  }
  if (total > 32 * 1024 ** 3) throw new Error('package_too_large');
  return { manifest, body };
}
