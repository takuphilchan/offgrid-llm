import { defineMessages } from './catalog';

type Copy = { archive: string; archiveHint: string; retire: string; retireHint: string; unknown: string; receipts: string; policies: string; exportPage: string; evidenceOnly: string; allowedThen: string; deniedThen: string };
export const endpointMaintenance = defineMessages<Copy>('i18n/endpoint-maintenance:endpointMaintenance', {
  "en": {
    "archive": "Archive settled receipts",
    "archiveHint": "Keep the listed receipts in workspace history and free space in the active journal. Configuration and access stay unchanged.",
    "retire": "Retire obsolete decisions",
    "retireHint": "Archive the previous policy and remove only the listed obsolete decisions. Current and active endpoints are protected. This does not enable access.",
    "unknown": "The maintenance outcome is unconfirmed. Refresh and inspect the current state before reviewing another action. Do not repeat automatically.",
    "receipts": "Archived receipts",
    "policies": "Policy history",
    "exportPage": "Export this page",
    "evidenceOnly": "Historical evidence only. These records never grant current access.",
    "allowedThen": "Allowed in this snapshot",
    "deniedThen": "Denied in this snapshot"
  },
  "fr": {
    "archive": "Archiver les reçus résolus",
    "archiveHint": "Conserver les reçus affichés dans l’historique de l’espace et libérer le journal actif. La configuration et les accès restent inchangés.",
    "retire": "Retirer les décisions obsolètes",
    "retireHint": "Archiver la politique précédente et retirer uniquement les décisions obsolètes affichées. Les points actuels et actifs sont protégés. Aucun accès n’est activé.",
    "unknown": "Le résultat de la maintenance est incertain. Actualisez et vérifiez l’état actuel avant d’examiner une autre action. Ne répétez pas automatiquement.",
    "receipts": "Reçus archivés",
    "policies": "Historique des politiques",
    "exportPage": "Exporter cette page",
    "evidenceOnly": "Preuves historiques uniquement. Ces enregistrements ne donnent aucun accès actuel.",
    "allowedThen": "Autorisé dans cet instantané",
    "deniedThen": "Refusé dans cet instantané"
  },
  "es": {
    "archive": "Archivar recibos resueltos",
    "archiveHint": "Conservar los recibos indicados en el historial del espacio y liberar el registro activo. La configuración y el acceso no cambian.",
    "retire": "Retirar decisiones obsoletas",
    "retireHint": "Archivar la política anterior y retirar solo las decisiones obsoletas indicadas. Los puntos actuales y activos están protegidos. Esto no habilita el acceso.",
    "unknown": "El resultado del mantenimiento no está confirmado. Actualice e inspeccione el estado actual antes de revisar otra acción. No repita automáticamente.",
    "receipts": "Recibos archivados",
    "policies": "Historial de políticas",
    "exportPage": "Exportar esta página",
    "evidenceOnly": "Solo evidencia histórica. Estos registros nunca conceden acceso actual.",
    "allowedThen": "Permitido en esta instantánea",
    "deniedThen": "Denegado en esta instantánea"
  },
  "ar": {
    "archive": "أرشفة الإيصالات المحسومة",
    "archiveHint": "الاحتفاظ بالإيصالات المدرجة في سجل مساحة العمل وإفراغ سعة في السجل النشط. تبقى الإعدادات وصلاحيات الوصول كما هي.",
    "retire": "إزالة القرارات القديمة",
    "retireHint": "أرشفة السياسة السابقة وإزالة القرارات القديمة المدرجة فقط. نقاط الاتصال الحالية والنشطة محمية. لا يؤدي هذا إلى تفعيل الوصول.",
    "unknown": "نتيجة الصيانة غير مؤكدة. حدّث الحالة الحالية وافحصها قبل مراجعة إجراء آخر. لا تكرر الإجراء تلقائيًا.",
    "receipts": "الإيصالات المؤرشفة",
    "policies": "سجل السياسات",
    "exportPage": "تصدير هذه الصفحة",
    "evidenceOnly": "أدلة تاريخية فقط. لا تمنح هذه السجلات أي وصول حالي.",
    "allowedThen": "مسموح في هذه اللقطة",
    "deniedThen": "مرفوض في هذه اللقطة"
  },
  "sw": {
    "archive": "Hifadhi risiti zilizotatuliwa kwenye kumbukumbu",
    "archiveHint": "Weka risiti zilizoorodheshwa katika historia ya nafasi ya kazi na punguza daftari linalotumika. Usanidi na ufikiaji havibadiliki.",
    "retire": "Ondoa maamuzi yaliyopitwa na wakati",
    "retireHint": "Hifadhi sera ya awali na uondoe tu maamuzi ya zamani yaliyoorodheshwa. Vituo vya sasa na vinavyotumika vinalindwa. Hii haiwezeshi ufikiaji.",
    "unknown": "Matokeo ya matengenezo hayajathibitishwa. Sasisha na ukague hali ya sasa kabla ya kukagua hatua nyingine. Usirudie kiotomatiki.",
    "receipts": "Risiti zilizohifadhiwa",
    "policies": "Historia ya sera",
    "exportPage": "Hamisha ukurasa huu",
    "evidenceOnly": "Ushahidi wa kihistoria tu. Rekodi hizi hazitoi ufikiaji wa sasa.",
    "allowedThen": "Imeruhusiwa katika nakala hii",
    "deniedThen": "Imekataliwa katika nakala hii"
  },
  "sn": {
    "archive": "Chengeta marisiti apedzwa munhoroondo",
    "archiveHint": "Chengeta marisiti akanyorwa munhoroondo yenzvimbo yebasa uye sunungura nzvimbo mugwaro rinoshanda. Marongero nekuwana hazvichinji.",
    "retire": "Bvisa zvisarudzo zvekare",
    "retireHint": "Chengeta mutemo wekare uye bvisa chete zvisarudzo zvekare zvakanyorwa. Nzvimbo dzazvino nedziri kushanda dzinodzivirirwa. Izvi hazvipi mvumo yekuwana.",
    "unknown": "Mhedzisiro yekugadzirisa haisati yasimbiswa. Rodhazve uye ongorora mamiriro azvino usati waongorora chimwe chiito. Usadzokorora otomatiki.",
    "receipts": "Marisiti akachengetwa",
    "policies": "Nhoroondo yemitemo",
    "exportPage": "Tumira peji rino",
    "evidenceOnly": "Uchapupu hwekare chete. Zvinyorwa izvi hazvipi mvumo yekuwana zvino.",
    "allowedThen": "Zvakabvumirwa mukopi iyi",
    "deniedThen": "Zvakarambwa mukopi iyi"
  },
  "nd": {
    "archive": "Gcina amarisidi axazululiweyo embalini",
    "archiveHint": "Gcina amarisidi abhalwe lapha embalini yendawo yokusebenza futhi uvule indawo kurejista esebenzayo. Izilungiselelo lokufinyelela akuguquki.",
    "retire": "Susa izinqumo ezindala",
    "retireHint": "Gcina umthetho wangaphambilini futhi ususe kuphela izinqumo ezindala ezibhaliweyo. Izindawo zamanje lezisebenzayo zivikelekile. Lokhu akuvuli ukufinyelela.",
    "unknown": "Umphumela wokulungisa awuqinisekiswanga. Vuselela uhlole isimo samanje ungakahloli esinye isenzo. Ungaphindi ngokuzenzakalela.",
    "receipts": "Amarisidi agciniweyo",
    "policies": "Imbali yemithetho",
    "exportPage": "Thumela leli khasi",
    "evidenceOnly": "Ubufakazi bembali kuphela. Amarekhodi la awaniki ukufinyelela kwamanje.",
    "allowedThen": "Kuvunyelwe kule khophi",
    "deniedThen": "Kwaliwe kule khophi"
  },
  "zu": {
    "archive": "Gcina amarisidi axazululiwe emlandweni",
    "archiveHint": "Gcina amarisidi asohlwini emlandweni wendawo yokusebenza futhi uvule isikhala kurejista esebenzayo. Izilungiselelo nokufinyelela akushintshi.",
    "retire": "Susa izinqumo ezindala",
    "retireHint": "Gcina inqubomgomo yangaphambilini futhi ususe kuphela izinqumo ezindala ezisohlwini. Izindawo zamanje nezisebenzayo zivikelekile. Lokhu akuvumeli ukufinyelela.",
    "unknown": "Umphumela wokulungisa awuqinisekisiwe. Vuselela futhi uhlole isimo samanje ngaphambi kokubuyekeza esinye isenzo. Ungaphindi ngokuzenzakalelayo.",
    "receipts": "Amarisidi agciniwe",
    "policies": "Umlando wezinqubomgomo",
    "exportPage": "Thumela leli khasi",
    "evidenceOnly": "Ubufakazi bomlando kuphela. Lawa marekhodi awaniki ukufinyelela kwamanje.",
    "allowedThen": "Kuvunyelwe kule khophi",
    "deniedThen": "Kunqatshiwe kule khophi"
  },
  "de": {
    "archive": "Abgeschlossene Belege archivieren",
    "archiveHint": "Die aufgeführten Belege im Arbeitsbereichsverlauf behalten und das aktive Journal entlasten. Konfiguration und Zugriff bleiben unverändert.",
    "retire": "Veraltete Entscheidungen entfernen",
    "retireHint": "Die bisherige Richtlinie archivieren und nur die aufgeführten veralteten Entscheidungen entfernen. Aktuelle und aktive Endpunkte sind geschützt. Dies aktiviert keinen Zugriff.",
    "unknown": "Das Wartungsergebnis ist unbestätigt. Aktualisieren Sie den Status und prüfen Sie ihn vor einer weiteren Aktion. Nicht automatisch wiederholen.",
    "receipts": "Archivierte Belege",
    "policies": "Richtlinienverlauf",
    "exportPage": "Diese Seite exportieren",
    "evidenceOnly": "Nur historische Nachweise. Diese Datensätze gewähren keinen aktuellen Zugriff.",
    "allowedThen": "In diesem Stand erlaubt",
    "deniedThen": "In diesem Stand verweigert"
  }
});
