import type { LocaleCode } from './index';

// Service compatibility is not a host installation or workflow qualification.
// Independent speaker review remains part of locale qualification.
export const externalIntegrations: Record<LocaleCode, { ready: string; blocked: string; host: string; workflow: string; manual: string }> = {
  en: { ready: 'Service/model compatible', blocked: 'Service/model needs attention', host: 'Host setup: not inspected', workflow: 'Tool workflow: unverified', manual: 'Manual configuration reference' },
  fr: { ready: 'Service/modèle compatible', blocked: 'Service/modèle à vérifier', host: 'Configuration locale : non inspectée', workflow: 'Parcours avec outils : non vérifié', manual: 'Référence de configuration manuelle' },
  es: { ready: 'Servicio/modelo compatible', blocked: 'Servicio/modelo requiere atención', host: 'Configuración local: no inspeccionada', workflow: 'Flujo con herramientas: sin verificar', manual: 'Referencia de configuración manual' },
  de: { ready: 'Dienst/Modell kompatibel', blocked: 'Dienst/Modell prüfen', host: 'Lokale Einrichtung: nicht geprüft', workflow: 'Werkzeugablauf: unbestätigt', manual: 'Referenz zur manuellen Konfiguration' },
  ar: { ready: 'الخدمة والنموذج متوافقان', blocked: 'الخدمة أو النموذج يحتاج إلى مراجعة', host: 'إعداد الجهاز: لم يُفحص', workflow: 'سير عمل الأدوات: لم يُتحقق منه', manual: 'مرجع الإعداد اليدوي' },
  sw: { ready: 'Huduma/modeli zinaendana', blocked: 'Huduma/modeli zinahitaji kukaguliwa', host: 'Usanidi wa kifaa: haujakaguliwa', workflow: 'Mtiririko wa zana: haujathibitishwa', manual: 'Mwongozo wa usanidi wa mkono' },
  sn: { ready: 'Sevhisi/modhi zvinoenderana', blocked: 'Sevhisi/modhi zvinoda kutariswa', host: 'Kugadzirirwa kwemudziyo: hakuna kuongororwa', workflow: 'Kushanda nematurusi: hakuna kusimbiswa', manual: 'Gwaro rekugadzirisa nemaoko' },
  nd: { ready: 'Isevisi/imodeli kuyahambelana', blocked: 'Isevisi/imodeli kudinga ukuhlolwa', host: 'Ukulungiswa kwedivayisi: akukahlolwa', workflow: 'Umsebenzi wamathuluzi: awukaqinisekiswa', manual: 'Umhlahlandlela wokulungisa ngesandla' },
  zu: { ready: 'Isevisi/imodeli kuyahambelana', blocked: 'Isevisi/imodeli kudinga ukuhlolwa', host: 'Ukulungiswa kwedivayisi: akukahlolwa', workflow: 'Umsebenzi wamathuluzi: awukaqinisekiswa', manual: 'Umhlahlandlela wokulungisa ngesandla' },
};
