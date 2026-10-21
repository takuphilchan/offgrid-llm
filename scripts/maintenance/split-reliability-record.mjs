import { createHash } from 'node:crypto';
import { constants, copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const sourcePath = join(root, 'docs/advanced/product-reliability-plan.md');
const evidenceDir = join(root, 'docs/quality/evidence');
const manifestPath = join(evidenceDir, 'reliability-split-manifest.json');
const backupPath = join(root, 'build/documentation-backups/reliability-plan-2026-10-21.md');
const sourceBase = dirname(sourcePath);

const groups = [
  { file: 'program-foundations.md', label: 'Product outcome and delivery rules', start: null },
  { file: 'desktop-artifacts-2026-10-17.md', label: 'Desktop authority and artifact repairs, October 17', start: '## Follow-up: remembered Full access in Chat (2026-10-17)' },
  { file: 'model-adaptive-2026-10-16.md', label: 'Model-adaptive execution, October 16', start: '## Follow-up: model-adaptive execution (2026-10-16)' },
  { file: 'agents-and-launch-2026-10.md', label: 'Agents and application launch, October', start: '## Follow-up: conversational agents and profile persistence (2026-10-13)' },
  { file: 'native-documents-2026-10.md', label: 'Native document workflows, October', start: '## Native document provider implementation and test deployment (2026-10-12)' },
  { file: 'desktop-access-2026-10.md', label: 'Desktop access and consent, October', start: '## Bounded desktop context, progress and registered applications (2026-10-10)' },
  { file: 'workspace-2026-10.md', label: 'Workspace and UI, October', start: '## Workspace outcome experience: baseline (2026-10-08)' },
  { file: 'voice-2026-10.md', label: 'Voice program, October', start: '## Voice review corrections, source-only validation (2026-10-05)' },
  { file: 'quality-stages-and-release-evidence.md', label: 'Quality stages and release evidence', start: '## Stage 1: privacy and data safety' },
  { file: 'release-evidence-2026-09.md', label: 'September release and recovery evidence', start: '## 2026-09-19 — Real embedding and inference correctness slice' },
];

const digest = value => createHash('sha256').update(value).digest('hex');
const normalized = value => value.replaceAll('\\', '/');

function rebaseLinks(original, destination) {
  const changes = [];
  const rewritten = original.replace(/\]\(([^)]+)\)/g, (match, target) => {
    if (/^(?:https?:|mailto:|#)/.test(target)) return match;
    const hashAt = target.indexOf('#');
    const path = hashAt < 0 ? target : target.slice(0, hashAt);
    const suffix = hashAt < 0 ? '' : target.slice(hashAt);
    let absolute = resolve(sourceBase, path);
    if (path === 'ARCHITECTURE.md') absolute = join(root, 'docs/architecture/overview.md');
    if (path === 'production-readiness.md') absolute = join(root, 'docs/quality/production-readiness.md');
    const replacement = normalized(relative(dirname(destination), absolute)) + suffix;
    changes.push({ from: target, to: replacement });
    return `](${replacement})`;
  });
  return { rewritten, changes };
}

function verify() {
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  let reconstructed = '';
  for (const part of manifest.parts) {
    const value = readFileSync(join(evidenceDir, part.file), 'utf8');
    if (digest(value) !== part.writtenSha256 || Buffer.byteLength(value) !== part.writtenBytes) {
      throw new Error(`evidence part changed: ${part.file}`);
    }
    let original = value;
    for (const change of part.linkRebases) {
      original = original.replaceAll(`](${change.to})`, `](${change.from})`);
    }
    if (digest(original) !== part.originalSha256 || Buffer.byteLength(original) !== part.originalBytes) {
      throw new Error(`original section does not reconcile: ${part.file}`);
    }
    reconstructed += original;
  }
  if (digest(reconstructed) !== manifest.originalSha256 || Buffer.byteLength(reconstructed) !== manifest.originalBytes) {
    throw new Error('original reliability record does not reconcile');
  }
  return manifest;
}

function split() {
  const original = readFileSync(sourcePath, 'utf8');
  if (Buffer.byteLength(original) < 400_000 || existsSync(manifestPath)) {
    throw new Error('source is not the expected unsplit record, or split manifest already exists');
  }
  const starts = groups.map((group, index) => index === 0 ? 0 : original.indexOf(group.start));
  if (starts.some((position, index) => position < 0 || (index > 0 && position <= starts[index - 1]))) {
    throw new Error('reliability record headings changed; review boundaries before splitting');
  }
  mkdirSync(evidenceDir, { recursive: true });
  mkdirSync(dirname(backupPath), { recursive: true });
  if (existsSync(backupPath)) {
    if (digest(readFileSync(backupPath)) !== digest(original)) throw new Error('backup path contains different bytes');
  } else {
    copyFileSync(sourcePath, backupPath, constants.COPYFILE_EXCL);
  }
  const parts = groups.map((group, index) => {
    const content = original.slice(starts[index], starts[index + 1] ?? original.length);
    const destination = join(evidenceDir, group.file);
    if (existsSync(destination)) throw new Error(`evidence destination already exists: ${group.file}`);
    const { rewritten, changes } = rebaseLinks(content, destination);
    return { file: group.file, label: group.label, content: rewritten, linkRebases: changes,
      originalBytes: Buffer.byteLength(content), originalSha256: digest(content),
      writtenBytes: Buffer.byteLength(rewritten), writtenSha256: digest(rewritten) };
  });
  for (const part of parts) writeFileSync(join(evidenceDir, part.file), part.content, { flag: 'wx' });
  const manifest = { version: 1, source: 'docs/advanced/product-reliability-plan.md', originalBytes: Buffer.byteLength(original),
    originalSha256: digest(original), parts: parts.map(({ content, ...metadata }) => metadata) };
  writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx' });
  verify();
  const links = parts.map(part => `- [${part.label}](../quality/evidence/${part.file})`).join('\n');
  const index = `# Reliability evidence index\n\nThis former long-form record is split into dated evidence sections. These preserve the original claims and limitations; they do not certify a current release. The [quality index](../quality/README.md) explains how to read implemented, automatically tested and real-hardware-qualified states. The exact original section hashes and link rebases are recorded in [the split manifest](../quality/evidence/reliability-split-manifest.json).\n\n${links}\n\nA local backup of the pre-split working copy is retained under \`build/documentation-backups/reliability-plan-2026-10-21.md\` and is not tracked as source.\n`;
  writeFileSync(sourcePath, index);
  return manifest;
}

const mode = process.argv[2];
try {
  const manifest = mode === '--write' ? split() : mode === '--check' ? verify() : null;
  if (!manifest) throw new Error('use --write or --check');
  console.log(`Reliability evidence reconciled: ${manifest.parts.length} parts, ${manifest.originalBytes} original bytes, SHA-256 ${manifest.originalSha256}`);
} catch (error) {
  console.error(`Reliability evidence split failed: ${error.message}`);
  process.exitCode = 1;
}
