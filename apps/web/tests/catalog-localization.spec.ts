import {test,expect} from '@playwright/test';
import {catalogDescription} from '../src/i18n/catalog-descriptions';
import source from '../src/i18n/catalog-descriptions.json' with {type:'json'};
import registry from '../../desktop/language-registry.json' with {type:'json'};
import type {LanguageCode} from '../src/i18n/language';
import {formatBytes} from '../src/features/models/model-format';

test('curated descriptions use authored UI language while preserving external copy',()=>{
 for(const language of Object.keys(registry) as LanguageCode[]){
  for(const [id,text] of Object.entries(source)){
   const modelID=id==='speechPackage'?'model-32c96ae87d3e':id;
   const translated=catalogDescription(language,modelID,text);
   expect(translated).toBeTruthy();
   if(language!=='en')expect(translated,language+' '+id).not.toBe(text);
   expect(catalogDescription(language,modelID,'Custom description')).toBe('Custom description');
   expect(catalogDescription(language,'third-party-id',text)).toBe(text);
  }
  expect(catalogDescription(language,'unknown',undefined)).toBe('');
 }
});

test('model quantities follow the selected UI language, including decimal separators and digits',()=>{
 expect(formatBytes(1280835840,'de')).toBe('1,2 GB');
 expect(formatBytes(1280835840,'en')).toBe('1.2 GB');
 for(const language of Object.keys(registry)){
  expect(formatBytes(1280835840,language)).toBe(new Intl.NumberFormat(language,{minimumFractionDigits:1,maximumFractionDigits:1}).format(1280835840/1024**3)+' GB');
  expect(formatBytes(Number.NaN,language)).toBe(new Intl.NumberFormat(language).format(0)+' B');
 }
});
