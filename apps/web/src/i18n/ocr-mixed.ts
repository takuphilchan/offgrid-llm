import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const labels=defineMessages<string[]>('i18n/ocr-mixed:labels',{
 en:['Additional language','Add language','Remove language','Choose up to three languages present in the document. The first is the primary language.',"Selected languages installed. OCR remains experimental; review extracted text."],
 fr:['Langue supplémentaire','Ajouter une langue','Retirer la langue','Choisissez jusqu’à trois langues présentes dans le document. La première est la langue principale.',"Les langues sélectionnées sont installées. L’OCR reste expérimental ; vérifiez le texte extrait."],
 de:['Weitere Sprache','Sprache hinzufügen','Sprache entfernen','Wählen Sie bis zu drei Sprachen aus dem Dokument. Die erste ist die Hauptsprache.',"Die ausgewählten Sprachen sind installiert. OCR bleibt experimentell; prüfen Sie den erkannten Text."],
 es:['Idioma adicional','Añadir idioma','Quitar idioma','Elige hasta tres idiomas presentes en el documento. El primero es el idioma principal.',"Los idiomas seleccionados están instalados. El OCR sigue siendo experimental; revisa el texto extraído."],
 ar:['لغة إضافية','إضافة لغة','إزالة اللغة','اختر ما يصل إلى ثلاث لغات موجودة في المستند. اللغة الأولى هي اللغة الأساسية.',"تم تثبيت اللغات المحددة. يظل التعرف على النص تجريبيًا؛ راجع النص المستخرج."],
 sw:['Lugha ya ziada','Ongeza lugha','Ondoa lugha','Chagua hadi lugha tatu zilizopo kwenye hati. Ya kwanza ndiyo lugha kuu.',"Lugha zilizochaguliwa zimesakinishwa. OCR bado ni ya majaribio; kagua maandishi yaliyotolewa."],
 sn:['Mumwe mutauro','Wedzera mutauro','Bvisa mutauro','Sarudza mitauro inosvika mitatu iri mugwaro. Wokutanga ndiwo mutauro mukuru.',"Mitauro yakasarudzwa yaiswa. OCR ichiri kuyedzwa; ongorora mashoko aburitswa."],
 nd:['Olunye ulimi','Engeza ulimi','Susa ulimi','Khetha izilimi ezingafika kwezintathu ezisedokhumentini. Olokuqala yilo ulimi oluyinhloko.',"Izilimi ezikhethiweyo zifakiwe. I-OCR isahlolwa; hlola umbhalo okhitshiweyo."],
 zu:['Ulimi olwengeziwe','Engeza ulimi','Susa ulimi','Khetha izilimi ezingafika kwezintathu ezikhona kudokhumenti. Olokuqala luwulimi oluyinhloko.',"Izilimi ezikhethiwe zifakiwe. I-OCR isahlolwa; hlola umbhalo okhishiwe."],
});
export function ocrMixed(locale:LocaleCode){const [additional,add,remove,hint,installed]=labels[locale];return{additional,add,remove,hint,installed};}
