// Inventory source and quarantine only known, ignored workspace outputs.
// No installed-user paths or arbitrary source directories are accepted.
import { execFileSync, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, lstatSync, mkdirSync, readFileSync, readdirSync, readlinkSync, realpathSync, renameSync, writeFileSync } from 'node:fs';
import { dirname, isAbsolute, join, parse, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const fixedOutputs = [
  'apps/computer/node_modules',
  'apps/desktop/node_modules',
  'apps/desktop/dist',
  'apps/web/node_modules',
  'apps/web/dist',
  'apps/web/test-results',
  'apps/web/.vite-e2e.stdout.log',
  'apps/web/.vite-e2e.stderr.log',
  'sdk/python/build',
  'sdk/python/offgrid.egg-info',
  'sdk/python/.pytest_cache',
];
const mutableEvidence = 'openspec/changes/repository-workspace-cleanup/';

function git(root, args) {
  return execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
}

function within(parent, child) {
  const rel = relative(parent, child);
  return rel === '' || (rel !== '..' && !rel.startsWith(`..${sep}`) && !isAbsolute(rel));
}

function fileHash(path) {
  const info = lstatSync(path);
  const data = info.isSymbolicLink() ? readlinkSync(path) : readFileSync(path);
  return createHash('sha256').update(data).digest('hex');
}

function outputStats(path) {
  const info = lstatSync(path);
  if (!info.isDirectory() || info.isSymbolicLink()) return { files: 1, bytes: info.size, links: Number(info.isSymbolicLink()) };
  const total = { files: 0, bytes: 0, links: 0 };
  for (const entry of readdirSync(path)) {
    const child = outputStats(join(path, entry));
    total.files += child.files;
    total.bytes += child.bytes;
    total.links += child.links;
  }
  return total;
}

function pythonCaches(root, base) {
  const found = [];
  const visit = dir => {
    if (!existsSync(dir)) return;
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory() || entry.isSymbolicLink()) continue;
      const child = join(dir, entry.name);
      if (entry.name === '__pycache__') { found.push(relative(root, child).replaceAll('\\', '/')); continue; }
      if (['build', 'dist', 'node_modules', '.pytest_cache', 'offgrid.egg-info'].includes(entry.name)) continue;
      visit(child);
    }
  };
  visit(join(root, base));
  return found;
}

export function outputTargets(root) {
  const paths = [...fixedOutputs, ...pythonCaches(root, 'sdk/python'), ...pythonCaches(root, 'runtimes/python')];
  return paths.filter(path => existsSync(join(root, path))).sort().map(path => ({ path, ...outputStats(join(root, path)) }));
}

export function inventory(root) {
  const source = [...new Set(git(root, ['ls-files', '--cached', '--others', '--exclude-standard', '-z']).split('\0').filter(Boolean))]
    .filter(path => !path.replaceAll('\\', '/').startsWith(mutableEvidence))
    .filter(path => existsSync(join(root, path)) && !lstatSync(join(root, path)).isDirectory())
    .sort().map(path => {
      const absolute = join(root, path);
      return { path: path.replaceAll('\\', '/'), bytes: lstatSync(absolute).size, sha256: fileHash(absolute) };
    });
  const sourceDigest = createHash('sha256').update(source.map(entry =>
    `${entry.path}\0${entry.bytes}\0${entry.sha256}\n`).join('')).digest('hex');
  return {
    version: 1,
    root: realpathSync(root),
    capturedAt: new Date().toISOString(),
    sourceDigest,
    source,
    gitStatus: git(root, ['status', '--porcelain=v1']).split(/\r?\n/).filter(Boolean),
    outputs: outputTargets(root),
  };
}

function gitPaths(root) {
  return git(root, ['ls-files', '--cached', '--others', '--exclude-standard', '-z'])
    .split('\0').filter(Boolean).map(path => path.replaceAll('\\', '/'));
}

