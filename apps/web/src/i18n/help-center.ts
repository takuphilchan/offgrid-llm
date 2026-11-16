import { defineMessages } from './catalog';
// Translation drafts; native-language approval remains in the reviewer packs.
export const helpCenter = defineMessages<{search:string;empty:string;browse:string;topics:string}>('i18n/help-center:helpCenter', {
  "en": {
    "search": "Search help",
    "empty": "No matching help topics. Try a different word.",
    "browse": "Browse help topics",
    "topics": "Topics"
  },
  "fr": {
    "search": "Rechercher dans l’aide",
    "empty": "Aucun sujet trouvé. Essayez un autre mot.",
    "browse": "Parcourir les rubriques d’aide",
    "topics": "Rubriques"
  },
  "es": {
    "search": "Buscar en la ayuda",
    "empty": "No se encontraron temas. Prueba otra palabra.",
    "browse": "Explorar los temas de ayuda",
    "topics": "Temas"
  },
  "de": {
    "search": "Hilfe durchsuchen",
    "empty": "Keine passenden Hilfethemen. Versuche ein anderes Wort.",
    "browse": "Hilfethemen durchsuchen",
    "topics": "Themen"
  },
  "ar": {
    "search": "البحث في المساعدة",
    "empty": "لا توجد مواضيع مساعدة مطابقة. جرّب كلمة أخرى.",
    "browse": "تصفح مواضيع المساعدة",
    "topics": "المواضيع"
  },
  "sw": {
    "search": "Tafuta katika msaada",
    "empty": "Hakuna mada za msaada zinazolingana. Jaribu neno jingine.",
    "browse": "Vinjari mada za msaada",
    "topics": "Mada"
  },
  "sn": {
    "search": "Tsvaga rubatsiro",
    "empty": "Hapana misoro yerubatsiro inoenderana. Edza rimwe shoko.",
    "browse": "Tarisa misoro yerubatsiro",
    "topics": "Misoro"
  },
  "nd": {
    "search": "Dinga uncedo",
    "empty": "Akulazihloko zoncedo ezihambelanayo. Zama elinye ibala.",
    "browse": "Khangela izihloko zoncedo",
    "topics": "Izihloko"
  },
  "zu": {
    "search": "Sesha usizo",
    "empty": "Azikho izihloko zosizo ezihambisanayo. Zama elinye igama.",
    "browse": "Hlola izihloko zosizo",
    "topics": "Izihloko"
  }
});
