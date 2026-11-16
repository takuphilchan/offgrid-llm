import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const labels=defineMessages<string[]>('i18n/ocr-progress:labels',{
 en:['OCR pages saved: {count}. Retry reuses saved pages.'],
 fr:['Pages OCR enregistrées : {count}. Une nouvelle tentative réutilise les pages enregistrées.'],
 de:['Gespeicherte OCR-Seiten: {count}. Beim erneuten Versuch werden sie wiederverwendet.'],
 es:['Páginas de OCR guardadas: {count}. Al reintentar, se reutilizan las páginas guardadas.'],
 ar:['صفحات التعرف على النص المحفوظة: {count}. تستخدم إعادة المحاولة الصفحات المحفوظة.'],
 sw:['Kurasa za OCR zilizohifadhiwa: {count}. Ukijaribu tena, kurasa zilizohifadhiwa zitatumika.'],
 sn:['Mapeji eOCR akachengetwa: {count}. Ukayedza zvakare, mapeji akachengetwa anoshandiswazve.'],
 nd:['Amakhasi e-OCR agciniweyo: {count}. Ukuzama futhi kusebenzisa amakhasi agciniweyo.'],
 zu:['Amakhasi e-OCR agciniwe: {count}. Ukuzama futhi kusebenzisa amakhasi agciniwe.'],
});
export function ocrProgress(locale:LocaleCode,count:number){return labels[locale][0].replace('{count}',new Intl.NumberFormat(locale).format(count));}
