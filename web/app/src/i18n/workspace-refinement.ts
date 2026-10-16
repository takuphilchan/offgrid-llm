import type { LocaleCode } from './index';

export const workspaceRefinement = {
  en: { latest: 'Latest', clear: 'Clear search' },
  fr: { latest: 'Derniers messages', clear: 'Effacer la recherche' },
  es: { latest: 'Últimos mensajes', clear: 'Borrar búsqueda' },
  de: { latest: 'Neueste Nachrichten', clear: 'Suche löschen' },
  ar: { latest: 'أحدث الرسائل', clear: 'مسح البحث' },
  sw: { latest: 'Ujumbe wa karibuni', clear: 'Futa utafutaji' },
  sn: { latest: 'Mameseji matsva', clear: 'Bvisa tsvakiridzo' },
  nd: { latest: 'Imilayezo emitsha', clear: 'Sula ukusesha' },
  zu: { latest: 'Imilayezo yakamuva', clear: 'Sula ukusesha' },
} satisfies Record<LocaleCode, { latest: string; clear: string }>;
