import copy from './computer-experience.json';
import type {LocaleCode} from './index';
import {nativeAppText} from './native-app';
export const computerExperience = (locale: LocaleCode) => copy[locale];
const transferLabels:Record<LocaleCode,readonly [string,string]> = {
 en:['Download file','Attach selected file'], fr:['Télécharger le fichier','Joindre le fichier sélectionné'],
 de:['Datei herunterladen','Ausgewählte Datei anhängen'], es:['Descargar archivo','Adjuntar archivo seleccionado'],
 ar:['تنزيل ملف','إرفاق الملف المحدد'], sw:['Pakua faili','Ambatisha faili iliyochaguliwa'],
 sn:['Dhaunirodha faira','Batanidza faira rasarudzwa'], nd:['Dawuniloda ifayela','Namathisela ifayela elikhethiweyo'],
 zu:['Landa ifayela','Namathisela ifayela elikhethiwe']
};
const captureLabels:Record<LocaleCode,string> = {
 en:'Inspect current view',fr:'Inspecter la vue actuelle',de:'Aktuelle Ansicht prüfen',es:'Inspeccionar la vista actual',
 ar:'فحص العرض الحالي',sw:'Kagua mwonekano wa sasa',sn:'Ongorora zvinoonekwa',nd:'Hlola okubonakalayo',zu:'Hlola okubonakalayo'
};
export function browserActionLabel(locale: LocaleCode, name: string) {
 const native = ['computer_observe','computer_replace_text','computer_activate','computer_verify'].indexOf(name);
 if(native>=0)return nativeAppText(locale).actions[native];
 if(name==='computer_set_checked')return copy[locale].actions[6];
 if(name==='computer_verify_checked')return ({en:'Verify checkbox state',fr:'Vérifier la case',de:'Kontrollkästchen prüfen',es:'Verificar la casilla',ar:'التحقق من حالة مربع الاختيار',sw:'Thibitisha hali ya kisanduku',sn:'Simbisa mamiriro ebhokisi',nd:'Qinisekisa isimo sebhokisi',zu:'Qinisekisa isimo sebhokisi'} as Record<LocaleCode,string>)[locale];
 if(name==='computer_shortcut')return ({en:'Use application shortcut',fr:'Utiliser un raccourci',de:'Anwendungskürzel verwenden',es:'Usar atajo de la aplicación',ar:'استخدام اختصار التطبيق',sw:'Tumia njia ya mkato',sn:'Shandisa nzira pfupi',nd:'Sebenzisa indlela enqamulelayo',zu:'Sebenzisa isinqamuleli'} as Record<LocaleCode,string>)[locale];
 if(name==='browser_download')return transferLabels[locale][0];
 if(name==='browser_upload')return transferLabels[locale][1];
 if(name==='browser_capture')return captureLabels[locale];
 const index = ['browser_observe','browser_navigate','browser_fill','browser_click','browser_verify','browser_select','browser_set_checked'].indexOf(name);
 return index < 0 ? name : copy[locale].actions[index];
}
