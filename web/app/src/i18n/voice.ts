import type { LocaleCode } from './index';

export type VoiceCopy = {
  record: string;
  stop: string;
  recording: string;
  transcribing: string;
  unavailable: string;
  permission: string;
  unsupported: string;
  failed: string;
  speak: string;
  stopSpeaking: string;
  preparing: string;
  preparingHint: string;
  speechFailed: string;
  speechTimeout: string;
  noSpeech: string;
  speechUnavailable: string;
  speakResponses: string;
  speechWaiting: string;
  provisionalSpeech: string;
};

const english: VoiceCopy = {
  record: 'Use microphone',
  stop: 'Stop recording',
  recording: 'Recording…',
  transcribing: 'Transcribing…',
  unavailable: 'Speech recognition is not ready. Install a compatible local ASR runtime and model in Models.',
  permission: 'Microphone permission was denied. Allow it for OffGrid and try again.',
  unsupported: 'This app or browser does not support microphone recording.',
  failed: 'Voice input failed. Try again or type your request.',
  speak: 'Read aloud',
  stopSpeaking: 'Stop reading',
  preparing: 'Preparing audio · Stop',
  preparingHint: 'Generating speech on the connected service. You can stop at any time.',
  speechFailed: 'Audio playback failed. Check the selected speech model and output device.',
  speechTimeout: 'Speech took too long. Playback was stopped; try a shorter passage or a lighter model.',
  noSpeech: 'This response has no readable prose.',
  speechUnavailable: 'No compatible speech voice is ready. Open Models to install a direct-speech model.',
  speakResponses: 'Speak responses',
  speechWaiting: 'Waiting for response text',
  provisionalSpeech: 'Speaking the draft response. Text is final only when saved.'
};

export function voiceText(_locale: LocaleCode): VoiceCopy {
  // Keep the control available in every supported locale until the complete
  // speech copy set is translated. The task and transcript remain editable.
  return english;
}

const runtimeCopy: Record<LocaleCode, { available: string; tested: string; unavailable: string }> = {
  en: { available: 'Runtime available; this model has not been tested yet.', tested: 'Speech request passed this session. Quality and latency are not qualified.', unavailable: 'The matching speech runtime is unavailable.' },
  fr: { available: 'Moteur disponible ; ce modèle n’a pas encore été testé.', tested: 'Requête vocale réussie pendant cette session. Qualité et latence non qualifiées.', unavailable: 'Le moteur vocal correspondant est indisponible.' },
  es: { available: 'Motor disponible; este modelo aún no se ha probado.', tested: 'Solicitud de voz completada en esta sesión. Calidad y latencia sin validar.', unavailable: 'El motor de voz correspondiente no está disponible.' },
  de: { available: 'Laufzeit verfügbar; dieses Modell wurde noch nicht getestet.', tested: 'Sprachanfrage in dieser Sitzung erfolgreich. Qualität und Latenz nicht qualifiziert.', unavailable: 'Die passende Sprachlaufzeit ist nicht verfügbar.' },
  ar: { available: 'محرك التشغيل متاح؛ لم يُختبر هذا النموذج بعد.', tested: 'نجح طلب الصوت في هذه الجلسة. لم تُعتمد الجودة أو سرعة الاستجابة.', unavailable: 'محرك الصوت المطابق غير متاح.' },
  sw: { available: 'Injini inapatikana; modeli hii bado haijajaribiwa.', tested: 'Ombi la sauti limefaulu katika kipindi hiki. Ubora na kasi bado hazijathibitishwa.', unavailable: 'Injini ya sauti inayolingana haipatikani.' },
  sn: { available: 'Injini iripo; modhi iyi haisati yaedzwa.', tested: 'Chikumbiro chezwi chabudirira muchikamu chino. Hunhu nekumhanya hazvisati zvasimbiswa.', unavailable: 'Injini yezwi inoenderana haisipo.' },
  nd: { available: 'Injini ikhona; imodeli le kayikahlolwa.', tested: 'Isicelo selizwi siphumelele kulesi sikhathi. Ubuhle lesivinini akukaqinisekiswa.', unavailable: 'Injini yelizwi ehambelanayo kayitholakali.' },
  zu: { available: 'Injini iyatholakala; le modeli ayikahlolwa.', tested: 'Isicelo sezwi siphumelele kule seshini. Ikhwalithi nesivinini akuqinisekisiwe.', unavailable: 'Injini yezwi ehambisanayo ayitholakali.' }
};
export const speechRuntimeText = (locale: LocaleCode) => runtimeCopy[locale];
