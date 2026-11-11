import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

// Select recovery from service codes, never by matching English error text.
export const chatRecovery = defineMessages<string>("i18n/chat-recovery:inline1", {
  en: 'Not enough available memory for this model and context. Close memory-heavy apps or choose a smaller model, then send again. Your draft and settings are unchanged.',
  fr: 'Mémoire disponible insuffisante pour ce modèle et ce contexte. Fermez les applications gourmandes en mémoire ou choisissez un modèle plus petit, puis renvoyez la demande. Votre brouillon et vos réglages sont conservés.',
  de: 'Für dieses Modell und diesen Kontext ist nicht genügend Speicher verfügbar. Schließen Sie speicherintensive Apps oder wählen Sie ein kleineres Modell und senden Sie erneut. Entwurf und Einstellungen bleiben unverändert.',
  es: 'No hay suficiente memoria disponible para este modelo y contexto. Cierra aplicaciones que consuman mucha memoria o elige un modelo más pequeño y vuelve a enviar. Tu borrador y ajustes no han cambiado.',
  ar: 'لا توجد ذاكرة متاحة كافية لهذا النموذج والسياق. أغلق التطبيقات التي تستهلك ذاكرة كبيرة أو اختر نموذجًا أصغر، ثم أرسل مجددًا. لم تتغير مسودتك أو إعداداتك.',
  sw: 'Hakuna kumbukumbu ya kutosha kwa modeli hii na muktadha huu. Funga programu zinazotumia kumbukumbu nyingi au chagua modeli ndogo, kisha tuma tena. Rasimu na mipangilio yako haijabadilika.',
  sn: 'Hapana ndangariro yakakwana yemodeli iyi nemamiriro ehurukuro aya. Vhara mapurogiramu anoshandisa ndangariro zhinji kana sarudza modeli diki, wozotumira zvakare. Gwaro rako nemasetingi hazvina kuchinjwa.',
  nd: 'Akulandawo eyaneleyo yememori yemodeli le lengxoxo le. Vala izinhlelo ezisebenzisa imemori enengi kumbe ukhethe imodeli encane, ubusuthumela futhi. Okubhalileyo lezilungiselelo zakho akuntshintshanga.',
  zu: 'Ayikho inkumbulo eyanele yale modeli nengxoxo. Vala izinhlelo ezisebenzisa inkumbulo eningi noma ukhethe imodeli encane, bese uthumela futhi. Okubhalile nezilungiselelo zakho akushintshile.',
});

export const chatBusyRecovery = defineMessages<string>("i18n/chat-recovery:inline2", {
  en: 'OffGrid is processing other requests. Wait for that work to finish, then send again. Your draft is retained; this request was not sent to the model.',
  fr: 'OffGrid traite d’autres demandes. Attendez la fin de ce travail, puis renvoyez votre demande. Votre brouillon est conservé ; cette demande n’a pas été envoyée au modèle.',
  de: 'OffGrid verarbeitet andere Anfragen. Warten Sie, bis diese abgeschlossen sind, und senden Sie erneut. Ihr Entwurf bleibt erhalten; diese Anfrage wurde nicht an das Modell gesendet.',
  es: 'OffGrid está procesando otras solicitudes. Espera a que terminen y vuelve a enviar. Tu borrador se conserva; esta solicitud no se envió al modelo.',
  ar: 'يعالج OffGrid طلبات أخرى. انتظر حتى تنتهي، ثم أرسل مجددًا. تم الاحتفاظ بمسودتك؛ لم يُرسل هذا الطلب إلى النموذج.',
  sw: 'OffGrid inashughulikia maombi mengine. Subiri kazi hiyo ikamilike, kisha tuma tena. Rasimu yako imehifadhiwa; ombi hili halikutumwa kwa modeli.',
  sn: 'OffGrid iri kugadzirisa zvimwe zvikumbiro. Mirira kuti basa iroro ripere, wozotumira zvakare. Gwaro rako rakachengetwa; chikumbiro ichi hachina kutumirwa kumodeli.',
  nd: 'I-OffGrid isebenza kwezinye izicelo. Linda umsebenzi lowo uphele, ubusuthumela futhi. Okubhalileyo kugciniwe; isicelo lesi asithunyelwanga kumodeli.',
  zu: 'I-OffGrid icubungula ezinye izicelo. Linda lowo msebenzi uphele, bese uthumela futhi. Okubhalile kugciniwe; lesi sicelo asithunyelwanga kumodeli.',
});
