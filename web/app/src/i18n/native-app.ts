import type {LocaleCode} from './index';
const copy = {
 en:['Use this computer','Application window','Separate browser','Choose an application','Select a window','Connect application','No accessible windows were found. Open the application, then try again.','Control only the selected window. Existing browsers can be selected too. Protected fields are excluded; changes require approval.','This computer needs the matching OffGrid Desktop application. Open it to select an application locally.','Observe application','Replace text','Activate control','Verify control text'],
 fr:['Utiliser cet ordinateur','Fenêtre d’application','Navigateur séparé','Choisir une application','Sélectionner une fenêtre','Connecter l’application','Aucune fenêtre accessible. Ouvrez l’application et réessayez.','Contrôlez uniquement la fenêtre sélectionnée, y compris un navigateur existant. Champs protégés exclus ; modifications soumises à autorisation.','Ouvrez la version correspondante d’OffGrid Desktop pour sélectionner une application sur cet ordinateur.','Observer l’application','Remplacer le texte','Activer le contrôle','Vérifier le texte'],
 de:['Diesen Computer verwenden','Anwendungsfenster','Separater Browser','Anwendung auswählen','Fenster auswählen','Anwendung verbinden','Keine zugänglichen Fenster gefunden. Öffnen Sie die Anwendung und versuchen Sie es erneut.','Nur das ausgewählte Fenster steuern, auch bestehende Browser. Geschützte Felder sind ausgeschlossen; Änderungen erfordern Zustimmung.','Öffnen Sie die passende OffGrid-Desktop-Version, um eine lokale Anwendung auszuwählen.','Anwendung prüfen','Text ersetzen','Steuerelement aktivieren','Text überprüfen'],
 es:['Usar este ordenador','Ventana de aplicación','Navegador separado','Elegir aplicación','Seleccionar ventana','Conectar aplicación','No hay ventanas accesibles. Abra la aplicación e inténtelo de nuevo.','Controle solo la ventana seleccionada, incluidos navegadores existentes. Se excluyen campos protegidos; los cambios requieren aprobación.','Abra la versión compatible de OffGrid Desktop para seleccionar una aplicación local.','Inspeccionar aplicación','Reemplazar texto','Activar control','Verificar texto'],
 ar:['استخدام هذا الحاسوب','نافذة التطبيق','متصفح منفصل','اختيار تطبيق','اختيار نافذة','ربط التطبيق','لم يتم العثور على نوافذ متاحة. افتح التطبيق ثم حاول مجدداً.','التحكم في النافذة المختارة فقط، بما فيها المتصفحات المفتوحة. الحقول المحمية مستبعدة والتغييرات تتطلب موافقة.','افتح إصدار OffGrid Desktop المتوافق لاختيار تطبيق على هذا الحاسوب.','فحص التطبيق','استبدال النص','تفعيل العنصر','التحقق من النص'],
 sw:['Tumia kompyuta hii','Dirisha la programu','Kivinjari tofauti','Chagua programu','Chagua dirisha','Unganisha programu','Hakuna madirisha yanayopatikana. Fungua programu ujaribu tena.','Dhibiti dirisha lililochaguliwa tu, pamoja na kivinjari kilichofunguliwa. Sehemu zilizolindwa hazijumuishwi; mabadiliko yanahitaji idhini.','Fungua toleo linalolingana la OffGrid Desktop kuchagua programu kwenye kompyuta hii.','Kagua programu','Badilisha maandishi','Washa kidhibiti','Thibitisha maandishi'],
 sn:['Shandisa kombiyuta iyi','Hwindo rechirongwa','Bhurawuza rakaparadzana','Sarudza chirongwa','Sarudza hwindo','Batanidza chirongwa','Hapana mahwindo anowanikwa. Vhura chirongwa woedza zvakare.','Dzora hwindo rasarudzwa chete, kusanganisira bhurawuza rakavhurwa. Nzvimbo dzakachengetedzwa hadzibatanidzwi; shanduko dzinoda mvumo.','Vhura OffGrid Desktop inoenderana kuti usarudze chirongwa pakombiyuta iyi.','Ongorora chirongwa','Tsiva mashoko','Shandisa chinodzora','Simbisa mashoko'],
 nd:['Sebenzisa ikhompyutha le','Iwindi lohlelo','Ibhrawuza ehlukileyo','Khetha uhlelo','Khetha iwindi','Xhuma uhlelo','Akulamawindi atholakalayo. Vula uhlelo uzame futhi.','Lawula iwindi elikhethiweyo kuphela, kuhlanganise lebhrawuza evuliweyo. Izindawo ezivikelweyo azifakwanga; izinguquko zidinga imvumo.','Vula i-OffGrid Desktop ehambelanayo ukuze ukhethe uhlelo kukhompyutha le.','Hlola uhlelo','Tshintsha umbhalo','Sebenzisa isilawuli','Qinisekisa umbhalo'],
 zu:['Sebenzisa le khompyutha','Iwindi lohlelo','Isiphequluli esihlukile','Khetha uhlelo','Khetha iwindi','Xhuma uhlelo','Awekho amawindi afinyelelekayo. Vula uhlelo bese uzama futhi.','Lawula iwindi elikhethiwe kuphela, kuhlanganise neziphequluli ezivuliwe. Izinkambu ezivikelwe azifakiwe; izinguquko zidinga imvume.','Vula inguqulo ehambisanayo ye-OffGrid Desktop ukukhetha uhlelo kule khompyutha.','Hlola uhlelo','Shintsha umbhalo','Sebenzisa isilawuli','Qinisekisa umbhalo']
};
export function nativeAppText(locale:LocaleCode) {
 const c=copy[locale];
 const launchMap:{[key:string]:readonly string[]}={
  en:['Open an application','Launch selected application','Applications opened by OffGrid can be selected below.','Choose upload file','Selected upload','Remove upload'],
  fr:['Ouvrir une application','Lancer l’application sélectionnée','Les applications ouvertes par OffGrid peuvent être sélectionnées ci-dessous.','Choisir un fichier à téléverser','Fichier sélectionné','Retirer le fichier'],
  de:['Anwendung öffnen','Ausgewählte Anwendung starten','Von OffGrid geöffnete Anwendungen können unten ausgewählt werden.','Datei zum Hochladen wählen','Ausgewählte Datei','Datei entfernen'],
  es:['Abrir una aplicación','Iniciar aplicación seleccionada','Las aplicaciones abiertas por OffGrid se pueden seleccionar abajo.','Elegir archivo para subir','Archivo seleccionado','Quitar archivo'],
  ar:['فتح تطبيق','تشغيل التطبيق المحدد','يمكن اختيار التطبيقات التي فتحها OffGrid أدناه.','اختيار ملف للرفع','الملف المحدد','إزالة الملف'],
  sw:['Fungua programu','Anzisha programu iliyochaguliwa','Programu zilizofunguliwa na OffGrid zinaweza kuchaguliwa hapa chini.','Chagua faili ya kupakia','Faili iliyochaguliwa','Ondoa faili'],
  sn:['Vhura chirongwa','Tanga chirongwa chasarudzwa','Zvirongwa zvakavhurwa neOffGrid zvinogona kusarudzwa pazasi.','Sarudza faira rekukwidza','Faira rasarudzwa','Bvisa faira'],
  nd:['Vula uhlelo','Qalisa uhlelo olukhethiweyo','Izinhlelo ezivulwe yi-OffGrid zingakhethwa ngezansi.','Khetha ifayela lokulayisha','Ifayela elikhethiweyo','Susa ifayela'],
  zu:['Vula uhlelo','Qalisa uhlelo olukhethiwe','Izinhlelo ezivulwe yi-OffGrid zingakhethwa ngezansi.','Khetha ifayela ozolilayisha','Ifayela elikhethiwe','Susa ifayela']
 };
 const launch=launchMap[locale] ?? launchMap.en;
 return {title:c[0],window:c[1],browser:c[2],choose:c[3],select:c[4],connect:c[5],empty:c[6],scope:c[7],desktop:c[8],actions:c.slice(9),open:launch[0],launch:launch[1],opened:launch[2],chooseUpload:launch[3],selectedUpload:launch[4],removeUpload:launch[5]};
}

