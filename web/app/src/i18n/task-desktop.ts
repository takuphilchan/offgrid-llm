import type { LocaleCode } from './index';

const copy: Record<LocaleCode, readonly [string,string,string,string,string,string]> = {
  en:['Access scope','Whole desktop for this task','Selected window only','Allow desktop access','Approve once in the desktop dialog. OffGrid will open and switch supported applications itself.','Reviewing desktop access…'],
  fr:['Portée de l’accès','Bureau entier pour cette tâche','Fenêtre sélectionnée uniquement','Autoriser l’accès au bureau','Confirmez une fois dans la boîte de dialogue locale. OffGrid ouvrira et changera les applications prises en charge.','Vérification de l’accès au bureau…'],
  de:['Zugriffsbereich','Gesamter Desktop für diese Aufgabe','Nur ausgewähltes Fenster','Desktopzugriff erlauben','Bestätigen Sie einmal im lokalen Dialog. OffGrid öffnet unterstützte Anwendungen und wechselt selbst zwischen ihnen.','Desktopzugriff wird geprüft…'],
  es:['Ámbito de acceso','Todo el escritorio para esta tarea','Solo la ventana seleccionada','Permitir acceso al escritorio','Confirme una vez en el diálogo local. OffGrid abrirá y cambiará las aplicaciones compatibles.','Revisando acceso al escritorio…'],
  ar:['نطاق الوصول','سطح المكتب بالكامل لهذه المهمة','النافذة المحددة فقط','السماح بالوصول إلى سطح المكتب','وافق مرة واحدة في الحوار المحلي. سيفتح OffGrid التطبيقات المدعومة ويتنقل بينها بنفسه.','جارٍ مراجعة الوصول إلى سطح المكتب…'],
  sw:['Wigo wa ufikiaji','Kompyuta nzima kwa kazi hii','Dirisha lililochaguliwa pekee','Ruhusu ufikiaji wa kompyuta','Idhinisha mara moja kwenye kisanduku cha ndani. OffGrid itafungua na kubadilisha programu zinazotumika yenyewe.','Inakagua ufikiaji wa kompyuta…'],
  sn:['Muganhu wemvumo','Desktop yose yebasa iri','Hwindo rakasarudzwa chete','Bvumira kupinda padesktop','Bvumira kamwe pahwindo remuno. OffGrid ichavhura nekuchinja mapurogiramu anotsigirwa pachayo.','Kuongorora mvumo yedesktop…'],
  nd:['Umkhawulo wokufinyelela','Idesktop yonke yomsebenzi lo','Iwindi elikhethiweyo kuphela','Vumela ukufinyelela kudeskithophu','Vumela kanye engxoxweni yalapha. I-OffGrid izavula futhi intshintshe izinhlelo ezisekelwayo ngokwayo.','Kuhlolwa ukufinyelela kudeskithophu…'],
  zu:['Umkhawulo wokufinyelela','Ideskithophu yonke yalo msebenzi','Iwindi elikhethiwe kuphela','Vumela ukufinyelela kudeskithophu','Vuma kanye engxoxweni yalapha. I-OffGrid izovula futhi ishintshe izinhlelo ezisekelwayo ngokwayo.','Kubuyekezwa ukufinyelela kudeskithophu…'],
};
export function taskDesktopText(locale:LocaleCode){const [scope,desktop,window,allow,hint,working]=copy[locale];return {scope,desktop,window,allow,hint,working};}
