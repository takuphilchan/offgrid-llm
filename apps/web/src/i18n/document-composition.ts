import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const labels=defineMessages<string[]>('i18n/document-composition:labels',{
  "en": [
    "Summary report",
    "Compose summary",
    "Source reviews have changed. Create a new summary to include them.",
    "Merges completed",
    "Source sections not represented",
    "Section drafts"
  ],
  "fr": [
    "Rapport de synthèse",
    "Rédiger la synthèse",
    "Les vérifications des sources ont changé. Créez une nouvelle synthèse pour les prendre en compte.",
    "Fusions terminées",
    "Sections sources non représentées",
    "Brouillons des sections"
  ],
  "es": [
    "Informe de síntesis",
    "Redactar síntesis",
    "Las revisiones de las fuentes han cambiado. Crea una nueva síntesis para incorporarlas.",
    "Combinaciones completadas",
    "Secciones de origen no representadas",
    "Borradores de las secciones"
  ],
  "ar": [
    "تقرير ملخّص",
    "إنشاء الملخّص",
    "تغيّرت مراجعات المصادر. أنشئ ملخّصًا جديدًا ليشمل هذه التغييرات.",
    "عمليات الدمج المكتملة",
    "أقسام المصادر غير الممثلة",
    "مسودات الأقسام"
  ],
  "sw": [
    "Ripoti ya muhtasari",
    "Tengeneza muhtasari",
    "Ukaguzi wa vyanzo umebadilika. Tengeneza muhtasari mpya ili ujumuishe mabadiliko hayo.",
    "Miunganiko iliyokamilika",
    "Sehemu za vyanzo ambazo hazijajumuishwa",
    "Rasimu za sehemu"
  ],
  "sn": [
    "Mushumo wepfupiso",
    "Gadzira pfupiso",
    "Ongororo dzezvinyorwa zvabva mashoko dzachinja. Gadzira pfupiso itsva kuti ibatanidze shanduko idzi.",
    "Kubatanidza kwapera",
    "Zvikamu zvezvinyorwa zvisina kubatanidzwa",
    "Zvinyorwa zvekutanga zvezvikamu"
  ],
  "nd": [
    "Umbiko ofingqiweyo",
    "Yenza isifinqo",
    "Ukuhlolwa kwemithombo sekutshintshile. Yenza isifinqo esitsha ukuze sihlanganise lezi zinguquko.",
    "Ukuhlanganisa okuphelileyo",
    "Izingxenye zemithombo ezingafakwanga",
    "Izinhlaka zezingxenye"
  ],
  "zu": [
    "Umbiko ofingqiwe",
    "Dala isifinyezo",
    "Ukubuyekezwa kwemithombo kushintshile. Dala isifinyezo esisha ukuze sifake lezi zinguquko.",
    "Ukuhlanganisa okuqediwe",
    "Izingxenye zemithombo ezingafakiwe",
    "Izinhlaka zezingxenye"
  ],
  "de": [
    "Zusammenfassender Bericht",
    "Zusammenfassung erstellen",
    "Die Quellenprüfungen haben sich geändert. Erstellen Sie eine neue Zusammenfassung, um sie zu berücksichtigen.",
    "Zusammenführungen abgeschlossen",
    "Nicht berücksichtigte Quellabschnitte",
    "Abschnittsentwürfe"
  ]
});
export function compositionCopy(locale:LocaleCode){const [title,create,stale,progress,omitted,drafts]=labels[locale];return {title,create,stale,progress,omitted,drafts};}
