import type { LocaleCode } from './index';

// Installed files are not proof that their runtime can accept a conversation.
export const chatReadiness: Record<LocaleCode, { unavailable: string; setup: string }> = {
  en: { unavailable: 'The selected model is unavailable. Choose an available chat model; your draft is kept.', setup: 'Choose an installed chat model, import one offline, or review a download before starting.' },
  fr: { unavailable: 'Le modèle sélectionné est indisponible. Choisissez un modèle de discussion disponible ; votre brouillon est conservé.', setup: 'Choisissez un modèle installé, importez-en un hors ligne ou examinez un téléchargement avant de commencer.' },
  de: { unavailable: 'Das ausgewählte Modell ist nicht verfügbar. Wähle ein verfügbares Chatmodell; dein Entwurf bleibt erhalten.', setup: 'Wähle ein installiertes Chatmodell, importiere eines offline oder prüfe einen Download, bevor du beginnst.' },
  es: { unavailable: 'El modelo seleccionado no está disponible. Elige un modelo de chat disponible; tu borrador se conserva.', setup: 'Elige un modelo instalado, importa uno sin conexión o revisa una descarga antes de empezar.' },
  ar: { unavailable: 'النموذج المحدد غير متاح. اختر نموذج محادثة متاحًا؛ ستُحفظ مسودتك.', setup: 'اختر نموذج محادثة مثبتًا، أو استورد نموذجًا دون اتصال، أو راجع التنزيل قبل البدء.' },
  sw: { unavailable: 'Modeli iliyochaguliwa haipatikani. Chagua modeli ya mazungumzo inayopatikana; rasimu yako imehifadhiwa.', setup: 'Chagua modeli iliyosakinishwa, leta moja bila intaneti au kagua upakuaji kabla ya kuanza.' },
  sn: { unavailable: 'Modhi yawasarudza haisi kuwanikwa. Sarudza modhi yehurukuro inowanikwa; zvauri kunyora zvinochengetwa.', setup: 'Sarudza modhi yakaiswa, pinza imwe pasina intaneti kana ongorora kudhaunirodha usati watanga.' },
  nd: { unavailable: 'Imodeli ekhethiweyo ayitholakali. Khetha imodeli yengxoxo etholakalayo; okusalungiswayo kuyagcinwa.', setup: 'Khetha imodeli efakiweyo, ngenisa eyodwa ungaxhunyiwe ku-inthanethi kumbe uhlole ukuyidawuniloda ungaqali.' },
  zu: { unavailable: 'Imodeli ekhethiwe ayitholakali. Khetha imodeli yengxoxo etholakalayo; okusalungiswa kwakho kuyagcinwa.', setup: 'Khetha imodeli efakiwe, ngenisa eyodwa ungaxhunyiwe ku-inthanethi noma ubuyekeze ukulanda ngaphambi kokuqala.' },
};