const taskModeCopy:Record<LocaleCode,readonly [string,string,string]> = {
 en:['How should OffGrid work?','Agent only','Preview'],
 fr:['Comment OffGrid doit-il travailler ?','Agent uniquement','Aperçu'],
 de:['Wie soll OffGrid arbeiten?','Nur Agent','Vorschau'],
 es:['¿Cómo debe trabajar OffGrid?','Solo agente','Vista previa'],
 ar:['كيف يجب أن يعمل OffGrid؟','الوكيل فقط','معاينة'],
 sw:['OffGrid ifanye kazi vipi?','Wakala pekee','Hakiki'],
 sn:['OffGrid ishande sei?','Mumiririri chete','Ongororo'],
 nd:['I-OffGrid isebenze njani?','Umenzeli kuphela','Ukuhlola'],
 zu:['I-OffGrid isebenze kanjani?','Umenzeli kuphela','Ukubuka kuqala']
};
export function computerModeText(locale:LocaleCode){const [title,agent,preview]=taskModeCopy[locale];return {title,agent,preview};}

const approvalCopy:Record<LocaleCode,readonly [string,string,string,string,string,string,string]> = {
 en:['Action approvals','Ask every time','Review every change before it runs.','Approve scoped changes','Automatically allow reversible work in the selected scope.','Full task access','Allow all available typed actions in scope. Credentials, payments, privilege changes, installation, permanent deletion, scripts, and uncertain retries stay blocked.'],
 fr:['Autorisations','Toujours demander','VÃ©rifier chaque modification avant exÃ©cution.','Approuver les changements limitÃ©s','Autoriser automatiquement les actions rÃ©versibles dans la portÃ©e choisie.','AccÃ¨s complet Ã  la tÃ¢che','Autoriser les actions typÃ©es disponibles. Les identifiants, paiements et actions systÃ¨me dangereuses restent bloquÃ©s.'],
 de:['Aktionsfreigaben','Immer fragen','Jede Ã„nderung vor der AusfÃ¼hrung prÃ¼fen.','Begrenzte Ã„nderungen erlauben','Umkehrbare Aktionen im gewÃ¤hlten Bereich automatisch zulassen.','Voller Aufgabenzugriff','Alle verfÃ¼gbaren typisierten Aktionen im Bereich erlauben; sensible und gefÃ¤hrliche Aktionen bleiben gesperrt.'],
 es:['Aprobaciones','Preguntar siempre','Revise cada cambio antes de ejecutarlo.','Aprobar cambios limitados','Permitir automÃ¡ticamente acciones reversibles dentro del Ã¡mbito elegido.','Acceso completo a la tarea','Permitir las acciones tipadas disponibles; credenciales, pagos y acciones peligrosas siguen bloqueados.'],
 ar:['Ù…ÙˆØ§ÙÙ‚Ø§Øª Ø§Ù„Ø¥Ø¬Ø±Ø§Ø¡Ø§Øª','Ø§Ù„Ø³Ø¤Ø§Ù„ ÙÙŠ ÙƒÙ„ Ù…Ø±Ø©','Ù…Ø±Ø§Ø¬Ø¹Ø© ÙƒÙ„ ØªØºÙŠÙŠØ± Ù‚Ø¨Ù„ ØªÙ†ÙÙŠØ°Ù‡.','Ø§Ù„Ù…ÙˆØ§ÙÙ‚Ø© Ø¹Ù„Ù‰ Ø§Ù„ØªØºÙŠÙŠØ±Ø§Øª Ø§Ù„Ù…Ø­Ø¯ÙˆØ¯Ø©','Ø§Ù„Ø³Ù…Ø§Ø­ ØªÙ„Ù‚Ø§Ø¦ÙŠØ§Ù‹ Ø¨Ø§Ù„Ø¥Ø¬Ø±Ø§Ø¡Ø§Øª Ø§Ù„Ù‚Ø§Ø¨Ù„Ø© Ù„Ù„ØªØ±Ø§Ø¬Ø¹ Ø¶Ù…Ù† Ø§Ù„Ù†Ø·Ø§Ù‚.','ÙˆØµÙˆÙ„ ÙƒØ§Ù…Ù„ Ù„Ù„Ù…Ù‡Ù…Ø©','Ø§Ù„Ø³Ù…Ø§Ø­ Ø¨Ø§Ù„Ø¥Ø¬Ø±Ø§Ø¡Ø§Øª Ø§Ù„Ù…ØªØ§Ø­Ø© Ù…Ø¹ Ø¨Ù‚Ø§Ø¡ Ø§Ù„Ø¥Ø¬Ø±Ø§Ø¡Ø§Øª Ø§Ù„Ø­Ø³Ø§Ø³Ø© Ù…Ø­Ø¸ÙˆØ±Ø©.'],
 sw:['Idhini za vitendo','Uliza kila mara','Kagua kila badiliko kabla ya kutekelezwa.','Idhinisha mabadiliko yenye mipaka','Ruhusu vitendo vinavyoweza kutenduliwa ndani ya eneo lililochaguliwa.','Ufikiaji kamili wa kazi','Ruhusu vitendo vilivyowekewa aina; taarifa nyeti na vitendo hatari hubaki vimezuiwa.'],
 sn:['Mvumo yezviito','Bvunza nguva dzose','Ongorora shanduko yega yega isati yaitwa.','Bvumira shanduko dzine muganhu','Bvumira otomatiki zviito zvinogona kudzoserwa munzvimbo yakasarudzwa.','Mvumo yakazara yebasa','Bvumira zviito zvakatarwa; ruzivo rwakavanzika nezviito zvine ngozi zvinoramba zvakavharwa.'],
 nd:['Imvumo zezenzo','Buza isikhathi sonke','Hlola lonke utshintsho lungakenziwa.','Vumela izinguquko ezilinganiselweyo','Vumela izenzo ezibuyisekayo endaweni ekhethiweyo.','Ukufinyelela okupheleleyo','Vumela izenzo ezikhona; ulwazi oluyimfihlo lezenzo eziyingozi kuhlala kuvinjiwe.'],
 zu:['Izimvume zezenzo','Buza njalo','Buyekeza lonke ushintsho ngaphambi kokwenziwa.','Vumela izinguquko ezilinganiselwe','Vumela izenzo ezihlehlisekayo endaweni ekhethiwe.','Ukufinyelela okuphelele','Vumela izenzo ezikhona; imininingwane ebucayi nezenzo eziyingozi kuhlala kuvinjiwe.']
};
export function approvalPolicyText(locale:LocaleCode){const [title,ask,askHelp,scoped,scopedHelp,full,fullHelp]=approvalCopy[locale];return {title,ask,askHelp,scoped,scopedHelp,full,fullHelp};}

