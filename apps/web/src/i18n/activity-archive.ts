import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
const labels = defineMessages<readonly [string,string,string,string]>("i18n/activity-archive:labels", {
  en: ['Archived', 'Archive from Activity', 'Restore to Activity', 'This moves the record out of everyday Activity. Its outcome stays unchanged and recovery evidence is retained. It does not stop processes, undo actions or rerun work.'],
  fr: ['Archivés', 'Archiver l’activité', 'Restaurer l’activité', 'Le dossier quitte l’activité courante. Son résultat et ses preuves sont conservés. Aucun processus n’est arrêté, aucune action annulée ou relancée.'],
  de: ['Archiviert', 'Aktivität archivieren', 'Aktivität wiederherstellen', 'Der Eintrag wird aus der täglichen Aktivität entfernt. Ergebnis und Wiederherstellungsnachweise bleiben erhalten. Prozesse werden nicht beendet und Aktionen weder rückgängig gemacht noch wiederholt.'],
  es: ['Archivados', 'Archivar actividad', 'Restaurar actividad', 'El registro sale de la actividad diaria. Se conservan su resultado y las pruebas de recuperación. No detiene procesos, deshace acciones ni repite el trabajo.'],
  ar: ['المؤرشفة', 'أرشفة النشاط', 'استعادة النشاط', 'يُنقل السجل من النشاط اليومي مع الاحتفاظ بالنتيجة وأدلة الاستعادة. لا تُوقف العمليات ولا تُلغى الإجراءات ولا يُعاد تنفيذ العمل.'],
  sw: ['Zilizohifadhiwa', 'Hifadhi shughuli', 'Rudisha kwenye shughuli', 'Rekodi inaondolewa kwenye shughuli za kila siku. Matokeo na ushahidi wa urejeshaji vinabaki. Haisimamishi michakato, kutengua hatua wala kurudia kazi.'],
  sn: ['Zvakachengetwa', 'Chengeta chiitiko', 'Dzosa chiitiko', 'Rekodhi inobviswa pazviitiko zvezuva nezuva. Mhedzisiro nehumbowo zvinoramba zvakachengetwa. Hazvimisi mapurogiramu, kudzosa zviito kana kudzokorora basa.'],
  nd: ['Okugciniweyo', 'Gcina umsebenzi', 'Buyisela umsebenzi', 'Irekhodi iyasuswa emisebenzini yansuku zonke. Imiphumela lobufakazi kuyagcinwa. Lokhu akumisi izinhlelo, akuguquli izenzo kumbe kuphinde umsebenzi.'],
  zu: ['Okugciniwe', 'Gcina umsebenzi', 'Buyisela umsebenzi', 'Irekhodi lisuswa emisebenzini yansuku zonke. Imiphumela nobufakazi kuyagcinwa. Lokhu akumisi izinhlelo, akuhlehlisi izenzo noma kuphinde umsebenzi.'],
});
export function activityArchive(locale:LocaleCode) {
  const [archived,archive,restore,body]=labels[locale];
  return {archived,archive,restore,body};
}
