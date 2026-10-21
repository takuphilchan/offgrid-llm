import type { LocaleCode } from './index';
const en = {
  activity: 'Agent activity', inspect: 'Inspect work', details: 'Activity details',
  uncertain: 'An action was sent, but its outcome is not confirmed. It will not be repeated automatically.',
  manualReport: 'Report what you observed', reportHint: 'Inspect the affected application first. Your report is recorded as a human observation, not independent verification. This does not repeat the action.',
  recordReport: 'Record observation', savedJob: 'The result is saved with the job, but has not been attached to this conversation.', attach: 'Attach saved result',
  update: 'Send instruction', unavailable: 'This work needs attention before another message can run.',
  retain: 'Keep work in Agents', stop: 'Stop active work first', deleteChoice: 'If a conversation has active work',
  deleteHint: 'Keeping work preserves its canonical job. Stopping must be confirmed before this conversation can be removed.',
  preserveActive: 'Do not delete conversations with active work',
};
export const conversationWorkText: Record<LocaleCode, typeof en> = {
  en,
  fr: {
    activity: 'Activité de l’agent', inspect: 'Inspecter le travail', details: 'Détails de l’activité', uncertain: 'Une action a été envoyée, mais son résultat n’est pas confirmé. Elle ne sera pas répétée automatiquement.',
    manualReport: 'Rapporter votre observation', reportHint: 'Inspectez d’abord l’application concernée. Votre rapport est une observation humaine, pas une vérification indépendante. Cela ne répète pas l’action.', recordReport: 'Enregistrer l’observation', savedJob: 'Le résultat est enregistré avec la tâche, mais pas encore joint à cette conversation.', attach: 'Joindre le résultat enregistré',
    update: 'Envoyer une consigne', unavailable: 'Ce travail nécessite votre attention avant d’exécuter un autre message.', retain: 'Conserver le travail dans Agents', stop: 'Arrêter d’abord le travail actif', deleteChoice: 'Si une conversation contient du travail actif', deleteHint: 'Conserver le travail préserve sa tâche canonique. L’arrêt doit être confirmé avant de supprimer la conversation.', preserveActive: 'Ne pas supprimer les conversations avec du travail actif',
  },
  de: {
    activity: 'Agentenaktivität', inspect: 'Arbeit prüfen', details: 'Aktivitätsdetails', uncertain: 'Eine Aktion wurde gesendet, ihr Ergebnis ist aber nicht bestätigt. Sie wird nicht automatisch wiederholt.',
    manualReport: 'Beobachtung melden', reportHint: 'Prüfe zuerst die betroffene Anwendung. Dein Bericht wird als menschliche Beobachtung gespeichert, nicht als unabhängige Prüfung. Die Aktion wird nicht wiederholt.', recordReport: 'Beobachtung speichern', savedJob: 'Das Ergebnis ist beim Auftrag gespeichert, aber noch nicht an dieses Gespräch angehängt.', attach: 'Gespeichertes Ergebnis anhängen',
    update: 'Anweisung senden', unavailable: 'Diese Arbeit benötigt Aufmerksamkeit, bevor eine weitere Nachricht ausgeführt werden kann.', retain: 'Arbeit in Agents behalten', stop: 'Aktive Arbeit zuerst stoppen', deleteChoice: 'Falls ein Gespräch aktive Arbeit enthält', deleteHint: 'Beibehalten erhält den kanonischen Auftrag. Vor dem Löschen des Gesprächs muss das Stoppen bestätigt sein.', preserveActive: 'Gespräche mit aktiver Arbeit nicht löschen',
  },
  es: {
    activity: 'Actividad del agente', inspect: 'Inspeccionar trabajo', details: 'Detalles de actividad', uncertain: 'Se envió una acción, pero su resultado no está confirmado. No se repetirá automáticamente.',
    manualReport: 'Informar de lo observado', reportHint: 'Inspecciona primero la aplicación afectada. Tu informe se registra como observación humana, no como verificación independiente. Esto no repite la acción.', recordReport: 'Registrar observación', savedJob: 'El resultado está guardado en la tarea, pero aún no se adjuntó a esta conversación.', attach: 'Adjuntar resultado guardado',
    update: 'Enviar instrucción', unavailable: 'Este trabajo necesita atención antes de ejecutar otro mensaje.', retain: 'Conservar trabajo en Agentes', stop: 'Detener primero el trabajo activo', deleteChoice: 'Si una conversación tiene trabajo activo', deleteHint: 'Conservar el trabajo mantiene su tarea canónica. La detención debe confirmarse antes de eliminar la conversación.', preserveActive: 'No eliminar conversaciones con trabajo activo',
  },
  ar: {
    activity: 'نشاط الوكيل', inspect: 'فحص العمل', details: 'تفاصيل النشاط', uncertain: 'أُرسل إجراء لكن نتيجته لم تُؤكد. لن يتكرر تلقائياً.',
    manualReport: 'الإبلاغ عما لاحظته', reportHint: 'افحص التطبيق المعني أولاً. يُسجّل تقريرك كملاحظة بشرية وليس كتحقق مستقل. لا يُعاد تنفيذ الإجراء.', recordReport: 'تسجيل الملاحظة', savedJob: 'النتيجة محفوظة مع المهمة لكنها لم تُرفق بهذه المحادثة بعد.', attach: 'إرفاق النتيجة المحفوظة',
    update: 'إرسال تعليمات', unavailable: 'يحتاج هذا العمل إلى انتباهك قبل تنفيذ رسالة أخرى.', retain: 'إبقاء العمل في الوكلاء', stop: 'إيقاف العمل النشط أولاً', deleteChoice: 'إذا احتوت المحادثة على عمل نشط', deleteHint: 'الإبقاء على العمل يحفظ مهمته الأصلية. يجب تأكيد الإيقاف قبل حذف المحادثة.', preserveActive: 'عدم حذف المحادثات التي تحتوي على عمل نشط',
  },
  sw: {
    activity: 'Shughuli za wakala', inspect: 'Kagua kazi', details: 'Maelezo ya shughuli', uncertain: 'Kitendo kimetumwa, lakini matokeo yake hayajathibitishwa. Hakitarudiwa kiotomatiki.',
    manualReport: 'Ripoti ulichoona', reportHint: 'Kagua kwanza programu husika. Ripoti yako inarekodiwa kama uchunguzi wa binadamu, si uthibitishaji huru. Hii hairudii kitendo.', recordReport: 'Rekodi uchunguzi', savedJob: 'Matokeo yamehifadhiwa pamoja na kazi, lakini bado hayajaambatishwa kwenye mazungumzo haya.', attach: 'Ambatisha matokeo yaliyohifadhiwa',
    update: 'Tuma maelekezo', unavailable: 'Kazi hii inahitaji kuangaliwa kabla ya kutekeleza ujumbe mwingine.', retain: 'Weka kazi katika Mawakala', stop: 'Simamisha kazi hai kwanza', deleteChoice: 'Ikiwa mazungumzo yana kazi hai', deleteHint: 'Kuweka kazi kunahifadhi kazi yake asili. Kusimama lazima kuthibitishwe kabla ya kuondoa mazungumzo.', preserveActive: 'Usifute mazungumzo yenye kazi hai',
  },
  sn: {
    activity: 'Zviri kuitwa neagent', inspect: 'Ongorora basa', details: 'Tsananguro yezviri kuitwa', uncertain: 'Chiito chatumirwa, asi mhedzisiro yacho haisati yasimbiswa. Hachizodzokororwi choga.',
    manualReport: 'Taura zvaunoona', reportHint: 'Tanga waongorora app yakabatwa. Mushumo wako unochengetwa sezvakaonekwa nemunhu, kwete simbiso yakazvimirira. Izvi hazvidzokorori chiito.', recordReport: 'Chengeta zvawaona', savedJob: 'Mhedzisiro yakachengetwa nebasa, asi haisati yaiswa muhurukuro iyi.', attach: 'Isa mhedzisiro yakachengetwa',
    update: 'Tumira murayiro', unavailable: 'Basa iri rinoda kuongororwa imwe meseji isati yaitwa.', retain: 'Chengeta basa muAgents', stop: 'Tanga wamisa basa riri kuitwa', deleteChoice: 'Kana hurukuro iine basa riri kuitwa', deleteHint: 'Kuchengeta basa kunochengetedza rekodhi raro rekutanga. Kumira kunofanira kusimbiswa hurukuro isati yabviswa.', preserveActive: 'Usadzima hurukuro dzine basa riri kuitwa',
  },
  nd: {
    activity: 'Umsebenzi we-agent', inspect: 'Hlola umsebenzi', details: 'Imininingwane yomsebenzi', uncertain: 'Isenzo sithunyelwe, kodwa umphumela waso awukaqinisekiswa. Asiyikuphindwa ngokuzenzakalela.',
    manualReport: 'Bika okuboneleyo', reportHint: 'Qala uhlole uhlelo oluthintekayo. Umbiko wakho ugcinwa njengokubonwe ngumuntu, hatshi ukuqinisekisa okuzimeleyo. Lokhu akuphindi isenzo.', recordReport: 'Gcina okubonileyo', savedJob: 'Umphumela ugcinwe lomsebenzi, kodwa awukananyathiselwa kule ngxoxo.', attach: 'Namathisela umphumela ogciniweyo',
    update: 'Thumela umlayo', unavailable: 'Umsebenzi lo udinga ukunakwa omunye umlayezo ungakenziwa.', retain: 'Gcina umsebenzi kuAgents', stop: 'Qala umise umsebenzi oqhubekayo', deleteChoice: 'Nxa ingxoxo ilomsebenzi oqhubekayo', deleteHint: 'Ukugcina umsebenzi kugcina umsebenzi wokuqala. Ukuma kumele kuqinisekiswe ingxoxo ingakasuswa.', preserveActive: 'Ungasusi izingxoxo ezilomsebenzi oqhubekayo',
  },
  zu: {
    activity: 'Umsebenzi we-agent', inspect: 'Hlola umsebenzi', details: 'Imininingwane yomsebenzi', uncertain: 'Isenzo sithunyelwe, kodwa umphumela waso awukaqinisekiswa. Ngeke siphindwe ngokuzenzakalela.',
    manualReport: 'Bika okubone', reportHint: 'Qala uhlole uhlelo oluthintekayo. Umbiko wakho ugcinwa njengokubonwe umuntu, hhayi ukuqinisekiswa okuzimele. Lokhu akuphindi isenzo.', recordReport: 'Londoloza okubone', savedJob: 'Umphumela ugcinwe nomsebenzi, kodwa awukanamathiselwa kule ngxoxo.', attach: 'Namathisela umphumela ogciniwe',
    update: 'Thumela umyalelo', unavailable: 'Lo msebenzi udinga ukunakwa ngaphambi kokwenziwa komunye umlayezo.', retain: 'Gcina umsebenzi ku-Agents', stop: 'Misa umsebenzi oqhubekayo kuqala', deleteChoice: 'Uma ingxoxo inomsebenzi oqhubekayo', deleteHint: 'Ukugcina umsebenzi kugcina umsebenzi wawo wokuqala. Ukuma kufanele kuqinisekiswe ngaphambi kokususa ingxoxo.', preserveActive: 'Ungasusi izingxoxo ezinomsebenzi oqhubekayo',
  },
};
