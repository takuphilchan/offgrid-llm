import {inputs,entriesFor,terminologyVariants} from './translation-review.mjs';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';

export function terminologyReport(input){
 return Object.fromEntries(Object.keys(input.registry).map(language=>{
  const entries=entriesFor(language,input),variants=terminologyVariants(entries);
  const mainChat=entries.find(entry=>entry.namespace==='core'&&entry.key==='nav.chat');
  const terminalChat=entries.find(entry=>entry.namespace==='desktop/terminal'&&entry.key==='Chat');
  const failures=mainChat?.currentText!==terminalChat?.currentText?['Chat destination differs between main navigation and terminal']:[];
  for(const route of ['chat','knowledge','agents','models','activity','settings']){
   const navigation=entries.find(entry=>entry.namespace==='core'&&entry.key==='nav.'+route);
   const heading=entries.find(entry=>entry.namespace==='core'&&entry.key===route+'.title');
   if(navigation&&heading&&navigation.currentText!==heading.currentText)failures.push(route+' destination differs between navigation and page title');
  }
  return [language,{failures,variants}];
 }));
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 const report=terminologyReport(inputs());
 if(process.argv.includes('--json'))console.log(JSON.stringify(report,null,2));
 else for(const [language,row] of Object.entries(report))console.log(`${language}: ${row.failures.length} shared-navigation conflicts; ${row.variants.length} source phrases with contextual wording variants to review`);
 if(Object.values(report).some(row=>row.failures.length))process.exitCode=1;
}
