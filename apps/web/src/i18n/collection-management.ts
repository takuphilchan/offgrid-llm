import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const copy=defineMessages('i18n/collection-management:copy',{
  "en": {
    "title": "Collections",
    "all": "All collections",
    "name": "Collection name",
    "people": "People",
    "review": "Review changes",
    "apply": "Save changes",
    "help": "Changes apply to every document in this collection. Downloaded copies cannot be recalled.",
    "readOnly": "Only the collection owner can change access.",
    "move": "Move to collection",
    "current": "Current collection",
    "saved": "Changes saved"
  },
  "fr": {
    "title": "Collections",
    "all": "Toutes les collections",
    "name": "Nom de la collection",
    "people": "Personnes",
    "review": "Vérifier les modifications",
    "apply": "Enregistrer les modifications",
    "help": "Les modifications s’appliquent à tous les documents de cette collection. Les copies téléchargées ne peuvent pas être récupérées.",
    "readOnly": "Seul le propriétaire de la collection peut modifier les accès.",
    "move": "Déplacer vers une collection",
    "current": "Collection actuelle",
    "saved": "Modifications enregistrées"
  },
  "de": {
    "title": "Sammlungen",
    "all": "Alle Sammlungen",
    "name": "Name der Sammlung",
    "people": "Personen",
    "review": "Änderungen prüfen",
    "apply": "Änderungen speichern",
    "help": "Änderungen gelten für alle Dokumente dieser Sammlung. Heruntergeladene Kopien können nicht zurückgerufen werden.",
    "readOnly": "Nur der Eigentümer der Sammlung kann den Zugriff ändern.",
    "move": "In Sammlung verschieben",
    "current": "Aktuelle Sammlung",
    "saved": "Änderungen gespeichert"
  },
  "es": {
    "title": "Colecciones",
    "all": "Todas las colecciones",
    "name": "Nombre de la colección",
    "people": "Personas",
    "review": "Revisar cambios",
    "apply": "Guardar cambios",
    "help": "Los cambios se aplican a todos los documentos de esta colección. Las copias descargadas no pueden recuperarse.",
    "readOnly": "Solo el propietario de la colección puede cambiar el acceso.",
    "move": "Mover a una colección",
    "current": "Colección actual",
    "saved": "Cambios guardados"
  },
  "ar": {
    "title": "المجموعات",
    "all": "كل المجموعات",
    "name": "اسم المجموعة",
    "people": "الأشخاص",
    "review": "مراجعة التغييرات",
    "apply": "حفظ التغييرات",
    "help": "تنطبق التغييرات على كل مستند في هذه المجموعة. لا يمكن استرجاع النسخ التي تم تنزيلها.",
    "readOnly": "يمكن لمالك المجموعة وحده تغيير صلاحيات الوصول.",
    "move": "نقل إلى مجموعة",
    "current": "المجموعة الحالية",
    "saved": "تم حفظ التغييرات"
  },
  "sw": {
    "title": "Mikusanyiko",
    "all": "Mikusanyiko yote",
    "name": "Jina la mkusanyiko",
    "people": "Watu",
    "review": "Kagua mabadiliko",
    "apply": "Hifadhi mabadiliko",
    "help": "Mabadiliko yanahusu kila hati katika mkusanyiko huu. Nakala zilizopakuliwa haziwezi kurudishwa kwa lazima.",
    "readOnly": "Mmiliki wa mkusanyiko pekee ndiye anayeweza kubadilisha ruhusa za ufikiaji.",
    "move": "Hamisha kwenda mkusanyiko",
    "current": "Mkusanyiko wa sasa",
    "saved": "Mabadiliko yamehifadhiwa"
  },
  "sn": {
    "title": "Miunganidzwa",
    "all": "Miunganidzwa yose",
    "name": "Zita remuunganidzwa",
    "people": "Vanhu",
    "review": "Ongorora shanduko",
    "apply": "Chengeta shanduko",
    "help": "Shanduko dzinoshanda pamagwaro ose ari mumuunganidzwa uyu. Makopi akadhawunirodhwa haagoni kudzoserwa nechisimba.",
    "readOnly": "Muridzi wemuunganidzwa chete ndiye anogona kuchinja mvumo yekuwana magwaro.",
    "move": "Endesa kumuunganidzwa",
    "current": "Muunganidzwa wazvino",
    "saved": "Shanduko dzachengetwa"
  },
  "nd": {
    "title": "Amaqoqo",
    "all": "Wonke amaqoqo",
    "name": "Ibizo leqoqo",
    "people": "Abantu",
    "review": "Hlola izinguquko",
    "apply": "Gcina izinguquko",
    "help": "Izinguquko zisebenza kuwo wonke amadokhumenti akuleli qoqo. Amakhophi alandiweyo awasabuyiselwa ngenkani.",
    "readOnly": "Umnikazi weqoqo kuphela ongatshintsha imvumo yokufinyelela.",
    "move": "Hambisa eqoqweni",
    "current": "Iqoqo lamanje",
    "saved": "Izinguquko zigciniwe"
  },
  "zu": {
    "title": "Amaqoqo",
    "all": "Wonke amaqoqo",
    "name": "Igama leqoqo",
    "people": "Abantu",
    "review": "Buyekeza izinguquko",
    "apply": "Londoloza izinguquko",
    "help": "Izinguquko zisebenza kuwo wonke amadokhumenti akuleli qoqo. Amakhophi alandiwe awakwazi ukubuyiswa ngenkani.",
    "readOnly": "Umnikazi weqoqo kuphela ongashintsha izimvume zokufinyelela.",
    "move": "Hambisa eqoqweni",
    "current": "Iqoqo lamanje",
    "saved": "Izinguquko zilondoloziwe"
  }
});
export const collectionManagement=(locale:LocaleCode)=>copy[locale];
