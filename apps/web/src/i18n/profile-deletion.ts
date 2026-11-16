import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const copy=defineMessages('i18n/profile-deletion:copy',{
  "en": {
    "body": "Delete this profile, {jobs} saved jobs and {memory} memory entries? This cannot be undone. Shared files, exports and backups are kept.",
    "blocked": "Deletion blocked: {jobs} active or linked jobs; {dependencies} dependent profiles or schedules."
  },
  "fr": {
    "body": "Supprimer ce profil, {jobs} tâches enregistrées et {memory} entrées de mémoire ? Cette action est irréversible. Les fichiers partagés, exports et sauvegardes sont conservés.",
    "blocked": "Suppression bloquée : {jobs} tâches actives ou liées ; {dependencies} profils ou planifications dépendants."
  },
  "de": {
    "body": "Dieses Profil, {jobs} gespeicherte Aufgaben und {memory} Gedächtniseinträge löschen? Dies kann nicht rückgängig gemacht werden. Gemeinsame Dateien, Exporte und Sicherungen bleiben erhalten.",
    "blocked": "Löschen blockiert: {jobs} aktive oder verknüpfte Aufgaben; {dependencies} abhängige Profile oder Zeitpläne."
  },
  "es": {
    "body": "¿Eliminar este perfil, {jobs} tareas guardadas y {memory} entradas de memoria? No se puede deshacer. Se conservan los archivos compartidos, las exportaciones y las copias de seguridad.",
    "blocked": "Eliminación bloqueada: {jobs} tareas activas o vinculadas; {dependencies} perfiles o programaciones dependientes."
  },
  "ar": {
    "body": "هل تريد حذف هذا الملف و{jobs} من المهام المحفوظة و{memory} من إدخالات الذاكرة؟ لا يمكن التراجع. تُحفظ الملفات المشتركة والصادرات والنسخ الاحتياطية.",
    "blocked": "الحذف محظور: {jobs} من المهام النشطة أو المرتبطة؛ و{dependencies} من الملفات أو الجداول التابعة."
  },
  "sw": {
    "body": "Futa wasifu huu, kazi {jobs} zilizohifadhiwa na rekodi {memory} za kumbukumbu? Hatua hii haiwezi kutenduliwa. Faili za pamoja, zilizohamishwa na nakala rudufu zitabaki.",
    "blocked": "Kufuta kumezuiwa: kazi {jobs} zinazoendelea au zilizounganishwa; wasifu au ratiba {dependencies} zinazotegemea huu."
  },
  "sn": {
    "body": "Dzima profile iyi, mabasa {jobs} akachengetwa nezvinyorwa {memory} zvendangariro? Izvi hazvidzoreki. Mafaira akagovaniswa, akatumirwa kunze nemabackup zvinosara.",
    "blocked": "Kudzima kwavharwa: mabasa {jobs} ari kushanda kana akabatanidzwa; maprofile kana marongero {dependencies} anoenderana neiyi."
  },
  "nd": {
    "body": "Susa iphrofayili le, imisebenzi {jobs} egciniweyo lamanothi {memory} enkumbulo? Lokhu akubuyiseleki emuva. Amafayela abelwanayo, akhutshelweyo lamakhophi esipele ayasala.",
    "blocked": "Ukususa kuvinjiwe: imisebenzi {jobs} esebenzayo kumbe exhunyiweyo; amaphrofayili kumbe amashejuli {dependencies} ancike kuyo."
  },
  "zu": {
    "body": "Susa le phrofayela, imisebenzi {jobs} egciniwe nokufakiwe {memory} kwenkumbulo? Lokhu akuhlehliseki. Amafayela abiwe, athunyelwe ngaphandle nezipele kuyagcinwa.",
    "blocked": "Ukususa kuvinjiwe: imisebenzi {jobs} esebenzayo noma exhunyiwe; amaphrofayela noma amashejuli {dependencies} ancike kuyo."
  }
});
export const profileDeletion=(locale:LocaleCode)=>copy[locale];
