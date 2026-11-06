import type { LocaleCode } from './index';

export const externalSetupLabel: Record<LocaleCode, string> = {
  en: 'Setup instructions', fr: 'Instructions de configuration',
  es: 'Instrucciones de configuración', de: 'Einrichtungsanleitung',
  ar: 'تعليمات الإعداد', sw: 'Maelekezo ya usanidi',
  sn: 'Mirayiridzo yekugadzira', nd: 'Imiyalelo yokulungisa', zu: 'Imiyalelo yokusetha',
};

// Service compatibility is not a host installation or workflow qualification.
// Independent speaker review remains part of locale qualification.
export const externalIntegrations: Record<LocaleCode, { ready: string; blocked: string; host: string; workflow: string; manual: string }> = {
  en: { ready: 'Model compatible', blocked: 'Check requirements', host: 'Setup on this computer has not been checked.', workflow: 'Tool workflow: unverified', manual: 'Manual configuration reference' },
  fr: { ready: 'Modèle compatible', blocked: 'Vérifier les prérequis', host: 'La configuration sur cet ordinateur n’a pas été vérifiée.', workflow: 'Parcours avec outils : non vérifié', manual: 'Référence de configuration manuelle' },
  es: { ready: 'Modelo compatible', blocked: 'Revisar requisitos', host: 'No se ha comprobado la configuración en este equipo.', workflow: 'Flujo con herramientas: sin verificar', manual: 'Referencia de configuración manual' },
  de: { ready: 'Modell kompatibel', blocked: 'Voraussetzungen prüfen', host: 'Die Einrichtung auf diesem Computer wurde nicht geprüft.', workflow: 'Werkzeugablauf: unbestätigt', manual: 'Referenz zur manuellen Konfiguration' },
  ar: { ready: 'النموذج متوافق', blocked: 'تحقق من المتطلبات', host: 'لم يتم التحقق من الإعداد على هذا الجهاز.', workflow: 'سير عمل الأدوات: لم يُتحقق منه', manual: 'مرجع الإعداد اليدوي' },
  sw: { ready: 'Modeli inaendana', blocked: 'Kagua mahitaji', host: 'Usanidi kwenye kompyuta hii haujakaguliwa.', workflow: 'Mtiririko wa zana: haujathibitishwa', manual: 'Mwongozo wa usanidi wa mkono' },
  sn: { ready: 'Modhi inoenderana', blocked: 'Ongorora zvinodiwa', host: 'Kugadzirirwa kwekombiyuta iyi hakuna kuongororwa.', workflow: 'Kushanda nematurusi: hakuna kusimbiswa', manual: 'Gwaro rekugadzirisa nemaoko' },
  nd: { ready: 'Imodeli iyahambelana', blocked: 'Hlola izidingo', host: 'Ukulungiswa kwalikhompyutha akukahlolwa.', workflow: 'Umsebenzi wamathuluzi: awukaqinisekiswa', manual: 'Umhlahlandlela wokulungisa ngesandla' },
  zu: { ready: 'Imodeli iyahambelana', blocked: 'Hlola izidingo', host: 'Ukulungiswa kule khompyutha akukahlolwa.', workflow: 'Umsebenzi wamathuluzi: awukaqinisekiswa', manual: 'Umhlahlandlela wokulungisa ngesandla' },
};
