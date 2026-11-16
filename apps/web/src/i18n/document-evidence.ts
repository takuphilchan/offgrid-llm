import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const limited = defineMessages<string>('i18n/document-evidence:limited', {
 en: 'Some document content has not been interpreted. Review the source before relying on a complete answer.',
 fr: 'Une partie du contenu du document n’a pas été interprétée. Consultez la source avant de considérer la réponse comme complète.',
 de: 'Ein Teil des Dokumentinhalts wurde nicht interpretiert. Prüfen Sie die Quelle, bevor Sie die Antwort als vollständig betrachten.',
 es: 'Parte del contenido del documento no se ha interpretado. Revisa la fuente antes de considerar completa la respuesta.',
 ar: 'لم يُفسَّر بعض محتوى المستند. راجع المصدر قبل اعتبار الإجابة كاملة.',
 sw: 'Baadhi ya maudhui ya hati hayajachanganuliwa. Kagua chanzo kabla ya kuchukulia jibu kuwa kamili.',
 sn: 'Zvimwe zviri mugwaro hazvina kududzirwa. Ongorora gwaro rekutanga usati wavimba kuti mhinduro yakakwana.',
 nd: 'Okunye okuqukethwe ngumbhalo akukachazwa. Hlola umthombo ungakathembi ukuthi impendulo iphelele.',
 zu: 'Okunye okuqukethwe yidokhumenti akukahlaziywa. Hlola umthombo ngaphambi kokuthatha impendulo njengephelele.',
});
export const documentEvidenceNotice = (locale:LocaleCode) => limited[locale];
