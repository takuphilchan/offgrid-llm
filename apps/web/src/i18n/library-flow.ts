import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const copy=defineMessages('i18n/library-flow:copy',{
  "en": {
    "changed": "The selection has changed. Compare again to update the results."
  },
  "fr": {
    "changed": "La sélection a changé. Relancez la comparaison pour actualiser les résultats."
  },
  "de": {
    "changed": "Die Auswahl wurde geändert. Vergleichen Sie erneut, um die Ergebnisse zu aktualisieren."
  },
  "es": {
    "changed": "La selección ha cambiado. Compara de nuevo para actualizar los resultados."
  },
  "ar": {
    "changed": "تغيّر الاختيار. أعد المقارنة لتحديث النتائج."
  },
  "sw": {
    "changed": "Uteuzi umebadilika. Linganisha tena ili kusasisha matokeo."
  },
  "sn": {
    "changed": "Zvasarudzwa zvachinja. Enzanisa zvakare kuti uvandudze zvabuda."
  },
  "nd": {
    "changed": "Okukhethiweyo sekutshintshile. Qathanisa njalo ukuze uvuselele impumela."
  },
  "zu": {
    "changed": "Okukhethiwe kushintshile. Qhathanisa futhi ukuze ubuyekeze imiphumela."
  }
});
export const libraryFlow=(locale:LocaleCode)=>copy[locale];
