// Source-only localization audit and translator handoff. Never contacts a service.
import {readFileSync,writeFileSync,mkdirSync,existsSync,renameSync,unlinkSync} from 'node:fs';
import {resolve,dirname} from 'node:path';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import {catalogInventory,flatten} from './translation-inventory.mjs';
import {validateTranslation} from './check-translations.mjs';
import {runtimeCopyCandidates} from './runtime-copy-audit.mjs';

const root=resolve(import.meta.dirname,'../..');
const source=resolve(root,'apps/web/src/i18n');
const json=file=>JSON.parse(readFileSync(file,'utf8'));
const optional=file=>existsSync(file)?json(file):{};
export const digest=value=>createHash('sha256').update(JSON.stringify(value)).digest('hex');
export const placeholders=text=>[...text.matchAll(/\{([A-Za-z][\w]*)\}/g)].map(match=>match[1]).sort();
export const consequential=text=>/\b(access|permission|delete|remove|revoke|stop|approval|allow|deny|discard|private|privacy|credentials?|password)\b/i.test(text);
export function inputs(){
 const inventory=catalogInventory();
 const failures=inventory.tables.filter(table=>table.error);
 if(failures.length)throw Error(JSON.stringify(failures));
 const ids=inventory.tables.map(table=>table.id);
 if(new Set(ids).size!==ids.length)throw Error('Duplicate catalog identifiers');
 return {tables:inventory.tables,registry:json(resolve(root,'apps/desktop/language-registry.json')),
  packs:json(resolve(source,'language-packs.json')),overrides:json(resolve(source,'feature-packs.json')),
  reviews:optional(resolve(source,'translation-reviews.json')),literals:inventory.literals};
}

// The same precedence as the renderer: contextual override, preview core pack,
// selected-language dictionary, then the explicit fallback. A fallback is NOT a translation.
export function entriesFor(language,input){
 const definition=input.registry[language];
 if(!definition)throw Error('Unknown language: '+language);
 const core=flatten(input.packs[language]?.messages??{});
 return input.tables.flatMap(table=>{
  const english=flatten(table.values.en),native=flatten(table.values[language]??{});
  const fallback=flatten(table.values[definition.fallback]??table.values.en);
  return Object.entries(english).map(([key,original])=>{
   const override=input.overrides[language]?.[table.id];
   const translated=Object.hasOwn(override??{},key)?override[key]
    :table.id==='core'&&Object.hasOwn(core,key)?core[key]
    :Object.hasOwn(native,key)?native[key]:null;
   const hash=digest([table.id,key,original]);
   const review=input.reviews[language]?.[table.id]?.[key];
   const sensitive=consequential(original);
   const reviewed=!!translated&&review?.status==='approved'&&review.sourceHash===hash
    &&review.translationHash===digest(translated)&&!!review.reviewer?.trim()&&!!review.evidence?.trim()
    &&(!sensitive||!!review.safetyReviewer?.trim()&&review.safetyReviewer!==review.reviewer);
   return {namespace:table.id,key,source:original,sourceHash:hash,context:table.file,
    placeholders:placeholders(original),requiresSecondReview:sensitive,translation:translated??'',
    currentText:translated??fallback[key]??original,
    coverage:language==='en'?'source':translated===null?'missing':translated===original?'same-as-source':'translated',
    review:reviewed?review:{status:'pending',reviewer:'',safetyReviewer:'',evidence:''}};
  });
 });
}
export function terminologyVariants(entries){
 const grouped=new Map();
 for(const entry of entries){
  if(entry.coverage==='missing')continue;
  const source=entry.source.trim();if(!grouped.has(source))grouped.set(source,[]);
  grouped.get(source).push({namespace:entry.namespace,key:entry.key,text:entry.currentText,context:entry.context});
 }
 return [...grouped].filter(([,uses])=>new Set(uses.map(use=>use.text)).size>1).map(([source,uses])=>({source,uses}));
}
export function reviewPack(language,input){
 const entries=entriesFor(language,input);
 return {schema:1,language,direction:input.registry[language].direction,
  sourceRevision:digest(entries.map(({namespace,key,sourceHash})=>[namespace,key,sourceHash])),
  instructions:'Edit translation and review only. Translate whole messages in context. Preserve placeholders, commands, brands, paths and model IDs. Do not mark approval without named reviewer evidence. Empty translations remain missing; English fallback is not completion.',
  terminologyVariants:terminologyVariants(entries),entries};
}
export function coverageReport(input){
 return Object.fromEntries(Object.keys(input.registry).map(language=>{
  const entries=entriesFor(language,input),missing=entries.filter(e=>e.coverage==='missing');
  const identical=entries.filter(e=>e.coverage==='same-as-source');
  const reviewed=entries.filter(e=>e.review.status==='approved').length;
  return [language,{total:entries.length,authored:entries.length-missing.length,missing:missing.length,
   sameAsSource:identical.length,reviewed,ready:language==='en'||(!missing.length&&reviewed===entries.length),
   catalogs:Object.fromEntries(input.tables.map(t=>[t.id,{total:entries.filter(e=>e.namespace===t.id).length,
    missing:missing.filter(e=>e.namespace===t.id).map(e=>e.key),sameAsSource:identical.filter(e=>e.namespace===t.id).map(e=>e.key)}]))}];
 }));
}

