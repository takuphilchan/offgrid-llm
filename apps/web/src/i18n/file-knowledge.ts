import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
const labels = defineMessages<[string, string, string]>("i18n/file-knowledge:labels", {
 en: ['Add to Knowledge', 'A separate copy will be available to people who can access this workspace’s Knowledge. The original stays in this conversation.', 'Set up Knowledge first, then return to this saved file. Your conversation and draft will stay here.'],
 fr: ['Ajouter aux connaissances', 'Une copie distincte sera accessible aux personnes autorisées à consulter les connaissances de cet espace. L’original reste dans cette conversation.', 'Configurez d’abord les connaissances, puis revenez à ce fichier. Votre conversation et votre brouillon sont conservés.'],
 de: ['Zum Wissen hinzufügen', 'Eine separate Kopie wird für Personen mit Zugriff auf das Wissen dieses Arbeitsbereichs verfügbar. Das Original bleibt in diesem Gespräch.', 'Richten Sie zuerst Wissen ein und kehren Sie zu dieser Datei zurück. Gespräch und Entwurf bleiben erhalten.'],
 es: ['Añadir a Conocimiento', 'Una copia independiente estará disponible para quienes puedan acceder al Conocimiento de este espacio. El original permanece en esta conversación.', 'Configura Conocimiento y vuelve a este archivo guardado. La conversación y el borrador se conservan.'],
 ar: ['إضافة إلى المعرفة', 'ستتاح نسخة مستقلة لمن يمكنهم الوصول إلى معرفة مساحة العمل هذه. يبقى الأصل في هذه المحادثة.', 'أعدّ المعرفة أولًا ثم عد إلى هذا الملف المحفوظ. ستبقى المحادثة والمسودة هنا.'],
 sw: ['Ongeza kwenye Maarifa', 'Nakala tofauti itapatikana kwa wenye ruhusa ya Maarifa ya nafasi hii. Asili itabaki katika mazungumzo haya.', 'Sanidi Maarifa kwanza kisha urudi kwenye faili hili. Mazungumzo na rasimu vitabaki hapa.'],
 sn: ['Wedzera kuRuzivo', 'Kopi yakasiyana ichawanikwa nevane mvumo yeRuzivo rwenzvimbo ino. Rekutanga rinoramba riri muhurukuro iyi.', 'Gadzira Ruzivo kutanga wozodzoka kufaera iri. Hurukuro nezvawanyora zvinoramba zviripo.'],
 nd: ['Faka kuLwazi', 'Ikhophi ehlukileyo izatholakala kulabo abavunyelwe ukufinyelela uLwazi lwale indawo. Elokuqala lihlala kule ingxoxo.', 'Lungisa uLwazi kuqala ubuyele kuleli fayela. Ingxoxo lokubhalileyo kuzasala lapha.'],
 zu: ['Engeza kuLwazi', 'Ikhophi ehlukile izotholakala kwabavunyelwe ukufinyelela kuLwazi lwale ndawo. Elokuqala lihlala kule ngxoxo.', 'Setha uLwazi kuqala bese ubuyela kuleli fayela. Ingxoxo nokubhaliwe kuzohlala lapha.'],
});
export const fileKnowledge = (locale: LocaleCode) => { const [title, sharing, setup] = labels[locale]; return { title, sharing, setup }; };
