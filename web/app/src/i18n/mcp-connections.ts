import type { LocaleCode } from './index';

// As with other interface translations, native-speaker review is a release gate.
const copy = {
  en: ['Remove connection', 'Remove “{name}”?', 'This removes the saved connection and its tools from OffGrid. Task history is kept. Tasks using these tools may fail. Actions already sent to the server are not undone.', 'Connection removed.', 'Connected', 'Disconnected', 'Disabled'],
  fr: ['Supprimer la connexion', 'Supprimer « {name} » ?', 'La connexion enregistrée et ses outils seront supprimés d’OffGrid. L’historique des tâches est conservé. Les tâches utilisant ces outils peuvent échouer. Les actions déjà envoyées au serveur ne sont pas annulées.', 'Connexion supprimée.', 'Connecté', 'Déconnecté', 'Désactivé'],
  es: ['Eliminar conexión', '¿Eliminar «{name}»?', 'Se eliminarán de OffGrid la conexión guardada y sus herramientas. Se conserva el historial de tareas. Las tareas que usen estas herramientas podrían fallar. Las acciones ya enviadas al servidor no se deshacen.', 'Conexión eliminada.', 'Conectado', 'Desconectado', 'Desactivado'],
  de: ['Verbindung entfernen', '„{name}“ entfernen?', 'Die gespeicherte Verbindung und ihre Werkzeuge werden aus OffGrid entfernt. Der Aufgabenverlauf bleibt erhalten. Aufgaben mit diesen Werkzeugen können fehlschlagen. Bereits gesendete Aktionen werden nicht rückgängig gemacht.', 'Verbindung entfernt.', 'Verbunden', 'Getrennt', 'Deaktiviert'],
  ar: ['إزالة الاتصال', 'إزالة «{name}»؟', 'سيُزال الاتصال المحفوظ وأدواته من OffGrid. سيُحتفظ بسجل المهام. قد تفشل المهام التي تستخدم هذه الأدوات. لن يتم التراجع عن الإجراءات التي أُرسلت إلى الخادم بالفعل.', 'تمت إزالة الاتصال.', 'متصل', 'غير متصل', 'معطّل'],
  sw: ['Ondoa muunganisho', 'Ondoa “{name}”?', 'Hii inaondoa muunganisho uliohifadhiwa na zana zake kutoka OffGrid. Historia ya kazi inabaki. Kazi zinazotumia zana hizi zinaweza kushindwa. Vitendo vilivyotumwa tayari kwa seva havitatenguliwa.', 'Muunganisho umeondolewa.', 'Imeunganishwa', 'Haijaunganishwa', 'Imezimwa'],
  sn: ['Bvisa kubatana', 'Bvisa “{name}”?', 'Izvi zvinobvisa kubatana kwakachengetwa nematurusi ako kubva muOffGrid. Nhoroondo yemabasa inochengetwa. Mabasa anoshandisa maturusi aya anogona kutadza. Zviito zvakatotumirwa kuseva hazvidzoserwi shure.', 'Kubatana kwabviswa.', 'Yakabatana', 'Haina kubatana', 'Yakadzimwa'],
  nd: ['Susa ukuxhumana', 'Susa “{name}”?', 'Lokhu kususa ukuxhumana okulondoloziweyo lamathuluzi ako ku-OffGrid. Imbali yemisebenzi iyagcinwa. Imisebenzi esebenzisa amathuluzi la ingahluleka. Izenzo esezithunyelwe kuseva azihlehliswa.', 'Ukuxhumana kususiwe.', 'Kuxhunyiwe', 'Akuxhunyiwe', 'Kuvaliwe'],
  zu: ['Susa uxhumano', 'Susa “{name}”?', 'Lokhu kususa uxhumano olugciniwe namathuluzi alo ku-OffGrid. Umlando wemisebenzi uyagcinwa. Imisebenzi esebenzisa la mathuluzi ingahluleka. Izenzo esezithunyelwe kuseva azihlehliswa.', 'Uxhumano lususiwe.', 'Kuxhunyiwe', 'Akuxhunyiwe', 'Kuvaliwe'],
} satisfies Record<LocaleCode, string[]>;

export function mcpConnectionText(locale: LocaleCode) {
  const [remove, title, body, removed, connected, disconnected, disabled] = copy[locale];
  return { remove, title, body, removed, connected, disconnected, disabled };
}
