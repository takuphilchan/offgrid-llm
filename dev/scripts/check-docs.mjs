// Offline checks for maintained repository documentation. No network or writes.
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
export function maintained(file) {
  return file.endsWith('.md') && !/^(?:\.agents\/|docs\/(?:releases|templates)\/)/.test(file);
}

// Fences may contain Markdown examples; do not mistake their links for prose.
export function prose(source) {
  let fence;
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const result = lines.map((line, index) => {
    const match = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (fence) {
      if (match && match[1][0] === fence.char && match[1].length >= fence.length && !match[2].trim()) fence = null;
      return '';
    }
    if (match) { fence = {char:match[1][0], length:match[1].length, line:index + 1}; return ''; }
    return line.replace(/`[^`\n]+`/g, value => ' '.repeat(value.length));
  });
  return {lines:result, unclosedFence:fence?.line};
}

export function checkDocument(file, source, paths) {
  const errors = [];
  const parsed = prose(source);
  if (parsed.unclosedFence) errors.push({file, line:parsed.unclosedFence, message:'Unclosed fenced code block'});
  const lower = new Map([...paths].map(p => [p.toLowerCase(), p]));
  const check = (target, line) => {
    if (/^(?:[a-z][a-z0-9+.-]*:|\/\/|#)/i.test(target)) return;
    let name;
    try { name = decodeURIComponent(target.split(/[?#]/, 1)[0]); }
    catch { errors.push({file,line,message:`Invalid link encoding: ${target}`}); return; }
    if (!name) return;
    const resolved = path.posix.normalize(name.startsWith('/') ? name.slice(1) : path.posix.join(path.posix.dirname(file), name)).replace(/\/$/, '');
    if (paths.has(resolved) || [...paths].some(p => p.startsWith(resolved + '/'))) return;
    const actual = lower.get(resolved.toLowerCase());
    errors.push({file,line,message:actual ? `Link case mismatch: ${target} (tracked as ${actual})` : `Missing local link: ${target}`});
  };
  for (const [index, line] of parsed.lines.entries()) {
    // Inline links/images and reference definitions. Angle destinations may contain spaces.
    for (const match of line.matchAll(/!?\[[^\]\n]*\]\(\s*(?:<([^>]+)>|([^\s)]+))(?:\s+["'][^\n]*?["'])?\s*\)/g)) check(match[1] ?? match[2], index + 1);
    const reference = line.match(/^ {0,3}\[[^\]]+\]:\s*(?:<([^>]+)>|(\S+))/);
    if (reference) check(reference[1] ?? reference[2], index + 1);
  }
  return errors;
}

export function checkRepository() {
  // Git supplies exact casing even on case-insensitive Windows filesystems.
  const paths = new Set(execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], {cwd:root, encoding:'utf8'}).split('\0').filter(Boolean));
  const documents = [...paths].filter(maintained);
  const errors = documents.flatMap(file => checkDocument(file, readFileSync(path.join(root, file), 'utf8'), paths));
  for (const error of errors) console.error(`${error.file}:${error.line}: ${error.message}`);
  console.log(`Checked ${documents.length} maintained Markdown files: ${errors.length} problem(s). Release archives and example templates excluded. External URLs and heading anchors are not checked.`);
  return errors.length;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) process.exitCode = checkRepository() ? 1 : 0;
