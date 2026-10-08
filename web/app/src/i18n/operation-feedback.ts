import type { LocaleCode } from './index';

type Feedback = {
  emptyApplications: string;
  checkingVoice: string; permissionPending: string; cancelVoice: string; cancelTranscription: string;
  checkingKnowledge: string; retryKnowledge: string; configureKnowledge: string; knowledgeCheckFailed: string;
  retrySearch: string; retryInspection: string; refreshCatalog: string; checkStatus: string;
};

const copy: Record<LocaleCode, Feedback> = {
  en: {
    emptyApplications: 'No launchable applications were found. Open an application yourself, then choose its window.',
    checkingVoice: 'Checking voice availability…', permissionPending: 'Waiting for microphone permission…', cancelVoice: 'Cancel microphone setup', cancelTranscription: 'Cancel transcription',
    checkingKnowledge: 'Checking Knowledge…', retryKnowledge: 'Check Knowledge again', configureKnowledge: 'Set up Knowledge', knowledgeCheckFailed: 'Could not check Knowledge.',
    retrySearch: 'Retry search', retryInspection: 'Retry model review', refreshCatalog: 'Refresh catalog', checkStatus: 'Check saved state',
  },
  fr: {
    emptyApplications: 'Aucune application à lancer. Ouvrez une application, puis choisissez sa fenêtre.',
    checkingVoice: 'Vérification de la reconnaissance vocale…', permissionPending: 'En attente de l’autorisation du microphone…', cancelVoice: 'Annuler la préparation du microphone', cancelTranscription: 'Annuler la transcription',
    checkingKnowledge: 'Vérification des connaissances…', retryKnowledge: 'Revérifier les connaissances', configureKnowledge: 'Configurer les connaissances', knowledgeCheckFailed: 'Vérification des connaissances impossible.',
    retrySearch: 'Relancer la recherche', retryInspection: 'Réessayer l’examen du modèle', refreshCatalog: 'Actualiser le catalogue', checkStatus: 'Vérifier l’état enregistré',
  },
  es: {
    emptyApplications: 'No hay aplicaciones disponibles para iniciar. Abre una aplicación y elige su ventana.',
    checkingVoice: 'Comprobando el reconocimiento de voz…', permissionPending: 'Esperando permiso para el micrófono…', cancelVoice: 'Cancelar preparación del micrófono', cancelTranscription: 'Cancelar transcripción',
    checkingKnowledge: 'Comprobando Conocimiento…', retryKnowledge: 'Volver a comprobar Conocimiento', configureKnowledge: 'Configurar Conocimiento', knowledgeCheckFailed: 'No se pudo comprobar Conocimiento.',
    retrySearch: 'Reintentar búsqueda', retryInspection: 'Reintentar revisión del modelo', refreshCatalog: 'Actualizar catálogo', checkStatus: 'Comprobar estado guardado',
  },
  de: {
    emptyApplications: 'Keine startbaren Anwendungen gefunden. Öffne eine Anwendung und wähle ihr Fenster.',
    checkingVoice: 'Spracherkennung wird geprüft…', permissionPending: 'Warten auf Mikrofonfreigabe…', cancelVoice: 'Mikrofonvorbereitung abbrechen', cancelTranscription: 'Transkription abbrechen',
    checkingKnowledge: 'Wissensbasis wird geprüft…', retryKnowledge: 'Wissensbasis erneut prüfen', configureKnowledge: 'Wissensbasis einrichten', knowledgeCheckFailed: 'Wissensbasis konnte nicht geprüft werden.',
    retrySearch: 'Suche wiederholen', retryInspection: 'Modellprüfung wiederholen', refreshCatalog: 'Katalog aktualisieren', checkStatus: 'Gespeicherten Status prüfen',
  },
  ar: {
    emptyApplications: 'لم يتم العثور على تطبيقات قابلة للتشغيل. افتح تطبيقًا بنفسك ثم اختر نافذته.',
    checkingVoice: 'جارٍ التحقق من توفر التعرف على الصوت…', permissionPending: 'في انتظار إذن الميكروفون…', cancelVoice: 'إلغاء إعداد الميكروفون', cancelTranscription: 'إلغاء التفريغ النصي',
    checkingKnowledge: 'جارٍ التحقق من المعرفة…', retryKnowledge: 'التحقق من المعرفة مجددًا', configureKnowledge: 'إعداد المعرفة', knowledgeCheckFailed: 'تعذر التحقق من المعرفة.',
    retrySearch: 'إعادة البحث', retryInspection: 'إعادة مراجعة النموذج', refreshCatalog: 'تحديث الكتالوج', checkStatus: 'التحقق من الحالة المحفوظة',
  },
  sw: {
    emptyApplications: 'Hakuna programu zinazoweza kuanzishwa. Fungua programu mwenyewe, kisha chagua dirisha lake.',
    checkingVoice: 'Inakagua utambuzi wa sauti…', permissionPending: 'Inasubiri ruhusa ya maikrofoni…', cancelVoice: 'Ghairi maandalizi ya maikrofoni', cancelTranscription: 'Ghairi unukuzi',
    checkingKnowledge: 'Inakagua Maarifa…', retryKnowledge: 'Kagua Maarifa tena', configureKnowledge: 'Sanidi Maarifa', knowledgeCheckFailed: 'Imeshindwa kukagua Maarifa.',
    retrySearch: 'Tafuta tena', retryInspection: 'Kagua modeli tena', refreshCatalog: 'Sasisha katalogi', checkStatus: 'Kagua hali iliyohifadhiwa',
  },
  sn: {
    emptyApplications: 'Hapana zvirongwa zvinogona kutangwa. Vhura chirongwa wega, wozosarudza hwindo racho.',
    checkingVoice: 'Kutarisa kuzivikanwa kwezwi…', permissionPending: 'Kumirira mvumo yemakrofoni…', cancelVoice: 'Kanzura kugadzirira makrofoni', cancelTranscription: 'Kanzura kunyora zvakataurwa',
    checkingKnowledge: 'Kutarisa Ruzivo…', retryKnowledge: 'Tarisa Ruzivo zvakare', configureKnowledge: 'Gadzirisa Ruzivo', knowledgeCheckFailed: 'Hatina kukwanisa kutarisa Ruzivo.',
    retrySearch: 'Tsvaga zvakare', retryInspection: 'Ongorora modhi zvakare', refreshCatalog: 'Vandudza katalogi', checkStatus: 'Tarisa mamiriro akachengetwa',
  },
  nd: {
    emptyApplications: 'Akutholakalanga izinhlelo ezingaqaliswa. Vula uhlelo ngokwakho, ukhethe iwindi lalo.',
    checkingVoice: 'Kuhlolwa ukubonwa kwelizwi…', permissionPending: 'Kulindelwe imvumo yemakrofoni…', cancelVoice: 'Khansela ukulungiselela imakrofoni', cancelTranscription: 'Khansela ukubhala okukhulunyiweyo',
    checkingKnowledge: 'Kuhlolwa Ulwazi…', retryKnowledge: 'Hlola Ulwazi futhi', configureKnowledge: 'Lungisa Ulwazi', knowledgeCheckFailed: 'Ulwazi aluhlolwanga.',
    retrySearch: 'Dinga futhi', retryInspection: 'Hlola imodeli futhi', refreshCatalog: 'Vuselela ikhathalogi', checkStatus: 'Hlola isimo esigciniweyo',
  },
  zu: {
    emptyApplications: 'Azitholakalanga izinhlelo ezingaqaliswa. Vula uhlelo ngokwakho, bese ukhetha iwindi lalo.',
    checkingVoice: 'Kuhlolwa ukuqashelwa kwezwi…', permissionPending: 'Kulindelwe imvume yemakrofoni…', cancelVoice: 'Khansela ukulungiselela imakrofoni', cancelTranscription: 'Khansela ukubhala okukhulunyiwe',
    checkingKnowledge: 'Kuhlolwa Ulwazi…', retryKnowledge: 'Hlola Ulwazi futhi', configureKnowledge: 'Lungiselela Ulwazi', knowledgeCheckFailed: 'Ulwazi alukwazanga ukuhlolwa.',
    retrySearch: 'Sesha futhi', retryInspection: 'Hlola imodeli futhi', refreshCatalog: 'Buyekeza ikhathalogi', checkStatus: 'Hlola isimo esilondoloziwe',
  },
};

export const operationFeedback = (locale: LocaleCode): Feedback => copy[locale];
