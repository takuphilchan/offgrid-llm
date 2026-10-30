// Freeze authored palette and color-bearing declarations, including legacy rules.
// Uses the parser already installed with Vite; no additional dependency.
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve, dirname } from 'node:path';
const root = resolve(import.meta.dirname, '../..');
const require = createRequire(resolve(root, 'apps/web/package.json'));
const postcss = require('postcss');
const base = process.argv[2];
if (!base) throw Error('Usage: node scripts/test/check-ui-colors.mjs BASE [SNAPSHOT.json]');
const git = args => execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
const roots = ['apps/web/src', 'apps/web/public', 'apps/desktop'];
const baselinePaths = new Set(git(['ls-tree', '-r', '--name-only', base, '--', ...roots]).trim().split('\n'));
const currentPaths = git(['ls-files', '--cached', '--others', '--exclude-standard', '--', ...roots]).trim().split('\n');
const paths = [...new Set([...baselinePaths, ...currentPaths])]
  .filter(p => /\.(css|tsx?|[cm]?js|html|svg)$/.test(p) && !/\/test\//.test(p));
const literals = /#[\da-f]{3,8}\b|\b(?:rgba?|hsla?|oklch|oklab|color-mix)\([^;{}\n]+\)/gi;
const colorProperty = /^(?:color|background(?:-color|-image)?|(?:border(?:-(?:top|right|bottom|left|inline(?:-start|-end)?|block(?:-start|-end)?))?)(?:-color)?|outline(?:-color)?|box-shadow|text-shadow|fill|stroke|accent-color|caret-color|scrollbar-color)$/;
function inventory(read) {
  return paths.flatMap(path => {
    const source = read(path);
    const result = [...source.matchAll(literals)].map(m => [path, 'literal', m[0]]);
    if (path.endsWith('.css')) postcss.parse(source).walkDecls(d => {
      if (!colorProperty.test(d.prop) && !(d.prop.startsWith('--') && /#|rgba?\(|hsla?\(|var\(--(?:surface|text|muted|line|accent|shadow|scrim|red|green|amber|faint)/.test(d.value))) return;
      const context = []; for (let p = d.parent; p; p = p.parent) if (p.selector || p.name) context.unshift(p.selector ?? `@${p.name} ${p.params}`);
      result.push([path, context.join(' / '), d.prop, d.value]);
    });
    return result;
  }).sort((a,b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
}
const before = inventory(path => baselinePaths.has(path) ? git(['show', `${base}:${path}`]) : '');
const after = inventory(path => existsSync(resolve(root,path)) ? readFileSync(resolve(root,path),'utf8') : '');
if (process.argv[3]) { mkdirSync(dirname(resolve(process.argv[3])), {recursive:true}); writeFileSync(process.argv[3], JSON.stringify({base, entries:before},null,2)); }
const counts = list => { const map = new Map(); for (const entry of list) { const key=JSON.stringify(entry); map.set(key,(map.get(key)??0)+1); } return map; };
const a=counts(before), b=counts(after), changes=[];
for (const key of new Set([...a.keys(),...b.keys()])) if(a.get(key)!==b.get(key)) changes.push({entry:JSON.parse(key), before:a.get(key)??0, after:b.get(key)??0});
console.log(JSON.stringify({base,files:paths.length,entries:before.length,colorChanges:changes},null,2));
if(changes.length) process.exitCode=1;
