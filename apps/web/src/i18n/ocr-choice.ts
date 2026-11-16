import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const labels=defineMessages<string[]>('i18n/ocr-choice:labels',{
 en:['OCR language','Applies to newly added files. Saved files keep their original language selection.','Close','No OCR pack available'],
 fr:['Langue OCR','S’applique aux nouveaux fichiers. Les fichiers enregistrés conservent la langue choisie à l’origine.','Fermer','Aucun pack OCR disponible'],
 de:['OCR-Sprache','Gilt für neu hinzugefügte Dateien. Gespeicherte Dateien behalten ihre ursprüngliche Sprachauswahl.','Schließen','Kein OCR-Sprachpaket verfügbar'],
 es:['Idioma de OCR','Se aplica a los archivos nuevos. Los archivos guardados conservan el idioma elegido originalmente.','Cerrar','No hay un paquete de OCR disponible'],
 ar:['لغة التعرف على النص','ينطبق على الملفات المضافة حديثًا. تحتفظ الملفات المحفوظة باختيار اللغة الأصلي.','إغلاق','لا تتوفر حزمة للتعرف على النص'],
 sw:['Lugha ya OCR','Inatumika kwa faili zinazoongezwa sasa. Faili zilizohifadhiwa zinabaki na lugha iliyochaguliwa awali.','Funga','Hakuna kifurushi cha OCR kinachopatikana'],
 sn:['Mutauro weOCR','Zvinoshanda kumafaira matsva anowedzerwa. Mafaira akachengetwa anochengeta mutauro wakasarudzwa pakutanga.','Vhara','Hapana paki yeOCR iripo'],
 nd:['Ulimi lwe-OCR','Kusebenza kumafayela amatsha angeziweyo. Amafayela agciniweyo agcina ulimi olukhethwe ekuqaleni.','Vala','Akulaphakheji ye-OCR etholakalayo'],
 zu:['Ulimi lwe-OCR','Kusebenza kumafayela asanda kwengezwa. Amafayela agciniwe agcina ulimi olukhethwe ekuqaleni.','Vala','Ayikho iphakheji ye-OCR etholakalayo'],
});
export function ocrChoice(locale:LocaleCode){const [language,hint,close,unavailable]=labels[locale];return{language,hint,close,unavailable};}
