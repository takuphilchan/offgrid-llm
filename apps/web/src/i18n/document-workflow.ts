import type { LocaleCode } from './index';

type Labels = readonly [string, string, string, string, string, string, string, string, string, string];
const labels: Record<LocaleCode, Labels> = {
 en: ['Inspect documents', 'Create a blank document', 'Select document', 'Read document text', 'Select text', 'Insert text', 'Replace selected text', 'Input dispatched; independent verification pending.', 'This read failed without input. Earlier actions remain recorded. Resume continues this same task; it does not replay those actions.', 'Document verification'],
 fr: ['Inspecter les documents', 'Créer un document vide', 'Sélectionner le document', 'Lire le texte du document', 'Sélectionner le texte', 'Insérer du texte', 'Remplacer la sélection', 'Saisie envoyée ; vérification indépendante en attente.', 'Cette lecture a échoué sans saisie. Les actions précédentes restent enregistrées. Reprendre continue la même tâche sans les rejouer.', 'Vérification du document'],
 de: ['Dokumente prüfen', 'Leeres Dokument erstellen', 'Dokument auswählen', 'Dokumenttext lesen', 'Text auswählen', 'Text einfügen', 'Auswahl ersetzen', 'Eingabe gesendet; unabhängige Prüfung ausstehend.', 'Dieser Lesevorgang scheiterte ohne Eingabe. Frühere Aktionen bleiben gespeichert. Fortsetzen führt dieselbe Aufgabe weiter, ohne diese Aktionen zu wiederholen.', 'Dokumentprüfung'],
 es: ['Inspeccionar documentos', 'Crear un documento vacío', 'Seleccionar documento', 'Leer el texto del documento', 'Seleccionar texto', 'Insertar texto', 'Reemplazar la selección', 'Entrada enviada; verificación independiente pendiente.', 'Esta lectura falló sin introducir datos. Las acciones anteriores siguen registradas. Reanudar continúa la misma tarea sin repetirlas.', 'Verificación del documento'],
 ar: ['فحص المستندات', 'إنشاء مستند فارغ', 'اختيار المستند', 'قراءة نص المستند', 'تحديد النص', 'إدراج نص', 'استبدال النص المحدد', 'تم إرسال الإدخال؛ التحقق المستقل قيد الانتظار.', 'فشلت هذه القراءة دون إدخال. تبقى الإجراءات السابقة مسجلة. الاستئناف يتابع المهمة نفسها دون تكرار تلك الإجراءات.', 'التحقق من المستند'],
 sw: ['Kagua hati', 'Unda hati tupu', 'Chagua hati', 'Soma maandishi ya hati', 'Chagua maandishi', 'Ingiza maandishi', 'Badilisha maandishi yaliyochaguliwa', 'Ingizo limetumwa; uthibitishaji huru unasubiri.', 'Usomaji huu umeshindwa bila kuingiza data. Vitendo vya awali vimehifadhiwa. Kuendelea kunatumia kazi hii hii bila kuvirudia.', 'Uthibitishaji wa hati'],
 sn: ['Ongorora magwaro', 'Gadzira gwaro risina chinhu', 'Sarudza gwaro', 'Verenga zviri mugwaro', 'Sarudza mashoko', 'Isa mashoko', 'Tsiva mashoko asarudzwa', 'Mashoko atumirwa; kuongorora kwakazvimirira kwakamirira.', 'Kuverenga uku kwakundikana pasina kuisa mashoko. Zviito zvekare zvakachengetwa. Kuenderera mberi kunoenderera nebasa rimwe chete pasina kuzvidzokorora.', 'Kuongorora gwaro'],
 nd: ['Hlola imibhalo', 'Dala umbhalo ongelalutho', 'Khetha umbhalo', 'Bala amazwi ombhalo', 'Khetha amazwi', 'Faka amazwi', 'Tshintsha amazwi akhethiweyo', 'Okufakiweyo kuthunyelwe; ukuqinisekisa okuzimeleyo kusalindile.', 'Ukubala lokhu kwehlulekile kungelalutho olufakiweyo. Izenzo zangaphambili zigciniwe. Ukuqhubeka kuqhubekisa umsebenzi lo ngaphandle kokuziphinda.', 'Ukuhlola umbhalo'],
 zu: ['Hlola imibhalo', 'Dala umbhalo ongenalutho', 'Khetha umbhalo', 'Funda umbhalo', 'Khetha amagama', 'Faka amagama', 'Shintsha amagama akhethiwe', 'Okufakiwe kuthunyelwe; ukuqinisekisa okuzimele kusalindile.', 'Lokhu kufunda kwehlulekile ngaphandle kokufaka. Izenzo zangaphambili zigciniwe. Ukuqhubeka kuqhubekisa lo msebenzi ngaphandle kokuziphinda.', 'Ukuhlola umbhalo'],
};
export function documentWorkflowText(locale: LocaleCode) {
 const [documents, create, select, read, selectText, insert, replaceSelection, dispatched, readFailed, verification] = labels[locale];
 return { documents, create, select, read, selectText, insert, replaceSelection, dispatched, readFailed, verification };
}
export function documentActionLabel(locale: LocaleCode, tool: string): string | undefined {
 const copy = documentWorkflowText(locale);
 return ({ computer_documents:copy.documents, computer_create_document:copy.create, computer_select_document:copy.select, computer_read_text:copy.read, computer_select_text:copy.selectText, computer_insert_text:copy.insert, computer_replace_selection:copy.replaceSelection } as Record<string,string>)[tool];
}
