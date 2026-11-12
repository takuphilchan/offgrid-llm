import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

export const deleteConversationBody = defineMessages<string>("i18n/history-deletion:deleteConversationBody", {
  en: 'Delete this conversation and its local drafts? This cannot be undone. Saved work records and backups are kept.',
  fr: 'Supprimer cette conversation et ses brouillons locaux ? Cette action est irréversible. Les tâches enregistrées et les sauvegardes sont conservées.',
  de: 'Dieses Gespräch und seine lokalen Entwürfe löschen? Dies kann nicht rückgängig gemacht werden. Gespeicherte Aufgaben und Sicherungen bleiben erhalten.',
  es: '¿Eliminar esta conversación y sus borradores locales? No se puede deshacer. Se conservan las tareas guardadas y las copias de seguridad.',
  ar: 'هل تريد حذف هذه المحادثة ومسوداتها المحلية؟ لا يمكن التراجع عن ذلك. ستبقى سجلات العمل المحفوظة والنسخ الاحتياطية.',
  sw: 'Ufute mazungumzo haya na rasimu zake za ndani? Hili haliwezi kutenduliwa. Rekodi za kazi zilizohifadhiwa na nakala rudufu zitabaki.',
  sn: 'Dzima hurukuro iyi nemadhirafiti ayo emuno? Izvi hazvigoni kudzoserwa. Marekodhi ebasa akachengetwa nemabackup anosara.',
  nd: 'Susa ingxoxo le lemibhalo yayo engagqitywanga egcinwe lapha? Lokhu akubuyiseleki. Amarekhodi omsebenzi lamakhophi agciniweyo ayasala.',
  zu: 'Susa le ngxoxo nemibhalo yayo engagciniwe yasendaweni? Lokhu akubuyiseleki. Amarekhodi omsebenzi namakhophi ayisipele ayagcinwa.',
});
