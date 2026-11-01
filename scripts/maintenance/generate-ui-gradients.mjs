// Palette-derived only. The eased ramp is sampled from smoothstep, never from
// literal colors. Generated CSS is committed inside the authoritative token file.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
const root = resolve(import.meta.dirname, '../..');
const names = ['sidebar', 'composer', 'primary', 'progress', 'floating'];
const flat = names.map(name => `    --gradient-${name}: none;`).join('\n');
const opaque = `    --material-fill: var(--surface-1);
    --material-blur: none;`;
const ramp = (end) => Array.from({length:9}, (_, i) => {
  const x=i/8, weight=(100-end*(x*x*(3-2*x))).toFixed(3).replace(/\.?0+$/,'');
  return `color-mix(in oklab, var(--text) ${weight}%, var(--surface-3)) ${x*100}%`;
}).join(', ');
const css = `/* BEGIN GENERATED TONAL GRADIENTS */
@layer base {
  :root {
${flat}
${opaque}
    --primary-hover: var(--accent-strong);
  }
  @supports (background-image: linear-gradient(180deg in oklab, currentColor, currentColor)) and (color: color-mix(in oklab, currentColor, currentColor)) {
    :root {
      --gradient-sidebar: linear-gradient(165deg in oklab, var(--surface-2) 0%, var(--surface-1) 35%, var(--surface-0) 100%);
      --gradient-composer: linear-gradient(165deg in oklab, var(--surface-2) 0%, var(--surface-1) 70%);
      --gradient-floating: linear-gradient(165deg in oklab, color-mix(in oklab, var(--surface-2) 35%, transparent), transparent 65%);
      --primary-hover: color-mix(in oklab, var(--text) 92%, var(--surface-3));
      --gradient-progress: linear-gradient(180deg in oklab, ${ramp(12)});
    }
    :root[data-theme='light'] {
      --gradient-sidebar: linear-gradient(165deg in oklab, var(--surface-1) 0%, var(--surface-0) 35%, var(--surface-2) 100%);
      --gradient-composer: linear-gradient(165deg in oklab, var(--surface-1) 0%, color-mix(in oklab, var(--surface-1) 60%, var(--surface-2)) 100%);
      --gradient-progress: linear-gradient(180deg in oklab, ${ramp(18)});
    }
  }
  @supports (backdrop-filter: blur(12px)) and (color: color-mix(in srgb, currentColor, transparent)) {
    :root {
      --material-fill: color-mix(in srgb, var(--surface-1) 96%, transparent);
      --material-blur: blur(12px);
    }
  }
  :root[data-gradients='off'] {
${flat}
${opaque}
  }
  @media (forced-colors: active), (prefers-contrast: more), (prefers-reduced-transparency: reduce) {
    :root, :root[data-theme='light'] {
${flat}
${opaque}
    }
  }
}
/* END GENERATED TONAL GRADIENTS */`;
const consumers = `
@layer workspace {
  .app-shell > .sidebar { background-image: var(--gradient-sidebar); }
  .chat-workspace .composer { background-image: var(--gradient-composer); }
  .download-state.downloading > div span, .download-state.finalizing > div span { background-image: var(--gradient-progress); }
  .utility-popover-panel, dialog.command-palette {
    background-color: var(--material-fill); background-image: var(--gradient-floating); backdrop-filter: var(--material-blur);
  }
}
@layer controls {
  .primary-button:not(:disabled), .composer-actions > button:last-child:not(:disabled) { background-image: var(--gradient-primary); }
}
`;
if(process.argv.includes('--preview')) {
  mkdirSync(resolve(root,'build/ui-craft'),{recursive:true});
  writeFileSync(resolve(root,'build/ui-craft/gradient-prototype.css'),css+consumers);
  console.log('Wrote isolated preview; production CSS unchanged.');
} else {
  const path=resolve(root,'apps/web/src/tokens.css'), source=readFileSync(path,'utf8');
  const marker=/\/\* BEGIN GENERATED TONAL GRADIENTS \*\/[\s\S]*?\/\* END GENERATED TONAL GRADIENTS \*\//;
  const next=marker.test(source)?source.replace(marker,css):`${source.trimEnd()}\n\n${css}\n`;
  if(process.argv.includes('--write')) {writeFileSync(path,next);console.log('Updated gradient tokens.');}
  else if(source.replaceAll('\r\n','\n')!==next.replaceAll('\r\n','\n')) {console.error('Gradient tokens are stale. Run with --write.');process.exitCode=1;}
  else console.log('Gradient tokens match their generator.');
}