export function validateQuarantine(root, snapshot, destination, backupBase = resolve(root, '..', 'offgrid-llm-backups')) {
  if (!isAbsolute(destination)) throw new Error('Quarantine destination must be absolute');
  const actualRoot = realpathSync(root);
  const target = resolve(destination);
  const approvedBase = realpathSync(backupBase);
  const parent = realpathSync(dirname(target));
  if (within(actualRoot, target) || within(target, actualRoot)) throw new Error('Quarantine must be outside the repository');
  if (lstatSync(backupBase).isSymbolicLink() || parent !== approvedBase) throw new Error('Quarantine must be a new child of the approved backup directory');
  if (parse(actualRoot).root.toLowerCase() !== parse(parent).root.toLowerCase()) throw new Error('Quarantine must be on the same volume');
  if (within(actualRoot, parent)) throw new Error('Quarantine parent is unsafe');
  if (existsSync(target)) throw new Error('Quarantine destination already exists');
  if (snapshot.root !== actualRoot) throw new Error('Inventory belongs to another repository');
  const current = inventory(root);
  if (snapshot.sourceDigest !== current.sourceDigest) throw new Error('Source changed after inventory');
  if (JSON.stringify(snapshot.outputs) !== JSON.stringify(current.outputs)) throw new Error('Output changed after inventory');
  const nonIgnored = gitPaths(root);
  for (const entry of current.outputs) {
    const rel = entry.path;
    const absolute = resolve(root, rel);
    if (!within(actualRoot, absolute) || absolute === actualRoot || lstatSync(absolute).isSymbolicLink()) {
      throw new Error(`Unsafe output target: ${rel}`);
    }
    if (nonIgnored.some(path => path === rel || path.startsWith(`${rel}/`))) throw new Error(`Source inside output target: ${rel}`);
    const ignored = spawnSync('git', ['check-ignore', '-q', '--', rel], { cwd: root });
    if (ignored.status !== 0) throw new Error(`Output target is not ignored: ${rel}`);
    if (existsSync(join(target, rel))) throw new Error(`Occupied output destination: ${rel}`);
  }
  return current.outputs;
}

function saveManifest(path, manifest) {
  const temporary = `${path}.tmp`;
  writeFileSync(temporary, `${JSON.stringify(manifest, null, 2)}\n`, { flag: 'w' });
  renameSync(temporary, path);
}

export function quarantine(root, snapshot, destination, backupBase) {
  const outputs = validateQuarantine(root, snapshot, destination, backupBase);
  mkdirSync(destination);
  const manifestPath = join(destination, 'quarantine-manifest.json');
  const manifest = {
    version: 1, root: snapshot.root, sourceDigest: snapshot.sourceDigest,
    createdAt: new Date().toISOString(),
    entries: outputs.map(entry => ({ ...entry, state: 'pending', destination: join(destination, entry.path) })),
  };
  saveManifest(manifestPath, manifest);
  for (const entry of manifest.entries) {
    mkdirSync(dirname(entry.destination), { recursive: true });
    renameSync(join(root, entry.path), entry.destination);
    const moved = outputStats(entry.destination);
    if (moved.files !== entry.files || moved.bytes !== entry.bytes || moved.links !== entry.links) {
      throw new Error(`Moved output does not match inventory: ${entry.path}`);
    }
    entry.state = 'moved';
    saveManifest(manifestPath, manifest);
  }
  manifest.completedAt = new Date().toISOString();
  saveManifest(manifestPath, manifest);
  return manifest;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const [action, argument, destination] = process.argv.slice(2);
    if (action === 'inventory' && argument && isAbsolute(argument) && !existsSync(argument) && !within(repositoryRoot, resolve(argument))) {
      const result = inventory(repositoryRoot);
      writeFileSync(argument, `${JSON.stringify(result, null, 2)}\n`, { flag: 'wx' });
      console.log(`Source: ${result.source.length} files, SHA-256 ${result.sourceDigest}; outputs: ${result.outputs.length}; inventory: ${argument}`);
    } else if (action === 'quarantine' && argument && destination) {
      const result = quarantine(repositoryRoot, JSON.parse(readFileSync(argument, 'utf8')), destination);
      console.log(`Quarantined ${result.entries.length} targets in ${destination}`);
    } else {
      throw new Error('Usage: workspace-cleanup.mjs inventory <new-absolute-json> | quarantine <inventory-json> <new-absolute-directory>');
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
