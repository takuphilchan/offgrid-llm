import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

const copy = defineMessages<[string, string, string]>("i18n/knowledge-start:copy", {
 en: ['Search your documents', 'Set up document search once, then add files and ask questions. The search model runs on the connected OffGrid service.', 'Search model'],
 fr: ['Recherchez dans vos documents', 'Configurez la recherche une fois, puis ajoutez des fichiers et posez vos questions. Le modèle de recherche fonctionne sur le service OffGrid connecté.', 'Modèle de recherche'],
 de: ['Ihre Dokumente durchsuchen', 'Richten Sie die Dokumentensuche einmal ein. Danach können Sie Dateien hinzufügen und Fragen stellen. Das Suchmodell läuft auf dem verbundenen OffGrid-Dienst.', 'Suchmodell'],
 es: ['Busca en tus documentos', 'Configura la búsqueda una vez, añade archivos y haz preguntas. El modelo de búsqueda se ejecuta en el servicio OffGrid conectado.', 'Modelo de búsqueda'],
 ar: ['ابحث في مستنداتك', 'أعدّ البحث في المستندات مرة واحدة، ثم أضف الملفات واطرح أسئلتك. يعمل نموذج البحث على خدمة OffGrid المتصلة.', 'نموذج البحث'],
 sw: ['Tafuta katika hati zako', 'Sanidi utafutaji wa hati mara moja, kisha ongeza faili na uulize maswali. Modeli ya utafutaji inaendeshwa kwenye huduma ya OffGrid iliyounganishwa.', 'Modeli ya utafutaji'],
 sn: ['Tsvaga mumagwaro ako', 'Gadzira kutsvaga mumagwaro kamwe chete, wobva wawedzera mafaera nekubvunza mibvunzo. Modhi yekutsvaga inomhanya pasevhisi yeOffGrid yakabatana.', 'Modhi yekutsvaga'],
 nd: ['Sesha emibhalweni yakho', 'Lungisa ukusesha imibhalo kanye, ubusungeza amafayela ubuze imibuzo. Imodeli yokusesha isebenza kusevisi yeOffGrid exhunywe lapha.', 'Imodeli yokusesha'],
 zu: ['Sesha emibhalweni yakho', 'Setha ukusesha imibhalo kanye, bese wengeza amafayela ubuze imibuzo. Imodeli yokusesha isebenza kusevisi ye-OffGrid exhunyiwe.', 'Imodeli yokusesha'],
});

export function knowledgeStart(locale: LocaleCode) {
 const [title, body, model] = copy[locale];
 return { title, body, model };
}
