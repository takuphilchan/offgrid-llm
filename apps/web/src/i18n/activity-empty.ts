import type { LocaleCode } from './index';

// Empty views describe the selected filter, never imply saved archives vanished.
const copy: Record<LocaleCode, readonly [string, string, string, string, string, string]> = {
  en: ['No work needs attention', 'Nothing is running', 'No recent history', 'No archived work', 'Your work is up to date', 'Earlier work is available in Archived.'],
  fr: ['Aucune intervention nécessaire', 'Aucun travail en cours', 'Aucun historique récent', 'Aucun travail archivé', 'Votre travail est à jour', 'Les travaux précédents sont disponibles dans les archives.'],
  de: ['Kein Handlungsbedarf', 'Keine laufenden Aufgaben', 'Kein aktueller Verlauf', 'Keine archivierten Aufgaben', 'Alles ist auf dem aktuellen Stand', 'Frühere Aufgaben finden Sie im Archiv.'],
  es: ['Nada requiere atención', 'No hay trabajo en curso', 'No hay historial reciente', 'No hay trabajo archivado', 'Tu trabajo está al día', 'El trabajo anterior está disponible en Archivados.'],
  ar: ['لا يوجد عمل يحتاج إلى انتباه', 'لا يوجد عمل قيد التشغيل', 'لا يوجد سجل حديث', 'لا يوجد عمل مؤرشف', 'عملك محدّث', 'العمل السابق متاح في الأرشيف.'],
  sw: ['Hakuna kazi inayohitaji uangalizi', 'Hakuna kazi inayoendelea', 'Hakuna historia ya hivi karibuni', 'Hakuna kazi iliyohifadhiwa', 'Kazi yako iko sawa', 'Kazi za awali zinapatikana kwenye kumbukumbu.'],
  sn: ['Hapana basa rinoda kutariswa', 'Hapana basa riri kuitwa', 'Hapana nhoroondo ichangobva kuitika', 'Hapana basa rakachengetwa', 'Basa rako riri mugwara', 'Basa rekare rinowanikwa pane Zvakachengetwa.'],
  nd: ['Akulamsebenzi odinga ukunakwa', 'Akulamsebenzi oqhubekayo', 'Akulambali yakamuva', 'Akulamsebenzi ogciniweyo', 'Umsebenzi wakho ulungile', 'Umsebenzi wangaphambilini uyatholakala kokugciniweyo.'],
  zu: ['Awukho umsebenzi odinga ukunakwa', 'Awukho umsebenzi oqhubekayo', 'Awukho umlando wakamuva', 'Awukho umsebenzi ogciniwe', 'Umsebenzi wakho usesimweni', 'Umsebenzi wangaphambilini uyatholakala kokugciniwe.'],
};
export function activityEmpty(locale: LocaleCode) {
  const [attention, running, history, archived, all, archiveHint] = copy[locale];
  return { attention, running, history, archived, all, archiveHint };
}
