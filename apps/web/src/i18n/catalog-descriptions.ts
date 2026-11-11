import source from './catalog-descriptions.json' with {type:'json'};
import {featureTranslations} from './catalog';
import type {LanguageCode} from './language';

const descriptions:Record<string,string>=source;
const speechPackages=new Set(['model-32c96ae87d3e','en_us-lessac-medium-561cb333873c','ggml-tiny.en-f56b5b8014a2','encoder-epoch-99-avg-1-chunk-16-left-128.int8-7df0f9c4c838']);

/** Localize only an exact built-in ID/source pair. Preserve custom/service copy. */
export function catalogDescription(language:LanguageCode,id:string,description:string|undefined):string {
 const key=speechPackages.has(id)?'speechPackage':id;
 if(!description||!Object.hasOwn(descriptions,key)||description!==descriptions[key])return description??'';
 return featureTranslations(language,'i18n/catalog-descriptions')?.[key]??description;
}
