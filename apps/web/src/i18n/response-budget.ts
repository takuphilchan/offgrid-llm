import type { LocaleCode } from './index';

export const responseBudgetHint: Record<LocaleCode, string> = {
  en: 'This limits one answer, not the conversation. History, document excerpts and the answer share the model’s context. A lower response limit leaves more room for questions and sources.',
  fr: 'Cette limite concerne une réponse, pas la conversation. L’historique, les extraits et la réponse partagent le contexte du modèle. Une limite plus basse laisse davantage de place aux questions et aux sources.',
  de: 'Dies begrenzt eine Antwort, nicht das Gespräch. Verlauf, Dokumentauszüge und Antwort teilen sich den Modellkontext. Ein niedrigeres Antwortlimit lässt mehr Platz für Fragen und Quellen.',
  es: 'Este límite se aplica a una respuesta, no a la conversación. El historial, los fragmentos y la respuesta comparten el contexto del modelo. Un límite menor deja más espacio para preguntas y fuentes.',
  ar: 'هذا حد لإجابة واحدة، وليس للمحادثة. يتشارك السجل ومقتطفات المستندات والإجابة سياق النموذج. يترك حد إجابة أقل مساحة أكبر للأسئلة والمصادر.',
  sw: 'Hiki ni kikomo cha jibu moja, si mazungumzo. Historia, dondoo za hati na jibu hushiriki muktadha wa modeli. Kikomo kidogo cha jibu huacha nafasi zaidi kwa maswali na vyanzo.',
  sn: 'Uyu muganhu wemhinduro imwe, kwete hurukuro yose. Nhoroondo, zvikamu zvegwaro nemhinduro zvinoshandisa nzvimbo imwe yemodhi. Muganhu mudiki wemhinduro unosiya nzvimbo yakawanda yemibvunzo nezvinyorwa.',
  nd: 'Lo ngumkhawulo wempendulo eyodwa, hatshi ingxoxo yonke. Imbali, izingxenye zombhalo lempendulo kwabelana umongo wemodeli. Umkhawulo omncane wempendulo utshiya indawo enengi yemibuzo lemithombo.',
  zu: 'Lona umkhawulo wempendulo eyodwa, hhayi ingxoxo yonke. Umlando, izingcaphuno zemibhalo nempendulo kwabelana ngomongo wemodeli. Umkhawulo omncane wempendulo ushiya indawo eyengeziwe yemibuzo nemithombo.',
};
