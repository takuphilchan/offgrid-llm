import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

// Authored translation drafts; native-speaker approval is tracked separately.
const labels=defineMessages<string[]>('i18n/word-revisions:labels',{
 en:['Tracked changes','Formatting changes'],
 fr:['Suivi des modifications','Modifications de mise en forme'],
 es:['Control de cambios','Cambios de formato'],
 ar:['تعقّب التغييرات','تغييرات التنسيق'],
 sw:['Mabadiliko yaliyofuatiliwa','Mabadiliko ya umbizo'],
 sn:['Shanduko dzakatevedzwa','Shanduko dzemanyorerwo'],
 nd:['Inguquko ezilandelelweyo','Inguquko zendlela yokubhala'],
 zu:['Izinguquko ezilandelelwe','Izinguquko zokufometha'],
 de:['Änderungsverfolgung','Formatierungsänderungen'],
});
export const wordRevisions=(locale:LocaleCode)=>({title:labels[locale][0],formatting:labels[locale][1]});
