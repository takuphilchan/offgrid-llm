import type { LocaleCode } from './index';

// Shared pending/application language; native-speaker review is a separate gate.
export const taskSteering = {
  en: { queued: 'Instruction queued for the next safe step. Actions already sent are not cancelled.', applied: 'Instruction applied.' },
  fr: { queued: 'Consigne en attente de la prochaine étape sûre. Les actions déjà envoyées ne sont pas annulées.', applied: 'Consigne appliquée.' },
  de: { queued: 'Anweisung für den nächsten sicheren Schritt vorgemerkt. Bereits gesendete Aktionen werden nicht abgebrochen.', applied: 'Anweisung angewendet.' },
  es: { queued: 'Instrucción en espera del siguiente paso seguro. Las acciones ya enviadas no se cancelan.', applied: 'Instrucción aplicada.' },
  ar: { queued: 'التعليمات في انتظار الخطوة الآمنة التالية. لا تُلغى الإجراءات التي أُرسلت بالفعل.', applied: 'تم تطبيق التعليمات.' },
  sw: { queued: 'Maelekezo yanasubiri hatua salama inayofuata. Vitendo vilivyotumwa tayari havighairiwi.', applied: 'Maelekezo yametumika.' },
  sn: { queued: 'Murayiro wakamirira danho rinotevera rakachengeteka. Zviito zvakatotumirwa hazvimiswe.', applied: 'Murayiro washandiswa.' },
  nd: { queued: 'Umlayo ulindele inyathelo elilandelayo eliphephileyo. Izenzo esezithunyelwe azikhanselwa.', applied: 'Umlayo ususetshenzisiwe.' },
  zu: { queued: 'Umyalelo ulinde isinyathelo esilandelayo esiphephile. Izenzo esezithunyelwe azikhanselwa.', applied: 'Umyalelo ususetshenzisiwe.' },
} satisfies Record<LocaleCode, { queued: string; applied: string }>;
