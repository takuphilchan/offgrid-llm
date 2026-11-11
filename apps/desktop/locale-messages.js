'use strict';
const fs=require('node:fs');
const path=require('node:path');

function readOverrides(directory=__dirname){
 const packaged=path.join(directory,'locale-overrides','feature-packs.json');
 const development=path.join(directory,'../web/src/i18n/feature-packs.json');
 try{return JSON.parse(fs.readFileSync(fs.existsSync(packaged)?packaged:development,'utf8'));}
 catch{return {};}
}
const overrides=readOverrides();
function resolveMessages(base,additions,prefix=''){
 if(typeof base==='string')return typeof additions?.[prefix]==='string'&&additions[prefix].trim()?additions[prefix]:base;
 if(Array.isArray(base))return base.map((value,index)=>resolveMessages(value,additions,prefix?`${prefix}.${index}`:String(index)));
 if(base&&typeof base==='object')return Object.fromEntries(Object.entries(base).map(([key,value])=>[key,resolveMessages(value,additions,prefix?`${prefix}.${key}`:key)]));
 return base;
}
function localizedCopy(catalogs,locale,namespace,additions=overrides){
 return resolveMessages({...catalogs.en,...catalogs[locale]},additions[locale]?.[namespace]);
}
module.exports={readOverrides,resolveMessages,localizedCopy,overrides};
