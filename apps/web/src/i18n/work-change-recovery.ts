import {defineMessages} from './catalog';
import type {LocaleCode} from './index';

const labels = defineMessages<readonly string[]>('i18n/work-change-recovery:labels', {
  en: ['Preserved files','Download retained file','Review these copies before restoring a file. Unconfirmed copies may contain staged changes.'],
  fr: ['Fichiers conservés','Télécharger le fichier conservé','Examinez ces copies avant de restaurer un fichier. Les copies non confirmées peuvent contenir des modifications préparées.'],
  de: ['Aufbewahrte Dateien','Aufbewahrte Datei herunterladen','Prüfe diese Kopien vor der Wiederherstellung. Unbestätigte Kopien können vorbereitete Änderungen enthalten.'],
  es: ['Archivos conservados','Descargar archivo conservado','Revisa estas copias antes de restaurar un archivo. Las copias sin confirmar pueden contener cambios preparados.'],
  ar: ['الملفات المحفوظة','تنزيل الملف المحفوظ','راجع هذه النسخ قبل استعادة ملف. قد تحتوي النسخ غير المؤكدة على تغييرات مُعدّة.'],
  sw: ['Faili zilizohifadhiwa','Pakua faili iliyohifadhiwa','Kagua nakala hizi kabla ya kurejesha faili. Nakala zisizothibitishwa zinaweza kuwa na mabadiliko yaliyoandaliwa.'],
  sn: ['Mafaira akachengetwa','Dhaunirodha faira rakachengetwa','Ongorora makopi aya usati wadzorera faira. Makopi asina kusimbiswa anogona kuva neshanduko dzakagadzirirwa.'],
  nd: ['Amafayela agciniweyo','Dawuniloda ifayela eligciniweyo','Hlola amakhophi la ungakabuyiseli ifayela. Amakhophi angaqinisekiswanga angaba lenguquko ezilungiselelweyo.'],
  zu: ['Amafayela agciniwe','Landa ifayela eligciniwe','Hlola la makhophi ngaphambi kokubuyisela ifayela. Amakhophi angaqinisekisiwe angaba nezinguquko ezilungisiwe.'],
});

export function workChangeRecoveryText(locale: LocaleCode) {
  const [recovery,downloadRetained,recoveryHint] = labels[locale];
  return {recovery,downloadRetained,recoveryHint};
}
