import type { LocaleCode } from './index';
type Labels = readonly [string, string, string, string, string, string, string, string, string, string, string, string];
const labels: Record<LocaleCode, Labels> = {
  en: ['Documents', 'Manage Knowledge', 'Manage document', 'Ask using Knowledge', 'Searches the permitted knowledge base, not just one selected document. Your draft is preserved; nothing is sent until you choose Send.', 'Indexed', 'Indexing', 'Indexing failed', 'No documents match this filter.', 'Preferences', 'Workspace', 'Diagnostics'],
  fr: ['Documents', 'Gérer les connaissances', 'Gérer le document', 'Questionner les connaissances', 'Recherche dans la base autorisée, pas uniquement dans un document sélectionné. Votre brouillon est conservé ; rien n’est envoyé avant Envoyer.', 'Indexé', 'Indexation', 'Échec de l’indexation', 'Aucun document ne correspond à ce filtre.', 'Préférences', 'Espace de travail', 'Diagnostics'],
  es: ['Documentos', 'Gestionar conocimientos', 'Gestionar documento', 'Preguntar con conocimientos', 'Busca en la base de conocimientos permitida, no solo en un documento. Tu borrador se conserva; nada se envía hasta elegir Enviar.', 'Indexado', 'Indexando', 'Error de indexación', 'Ningún documento coincide con este filtro.', 'Preferencias', 'Espacio de trabajo', 'Diagnóstico'],
  de: ['Dokumente', 'Wissen verwalten', 'Dokument verwalten', 'Mit Wissen fragen', 'Durchsucht die freigegebene Wissensbasis, nicht nur ein ausgewähltes Dokument. Der Entwurf bleibt erhalten; erst Senden übermittelt ihn.', 'Indiziert', 'Indizierung', 'Indizierung fehlgeschlagen', 'Keine Dokumente entsprechen diesem Filter.', 'Einstellungen', 'Arbeitsbereich', 'Diagnose'],
  ar: ['المستندات', 'إدارة المعرفة', 'إدارة المستند', 'السؤال باستخدام المعرفة', 'يبحث في قاعدة المعرفة المسموح بها، وليس في مستند محدد فقط. تبقى مسودتك محفوظة ولا يُرسل شيء حتى تختار إرسال.', 'مفهرس', 'جارٍ الفهرسة', 'فشلت الفهرسة', 'لا توجد مستندات تطابق هذا المرشح.', 'التفضيلات', 'مساحة العمل', 'التشخيص'],
  sw: ['Hati', 'Dhibiti Maarifa', 'Dhibiti hati', 'Uliza kwa Maarifa', 'Hutafuta katika msingi wa maarifa unaoruhusiwa, si hati moja tu. Rasimu yako inahifadhiwa; hakuna kinachotumwa hadi uchague Tuma.', 'Imeorodheshwa', 'Inaorodhesha', 'Uorodheshaji umeshindwa', 'Hakuna hati zinazolingana na kichujio hiki.', 'Mapendeleo', 'Eneo la kazi', 'Uchunguzi'],
  sn: ['Magwaro', 'Tonga Ruzivo', 'Tonga gwaro', 'Bvunza uchishandisa Ruzivo', 'Inotsvaga muruzivo rwunobvumidzwa, kwete gwaro rimwe chete. Zvauri kunyora zvinochengetwa; hapana chinotumirwa kusvikira wasarudza Tumira.', 'Yakaiswa muindekisi', 'Iri kuisa muindekisi', 'Kuisa muindekisi kwakundikana', 'Hapana magwaro anoenderana nesefa iyi.', 'Zvaunofarira', 'Nzvimbo yebasa', 'Kuongorora'],
  nd: ['Amadokhumenti', 'Phatha Ulwazi', 'Phatha idokhumenti', 'Buza usebenzisa Ulwazi', 'Kudinga kulwazi oluvunyelweyo, hatshi idokhumenti eyodwa kuphela. Okubhalayo kuyagcinwa; akuthunyelwa lutho ungakakhethi Thumela.', 'Kufakwe ku-indeksi', 'Kufakwa ku-indeksi', 'Ukufakwa ku-indeksi kwehlulekile', 'Akulamadokhumenti ahambelana lalesi sihlungi.', 'Okuncamelayo', 'Indawo yokusebenza', 'Ukuhlola'],
  zu: ['Amadokhumenti', 'Phatha Ulwazi', 'Phatha idokhumenti', 'Buza usebenzisa Ulwazi', 'Kusesha olwazini oluvunyelwe, hhayi idokhumenti eyodwa kuphela. Okubhalayo kuyagcinwa; akuthunyelwa lutho uze ukhethe Thumela.', 'Kufakwe ku-indeksi', 'Kufakwa ku-indeksi', 'Ukufakwa ku-indeksi kwehlulekile', 'Awekho amadokhumenti afana nalesi sihlungi.', 'Okuncamelayo', 'Indawo yokusebenza', 'Ukuhlola'],
};
export function workspaceManagement(locale: LocaleCode) {
  const [documents, manageKnowledge, manageDocument, askKnowledge, retrievalScope, indexed, indexing, indexFailed, noDocuments, preferences, workspace, diagnostics] = labels[locale];
  const openTask: Record<LocaleCode, string> = { en: 'Open task', fr: 'Ouvrir la tâche', es: 'Abrir tarea', de: 'Aufgabe öffnen', ar: 'فتح المهمة', sw: 'Fungua kazi', sn: 'Vhura basa', nd: 'Vula umsebenzi', zu: 'Vula umsebenzi' };
  const desktopStorage: Record<LocaleCode, string> = {
    en: 'Desktop application folders. An external service or container uses its own storage; these are not its workspace paths.',
    fr: 'Dossiers de l’application de bureau. Un service externe ou conteneur utilise son propre stockage ; ce ne sont pas ses chemins de travail.',
    es: 'Carpetas de la aplicación de escritorio. Un servicio externo o contenedor utiliza su propio almacenamiento; estas no son sus rutas de trabajo.',
    de: 'Ordner der Desktop-Anwendung. Externe Dienste und Container verwenden eigenen Speicher; dies sind nicht deren Arbeitsbereichspfade.',
    ar: 'مجلدات تطبيق سطح المكتب. تستخدم الخدمة الخارجية أو الحاوية تخزينها الخاص؛ هذه ليست مسارات مساحة عملها.',
    sw: 'Folda za programu ya kompyuta. Huduma ya nje au kontena hutumia hifadhi yake; hizi si njia za eneo lake la kazi.',
    sn: 'Maforodha eapp yepakombiyuta. Sevhisi yekunze kana kontena inoshandisa nzvimbo yayo; idzi hadzisi nzira dzenzvimbo yayo yebasa.',
    nd: 'Amafolda ohlelo lwekhompyutha. Isevisi yangaphandle kumbe ikhontena isebenzisa indawo yayo yokugcina; lezi kayisizo izindlela zayo zokusebenza.',
    zu: 'Amafolda ohlelo lwekhompyutha. Isevisi yangaphandle noma ikhonthenela isebenzisa indawo yayo yokugcina; lezi akuzona izindlela zendawo yayo yokusebenza.',
  };
  const serviceState: Record<LocaleCode, string> = { en: 'Connection', fr: 'Connexion', es: 'Conexión', de: 'Verbindung', ar: 'الاتصال', sw: 'Muunganisho', sn: 'Kubatanidzwa', nd: 'Uxhumano', zu: 'Uxhumano' };
  return { documents, manageKnowledge, manageDocument, askKnowledge, retrievalScope, indexed, indexing, indexFailed, noDocuments, preferences, workspace, diagnostics, openTask: openTask[locale], desktopStorage: desktopStorage[locale], serviceState: serviceState[locale] };
}
