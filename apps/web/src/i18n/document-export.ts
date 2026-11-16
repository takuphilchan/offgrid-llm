import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const labels=defineMessages<string[]>('i18n/document-export:labels',{
  "en": [
    "Export comparison",
    "Downloaded copies stay on this device even if source access is later removed."
  ],
  "fr": [
    "Exporter la comparaison",
    "Les copies téléchargées restent sur cet appareil même si l’accès aux sources est ensuite supprimé."
  ],
  "es": [
    "Exportar comparación",
    "Las copias descargadas permanecen en este dispositivo aunque después se retire el acceso a las fuentes."
  ],
  "de": [
    "Vergleich exportieren",
    "Heruntergeladene Kopien bleiben auf diesem Gerät, auch wenn der Zugriff auf die Quellen später entzogen wird."
  ],
  "ar": [
    "تصدير المقارنة",
    "تبقى النسخ التي تم تنزيلها على هذا الجهاز حتى إذا أُلغي الوصول إلى المصادر لاحقًا."
  ],
  "sw": [
    "Hamisha ulinganisho",
    "Nakala zilizopakuliwa hubaki kwenye kifaa hiki hata ruhusa ya kufikia vyanzo ikiondolewa baadaye."
  ],
  "sn": [
    "Dhawunirodha kuenzanisa",
    "Makopi akadhawunirodhwa anoramba ari pamudziyo uyu kunyange mvumo yekuwana magwaro ikazobviswa."
  ],
  "nd": [
    "Landa ukuqhathanisa",
    "Amakhophi alandiweyo ahlala kule idivayisi lanxa imvumo yokufinyelela imithombo ingasuswa ngemva kwesikhathi."
  ],
  "zu": [
    "Landa ukuqhathanisa",
    "Amakhophi alandiwe ahlala kule divayisi ngisho noma imvume yokufinyelela imithombo isuswa kamuva."
  ]
});
export const documentExport=(locale:LocaleCode)=>({title:labels[locale][0],retention:labels[locale][1]});
