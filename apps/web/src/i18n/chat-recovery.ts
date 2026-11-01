import type { LocaleCode } from './index';

// Select recovery from service codes, never by matching English error text.
export const chatRecovery = {
  en: 'Not enough available memory for this model and context. Close memory-heavy apps or choose a smaller model, then send again. Your draft and settings are unchanged.',
  fr: 'Mémoire disponible insuffisante pour ce modèle et ce contexte. Fermez les applications gourmandes en mémoire ou choisissez un modèle plus petit, puis renvoyez la demande. Votre brouillon et vos réglages sont conservés.',
  de: 'Für dieses Modell und diesen Kontext ist nicht genügend Speicher verfügbar. Schließen Sie speicherintensive Apps oder wählen Sie ein kleineres Modell und senden Sie erneut. Entwurf und Einstellungen bleiben unverändert.',
  es: 'No hay suficiente memoria disponible para este modelo y contexto. Cierra aplicaciones que consuman mucha memoria o elige un modelo más pequeño y vuelve a enviar. Tu borrador y ajustes no han cambiado.',
  ar: 'لا توجد ذاكرة متاحة كافية لهذا النموذج والسياق. أغلق التطبيقات التي تستهلك ذاكرة كبيرة أو اختر نموذجًا أصغر، ثم أرسل مجددًا. لم تتغير مسودتك أو إعداداتك.',
  sw: 'Hakuna kumbukumbu ya kutosha kwa modeli hii na muktadha huu. Funga programu zinazotumia kumbukumbu nyingi au chagua modeli ndogo, kisha tuma tena. Rasimu na mipangilio yako haijabadilika.',
  sn: 'Hapana ndangariro yakakwana yemodeli iyi nemamiriro ehurukuro aya. Vhara mapurogiramu anoshandisa ndangariro zhinji kana sarudza modeli diki, wozotumira zvakare. Gwaro rako nemasetingi hazvina kuchinjwa.',
  nd: 'Akulandawo eyaneleyo yememori yemodeli le lengxoxo le. Vala izinhlelo ezisebenzisa imemori enengi kumbe ukhethe imodeli encane, ubusuthumela futhi. Okubhalileyo lezilungiselelo zakho akuntshintshanga.',
  zu: 'Ayikho inkumbulo eyanele yale modeli nengxoxo. Vala izinhlelo ezisebenzisa inkumbulo eningi noma ukhethe imodeli encane, bese uthumela futhi. Okubhalile nezilungiselelo zakho akushintshile.',
} satisfies Record<LocaleCode, string>;
