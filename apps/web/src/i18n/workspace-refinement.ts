import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

export const workspaceRefinement = defineMessages<{ latest: string; clear: string }>("i18n/workspace-refinement:inline1", {
  en: { latest: 'Latest', clear: 'Clear search' },
  fr: { latest: 'Derniers messages', clear: 'Effacer la recherche' },
  es: { latest: 'Últimos mensajes', clear: 'Borrar búsqueda' },
  de: { latest: 'Neueste Nachrichten', clear: 'Suche löschen' },
  ar: { latest: 'أحدث الرسائل', clear: 'مسح البحث' },
  sw: { latest: 'Ujumbe wa karibuni', clear: 'Futa utafutaji' },
  sn: { latest: 'Mameseji matsva', clear: 'Bvisa tsvakiridzo' },
  nd: { latest: 'Imilayezo emitsha', clear: 'Sula ukusesha' },
  zu: { latest: 'Imilayezo yakamuva', clear: 'Sula ukusesha' },
});
