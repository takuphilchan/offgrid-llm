// Build-time check of the copied archive, not the source manifest alone.
const fs=require('node:fs');
const path=require('node:path');
const required=['theme-palettes.json','language-registry.json','theme-NOTICE.txt','select-controls.css','terminal-translations.js','terminal-accessibility.js','locale-messages.js'];
const mapped={'locale-overrides/feature-packs.json':'../web/src/i18n/feature-packs.json'};
function verify(resources, archive=require('@electron/asar')) {
 const file=path.join(resources,'app.asar');
 for(const name of [...required,...Object.keys(mapped)]){
  let data;
  try{data=archive.extractFile(file,name);}catch{throw Error('Desktop presentation asset missing from app.asar: '+name);}
  if(!data.length)throw Error('Desktop presentation asset is empty: '+name);
  if(!data.equals(fs.readFileSync(path.join(__dirname,mapped[name]??name))))throw Error('Desktop presentation asset differs from source: '+name);
 }
}
module.exports={verify,required,mapped};
