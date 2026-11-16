import {defineMessages} from './catalog';
import type {LocaleCode} from './index';
const labels=defineMessages<string[]>('i18n/document-synthesis:labels',{
  "en": [
    "Analyse documents",
    "Summary",
    "Requirements",
    "Create draft",
    "Draft needs review",
    "AI-generated draft. Check every claim against its sources. Unread content is not included.",
    "Source passages",
    "Conditions",
    "Exceptions",
    "No candidate text in this section.",
    "Analysis history"
  ],
  "fr": [
    "Analyser des documents",
    "Résumé",
    "Exigences",
    "Créer un brouillon",
    "Brouillon à vérifier",
    "Brouillon généré par IA. Vérifiez chaque affirmation dans ses sources. Le contenu non lu est exclu.",
    "Passages sources",
    "Conditions",
    "Exceptions",
    "Aucun texte proposé dans cette section.",
    "Historique des analyses"
  ],
  "es": [
    "Analizar documentos",
    "Resumen",
    "Requisitos",
    "Crear borrador",
    "Borrador pendiente de revisión",
    "Borrador generado por IA. Comprueba cada afirmación en sus fuentes. El contenido no leído queda excluido.",
    "Pasajes de las fuentes",
    "Condiciones",
    "Excepciones",
    "No hay texto propuesto en esta sección.",
    "Historial de análisis"
  ],
  "ar": [
    "تحليل المستندات",
    "ملخص",
    "المتطلبات",
    "إنشاء مسودة",
    "المسودة بحاجة إلى مراجعة",
    "مسودة أنشأها الذكاء الاصطناعي. تحقق من كل ادعاء بالرجوع إلى مصادره. المحتوى غير المقروء غير مشمول.",
    "مقتطفات المصادر",
    "الشروط",
    "الاستثناءات",
    "لا يوجد نص مقترح في هذا القسم.",
    "سجل التحليلات"
  ],
  "sw": [
    "Changanua hati",
    "Muhtasari",
    "Mahitaji",
    "Unda rasimu",
    "Rasimu inahitaji ukaguzi",
    "Rasimu iliyotengenezwa na AI. Hakiki kila dai kwa kutumia vyanzo vyake. Maudhui ambayo hayajasomwa hayajajumuishwa.",
    "Dondoo za vyanzo",
    "Masharti",
    "Vighairi",
    "Hakuna maandishi yaliyopendekezwa katika sehemu hii.",
    "Historia ya uchanganuzi"
  ],
  "sn": [
    "Ongorora magwaro",
    "Pfupiso",
    "Zvinodiwa",
    "Gadzira gwaro rekutanga",
    "Gwaro rekutanga rinoda kuongororwa",
    "Gwaro rekutanga rakagadzirwa neAI. Tarisa chirevo chimwe nechimwe uchishandisa magwaro acho. Zvisina kuverengwa hazvina kuiswa.",
    "Zvikamu zvemagwaro akashandiswa",
    "Mamiriro anodiwa",
    "Zvisingabatanidzwi",
    "Hapana mashoko akurudzirwa muchikamu ichi.",
    "Nhoroondo yeongororo"
  ],
  "nd": [
    "Hlaziya imibhalo",
    "Isifinyezo",
    "Izidingo",
    "Dala uhlaka",
    "Uhlaka ludinga ukuhlolwa",
    "Uhlaka olwenziwe yi-AI. Hlola isitatimende ngasinye emithonjeni yaso. Okungabalwanga akufakwanga.",
    "Izingxenye zemithombo",
    "Imibandela",
    "Okungafakwanga",
    "Akulambhalo ohlongozwayo kulesi sigaba.",
    "Imbali yokuhlaziya"
  ],
  "zu": [
    "Hlaziya imibhalo",
    "Isifinyezo",
    "Izidingo",
    "Dala uhlaka",
    "Uhlaka ludinga ukubuyekezwa",
    "Uhlaka olwenziwe yi-AI. Hlola isitatimende ngasinye emithonjeni yaso. Okuqukethwe okungafundiwe akufakiwe.",
    "Izingcaphuno zemithombo",
    "Imibandela",
    "Okuhlukile",
    "Awukho umbhalo ohlongozwayo kulesi sigaba.",
    "Umlando wokuhlaziya"
  ],
  "de": [
    "Dokumente analysieren",
    "Zusammenfassung",
    "Anforderungen",
    "Entwurf erstellen",
    "Entwurf prüfen",
    "KI-generierter Entwurf. Prüfen Sie jede Aussage anhand ihrer Quellen. Ungelesene Inhalte sind nicht enthalten.",
    "Quellenauszüge",
    "Bedingungen",
    "Ausnahmen",
    "Kein Textvorschlag in diesem Abschnitt.",
    "Analyseverlauf"
  ]
});
export function documentSynthesis(locale:LocaleCode){const [title,summary,requirements,create,review,help,evidence,conditions,exceptions,empty,history]=labels[locale];return {title,summary,requirements,create,review,help,evidence,conditions,exceptions,empty,history};}