const approvalAuditCopy:Record<LocaleCode,readonly [string,string]> = {
 en:['Approved before execution','Automatically approved'],
 fr:['Approuvé avant exécution','Approuvé automatiquement'],
 de:['Vor der Ausführung genehmigt','Automatisch genehmigt'],
 es:['Aprobado antes de ejecutar','Aprobado automáticamente'],
 ar:['تمت الموافقة قبل التنفيذ','تمت الموافقة تلقائياً'],
 sw:['Imeidhinishwa kabla ya kutekelezwa','Imeidhinishwa kiotomatiki'],
 sn:['Zvabvumidzwa zvisati zvaitwa','Zvabvumidzwa otomatiki'],
 nd:['Kuvunyelwe kungakenziwa','Kuvunyelwe ngokuzenzakalela'],
 zu:['Kuvunyelwe ngaphambi kokwenziwa','Kuvunyelwe ngokuzenzakalelayo']
};
export function approvalAuditText(locale:LocaleCode){const [exact,automatic]=approvalAuditCopy[locale];return {exact,automatic};}

const feedback:Record<LocaleCode,readonly string[]>={
 en:['Stop control','Application connected','Allow accessibility access in your operating system settings, then reopen OffGrid Desktop. Elevated or protected applications cannot be controlled.','The selected window changed or closed. Stop this session, choose the window again and review any changes already made.','Update the local OffGrid service and desktop app to matching builds with application-control support. Your workspace is not changed.','Local approval was declined or expired. No further changes are authorized. Stop this session to choose another task.','Control is stopping. The application stays open; review any action already dispatched.'],
 fr:['Arrêter le contrôle','Application connectée','Autorisez l’accessibilité dans les réglages système, puis rouvrez OffGrid Desktop. Les applications protégées ou élevées sont exclues.','La fenêtre a changé ou a été fermée. Arrêtez la session, sélectionnez-la à nouveau et vérifiez les changements.','Mettez à jour le service local et le bureau vers des versions compatibles avec le contrôle des applications. Les données restent intactes.','L’autorisation locale a été refusée ou a expiré. Arrêtez la session avant une autre tâche.','Arrêt du contrôle. L’application reste ouverte ; vérifiez les actions déjà lancées.'],
 de:['Steuerung stoppen','Anwendung verbunden','Erlauben Sie Bedienungshilfen in den Systemeinstellungen und öffnen Sie OffGrid Desktop erneut. Geschützte oder erhöhte Anwendungen sind ausgeschlossen.','Das Fenster wurde geändert oder geschlossen. Stoppen Sie die Sitzung, wählen Sie es erneut und prüfen Sie bisherige Änderungen.','Aktualisieren Sie Dienst und Desktop auf passende Versionen mit Anwendungssteuerung. Ihre Daten bleiben unverändert.','Die lokale Zustimmung wurde abgelehnt oder ist abgelaufen. Stoppen Sie die Sitzung vor einer neuen Aufgabe.','Steuerung wird gestoppt. Die Anwendung bleibt geöffnet; prüfen Sie bereits ausgelöste Aktionen.'],
 es:['Detener control','Aplicación conectada','Permita el acceso de accesibilidad en los ajustes del sistema y vuelva a abrir OffGrid Desktop. Se excluyen aplicaciones protegidas o elevadas.','La ventana cambió o se cerró. Detenga la sesión, selecciónela de nuevo y revise los cambios realizados.','Actualice el servicio local y la aplicación a versiones compatibles con el control de aplicaciones. Sus datos no cambian.','La aprobación local fue rechazada o caducó. Detenga la sesión antes de otra tarea.','Deteniendo el control. La aplicación sigue abierta; revise las acciones ya enviadas.'],
 ar:['إيقاف التحكم','التطبيق متصل','اسمح بإمكانية الوصول في إعدادات النظام ثم أعد فتح OffGrid Desktop. التطبيقات المحمية أو ذات الصلاحيات المرتفعة مستبعدة.','تغيّرت النافذة أو أُغلقت. أوقف الجلسة واخترها مجدداً وراجع التغييرات السابقة.','حدّث الخدمة المحلية وتطبيق سطح المكتب إلى إصدارين متوافقين مع التحكم بالتطبيقات. بياناتك لن تتغير.','رُفضت الموافقة المحلية أو انتهت. أوقف الجلسة قبل مهمة أخرى.','جارٍ إيقاف التحكم. يبقى التطبيق مفتوحاً؛ راجع الإجراءات التي بدأت بالفعل.'],
 sw:['Simamisha udhibiti','Programu imeunganishwa','Ruhusu ufikivu kwenye mipangilio ya mfumo, kisha fungua OffGrid Desktop tena. Programu zilizolindwa au zilizoinuliwa haziruhusiwi.','Dirisha limebadilika au limefungwa. Simamisha kipindi, lichague tena na ukague mabadiliko.','Sasisha huduma na programu ya mezani ziwe matoleo yanayolingana yenye udhibiti wa programu. Data yako haibadilishwi.','Idhini ya ndani imekataliwa au imeisha. Simamisha kipindi kabla ya kazi nyingine.','Udhibiti unasimama. Programu inabaki wazi; kagua vitendo vilivyoanza.'],
 sn:['Misa kudzora','Chirongwa chabatana','Bvumira accessibility mumarongero ekombiyuta wovhurazve OffGrid Desktop. Zvirongwa zvakachengetedzwa kana zvine mvumo yepamusoro hazvidzorwi.','Hwindo rachinja kana kuvharwa. Misa chikamu, sarudza zvakare woongorora shanduko.','Gadziridza sevhisi neDesktop kumavhezheni anoenderana anotsigira kudzora zvirongwa. Data harichinjwi.','Mvumo yarambwa kana kupera. Misa chikamu usati watanga rimwe basa.','Kudzora kuri kumiswa. Chirongwa chinoramba chakavhurwa; ongorora zviito zvakatotanga.'],
 nd:['Misa ukulawula','Uhlelo luxhunyiwe','Vumela ukufinyeleleka kuzilungiselelo zohlelo, uvule i-OffGrid Desktop futhi. Izinhlelo ezivikelweyo kumbe eziphakanyisiweyo azilawulwa.','Iwindi lintshintshile kumbe livaliwe. Misa iseshini, ukhethe futhi uhlole izinguquko.','Buyekeza isevisi leDesktop kube yizinguqulo ezihambelanayo ezilawula izinhlelo. Idatha yakho ayitshintshwa.','Imvumo yalapha yaliwe kumbe iphelile. Misa iseshini ngaphambi komunye umsebenzi.','Ukulawula kuyamiswa. Uhlelo luhlala luvulekile; hlola izenzo eseziqalile.'],
 zu:['Misa ukulawula','Uhlelo luxhunyiwe','Vumela ukufinyeleleka kuzilungiselelo zesistimu, uvule i-OffGrid Desktop futhi. Izinhlelo ezivikelwe noma eziphakanyisiwe azilawulwa.','Iwindi lishintshile noma livaliwe. Misa iseshini, ulikhethe futhi ubuyekeze izinguquko.','Buyekeza isevisi neDesktop kube yizinguqulo ezihambisanayo ezilawula izinhlelo. Idatha yakho ayishintshwa.','Imvume yalapha yenqatshiwe noma iphelile. Misa iseshini ngaphambi komunye umsebenzi.','Ukulawula kuyamiswa. Uhlelo luhlala luvulekile; hlola izenzo eseziqalile.']
};
export function nativeAppFeedback(locale:LocaleCode){const [stop,connected,permission,stale,upgrade,approval,stopping]=feedback[locale];return {stop,connected,permission,stale,upgrade,approval,stopping};}
export function nativeAppError(locale:LocaleCode,code:string,fallback:string){
 const f=nativeAppFeedback(locale);
 if(/permission_denied|desktop_unavailable/.test(code))return f.permission;
 if(/stale_|scope_violation/.test(code))return f.stale;
 if(/upgrade_required|workspace_changed|workspace_unavailable/.test(code))return f.upgrade;
 if(/approval_invalid/.test(code))return f.approval;
 return fallback;
}