export function validateReviewPack(pack,input){
 const errors=[];
 if(pack?.schema!==1||typeof pack.language!=='string'||!Object.hasOwn(input.registry,pack.language)||!Array.isArray(pack.entries))return ['Invalid review pack schema or language'];
 const expected=reviewPack(pack.language,input),current=new Map(expected.entries.map(e=>[JSON.stringify([e.namespace,e.key]),e]));
 if(pack.sourceRevision!==expected.sourceRevision)errors.push('Source revision changed; re-export and reconcile the review');
 const seen=new Set();
 for(const entry of pack.entries){
  if(!entry||typeof entry!=='object'){errors.push('Invalid review entry');continue;}
  const id=JSON.stringify([entry.namespace,entry.key]),original=current.get(id);
  if(!original||seen.has(id)){errors.push(id+': unknown or duplicate message');continue;}seen.add(id);
  if(entry.source!==original.source||entry.sourceHash!==original.sourceHash)errors.push(id+': changed source');
  // Exported currentText is the optimistic concurrency token for existing translations.
  if(entry.currentText!==original.currentText)errors.push(id+': translation changed since export');
  if(typeof entry.translation!=='string'){errors.push(id+': translation must be text');continue;}
  if(entry.translation){
   errors.push(...validateTranslation({message:original.source},{message:entry.translation},{language:pack.language}).errors.map(error=>id+': '+error));
  }else if(original.translation)errors.push(id+': cannot erase an existing translation');
  if(entry.review?.status==='approved'){
   const named=value=>typeof value==='string'&&value.trim();
   if(!entry.translation||!named(entry.review.reviewer)||!named(entry.review.evidence))errors.push(id+': approval requires translation, reviewer and evidence');
   if(original.requiresSecondReview&&(!named(entry.review.safetyReviewer)||entry.review.safetyReviewer===entry.review.reviewer))errors.push(id+': permission, deletion and recovery copy requires a second reviewer');
  }
  if(!['pending','approved'].includes(entry.review?.status))errors.push(id+': invalid review status');
 }
 for(const id of current.keys())if(!seen.has(id))errors.push(id+': missing review entry');
 return errors;
}

export function importReview(pack,input){
 if(pack?.language==='en')throw Error('English is the source reference; edit the canonical source instead of importing an override');
 const errors=validateReviewPack(pack,input);if(errors.length)throw Error(errors.join('\n'));
 const overrides=structuredClone(input.overrides),reviews=structuredClone(input.reviews);
 const sources=optional(resolve(source,'translation-sources.json'));
 for(const entry of pack.entries){
  if(!entry.translation)continue;
  ((overrides[pack.language]??={})[entry.namespace]??={})[entry.key]=entry.translation;
  ((sources[pack.language]??={})[entry.namespace]??={})[entry.key]=entry.source;
  ((reviews[pack.language]??={})[entry.namespace]??={})[entry.key]={...entry.review,
   sourceHash:entry.sourceHash,translationHash:digest(entry.translation)};
 }
 return {overrides,sources,reviews};
}

