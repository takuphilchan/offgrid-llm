import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const notice=defineMessages('i18n/linked-evidence:notice',{
 en:'Some linked evidence was omitted. This excerpt may be missing a heading, table header or exception; inspect the source before relying on it.',
 fr:'Certains éléments liés ont été omis. Il peut manquer un titre, un en-tête de tableau ou une exception dans cet extrait ; vérifiez la source avant de vous y fier.',
 es:'Se omitieron algunas pruebas vinculadas. A este fragmento puede faltarle un título, un encabezado de tabla o una excepción; revisa la fuente antes de basarte en él.',
 de:'Einige verknüpfte Belege wurden ausgelassen. Diesem Auszug fehlen möglicherweise eine Überschrift, ein Tabellenkopf oder eine Ausnahme. Prüfen Sie die Quelle, bevor Sie sich darauf verlassen.',
 ar:'حُذفت بعض الأدلة المرتبطة. قد يفتقد هذا المقتطف عنوانًا أو رأس جدول أو استثناءً؛ راجع المصدر قبل الاعتماد عليه.',
 sw:'Baadhi ya ushahidi unaohusiana umeachwa. Dondoo hili linaweza kukosa kichwa, kichwa cha jedwali au hali ya kipekee; kagua chanzo kabla ya kulitegemea.',
 sn:'Humwe humbowo hwakabatana hwakasiiwa. Chidimbu ichi chingashaya musoro, musoro wetafura kana mamiriro asingateveri mutemo; ongorora gwaro rekutanga usati wavimba nacho.',
 nd:'Obunye ubufakazi obuxhumene labo butshiyiwe. Ingcaphuno le ingabe iswela isihloko, isihloko sethebula loba okuhlukile emthethweni; hlola umthombo ungakathembi yona.',
 zu:'Obunye ubufakazi obuhlobene bushiyiwe. Le ngcaphuno ingase ingabi nesihloko, isihloko sethebula noma okuhlukile emthethweni; hlola umthombo ngaphambi kokuyethemba.',
});
export const linkedEvidenceNotice=(locale:LocaleCode)=>notice[locale];
