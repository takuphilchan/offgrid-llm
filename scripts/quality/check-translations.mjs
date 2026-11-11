import {readFileSync, readdirSync, existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {resolve, dirname} from 'node:path';
import {createRequire} from 'node:module';
import vm from 'node:vm';
import {catalogInventory,flatten as flattenCatalog} from './translation-inventory.mjs';
import {validateLanguageScript} from './language-script-check.mjs';
import {validateUiLiterals} from './ui-literal-check.mjs';

const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const require=createRequire(resolve(root,'apps/web/package.json'));
const ts=require('typescript');
const source=resolve(root,'apps/web/src/i18n');
const load=code=>{
 const js=ts.transpileModule(readFileSync(resolve(source,'locales',code+'.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText;
 const scope={exports:{}};vm.runInNewContext(js,scope,{timeout:1000});return scope.exports[code];
};
const flatten=flattenCatalog;
const placeholders=text=>[...text.matchAll(/\{([A-Za-z][\w]*)\}/g)].map(m=>m[1]).sort().join(',');
export function protectedLiterals(text){
 const commands=['offgrid agent workspace start FOLDER'].filter(command=>text.includes(command));
 const paths=/(?<![\w/])\/workspace\b/.test(text)?['/workspace']:[];
 return [...new Set([...commands,...paths,...[...text.matchAll(/https?:\/\/[^\s<>)]*/g)].map(match=>match[0].replace(/[.,;]+$/,''))])];
}
export function validateTranslation(base,translation,{complete=false,language}={}) {
 const errors=[],original=flatten(base),translated=flatten(translation);
 for(const [key,value] of Object.entries(translated)){
  if(!(key in original)){errors.push(key+': unknown message');continue;}
  if(!value.trim())errors.push(key+': empty translation');
  if(placeholders(value)!==placeholders(original[key]))errors.push(key+': changed placeholders');
  for(const literal of protectedLiterals(original[key]))if(!value.includes(literal))errors.push(key+': changed command, path or URL: '+literal);
  if(/[\uFFFD\u0080-\u009f]|Ã[\u00a0-\u00bf]|â€|Ø[\u00a0-\u00bf]/u.test(value))errors.push(key+': possible corrupted encoding');
  if(/[\u202a-\u202e\u2066-\u2069]/u.test(value))errors.push(key+': use markup instead of hidden direction controls');
  errors.push(...validateLanguageScript(original[key],value,language).map(error=>key+': '+error));
 }
 if(complete)for(const key of Object.keys(original))if(!(key in translated))errors.push(key+': missing translation');
 return {errors,translated:Object.keys(translated).length,total:Object.keys(original).length};
}

export function validateFeaturePack(catalogs,pack,sources,language){
 const errors=[];
 for(const [namespace,entries] of Object.entries(pack)){
  const base=catalogs[namespace];
  if(!base){errors.push(namespace+': unknown catalog');continue;}
  for(const [key,value] of Object.entries(entries)){
   if(!Object.hasOwn(base,key)){errors.push(`${namespace}.${key}: unknown message`);continue;}
   if(sources[namespace]?.[key]!==base[key])errors.push(`${namespace}.${key}: source changed or untracked; translation review required`);
   errors.push(...validateTranslation({message:base[key]},{message:value},{language}).errors.map(e=>`${namespace}.${key}: ${e}`));
  }
 }
 for(const [namespace,entries] of Object.entries(sources))for(const key of Object.keys(entries))if(!Object.hasOwn(pack[namespace]??{},key))errors.push(`${namespace}.${key}: translated coverage regressed`);
 return errors;
}

export function checkTranslations(){
 const english=load('en'),failures=[],report=[];
 let scanned=0;
 for(const file of readdirSync(source).filter(name=>/\.tsx?$/.test(name))){
  const ast=ts.createSourceFile(file,readFileSync(resolve(source,file),'utf8'),ts.ScriptTarget.Latest,true);
  const visit=node=>{
   if(ts.isStringLiteral(node)||ts.isNoSubstitutionTemplateLiteral(node)){
    scanned++;
    if(/[\uFFFD\u0080-\u009f]|\u00c3[\u00a0-\u00bf]|\u00e2\u20ac|\u00d8[\u00a0-\u00bf]/u.test(node.text))failures.push(file+': possible corrupted translation literal');
   }
   ts.forEachChild(node,visit);
  };visit(ast);
 }
 report.push(`${scanned} supplemental source literals checked for encoding corruption`);
 for(const file of readdirSync(resolve(source,'locales')).filter(f=>f.endsWith('.ts'))){
  const code=file.slice(0,-3),result=validateTranslation(english,load(code),{complete:true,language:code});
  failures.push(...result.errors.map(e=>code+': '+e));report.push(`${code}: ${result.translated}/${result.total} core entries (linguistic quality is not certified by this check)`);
 }
 const packs=JSON.parse(readFileSync(resolve(source,'language-packs.json'),'utf8'));
 const features=JSON.parse(readFileSync(resolve(source,'feature-packs.json'),'utf8'));
 const sources=JSON.parse(readFileSync(resolve(source,'feature-sources.json'),'utf8'));
 const languageSources=existsSync(resolve(source,'translation-sources.json'))?JSON.parse(readFileSync(resolve(source,'translation-sources.json'),'utf8')):{};
 const inventory=catalogInventory(),catalogs=inventory.tables;
 failures.push(...validateUiLiterals(inventory.literals,JSON.parse(readFileSync(resolve(root,'scripts/quality/ui-literal-allowlist.json'),'utf8'))));
 for(const table of catalogs){
  if(table.error){failures.push(table.file+': '+table.error);continue;}
  if(table.wrapped===false)failures.push(table.id+': contextual catalog must use defineMessages to resolve every selected language');
  for(const code of ['fr','es','ar','sw','sn','nd','zu','de']){
   if(table.sourceOnly){
    const result=validateTranslation(table.values.en,features[code]?.[table.id]??{},{complete:true,language:code});
    failures.push(...result.errors.map(error=>`${code} ${table.id}: ${error}`));continue;
   }
   if(!table.values[code]){failures.push(`${table.id}: missing ${code} catalog`);continue;}
   const result=validateTranslation(table.values.en,table.values[code],{complete:true,language:code});
   failures.push(...result.errors.map(error=>`${code} ${table.id}: ${error}`));
  }
 }
 const total=catalogs.reduce((n,table)=>n+Object.keys(flatten(table.values?.en??{})).length,0);
 report.push(`${catalogs.length} contextual catalogs / ${total} authored messages checked across existing languages`);
 const registry=require(resolve(root,'apps/desktop/language-registry.json'));
 const sourceCatalogs=Object.fromEntries(catalogs.filter(c=>c.values).map(c=>[c.id,flatten(c.values.en)]));
 for(const [code,entries] of Object.entries(languageSources)){
  if(!Object.hasOwn(registry,code)){failures.push(code+': unregistered translation language');continue;}
  if(!Object.hasOwn(packs,code))failures.push(...validateFeaturePack(sourceCatalogs,features[code]??{},entries,code).map(e=>code+': '+e));
 }
 for(const [code,pack] of Object.entries(packs)){
  const result=validateTranslation(english,pack.messages,{language:code});
  failures.push(...result.errors.map(e=>code+': '+e));
  if(Intl.getCanonicalLocales(code)[0]!==code)failures.push(code+': noncanonical language tag');
  if(!pack.notice?.trim()||pack.direction!==registry[code]?.direction)failures.push(code+': missing notice or invalid direction');
  failures.push(...validateLanguageScript('Translation preview. Native-language review is pending.',pack.notice,code).map(error=>code+': preview notice: '+error));
  if(pack.stage!=='preview'||registry[code]?.stage!=='preview'||pack.review.status!=='pending')failures.push(code+': promotion requires complete feature coverage and recorded language review; update the release gate deliberately');
  const tracked=Object.fromEntries([...new Set([...Object.keys(sources),...Object.keys(languageSources[code]??{})])].map(namespace=>[namespace,{...sources[namespace],...languageSources[code]?.[namespace]}]));
  failures.push(...validateFeaturePack(sourceCatalogs,features[code]??{},tracked,code).map(e=>code+': '+e));
  const covered=new Set(Object.keys(flatten(pack.messages)).map(key=>'core/'+key));
  for(const table of catalogs)if(table.values?.[code]){
   const result=validateTranslation(table.values.en,table.values[code],{language:code});
   failures.push(...result.errors.map(error=>`${code} ${table.id}: ${error}`));
   for(const key of Object.keys(flatten(table.values[code])))covered.add(table.id+'/'+key);
  }
  for(const [namespace,entries] of Object.entries(features[code]??{}))for(const key of Object.keys(entries))covered.add(namespace+'/'+key);
  report.push(`${code}: PREVIEW ${covered.size}/${total} app messages; missing ${total-covered.size}; ${covered.size===total?'no missing catalog copy':'English fallback for missing copy'}; native review pending`);
 }
 return {failures,report};
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 const {failures,report}=checkTranslations();console.log(report.join('\n'));
 if(failures.length){console.error(failures.join('\n'));process.exitCode=1;}
}
