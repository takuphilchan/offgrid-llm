const {test}=require('node:test');
const assert=require('node:assert/strict');
test('translation quality gate catches omissions, corruption, placeholder changes and hidden bidi',async()=>{
 const {validateTranslation}=await import('../../../scripts/quality/check-translations.mjs');
 const source={status:'Removed {count} files',action:'Stop'};
 assert.equal(validateTranslation(source,{status:'{count} Dateien entfernt',action:'Stoppen'},{complete:true}).errors.length,0);
 assert.match(validateTranslation(source,{status:'Dateien entfernt'}).errors.join(','),/placeholders/);
 assert.match(validateTranslation(source,{status:'{count} Dateien entfernt'},{complete:true}).errors.join(','),/missing translation/);
 assert.match(validateTranslation(source,{action:'\uFFFD'}).errors.join(','),/corrupted/);
 assert.match(validateTranslation(source,{action:'\u202eStop'}).errors.join(','),/direction controls/);
});
test('desktop preserves every registered language and rejects unsupported identifiers',()=>{
 const {normalize,languages}=require('../presentation');
 for(const code of Object.keys(languages))assert.equal(normalize({locale:code}).locale,code);
 assert.equal(normalize({locale:'<script>'}).locale,'en');
 for(const code of ['ur','fa','he','ar'])assert.equal(languages[code].direction,'rtl');
});

test('translation validation preserves executable instructions and does not mistake prose for paths',async()=>{
 const {validateTranslation}=await import('../../../scripts/quality/check-translations.mjs');
 const base={host:'Start with offgrid agent workspace start FOLDER, then refresh.',path:'Commands use /workspace',prose:'Resets on account/workspace change.'};
 const valid={host:'Exécutez offgrid agent workspace start FOLDER, puis actualisez.',path:'Les commandes utilisent /workspace',prose:'Réinitialisé au changement de compte ou d’espace.'};
 assert.deepEqual(validateTranslation(base,valid).errors,[]);
 assert.match(validateTranslation(base,{...valid,host:'Exécutez offgrid agent espace démarrer DOSSIER.'}).errors.join(','),/changed command/);
 assert.match(validateTranslation(base,{...valid,path:'Les commandes utilisent /espace'}).errors.join(','),/changed command/);
});

test('all 29 terminal languages retain every label and interpolation token',async()=>{
 const {catalogs,text}=require('../terminal-translations');
 const {overrides}=require('../locale-messages');
 const registry=require('../language-registry.json');
 const {validateTranslation}=await import('../../../scripts/quality/check-translations.mjs');
 for(const locale of Object.keys(registry))assert.deepEqual(validateTranslation(catalogs.en,{...catalogs[locale],...overrides[locale]?.['desktop/terminal']},{complete:true}).errors,[],locale);
 assert.equal(text('fr','Close {title}',{title:'user <script> $&.txt'}),'Fermer user <script> $&.txt');
 assert.equal(text('unknown','Close {title}',{title:'shell'}),'Close shell');
 assert.equal(text('ar','No matches'),'لا توجد تطابقات');
});

test('feature quality gate rejects stale sources and removal of translated coverage',async()=>{
 const {validateFeaturePack}=await import('../../../scripts/quality/check-translations.mjs');
 const original={settings:{save:'Save',count:'{count} files'}},translated={settings:{save:'Enregistrer',count:'{count} fichiers'}};
 assert.deepEqual(validateFeaturePack(original,translated,original),[]);
 assert.match(validateFeaturePack(original,{settings:{save:'Enregistrer'}},original).join(','),/coverage regressed/);
 assert.match(validateFeaturePack({settings:{...original.settings,save:'Save changes'}},translated,original).join(','),/source changed/);
 assert.match(validateFeaturePack(original,{settings:{...translated.settings,count:'Fichiers'}},original).join(','),/placeholders/);
});
