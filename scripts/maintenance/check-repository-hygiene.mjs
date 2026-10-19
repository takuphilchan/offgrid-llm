import { execFileSync } from 'node:child_process';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const generatedSegments = new Set([
  'bin',
  'build',
  'dist',
  '_work',
  '.gocache',
  '.pytest_cache',
  '__pycache__',
  'node_modules',
  'test-results',
  'playwright-report',
  'coverage',
]);

export function hygieneIssues(trackedPaths, packagePaths) {
  const issues = [];
  for (const original of trackedPaths) {
    const path = original.replaceAll('\\', '/');
    const segments = path.split('/');
    if (segments.some((segment, index) => {
      // scripts/build contains maintained build automation, not build output.
      if (segment === 'build' && index === 1 && segments[0] === 'scripts') return false;
      return generatedSegments.has(segment) || segment.endsWith('.egg-info');
    })) {
      issues.push(`tracked generated output: ${path}`);
    }
  }
  for (const path of packagePaths) {
    const normalized = path.replaceAll('\\', '/');
    if (/(?:^|\/)(?:build|_work)(?:\/|$)/.test(normalized)) {
      issues.push(`Go package in output directory: ${normalized}`);
    }
  }
  return issues;
}

export function checkRepository(root = repositoryRoot) {
  const tracked = execFileSync('git', ['ls-files', '-z'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 32 * 1024 * 1024,
  }).split('\0').filter(Boolean);
  const packages = execFileSync('go', ['list', './...'], {
    cwd: root,
    encoding: 'utf8',
    maxBuffer: 32 * 1024 * 1024,
  }).split(/\r?\n/).filter(Boolean);
  return hygieneIssues(tracked, packages);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const issues = checkRepository();
    if (issues.length) {
      for (const issue of issues) console.error(issue);
      process.exitCode = 1;
    } else {
      console.log('Repository hygiene: clean');
    }
  } catch (error) {
    console.error(`Repository hygiene check could not complete: ${error.message}`);
    process.exitCode = 1;
  }
}
