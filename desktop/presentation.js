'use strict';
const fs = require('node:fs');
const path = require('node:path');
const locales = ['en', 'fr', 'es', 'ar', 'sw', 'sn', 'nd', 'zu', 'de'];
function normalize(value, fallback = 'en') {
  return { locale: locales.includes(value?.locale) ? value.locale : locales.includes(fallback) ? fallback : 'en', theme: ['light', 'dark', 'system'].includes(value?.theme) ? value.theme : 'system' };
}
function readCopy(uiDir) {
  return JSON.parse(fs.readFileSync(path.join(uiDir, 'desktop-presentation.json'), 'utf8'));
}
module.exports = { normalize, readCopy };
