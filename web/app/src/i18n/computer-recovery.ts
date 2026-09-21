import type { LocaleCode } from './index';

// Draft translations; independent speaker review remains a qualification gate.
export const computerActionReview: Record<LocaleCode,string> = {
 en: 'Browser assistance stopped because an action was duplicated, changed, or its outcome could not be confirmed. Inspect the task and the affected page before starting a new session. Do not repeat a submission without checking its result.',
 fr: 'L’assistance s’est arrêtée : une action a été répétée, modifiée ou son résultat est incertain. Vérifiez la tâche et la page avant une nouvelle session. Ne répétez pas un envoi sans vérifier son résultat.',
 de: 'Die Browserhilfe wurde gestoppt: Eine Aktion wurde wiederholt, geändert oder ihr Ergebnis ist unklar. Prüfen Sie Aufgabe und Seite vor einer neuen Sitzung. Senden Sie nichts erneut, ohne das Ergebnis zu prüfen.',
 es: 'La asistencia se detuvo: una acción se repitió, cambió o su resultado es incierto. Revise la tarea y la página antes de iniciar otra sesión. No repita un envío sin comprobar su resultado.',
 ar: 'توقفت مساعدة المتصفح لأن إجراءً تكرر أو تغيّر أو تعذر تأكيد نتيجته. راجع المهمة والصفحة قبل بدء جلسة جديدة. لا تكرر الإرسال دون التحقق من النتيجة.',
 sw: 'Usaidizi wa kivinjari umesimama kwa sababu kitendo kilirudiwa, kilibadilishwa au matokeo hayajathibitishwa. Kagua kazi na ukurasa kabla ya kipindi kipya. Usitume tena bila kukagua matokeo.',
 sn: 'Rubatsiro rwebhurawuza rwamira nekuti chiito chadzokororwa, chachinjwa kana mhedzisiro yacho isina kusimbiswa. Tarisa basa nepeji usati watanga patsva. Usatumira zvakare usina kutarisa mhedzisiro.',
 nd: 'Usizo lwebhrawuza lumisiwe ngoba isenzo siphindiwe, sitshintshiwe kumbe impumela ayiqinisekiswanga. Hlola umsebenzi lekhasi ungakaqali futhi. Ungathumeli futhi ungakahloli impumela.',
 zu: 'Usizo lwesiphequluli lumisiwe ngoba isenzo siphindiwe, sishintshiwe noma umphumela awuqinisekisiwe. Hlola umsebenzi nekhasi ngaphambi kokuqala futhi. Ungathumeli futhi ungakahloli umphumela.'
};

export const computerModelCopy: Record<LocaleCode, { check: string; checking: string; help: string }> = {
 en: {check:'Check selected model', checking:'Checking tool calls…', help:'Check the selected model before running a computer task. This may load the model and uses synthetic data only; it does not control the browser. The service checks again when starting.'},
 fr: {check:'Vérifier le modèle sélectionné', checking:'Vérification des appels d’outils…', help:'Vérifiez le modèle avant la tâche. Le test peut charger le modèle et utilise uniquement des données synthétiques, sans contrôler le navigateur. Le service vérifie à nouveau au démarrage.'},
 de: {check:'Ausgewähltes Modell prüfen', checking:'Werkzeugaufrufe werden geprüft…', help:'Prüfen Sie das Modell vor der Aufgabe. Der Test kann das Modell laden und verwendet nur synthetische Daten, ohne den Browser zu steuern. Beim Start wird erneut geprüft.'},
 es: {check:'Comprobar modelo seleccionado', checking:'Comprobando llamadas a herramientas…', help:'Compruebe el modelo antes de ejecutar la tarea. La prueba puede cargarlo y usa solo datos sintéticos, sin controlar el navegador. Se comprueba de nuevo al iniciar.'},
 ar: {check:'فحص النموذج المحدد', checking:'جارٍ فحص استدعاءات الأدوات…', help:'افحص النموذج قبل المهمة. قد يحمّل الفحص النموذج ويستخدم بيانات اختبار فقط دون التحكم في المتصفح. يُعاد الفحص عند البدء.'},
 sw: {check:'Kagua modeli iliyochaguliwa', checking:'Inakagua miito ya zana…', help:'Kagua modeli kabla ya kazi. Jaribio linaweza kupakia modeli; linatumia data ya majaribio tu bila kudhibiti kivinjari. Huduma hukagua tena wakati wa kuanza.'},
 sn: {check:'Tarisa modhi yakasarudzwa', checking:'Kuongorora mashandisirwo ematurusi…', help:'Tarisa modhi usati watanga basa. Muedzo unogona kuisa modhi uye unoshandisa data rekuyedza chete, usingadzori bhurawuza. Sevhisi inoongorora zvakare pakutanga.'},
 nd: {check:'Hlola imodeli ekhethiweyo', checking:'Kuhlolwa ukubizwa kwamathuluzi…', help:'Hlola imodeli ungakaqali umsebenzi. Ukuhlola kungalayitsha imodeli, kusebenzisa idatha yokuhlola kuphela kungalawuli ibhrawuza. Isevisi ihlola njalo ekuqaleni.'},
 zu: {check:'Hlola imodeli ekhethiwe', checking:'Kuhlolwa ukubizwa kwamathuluzi…', help:'Hlola imodeli ngaphambi komsebenzi. Ukuhlola kungalayisha imodeli, kusebenzisa idatha yokuhlola kuphela ngaphandle kokulawula isiphequluli. Isevisi ihlola futhi ekuqaleni.'}
};

