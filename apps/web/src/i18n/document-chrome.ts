import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const labels=defineMessages<string[]>('i18n/document-chrome:labels',{
  "en": [
    "Expand document",
    "Restore document view"
  ],
  "fr": [
    "Agrandir le document",
    "Rétablir la vue du document"
  ],
  "es": [
    "Ampliar documento",
    "Restaurar vista del documento"
  ],
  "de": [
    "Dokument vergrößern",
    "Dokumentansicht wiederherstellen"
  ],
  "ar": [
    "توسيع المستند",
    "استعادة عرض المستند"
  ],
  "sw": [
    "Panua hati",
    "Rejesha mwonekano wa hati"
  ],
  "sn": [
    "Kurisa gwaro",
    "Dzorera maonero egwaro"
  ],
  "nd": [
    "Khulisa umbhalo",
    "Buyisela ukubonakala kombhalo"
  ],
  "zu": [
    "Khulisa idokhumenti",
    "Buyisela ukubuka idokhumenti"
  ]
});
export const documentChrome=(locale:LocaleCode)=>({expand:labels[locale][0],restore:labels[locale][1]});
