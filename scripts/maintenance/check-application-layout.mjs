import { execFileSync } from 'node:child_process';
import { readFileSync, statSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const sourcePrefixes = ['web/app/', 'desktop/', 'computer/'];
const textExtensions = /(?:\.go|\.js|\.mjs|\.cjs|\.ts|\.tsx|\.sh|\.ps1|\.yml|\.yaml|\.json|\.md|\.gitignore|\.dockerignore|\.gitattributes|Dockerfile(?:\.[^/]*)?)$/;
const historical = path => path.startsWith('docs/releases/') ||
  path.startsWith('docs/quality/evidence/') ||
  path.startsWith('openspec/changes/') ||
  path === 'docs/advanced/product-reliability-plan.md';
const fixtureSource = path => path === 'scripts/maintenance/check-application-layout.mjs' ||
  path === 'scripts/maintenance/check-application-layout.test.mjs';

function allowedSibling(path, kind, prefix) {
  if (kind === 'web/dist') return path.startsWith('apps/desktop/') && prefix.endsWith('../');
  if (kind === 'desktop/') return path.startsWith('apps/web/') && prefix.endsWith('../') ||
    path.startsWith('apps/computer/') && prefix.endsWith('../../');
  if (kind === 'computer/') return path.startsWith('apps/desktop/') && prefix.endsWith('../') ||
    prefix.endsWith('/api/v2/');
  return false;
}

const patterns = [
  ['web/app', /(?<!apps\/)web\/app(?:\/|\b)/g],
  ['web/dist', /(?<!apps\/)web\/dist(?:\/|\b)/g],
  ['application command', /(?:working-directory:\s*|(?:^|[;\s])cd\s+|--prefix\s+)(?:desktop|computer)(?=\s|$)/gm],
  ['old desktop root navigation', /\bcd\s+\.\.\/\.\.\/desktop(?=\s|$)/gm],
  ['desktop/', /(?<!apps\/)desktop\/(?:package(?:-lock)?\.json|README\.md|main\.js|preload\.js|dist\/|compatibility\.cjs|assets\/|test\/|node_modules\/|legacy-compatibility\.json|prepare-computer\.cjs|verify-computer\.cjs)/g],
  ['computer/', /(?<!apps\/)computer\/(?:README\.md|package(?:-lock)?\.json|native\/|pack\.cjs|companion\/|companion\.mjs|test\/|native-worker\.cjs|native-session\.mjs)/g],
];

export function applicationLayoutIssues(entries) {
  const issues = [];
  for (const [path, content] of entries) {
    if (sourcePrefixes.some(prefix => path.startsWith(prefix))) {
      issues.push(`${path}: tracked source remains at the old application path`);
    }
    if (historical(path) || fixtureSource(path) || !textExtensions.test(path) || path.endsWith('package-lock.json')) continue;
    for (const [kind, pattern] of patterns) {
      pattern.lastIndex = 0;
      for (const match of content.matchAll(pattern)) {
        const prefix = content.slice(Math.max(0, match.index - 24), match.index);
        if (allowedSibling(path, kind, prefix)) continue;
        const line = content.slice(0, match.index).split('\n').length;
        issues.push(`${path}:${line}: obsolete ${kind} source reference`);
      }
    }
  }
  return issues;
}

export function checkApplicationLayout(repositoryRoot = root) {
  const paths = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], {
    cwd: repositoryRoot, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024,
  }).split('\0').filter(Boolean);
  const entries = [];
  for (const path of paths) {
    // The index can still name a file removed in the working tree during a
    // move. Inspect the buildable tree, including staged/untracked additions.
    try {
      statSync(resolve(repositoryRoot, path));
      const content = historical(path) || fixtureSource(path) || !textExtensions.test(path) || path.endsWith('package-lock.json')
        ? '' : readFileSync(resolve(repositoryRoot, path), 'utf8');
      entries.push([path, content]);
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
  }
  return applicationLayoutIssues(entries);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const issues = checkApplicationLayout();
    if (issues.length) {
      for (const issue of issues) console.error(issue);
      process.exitCode = 1;
    } else {
      console.log('Application source layout: clean');
    }
  } catch (error) {
    console.error(`Application source layout check could not complete: ${error.message}`);
    process.exitCode = 1;
  }
}
