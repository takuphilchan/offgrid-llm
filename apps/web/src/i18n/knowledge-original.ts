import { defineMessages } from './catalog';
import type { LocaleCode } from './index';
export const sourceClose = defineMessages<string>("i18n/knowledge-original:sourceClose", { en: 'Close', fr: 'Fermer', de: 'Schließen', es: 'Cerrar', ar: 'إغلاق', sw: 'Funga', sn: 'Vhara', nd: 'Vala', zu: 'Vala' });
const labels = defineMessages<[string, string]>("i18n/knowledge-original:labels", {
 en: ['Download original file', 'Only extracted text was retained for this document.'],
 fr: ['Télécharger le fichier original', 'Seul le texte extrait a été conservé pour ce document.'],
 de: ['Originaldatei herunterladen', 'Für dieses Dokument wurde nur der extrahierte Text gespeichert.'],
 es: ['Descargar archivo original', 'Solo se conservó el texto extraído de este documento.'],
 ar: ['تنزيل الملف الأصلي', 'تم الاحتفاظ بالنص المستخرج فقط لهذا المستند.'],
 sw: ['Pakua faili asili', 'Maandishi yaliyotolewa pekee yamehifadhiwa kwa hati hii.'],
 sn: ['Dhaunirodha faera rekutanga', 'Mashoko akabviswa chete ndiwo akachengetwa egwaro iri.'],
 nd: ['Landa ifayela lakuqala', 'Kugcinwe kuphela umbhalo okhutshiweyo walumbhalo.'],
 zu: ['Landa ifayela lokuqala', 'Kugcinwe kuphela umbhalo okhishiwe wale dokhumenti.'],
});
export function knowledgeOriginal(locale: LocaleCode) {
 const [download, missing] = labels[locale]; return { download, missing };
}
