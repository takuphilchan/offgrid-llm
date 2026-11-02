import type { LocaleCode } from './index';
const labels: Record<LocaleCode, [string, string, string, string, string, string]> = {
 en: ['Partial extraction · not indexed', 'Review the readable pages below. Some content needs OCR or review before this document can be used in answers.', 'Readable', 'No text', 'Needs review', 'Page'],
 fr: ['Extraction partielle · non indexée', 'Consultez les pages lisibles. Certains contenus nécessitent une reconnaissance de texte ou une vérification avant de pouvoir être utilisés dans les réponses.', 'Lisible', 'Sans texte', 'À vérifier', 'Page'],
 de: ['Teilweise extrahiert · nicht indexiert', 'Prüfen Sie die lesbaren Seiten. Einige Inhalte benötigen Texterkennung oder Prüfung, bevor das Dokument für Antworten verwendet werden kann.', 'Lesbar', 'Kein Text', 'Prüfung nötig', 'Seite'],
 es: ['Extracción parcial · sin indexar', 'Revisa las páginas legibles. Parte del contenido necesita reconocimiento de texto o revisión antes de usarse en respuestas.', 'Legible', 'Sin texto', 'Requiere revisión', 'Página'],
 ar: ['استخراج جزئي · غير مفهرس', 'راجع الصفحات المقروءة أدناه. يحتاج بعض المحتوى إلى التعرّف على النص أو المراجعة قبل استخدام المستند في الإجابات.', 'مقروء', 'بلا نص', 'يحتاج إلى مراجعة', 'صفحة'],
 sw: ['Utoaji wa sehemu · haijaorodheshwa', 'Kagua kurasa zinazosomeka. Baadhi ya maudhui yanahitaji utambuzi wa maandishi au ukaguzi kabla ya kutumiwa katika majibu.', 'Inasomeka', 'Hakuna maandishi', 'Inahitaji ukaguzi', 'Ukurasa'],
 sn: ['Mashoko atorwa zvishoma · haisati yaiswa muindekisi', 'Ongorora mapeji anoverengeka. Mamwe mashoko anoda OCR kana kuongororwa gwaro risati rashandiswa pamhinduro.', 'Anoverengeka', 'Hapana mashoko', 'Zvinoda kuongororwa', 'Peji'],
 nd: ['Umbhalo okhutshwe ingxenye · awukho enkombeni', 'Hlola amakhasi afundekayo. Okunye kudinga ukuqashelwa kombhalo kumbe ukuhlolwa kungakasetshenziswa ezimpendulweni.', 'Kuyafundeka', 'Akulambhalo', 'Kudinga ukuhlolwa', 'Ikhasi'],
 zu: ['Umbhalo okhishwe ingxenye · awukho enkombeni', 'Hlola amakhasi afundekayo. Okunye kudinga ukubonwa kombhalo noma ukuhlolwa ngaphambi kokusetshenziswa ezimpendulweni.', 'Kuyafundeka', 'Akukho mbhalo', 'Kudinga ukuhlolwa', 'Ikhasi'],
};
export const knowledgePartial = (locale: LocaleCode) => {
 const [title, help, ready, empty, review, page] = labels[locale];
 return { title, help, ready, empty, review, page };
};
