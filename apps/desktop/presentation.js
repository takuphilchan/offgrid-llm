'use strict';
const fs = require('node:fs');
const path = require('node:path');
// Appearance assets must never make an otherwise usable desktop fail to start.
// The packaging verifier rejects missing assets; this fallback also handles
// partial/manual upgrades and damaged installations at runtime.
function loadAssets(directory = __dirname) {
  const read = (name, fallback, valid) => {
    try { const data=JSON.parse(fs.readFileSync(path.join(directory,name),'utf8'));if(!valid(data))throw Error('Invalid '+name);return data; }
    catch { return fallback; }
  };
  const legacy=['en','fr','es','ar','sw','sn','nd','zu','de'];
  const languages=read('language-registry.json',Object.fromEntries(legacy.map(code=>[code,{fallback:code,direction:code==='ar'?'rtl':'ltr',stage:'existing'}])),
    data=>data&&typeof data==='object'&&data.en&&Object.values(data).every(item=>item&&legacy.includes(item.fallback)&&['ltr','rtl'].includes(item.direction)));
  const catalog=read('theme-palettes.json',{themes:{},terminal:{}},data=>data?.version===1&&data.themes?.offgrid?.light&&data.themes?.offgrid?.dark&&data.terminal&&typeof data.terminal==='object');
  return {catalog,languages};
}
const {catalog,languages} = loadAssets();
const locales = Object.keys(languages);
function normalizeAppearance(value) {
  if (!value || value.version !== 1) return undefined;
  const valid=(id,mode)=>typeof id==='string'&&Object.hasOwn(catalog.themes,id)&&catalog.themes[id][mode];
  return {version:1,light:valid(value.light,'light')?value.light:'offgrid',dark:valid(value.dark,'dark')?value.dark:'offgrid',textSize:[100,110,125,150].includes(value.textSize)?value.textSize:100,reduceMotion:value.reduceMotion===true};
}
function normalize(value, fallback = 'en') {
  return { ...(normalizeAppearance(value?.appearance) ? {appearance:normalizeAppearance(value.appearance)} : {}), locale: locales.includes(value?.locale) ? value.locale : locales.includes(fallback) ? fallback : 'en', theme: ['light', 'dark', 'system'].includes(value?.theme) ? value.theme : 'system' };
}
function readCopy(uiDir) {
  return JSON.parse(fs.readFileSync(path.join(uiDir, 'desktop-presentation.json'), 'utf8'));
}
module.exports = { normalize, readCopy, languages, catalog, loadAssets };
