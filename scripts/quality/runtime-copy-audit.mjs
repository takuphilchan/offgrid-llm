// Source candidates outside the renderer catalog. These are not assumed to be
// user content merely because they arrive from a service or CLI process.
import {readFileSync,readdirSync} from 'node:fs';
import {resolve,relative,join} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=resolve(import.meta.dirname,'../..');
const walk=dir=>readdirSync(dir,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?walk(join(dir,entry.name)):[join(dir,entry.name)]);
export function runtimeCopyCandidates(){
 const files=[...walk(resolve(root,'internal/cli')),...walk(resolve(root,'cmd/offgrid')),
 ...walk(resolve(root,'internal/models')).filter(file=>/catalog[^\\/]*\.go$/.test(file)),
 ...readdirSync(resolve(root,'apps/desktop')).filter(name=>/^terminal.*\.js$/.test(name)&&!/(translations|accessibility)/.test(name)).map(name=>resolve(root,'apps/desktop',name))]
 .filter(file=>!/(_test\.go|\.test\.js)$/.test(file)&&/\.(go|js)$/.test(file));
 const rows=[];
 for(const file of files){
  const source=readFileSync(file,'utf8');
  for(const match of source.matchAll(/"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`[^`]*`/gs)){
   let text=match[0].slice(1,-1);if(match[0][0]==='"')try{text=JSON.parse(match[0]);}catch{}
   if(!/[A-Za-z]{3,}[ \t]+[A-Za-z]{2,}/.test(text)||text.includes('<svg')||text.includes('<div'))continue;
   rows.push({file:relative(root,file).replaceAll('\\','/'),line:source.slice(0,match.index).split('\n').length,text,
    status:'candidate-needs-context-review',boundary:file.includes('catalog')?'service-catalog':file.endsWith('.go')?'cli':'terminal-host'});
  }
 }
 return rows;
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))console.log(JSON.stringify(runtimeCopyCandidates(),null,2));
