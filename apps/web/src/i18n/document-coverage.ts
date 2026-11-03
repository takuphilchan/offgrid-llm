import type { LocaleCode } from './index';

const labels: Record<LocaleCode, string> = {
 en: 'Text ready. Images and figures have not been interpreted.',
 fr: 'Texte prêt. Les images et figures n’ont pas été interprétées.',
 de: 'Text bereit. Bilder und Abbildungen wurden nicht interpretiert.',
 es: 'Texto listo. Las imágenes y figuras no se han interpretado.',
 ar: 'النص جاهز. لم تُفسَّر الصور والأشكال.',
 sw: 'Maandishi yako tayari. Picha na vielelezo havijatafsiriwa.',
 sn: 'Mashoko agadzirira. Mifananidzo nemadhayagiramu hazvina kududzirwa.',
 nd: 'Umbhalo usulungile. Imifanekiso lemidayagiramu akukachazwa.',
 zu: 'Umbhalo usulungile. Izithombe nemidwebo akukahunyushwa.',
};
export const documentCoverage = (locale: LocaleCode) => labels[locale];

const excerpts: Record<LocaleCode,string> = {
 en:'Based on selected document passages, not a complete review. Open the source to inspect the full text.',
 fr:'Basé sur des passages sélectionnés, pas sur une lecture complète. Ouvrez la source pour consulter tout le texte.',
 de:'Basiert auf ausgewählten Passagen, nicht auf einer vollständigen Prüfung. Öffnen Sie die Quelle für den gesamten Text.',
 es:'Basado en pasajes seleccionados, no en una revisión completa. Abra la fuente para consultar todo el texto.',
 ar:'تستند الإجابة إلى مقاطع مختارة، وليست مراجعة كاملة. افتح المصدر للاطلاع على النص الكامل.',
 sw:'Jibu linatumia sehemu zilizochaguliwa, si ukaguzi kamili. Fungua chanzo kuona maandishi yote.',
 sn:'Mhinduro inoshandisa zvikamu zvakasarudzwa, kwete ongororo yose. Vhura gwaro kuti uone mashoko ose.',
 nd:'Impendulo isebenzisa izingxenye ezikhethiweyo, hatshi ukuhlola konke. Vula umthombo ubone umbhalo wonke.',
 zu:'Impendulo isebenzisa izingxenye ezikhethiwe, hhayi ukuhlola konke. Vula umthombo ukuze ubone umbhalo wonke.',
};
export const documentExcerptCoverage = (locale:LocaleCode) => excerpts[locale];