// Speaker review remains required before claiming translation qualification.
export const computerRecovery: Record<LocaleCode, { missing: string; unpaired: string; setup: string }> = {
 en: { missing: 'The selected task is no longer available in this workspace. Your draft is preserved.', unpaired: 'Browser not paired', setup: 'Run npm.cmd start in the computer folder. Press Enter for the local service address. Choose demo for an isolated local test, or enter a public HTTPS origin. Complete browser checks before generating a pairing code.' },
 fr: { missing: 'La tâche sélectionnée n’est plus disponible dans cet espace. Votre brouillon est conservé.', unpaired: 'Navigateur non associé', setup: 'Lancez npm.cmd start dans le dossier computer. Appuyez sur Entrée pour le service local. Choisissez demo pour un test local isolé, ou une origine HTTPS publique. Vérifiez le navigateur avant de générer le code.' },
 de: { missing: 'Die ausgewählte Aufgabe ist in diesem Arbeitsbereich nicht mehr verfügbar. Ihr Entwurf bleibt erhalten.', unpaired: 'Browser nicht verbunden', setup: 'Starten Sie npm.cmd start im Ordner computer. Bestätigen Sie die lokale Dienstadresse mit Enter. Wählen Sie demo für einen isolierten lokalen Test oder eine öffentliche HTTPS-Adresse. Prüfen Sie den Browser vor dem Erstellen des Kopplungscodes.' },
 es: { missing: 'La tarea seleccionada ya no está disponible en este espacio. Se conserva su borrador.', unpaired: 'Navegador sin vincular', setup: 'Ejecute npm.cmd start en la carpeta computer. Pulse Intro para el servicio local. Elija demo para una prueba local aislada o un origen HTTPS público. Compruebe el navegador antes de generar el código.' },
 ar: { missing: 'المهمة المحددة لم تعد متاحة في مساحة العمل هذه. تم الاحتفاظ بالمسودة.', unpaired: 'المتصفح غير مرتبط', setup: 'شغّل npm.cmd start في مجلد computer. اضغط Enter لعنوان الخدمة المحلية. اختر demo لاختبار محلي معزول أو أدخل عنوان HTTPS عاماً. أكمل فحص المتصفح قبل إنشاء رمز الربط.' },
 sw: { missing: 'Kazi iliyochaguliwa haipatikani tena hapa. Rasimu yako imehifadhiwa.', unpaired: 'Kivinjari hakijaunganishwa', setup: 'Endesha npm.cmd start katika folda computer. Bonyeza Enter kwa huduma ya karibu. Chagua demo kwa jaribio la karibu au tovuti ya HTTPS. Kagua kivinjari kabla ya kuunda msimbo.' },
 sn: { missing: 'Basa rakasarudzwa harichawanikwi munzvimbo ino. Gwaro rako rekutanga rakachengetwa.', unpaired: 'Bhurawuza harina kubatanidzwa', setup: 'Mhanyisa npm.cmd start mufolda computer. Dzvanya Enter pakero yesevhisi. Sarudza demo kuti uedze pakombiyuta kana saiti yeHTTPS. Tarisa bhurawuza usati wagadzira kodhi.' },
 nd: { missing: 'Umsebenzi okhethiweyo awusatholakali lapha. Okubhalileyo kugciniwe.', unpaired: 'Ibhrawuza ayixhunyiwe', setup: 'Qalisa npm.cmd start kufolda computer. Cindezela Enter kusevisi yalapha. Khetha demo ukuhlola kumbe iwebhusayithi yeHTTPS. Hlola ibhrawuza ungakakhi ikhodi.' },
 zu: { missing: 'Umsebenzi okhethiwe awusatholakali lapha. Okubhalile kugciniwe.', unpaired: 'Isiphequluli asixhunyiwe', setup: 'Qalisa npm.cmd start kufolda computer. Cindezela Enter kusevisi yasendaweni. Khetha demo ukuhlola noma isayithi leHTTPS. Hlola isiphequluli ngaphambi kokwenza ikhodi.' }
};
