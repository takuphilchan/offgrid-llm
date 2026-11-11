import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
const labels = defineMessages<[string, string, string, string, string]>("i18n/knowledge-recovery:labels", {
  en: ['Unfinished imports', 'Your files are saved. Retry after resolving the issue; you do not need to upload again.', 'Remove saved import', 'Import interrupted', 'Removing an import deletes its pending file, not a document already in Knowledge.'],
  fr: ['Imports inachevés', 'Vos fichiers sont conservés. Corrigez le problème puis réessayez sans les envoyer de nouveau.', 'Supprimer l’import enregistré', 'Import interrompu', 'Supprimer un import efface son fichier en attente, pas un document déjà dans les connaissances.'],
  de: ['Unvollständige Importe', 'Ihre Dateien sind gespeichert. Beheben Sie das Problem und versuchen Sie es ohne erneuten Upload.', 'Gespeicherten Import entfernen', 'Import unterbrochen', 'Das Entfernen löscht die ausstehende Datei, nicht ein bereits gespeichertes Wissensdokument.'],
  es: ['Importaciones pendientes', 'Tus archivos están guardados. Resuelve el problema y vuelve a intentarlo sin subirlos otra vez.', 'Eliminar importación guardada', 'Importación interrumpida', 'Eliminar una importación borra el archivo pendiente, no un documento ya en Conocimiento.'],
  ar: ['عمليات استيراد غير مكتملة', 'ملفاتك محفوظة. عالج المشكلة ثم أعد المحاولة دون رفعها مجددًا.', 'إزالة الاستيراد المحفوظ', 'توقف الاستيراد', 'تزيل هذه العملية الملف المعلّق، وليس مستندًا أُضيف بالفعل إلى المعرفة.'],
  sw: ['Uingizaji ambao haujakamilika', 'Faili zako zimehifadhiwa. Tatua tatizo kisha jaribu tena bila kupakia upya.', 'Ondoa uingizaji uliohifadhiwa', 'Uingizaji umekatizwa', 'Kuondoa uingizaji hufuta faili inayosubiri, si hati iliyopo kwenye Maarifa.'],
  sn: ['Kupinza kusina kupera', 'Mafaera ako akachengetwa. Gadzirisa dambudziko woedza zvakare usingaaise patsva.', 'Bvisa kupinza kwakachengetwa', 'Kupinza kwamiswa', 'Kubvisa kupinza kunobvisa faera rakamirira, kwete gwaro ratova muRuzivo.'],
  nd: ['Ukufaka okungakapheli', 'Amafayela akho agciniwe. Lungisa uhlupho uzame futhi ungawalayishi kutsha.', 'Susa ukufaka okugcinwe', 'Ukufaka kuphazamisekile', 'Ukususa ukufaka kususa ifayela elilindileyo, hatshi umbhalo osukuLwazi.'],
  zu: ['Ukufaka okungakapheli', 'Amafayela akho agciniwe. Lungisa inkinga bese uzama futhi ngaphandle kokuwalayisha kabusha.', 'Susa ukufaka okulondoloziwe', 'Ukufaka kuphazamisekile', 'Ukususa ukufaka kususa ifayela elilindile, hhayi umbhalo osuvele ukuLwazi.'],
});
export function knowledgeRecovery(locale: LocaleCode) {
  const [title, body, remove, interrupted, removeHelp] = labels[locale];
  return { title, body, remove, interrupted, removeHelp };
}
