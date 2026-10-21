import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, statSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const manifestPath = 'scripts/maintenance/source-size-allowlist.json';
const source = /\.(?:go|py|js|mjs|cjs|ts|tsx|jsx|css|scss|html|swift|m|mm|rs|sh|ps1|c|h)$/;
const sourceRoots = /^(?:cmd|internal|apps|sdk|runtimes|scripts|tools|install\.sh)(?:\/|$)/;

export function isHandwrittenProduction(path) {
  const p = path.replaceAll('\\', '/');
  if (!sourceRoots.test(p) || !source.test(p)) return false;
  if (/(?:^|\/)(?:test|tests|__tests__|fixtures|generated|vendor|node_modules|archive|build|dist|coverage)(?:\/|$)/.test(p)) return false;
  if (/(?:_test\.go|\.(?:test|spec)\.[mc]?[jt]sx?|\.generated\.[^.]+|_generated\.[^.]+)$/.test(p)) return false;
  return true;
}

export function lineCount(content) {
  if (!content) return 0;
  return content.split(/\r?\n/).length - (content.endsWith('\n') ? 1 : 0);
}

export function sourceSizeIssues(entries, allowlist, today = new Date().toISOString().slice(0, 10)) {
  const issues = [];
  const candidates = new Map(entries.filter(([path]) => isHandwrittenProduction(path)));
  const seen = new Set();
  for (const exception of allowlist) {
    const { path, owner, responsibility, reviewDate, baselineLines } = exception;
    if (seen.has(path)) issues.push(`${path}: duplicate size exception`);
    seen.add(path);
    if (!candidates.has(path)) issues.push(`${path}: stale or non-production size exception`);
    if (typeof owner !== 'string' || !owner.trim() || typeof responsibility !== 'string' || !responsibility.trim()) {
      issues.push(`${path}: exception needs an owner and responsibility`);
    }
    if (typeof reviewDate !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(reviewDate) ||
        Number.isNaN(Date.parse(`${reviewDate}T00:00:00Z`)) || reviewDate < today) {
      issues.push(`${path}: exception needs a current ISO review date`);
    }
    if (!Number.isInteger(baselineLines) || baselineLines <= 600) {
      issues.push(`${path}: exception needs a baseline above 600 lines`);
    }
  }
  for (const [path, content] of candidates) {
    const count = lineCount(content);
    if (/^cmd\/[^/]+\/main\.go$/.test(path) && count > 150) {
      issues.push(`${path}: ${count} lines exceeds the 150-line entrypoint limit`);
    }
    if (count <= 600) continue;
    const exception = allowlist.find(item => item.path === path);
    if (!exception) issues.push(`${path}: ${count} lines exceeds 600 without a reviewed exception`);
    else if (Number.isInteger(exception.baselineLines) && count > exception.baselineLines) {
      issues.push(`${path}: ${count} lines exceeds the reviewed ${exception.baselineLines}-line baseline`);
    }
  }
  return issues;
}

export function checkSourceSize(repositoryRoot = root) {
  const paths = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], {
    cwd: repositoryRoot, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024,
  }).split('\0').filter(Boolean);
  const entries = paths.filter(path => {
    const full = resolve(repositoryRoot, path);
    return isHandwrittenProduction(path) && existsSync(full) && statSync(full).isFile();
  }).map(path => [path.replaceAll('\\', '/'), readFileSync(resolve(repositoryRoot, path), 'utf8')]);
  const allowlist = JSON.parse(readFileSync(resolve(repositoryRoot, manifestPath), 'utf8'));
  if (!Array.isArray(allowlist)) throw new Error('source-size allowlist must be an array');
  return sourceSizeIssues(entries, allowlist);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const issues = checkSourceSize();
    if (issues.length) {
      for (const issue of issues) console.error(issue);
      process.exitCode = 1;
    } else console.log('Source-size ratchet: clean');
  } catch (error) {
    console.error(`Source-size check could not complete: ${error.message}`);
    process.exitCode = 1;
  }
}
