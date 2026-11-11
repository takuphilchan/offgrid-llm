import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

export const knowledgeUnitLabels = defineMessages<Record<'page' | 'slide' | 'sheet' | 'section', string>>("i18n/knowledge-locators:knowledgeUnitLabels", {
  en: { page: 'Page', slide: 'Slide', sheet: 'Sheet', section: 'Section' },
  fr: { page: 'Page', slide: 'Diapositive', sheet: 'Feuille', section: 'Section' },
  de: { page: 'Seite', slide: 'Folie', sheet: 'Tabellenblatt', section: 'Abschnitt' },
  es: { page: 'Página', slide: 'Diapositiva', sheet: 'Hoja', section: 'Sección' },
  ar: { page: 'صفحة', slide: 'شريحة', sheet: 'ورقة', section: 'قسم' },
  sw: { page: 'Ukurasa', slide: 'Slaidi', sheet: 'Laha', section: 'Sehemu' },
  sn: { page: 'Peji', slide: 'Siraidhi', sheet: 'Pepa', section: 'Chikamu' },
  nd: { page: 'Ikhasi', slide: 'Isilayidi', sheet: 'Iphepha', section: 'Ingxenye' },
  zu: { page: 'Ikhasi', slide: 'Isilayidi', sheet: 'Ishidi', section: 'Ingxenye' },
});
