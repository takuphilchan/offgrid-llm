import {defineMessages} from './catalog';
import {resolveLanguage} from './language';
const copy=defineMessages('i18n/collection-errors:copy',{
  "en": {
    "unconfirmed": "Collection details could not be confirmed. Reload its audience and revision before continuing."
  },
  "fr": {
    "unconfirmed": "Les informations de la collection n’ont pas pu être confirmées. Rechargez les personnes autorisées et la révision avant de continuer."
  },
  "de": {
    "unconfirmed": "Die Sammlungsdaten konnten nicht bestätigt werden. Lade die Zugriffsberechtigten und die Revision neu, bevor du fortfährst."
  },
  "es": {
    "unconfirmed": "No se pudieron confirmar los datos de la colección. Vuelve a cargar quién tiene acceso y la revisión antes de continuar."
  },
  "ar": {
    "unconfirmed": "تعذّر تأكيد تفاصيل المجموعة. أعد تحميل قائمة من لديهم حق الوصول ورقم المراجعة قبل المتابعة."
  },
  "sw": {
    "unconfirmed": "Maelezo ya mkusanyiko hayakuweza kuthibitishwa. Pakia tena orodha ya wenye ruhusa ya kufikia na toleo kabla ya kuendelea."
  },
  "sn": {
    "unconfirmed": "Mashoko emuunganidzwa haana kukwanisa kusimbiswa. Torazve rondedzero yevanobvumirwa kuuwana neshanduro usati waenderera mberi."
  },
  "nd": {
    "unconfirmed": "Imininingwane yeqoqo ayiqinisekiswanga. Layitsha kutsha uhlu lwabavunyelwe ukufinyelela lenombolo yenguqulo ungakaqhubeki."
  },
  "zu": {
    "unconfirmed": "Imininingwane yeqoqo ayikwazanga ukuqinisekiswa. Layisha kabusha uhlu lwabavunyelwe ukufinyelela nenombolo yenguqulo ngaphambi kokuqhubeka."
  }
});
export function collectionErrorMessage(){
 const locale=typeof document==='undefined'?'en':resolveLanguage(document.documentElement.lang)??'en';
 return copy[locale].unconfirmed;
}
