import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

const labels = defineMessages<[string, string, string, string]>("i18n/document-progress:labels", {
 en: ['Waiting for other documents', 'Reading document', 'Making document searchable', 'Other documents are being processed. Your saved source is unchanged; try again after the current work finishes.'],
 fr: ['En attente des autres documents', 'Lecture du document', 'Préparation de la recherche', 'D’autres documents sont en cours de traitement. Votre source enregistrée reste intacte ; réessayez après leur traitement.'],
 de: ['Warten auf andere Dokumente', 'Dokument wird gelesen', 'Dokument wird durchsuchbar gemacht', 'Andere Dokumente werden verarbeitet. Ihre gespeicherte Quelle bleibt unverändert; versuchen Sie es danach erneut.'],
 es: ['Esperando a otros documentos', 'Leyendo el documento', 'Preparando el documento para buscar', 'Se están procesando otros documentos. Tu fuente guardada no ha cambiado; inténtalo al terminar.'],
 ar: ['بانتظار المستندات الأخرى', 'جارٍ قراءة المستند', 'جارٍ تجهيز المستند للبحث', 'تجري معالجة مستندات أخرى. مصدرك المحفوظ لم يتغير؛ أعد المحاولة بعد انتهاء العمل الحالي.'],
 sw: ['Inasubiri hati nyingine', 'Inasoma hati', 'Inaandaa hati kwa utafutaji', 'Hati nyingine zinachakatwa. Chanzo chako kilichohifadhiwa hakijabadilika; jaribu tena kazi ya sasa ikikamilika.'],
 sn: ['Kumirira mamwe magwaro', 'Kuverenga gwaro', 'Kugadzirira gwaro kuti ritsvagwe', 'Mamwe magwaro ari kugadziriswa. Gwaro rako rakachengetwa harina kuchinja; edza zvakare kana basa riripo rapera.'],
 nd: ['Kulindwe eminye imibhalo', 'Kufundwa umbhalo', 'Umbhalo ulungiselwa ukuseshwa', 'Eminye imibhalo iyasetshenzwa. Umbhalo wakho ogciniweyo awutshintshanga; zama futhi nxa umsebenzi wamanje usuphelile.'],
 zu: ['Kulindwe eminye imibhalo', 'Kufundwa umbhalo', 'Umbhalo ulungiselelwa ukuseshwa', 'Eminye imibhalo iyacutshungulwa. Umbhalo wakho ogciniwe awushintshile; zama futhi lapho umsebenzi wamanje usuphelile.'],
});

export function documentProgress(locale: LocaleCode, state: string) {
 const index = { queued: 0, extracting: 1, indexing: 2 }[state];
 return index === undefined ? undefined : labels[locale][index];
}
export const documentBusy = (locale: LocaleCode) => labels[locale][3];
