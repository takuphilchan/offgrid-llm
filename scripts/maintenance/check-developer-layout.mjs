import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, statSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const sourcePrefixes = ['dev/', 'python/'];
const requiredSource = [
  'docker/test/native-contracts.Dockerfile',
  'tools/model-package-metadata/main.go',
  'sdk/python/pyproject.toml',
  'runtimes/python/audio_service.py',
  'runtimes/python/provider_worker.py',
];
const textExtensions = /(?:\.go|\.py|\.js|\.mjs|\.cjs|\.ts|\.tsx|\.sh|\.ps1|\.bat|\.cmd|\.c|\.h|\.html|\.css|\.plist|\.service|\.yml|\.yaml|\.json|\.md|\.toml|Makefile|Dockerfile(?:\.[^/]*)?)$/;
const historical = path => path.startsWith('docs/releases/') ||
  path.startsWith('docs/quality/evidence/') ||
  path.startsWith('openspec/changes/') ||
  path === 'docs/advanced/product-reliability-plan.md';
const fixtureSource = path => path === 'scripts/maintenance/check-developer-layout.mjs' ||
  path === 'scripts/maintenance/check-developer-layout.test.mjs';
const patterns = [
  ['dev', /\bdev\/(?:scripts\/|Makefile\b|README\.md\b|install\.sh\b|containers\/|examples\/|tools\/)/g],
  ['python', /(?<!sdk\/)(?<!runtimes\/)\bpython\/(?:README\.md\b|pyproject\.toml\b|offgrid\/|tests\/|audio_service\.py\b|offgrid_provider_worker\.py\b)/g],
];

export function developerLayoutIssues(entries) {
  const issues = [];
  for (const [path, content] of entries) {
    if (sourcePrefixes.some(prefix => path.startsWith(prefix))) {
      issues.push(`${path}: tracked source remains at the old developer/Python path`);
    }
    if (historical(path) || fixtureSource(path) || !textExtensions.test(path) || path.endsWith('package-lock.json')) continue;
    const normalizedContent = content.replaceAll('\\', '/');
    for (const [kind, pattern] of patterns) {
      pattern.lastIndex = 0;
      for (const match of normalizedContent.matchAll(pattern)) {
        const line = normalizedContent.slice(0, match.index).split('\n').length;
        issues.push(`${path}:${line}: obsolete ${kind} source reference`);
      }
    }
  }
  return issues;
}

export function requiredDeveloperPathIssues(paths, dockerignore = '') {
  const existing = new Set(paths);
  const issues = requiredSource.filter(path => !existing.has(path))
    .map(path => `${path}: required relocated source is missing`);
  const ignored = new Set(dockerignore.split(/\r?\n/).map(line => line.trim())
    .filter(line => line && !line.startsWith('#') && !line.startsWith('!')));
  if (['docker/', 'docker/test/', 'docker/test/native-contracts.Dockerfile']
    .some(rule => ignored.has(rule))) {
    issues.push('docker/test/native-contracts.Dockerfile: native test context is excluded by .dockerignore');
  }
  return issues;
}

export function checkDeveloperLayout(repositoryRoot = root) {
  const paths = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], {
    cwd: repositoryRoot, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024,
  }).split('\0').filter(Boolean);
  const entries = paths.filter(path => {
    const absolute = resolve(repositoryRoot, path);
    return existsSync(absolute) && statSync(absolute).isFile();
  }).map(path => [path, historical(path) || fixtureSource(path) || !textExtensions.test(path)
    ? '' : readFileSync(resolve(repositoryRoot, path), 'utf8')]);
  return [
    ...developerLayoutIssues(entries),
    ...requiredDeveloperPathIssues(paths, readFileSync(resolve(repositoryRoot, '.dockerignore'), 'utf8')),
  ];
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const issues = checkDeveloperLayout();
    if (issues.length) {
      for (const issue of issues) console.error(issue);
      process.exitCode = 1;
    } else {
      console.log('Developer/Python source layout: clean');
    }
  } catch (error) {
    console.error(`Developer/Python layout check could not complete: ${error.message}`);
    process.exitCode = 1;
  }
}
