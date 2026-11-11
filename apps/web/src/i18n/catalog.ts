import {languageRegistry,type LanguageCode,type DictionaryLocale} from './language';
import additions from './feature-packs.json' with {type:'json'};

type Catalog = Record<string,Record<string,Record<string,string>>>;
const packs:Catalog=additions;
export const featureTranslations=(language:LanguageCode,namespace:string)=>packs[language]?.[namespace];

/** Resolve authored messages only. Never translate user content, paths or arbitrary API data. */
export function resolveMessages<T>(base:T,translations:Record<string,string>|undefined,path=''):T {
 if(!translations)return base;
 if(typeof base==='string')return (Object.hasOwn(translations,path)?translations[path]:base) as T;
 if(Array.isArray(base))return base.map((value,index)=>resolveMessages(value,translations,path?`${path}.${index}`:String(index))) as T;
 if(base&&typeof base==='object')return Object.fromEntries(Object.entries(base).map(([key,value])=>[key,resolveMessages(value,translations,path?`${path}.${key}`:key)])) as T;
 return base;
}

/** Every feature uses the selected language; fallback happens per message. */
export function defineMessages<T>(namespace:string,legacy:Record<DictionaryLocale,T>):Record<LanguageCode,T> {
 return Object.fromEntries(Object.entries(languageRegistry).map(([language,definition])=>[
  language,resolveMessages(legacy[definition.fallback as DictionaryLocale],packs[language]?.[namespace])
 ])) as Record<LanguageCode,T>;
}
