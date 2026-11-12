// Read-only source inventory. Computed-layout/browser checks live in dividers.spec.ts.
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { createRequire } from 'node:module';
const root = resolve(import.meta.dirname, '../..');
const require = createRequire(join(root, 'apps/web/package.json'));
const postcss = require('postcss');
const files = [];
function scan(directory) {
  for (const entry of readdirSync(directory, {withFileTypes:true})) {
    const file = join(directory, entry.name);
    if (entry.isDirectory()) scan(file);
    else if (entry.name.endsWith('.css')) files.push(file);
  }
}
scan(join(root, 'apps/web/src'));
for (const name of readdirSync(join(root, 'apps/desktop'))) {
  if (name.endsWith('.css') || name === 'startup.html') files.push(join(root, 'apps/desktop', name));
}
const declarations = [];
for (const file of files) {
  const source = readFileSync(file, 'utf8');
  const css = file.endsWith('.html') ? [...source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map(m=>m[1]).join('\n') : source;
  postcss.parse(css).walkDecls(d => {
    if (!/^border(?:$|-)/.test(d.prop) && !(d.prop === 'box-shadow' && d.value.includes('inset'))) return;
    const selector = d.parent.selector ?? '';
    const horizontal = /^border-(block(?:-start|-end)?|top|bottom)(?:-|$)/.test(d.prop);
    const role = d.value.includes('--separator') ? 'decorative'
      : /^(?:0|none)(?:\s|$)/.test(d.value) ? 'removed'
      : /spinner|indicator|voice-progress|summary::before|document-scan|orb/.test(selector) ? 'indicator-or-shape'
      : /focus|selected|pressed|current/.test(selector) ? 'focus-or-selection'
      : horizontal && selector !== '.command-search' && !d.value.includes('transparent') ? 'review'
      : 'control-or-enclosure';
    declarations.push({file:relative(root,file).replaceAll('\\','/'),line:d.source.start.line,selector,property:d.prop,value:d.value,role});
  });
}
const counts = Object.fromEntries([...new Set(declarations.map(d=>d.role))].map(role=>[role,declarations.filter(d=>d.role===role).length]));
const result = {scope:'Renderer source CSS and desktop-owned CSS/startup; excludes generated assets and terminal output',files:files.length,counts,declarations};
if (process.argv[2]) writeFileSync(resolve(process.argv[2]), JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify({files:result.files,counts,review:declarations.filter(d=>d.role==='review')},null,2));
