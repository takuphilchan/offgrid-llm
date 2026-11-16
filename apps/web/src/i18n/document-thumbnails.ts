import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const labels=defineMessages<string[]>('i18n/document-thumbnails:labels',{
 en:['Pages'],fr:['Pages'],de:['Seiten'],es:['Páginas'],ar:['الصفحات'],sw:['Kurasa'],sn:['Mapeji'],nd:['Amakhasi'],zu:['Amakhasi'],
});
export const documentThumbnails=(locale:LocaleCode)=>({pages:labels[locale][0]});
