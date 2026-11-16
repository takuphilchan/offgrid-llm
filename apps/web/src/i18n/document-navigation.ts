import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const copy=defineMessages('i18n/document-navigation:copy',{
  "en": {
    "outline": "Outline",
    "coverage": "Extraction coverage",
    "needsReview": "Needs review",
    "figures": "Figures not interpreted",
    "hidden": "Hidden content",
    "blocks": "Blocks",
    "sideBySide": "Side by side",
    "empty": "No outline entries in this section.",
    "unavailable": "Navigation is unavailable for this extraction."
  },
  "fr": {
    "outline": "Plan",
    "coverage": "Couverture de l’extraction",
    "needsReview": "À vérifier",
    "figures": "Figures non interprétées",
    "hidden": "Contenu masqué",
    "blocks": "Blocs",
    "sideBySide": "Côte à côte",
    "empty": "Aucune entrée de plan dans cette section.",
    "unavailable": "La navigation est indisponible pour cette extraction."
  },
  "de": {
    "outline": "Gliederung",
    "coverage": "Umfang der Extraktion",
    "needsReview": "Prüfung erforderlich",
    "figures": "Abbildungen nicht interpretiert",
    "hidden": "Ausgeblendeter Inhalt",
    "blocks": "Blöcke",
    "sideBySide": "Nebeneinander",
    "empty": "Keine Gliederungseinträge in diesem Abschnitt.",
    "unavailable": "Für diese Extraktion ist keine Navigation verfügbar."
  },
  "es": {
    "outline": "Esquema",
    "coverage": "Cobertura de extracción",
    "needsReview": "Requiere revisión",
    "figures": "Figuras sin interpretar",
    "hidden": "Contenido oculto",
    "blocks": "Bloques",
    "sideBySide": "En paralelo",
    "empty": "No hay entradas de esquema en esta sección.",
    "unavailable": "La navegación no está disponible para esta extracción."
  },
  "ar": {
    "outline": "المخطط",
    "coverage": "تغطية الاستخراج",
    "needsReview": "بحاجة إلى مراجعة",
    "figures": "أشكال لم تُفسَّر",
    "hidden": "محتوى مخفي",
    "blocks": "كتل",
    "sideBySide": "جنبًا إلى جنب",
    "empty": "لا توجد عناصر مخطط في هذا القسم.",
    "unavailable": "التنقل غير متاح لهذا الاستخراج."
  },
  "sw": {
    "outline": "Muhtasari wa sehemu",
    "coverage": "Upeo wa uchimbaji",
    "needsReview": "Inahitaji kukaguliwa",
    "figures": "Vielelezo havijafasiriwa",
    "hidden": "Maudhui yaliyofichwa",
    "blocks": "Vipande",
    "sideBySide": "Upande kwa upande",
    "empty": "Hakuna vipengee vya muhtasari katika sehemu hii.",
    "unavailable": "Urambazaji haupatikani kwa uchimbaji huu."
  },
  "sn": {
    "outline": "Rondedzero yezvikamu",
    "coverage": "Zvakafukidzwa pakuburitsa mashoko",
    "needsReview": "Zvinoda kuongororwa",
    "figures": "Mifananidzo haina kududzirwa",
    "hidden": "Zvakavanzwa",
    "blocks": "Zvikamu zvemashoko",
    "sideBySide": "Padivi nepadivi",
    "empty": "Hapana zvikamu zverondedzero muchikamu chino.",
    "unavailable": "Kufamba pakati pezvikamu hakuwanikwi pamashoko aya akaburitswa."
  },
  "nd": {
    "outline": "Uhlelo lwezigaba",
    "coverage": "Okufinyelelwe yikukhitshwa kombhalo",
    "needsReview": "Kudinga ukuhlolwa",
    "figures": "Imifanekiso ayichazwanga",
    "hidden": "Okuqukethwe okufihliweyo",
    "blocks": "Iziqephu",
    "sideBySide": "Eceleni kweceleni",
    "empty": "Akulazigaba zohlelo kulesi sigaba.",
    "unavailable": "Ukuhamba phakathi kwezigaba akutholakali kulo umbhalo okhitshiweyo."
  },
  "zu": {
    "outline": "Uhlaka lwezigaba",
    "coverage": "Okufinyelelwe ukukhishwa kombhalo",
    "needsReview": "Kudinga ukubuyekezwa",
    "figures": "Imifanekiso ayichazwanga",
    "hidden": "Okuqukethwe okufihliwe",
    "blocks": "Izingxenye",
    "sideBySide": "Eceleni kweceleni",
    "empty": "Azikho izigaba zohlaka kulesi sigaba.",
    "unavailable": "Ukuzulazula akutholakali kulo mbhalo okhishiwe."
  }
});
export const documentNavigation=(locale:LocaleCode)=>copy[locale];