function writeJSON(file,value){writeFileSync(file,JSON.stringify(value,null,2)+'\n');}
export function applyImport(result,directory=source,backupRoot=resolve(root,'build'),io={readFileSync,writeFileSync,mkdirSync,existsSync,renameSync,unlinkSync}){
 const backup=resolve(backupRoot,'translation-import-'+Date.now());io.mkdirSync(backup,{recursive:true});
 const files=Object.entries({'feature-packs.json':result.overrides,'translation-sources.json':result.sources,'translation-reviews.json':result.reviews})
  .map(([name,value])=>({name,file:resolve(directory,name),staged:resolve(directory,name+'.pending'),data:JSON.stringify(value,null,2)+'\n'}));
 const originals=new Map(),staged=new Set();
 try{
  for(const entry of files){
   const original=io.existsSync(entry.file)?io.readFileSync(entry.file):null;originals.set(entry.file,original);
   if(original!==null)io.writeFileSync(resolve(backup,entry.name),original);
   io.writeFileSync(entry.staged,entry.data,{flag:'wx'});
   staged.add(entry.staged);
  }
  const replaced=[];
  try{for(const entry of files){io.renameSync(entry.staged,entry.file);replaced.push(entry);}}
  catch(error){for(const entry of replaced){const original=originals.get(entry.file);if(original===null)io.unlinkSync(entry.file);else io.writeFileSync(entry.file,original);}throw error;}
 }finally{for(const file of staged)if(io.existsSync(file))io.unlinkSync(file);}
 return backup;
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 const [command,target]=process.argv.slice(2),input=inputs(),report=coverageReport(input);
 if(command==='export'){
  if(!target)throw Error('Pass an output directory');const out=resolve(target);mkdirSync(out,{recursive:true});
  for(const language of Object.keys(input.registry))writeJSON(resolve(out,language+'.json'),reviewPack(language,input));
  writeJSON(resolve(out,'coverage.json'),report);
  writeJSON(resolve(out,'literal-audit.json'),input.literals);
  writeJSON(resolve(out,'runtime-copy-candidates.json'),runtimeCopyCandidates());
  writeFileSync(resolve(out,'README.md'),readFileSync(resolve(root,'docs/development/translation-review.md')));
  const glossaryKeys=['nav.chat','nav.agents','nav.knowledge','nav.models','nav.activity','nav.settings','shell.work','shell.storage','chat.stop','models.cancel','models.delete','settings.computerUse'];
  writeJSON(resolve(out,'glossary.json'),Object.fromEntries(Object.keys(input.registry).map(language=>[language,entriesFor(language,input).filter(entry=>entry.namespace==='core'&&glossaryKeys.includes(entry.key)).map(({key,source,currentText,coverage})=>({key,source,currentText,coverage}))])));
  console.log(`Exported ${Object.keys(input.registry).length} language packs to ${out}`);
 }else if(command==='import'){
  if(!target)throw Error('Pass a reviewed JSON pack');const pack=json(resolve(target));
  const result=importReview(pack,input);
  if(process.argv.includes('--apply')){
   // Validate first, stage every file, retain backups, and roll back a failed write.
   const backup=applyImport(result);
   console.log('Imported '+pack.language+'; previous files saved to '+backup);
  }else console.log('Valid review pack. No files changed; add --apply to import.');
 }else if(command==='audit'||command==='complete'||command==='ready'){
  for(const [language,row] of Object.entries(report))console.log(`${language}: ${row.authored}/${row.total} authored; ${row.missing} missing; ${row.sameAsSource} identical to English; ${row.reviewed} reviewed; ${row.ready?'READY':'NOT READY'}`);
  if(command==='ready'&&Object.values(report).some(row=>!row.ready))process.exitCode=1;
  if(command==='complete'&&Object.values(report).some(row=>row.missing))process.exitCode=1;
 }else throw Error('Usage: translation-review.mjs audit | complete | ready | export DIRECTORY | import PACK [--apply]');
}
