import {readFileSync,readdirSync} from 'node:fs';
import {resolve,relative,join} from 'node:path';
import {createRequire} from 'node:module';
import vm from 'node:vm';
const root=resolve(import.meta.dirname,'../..');
const require=createRequire(resolve(root,'apps/web/package.json'));
const ts=require('typescript');
const source=resolve(root,'apps/web/src');
const walk=dir=>readdirSync(dir,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?walk(join(dir,entry.name)):[join(dir,entry.name)]);
export const flatten=(value,prefix='')=>typeof value==='string'?{[prefix]:value}:Object.fromEntries(Object.entries(value).flatMap(([key,item])=>Object.entries(flatten(item,prefix?`${prefix}.${key}`:key))));
export function isUiLiteral(node){
 if(ts.isJsxText(node))return /\p{Letter}/u.test(node.text.trim());
 const fragment=ts.isTemplateHead(node)||ts.isTemplateMiddle(node)||ts.isTemplateTail(node);
 if(!ts.isStringLiteral(node)&&!ts.isNoSubstitutionTemplateLiteral(node)&&!fragment)return false;
 const attributes=new Set(['aria-label','aria-description','title','placeholder','alt']);
 const expression=parent=>ts.isJsxExpression(parent)&&(!ts.isJsxAttribute(parent.parent)||attributes.has(parent.parent.name.getText()));
 const parent=fragment?(ts.isTemplateSpan(node.parent)?node.parent.parent.parent:node.parent.parent):node.parent,visible=ts.isJsxAttribute(parent)&&attributes.has(parent.name.getText())||expression(parent)
  ||ts.isConditionalExpression(parent)&&(parent.whenTrue===node||parent.whenFalse===node)&&expression(parent.parent);
 return visible&&/\p{Letter}/u.test(node.text)||/^[A-Z][a-z]+ [a-zA-Z ]{4,}/.test(node.text);
}
export function inventory(){
 const tables=[],literals=[];
 for(const file of walk(source).filter(f=>/\.tsx?$/.test(f)&&!f.endsWith('i18n\\index.tsx')&&!f.endsWith('i18n/index.tsx')&&!f.includes('/api/generated'))){
  const text=readFileSync(file,'utf8'),ast=ts.createSourceFile(file,text,ts.ScriptTarget.Latest,true),name=relative(source,file).replaceAll('\\','/');
  let sequence=0;
  const bindings={};
  for(const statement of ast.statements)if(ts.isVariableStatement(statement))for(const declaration of statement.declarationList.declarations){
   if(ts.isIdentifier(declaration.name)&&declaration.initializer){
    try{const js=ts.transpileModule(`result=(${declaration.initializer.getText(ast)})`,{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText;const scope={...bindings};vm.runInNewContext(js,scope,{timeout:1000});bindings[declaration.name.text]=scope.result;}catch{}
   }
  }
  const visit=node=>{
   if(ts.isObjectLiteralExpression(node)){
    const keys=node.properties.filter(p=>ts.isPropertyAssignment(p)||ts.isShorthandPropertyAssignment(p)).map(p=>p.name.getText(ast).replace(/^['"]|['"]$/g,''));
    if(keys.includes('en')&&keys.includes('fr')&&keys.includes('ar')){
     const variable=ts.isVariableDeclaration(node.parent)?node.parent.name.getText(ast):`inline${++sequence}`;
     const wrapped=ts.isCallExpression(node.parent)&&node.parent.expression.getText(ast)==='defineMessages';
     const id=wrapped?node.parent.arguments[0].text:`${name.replace(/\.tsx?$/,'')}:${variable}`;
     try{
      const js=ts.transpileModule(`result=(${node.getText(ast)})`,{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText;
      const scope={...bindings};vm.runInNewContext(js,scope,{timeout:1000});
      tables.push({id,file:name,start:node.getStart(ast),end:node.end,wrapped,values:scope.result});
     }catch(error){tables.push({file:name,error:String(error)});}
     return;
    }
   }
   if(!name.startsWith('i18n/')&&isUiLiteral(node))literals.push({file:name,line:ast.getLineAndCharacterOfPosition(node.getStart(ast)).line+1,text:node.text.trim()});
   ts.forEachChild(node,visit);
  };visit(ast);
 }
 return {tables,literals};
}
export function catalogInventory(){
 const result=inventory(),tables=result.tables;
 const load=(file,key)=>{const scope={exports:{}};const js=ts.transpileModule(readFileSync(resolve(source,file),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText;vm.runInNewContext(js,scope,{timeout:1000});return scope.exports[key];};
 const legacy=['en','fr','es','ar','sw','sn','nd','zu','de'];
 tables.push({id:'core',file:'i18n/locales',values:Object.fromEntries(legacy.map(code=>[code,load(`i18n/locales/${code}.ts`,code)]))});
 tables.push({id:'i18n/catalog-descriptions',file:'i18n/catalog-descriptions.json',sourceOnly:true,values:{en:JSON.parse(readFileSync(resolve(source,'i18n/catalog-descriptions.json'),'utf8'))}});
 tables.push({id:'i18n/computer-experience:copy',file:'i18n/computer-experience.json',values:JSON.parse(readFileSync(resolve(source,'i18n/computer-experience.json'),'utf8'))});
 const voiceSource=readFileSync(resolve(source,'i18n/voice.ts'),'utf8'),voiceAst=ts.createSourceFile('voice.ts',voiceSource,ts.ScriptTarget.Latest,true);
 let english;
 for(const statement of voiceAst.statements)if(ts.isVariableStatement(statement))for(const item of statement.declarationList.declarations)if(item.name.getText(voiceAst)==='english'){
  const scope={};vm.runInNewContext(`result=(${item.initializer.getText(voiceAst)})`,scope);english=scope.result;
 }
 tables.push({id:'i18n/voice:voices',file:'i18n/voice.ts',values:{en:english,...load('i18n/voice-locales.ts','voiceLocales')}});
 tables.push({id:'desktop/presentation',file:'apps/web/public/desktop-presentation.json',values:JSON.parse(readFileSync(resolve(root,'apps/web/public/desktop-presentation.json'),'utf8'))});
 tables.push({id:'desktop/terminal',file:'apps/desktop/terminal-translations.js',values:require(resolve(root,'apps/desktop/terminal-translations.js')).catalogs});
 const {folderTrustCopy}=require(resolve(root,'apps/desktop/folder-trust-copy.js'));
 tables.push({id:'desktop/folder-trust',file:'apps/desktop/folder-trust-copy.js',values:Object.fromEntries(legacy.map(code=>[code,folderTrustCopy(code)]))});
 return {...result,tables};
}
if(process.argv[1]&&resolve(process.argv[1])===resolve(import.meta.filename)){
 const result=inventory();
 console.log(JSON.stringify({tables:result.tables.map(({values,...rest})=>({...rest,entries:values?Object.keys(flatten(values.en)).length:0})),literals:result.literals},null,2));
}
