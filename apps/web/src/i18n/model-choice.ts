import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
export const modelChoiceText = defineMessages<{ filter: string; empty: string }>("i18n/model-choice:modelChoiceText", {
  en: { filter: 'Filter model choices', empty:'No matches' }, fr: { filter: 'Filtrer les choix de modèles', empty:'Aucune correspondance' }, de: { filter: 'Modellauswahl filtern', empty:'Keine Treffer' }, es: { filter: 'Filtrar opciones de modelos', empty:'Sin coincidencias' }, ar: { filter: 'تصفية خيارات النماذج', empty:'لا توجد تطابقات' }, sw: { filter: 'Chuja chaguo za modeli', empty:'Hakuna zinazolingana' }, sn: { filter: 'Sefa sarudzo dzemamodheru', empty:'Hapana zvinowirirana' }, nd: { filter: 'Hlunga izinketho zamamodeli', empty:'Akukho okufanayo' }, zu: { filter: 'Hlunga izinketho zamamodeli', empty:'Akukho okufanayo' },
});
