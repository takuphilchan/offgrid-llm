import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

const labels = defineMessages<string>("i18n/knowledge-options:labels", {
  en: 'Document options',
  fr: 'Options des documents',
  de: 'Dokumentoptionen',
  es: 'Opciones de documentos',
  ar: 'خيارات المستندات',
  sw: 'Chaguo za hati',
  sn: 'Zvirongwa zvemagwaro',
  nd: 'Izinketho zemibhalo',
  zu: 'Izinketho zemibhalo',
});

export function knowledgeOptions(locale: LocaleCode) { return labels[locale]; }

const guidance = defineMessages<[string, string]>("i18n/knowledge-options:guidance", {
  en: ['Add documents, then ask questions with sources.', 'Searches all documents you can access. Select sources to narrow your question.'],
  fr: ['Ajoutez des documents, puis posez des questions avec des sources.', 'Recherche dans tous les documents accessibles. Sélectionnez des sources pour cibler votre question.'],
  de: ['Dokumente hinzufügen und Fragen mit Quellen stellen.', 'Durchsucht alle zugänglichen Dokumente. Wählen Sie Quellen, um die Suche einzugrenzen.'],
  es: ['Añade documentos y haz preguntas con fuentes.', 'Busca en todos los documentos a los que tienes acceso. Selecciona fuentes para limitar la búsqueda.'],
  ar: ['أضف مستندات، ثم اطرح أسئلة مع مصادر.', 'يبحث في جميع المستندات المتاحة لك. حدد مصادر لتضييق نطاق سؤالك.'],
  sw: ['Ongeza hati, kisha uliza maswali yenye vyanzo.', 'Hutafuta katika hati zote unazoweza kufikia. Chagua vyanzo ili kupunguza upeo wa swali lako.'],
  sn: ['Wedzera magwaro, wobva wabvunza mibvunzo ine zvinyorwa zvinotsigira.', 'Inotsvaga mumagwaro ose aunobvumirwa kuona. Sarudza zvinyorwa kuti utsanangure panotsvagwa mhinduro.'],
  nd: ['Faka imibhalo, ubusubuza imibuzo ele mithombo.', 'Isesha kuyo yonke imibhalo ovunyelwe ukuyibona. Khetha imithombo ukuze unciphise indawo yokusesha.'],
  zu: ['Faka imibhalo, bese ubuza imibuzo enemithombo.', 'Isesha kuyo yonke imibhalo ovunyelwe ukuyifinyelela. Khetha imithombo ukuze unciphise ukusesha.'],
});

export function knowledgeGuidance(locale: LocaleCode) {
  const [intro, scope] = guidance[locale];
  return { intro, scope };
}
