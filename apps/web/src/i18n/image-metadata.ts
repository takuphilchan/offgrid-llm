import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const notice=defineMessages('i18n/image-metadata:notice',{
 en:'This image contains multiple frames or unreadable orientation data. Export a still PNG/JPEG in the correct orientation, then attach it again.',
 fr:'Cette image contient plusieurs vues ou des données d’orientation illisibles. Exportez une image fixe PNG/JPEG dans le bon sens, puis joignez-la à nouveau.',
 de:'Dieses Bild enthält mehrere Einzelbilder oder unlesbare Ausrichtungsdaten. Exportieren Sie ein korrekt ausgerichtetes Standbild als PNG/JPEG und hängen Sie es erneut an.',
 es:'Esta imagen contiene varios fotogramas o datos de orientación ilegibles. Exporta una imagen fija PNG/JPEG con la orientación correcta y vuelve a adjuntarla.',
 ar:'تحتوي هذه الصورة على إطارات متعددة أو بيانات اتجاه غير قابلة للقراءة. صدّر صورة ثابتة بصيغة PNG/JPEG بالاتجاه الصحيح، ثم أرفقها مجددًا.',
 sw:'Picha hii ina fremu nyingi au taarifa za mwelekeo zisizosomeka. Hifadhi picha tuli ya PNG/JPEG iliyo katika mwelekeo sahihi, kisha uiambatishe tena.',
 sn:'Mufananidzo uyu une mafuremu akawanda kana mashoko asingaverengeki ekuti unofanira kutarisa kupi. Chengeta mufananidzo mumwe usingafambi sePNG/JPEG wakatarisa zvakanaka, wozouisa zvakare.',
 nd:'Umfanekiso lo ulamafreyimu amanengi kumbe ulwazi olungafundekiyo lwendlela obheke ngayo. Gcina umfanekiso onganyakaziyo wePNG/JPEG ubheke ngendlela eqondileyo, uphinde uwunamathisele.',
 zu:'Lesi sithombe sinamafreyimu amaningi noma imininingwane yokuma engafundeki. Gcina isithombe esimile sePNG/JPEG sime ngendlela efanele, bese usinamathisela futhi.',
});
export const imageMetadataNotice=(locale:LocaleCode)=>notice[locale];
