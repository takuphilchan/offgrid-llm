import type { LocaleCode } from './index';

const copy: Record<LocaleCode, string> = {
  en: 'Model files and speech runtimes are separate. Re-downloading a model will not fix a missing runtime. Choose an available model, or ask the service administrator to set up its runtime.',
  fr: 'Les fichiers du modèle et le moteur vocal sont distincts. Télécharger à nouveau le modèle ne répare pas un moteur manquant. Choisissez un modèle disponible ou demandez à l’administrateur du service de configurer son moteur.',
  es: 'Los archivos del modelo y el motor de voz son distintos. Descargar de nuevo el modelo no soluciona un motor ausente. Elige un modelo disponible o pide al administrador del servicio que configure su motor.',
  de: 'Modelldateien und Sprachlaufzeit sind getrennt. Ein erneuter Modelldownload behebt keine fehlende Laufzeit. Wählen Sie ein verfügbares Modell oder bitten Sie die Dienstverwaltung, dessen Laufzeit einzurichten.',
  ar: 'ملفات النموذج ومحرك الصوت منفصلان. إعادة تنزيل النموذج لا تصلح محركاً مفقوداً. اختر نموذجاً متاحاً أو اطلب من مسؤول الخدمة إعداد محركه.',
  sw: 'Faili za modeli na programu ya sauti ni tofauti. Kupakua modeli tena hakutarekebisha programu inayokosekana. Chagua modeli inayopatikana au mwombe msimamizi wa huduma asanidi programu yake.',
  sn: 'Mafaira emodhi neinjini yezwi zvakasiyana. Kudhawunirodha modhi zvakare hakugadzirisi injini isipo. Sarudza modhi iripo kana kukumbira mutariri webasa kuisa injini yayo.',
  nd: 'Amafayela emodeli lenjini yelizwi kwehlukene. Ukulanda imodeli futhi akulungisi injini engekho. Khetha imodeli ekhona kumbe ucele umphathi wesevisi alungise injini yayo.',
  zu: 'Amafayela emodeli nenjini yezwi kwehlukene. Ukulanda imodeli futhi akulungisi injini engekho. Khetha imodeli etholakalayo noma ucele umphathi wesevisi alungise injini yayo.',
};
export const voiceSetupText = (locale: LocaleCode) => copy[locale];
