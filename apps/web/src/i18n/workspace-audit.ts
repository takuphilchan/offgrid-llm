import type { LocaleCode } from './index';

const labels: Record<LocaleCode, readonly [string, string, string, string]> = {
  en: ['Selected', 'Downloads', 'All work', 'History'],
  fr: ['Sélectionné', 'Téléchargements', 'Tout le travail', 'Historique'],
  es: ['Seleccionado', 'Descargas', 'Todo el trabajo', 'Historial'],
  de: ['Ausgewählt', 'Downloads', 'Alle Aufgaben', 'Verlauf'],
  ar: ['محدد', 'التنزيلات', 'كل العمل', 'السجل'],
  sw: ['Imechaguliwa', 'Vipakuliwa', 'Kazi zote', 'Historia'],
  sn: ['Yasarudzwa', 'Zviri kudhaunirodhwa', 'Mabasa ose', 'Nhoroondo'],
  nd: ['Ikhethiwe', 'Okulandiweyo', 'Yonke imisebenzi', 'Imbali'],
  zu: ['Ikhethiwe', 'Okulandiwe', 'Yonke imisebenzi', 'Umlando'],
};
export function workspaceAudit(locale: LocaleCode) {
  const [selected, downloads, allWork, history] = labels[locale];
  return {selected, downloads, allWork, history};
}

export const libraryScope: Record<LocaleCode, string> = {
  en: 'Add documents here to reuse them across conversations. Files attached in Chat stay with that conversation; they are not added to this library.',
  fr: 'Ajoutez ici des documents à réutiliser dans plusieurs conversations. Les fichiers joints dans Chat restent dans leur conversation, sans être ajoutés à cette bibliothèque.',
  es: 'Añade documentos aquí para reutilizarlos en varias conversaciones. Los archivos adjuntos en Chat permanecen en su conversación y no se añaden a esta biblioteca.',
  de: 'Fügen Sie hier Dokumente hinzu, um sie in mehreren Gesprächen zu nutzen. In Chat angehängte Dateien bleiben beim jeweiligen Gespräch und werden nicht in diese Bibliothek aufgenommen.',
  ar: 'أضف مستندات هنا لإعادة استخدامها في محادثات متعددة. تبقى الملفات المرفقة في الدردشة مع تلك المحادثة ولا تُضاف إلى هذه المكتبة.',
  sw: 'Ongeza hati hapa ili uzitumie katika mazungumzo mbalimbali. Faili zilizoambatishwa kwenye Gumzo hubaki katika mazungumzo hayo; haziongezwi kwenye maktaba hii.',
  sn: 'Wedzera magwaro pano kuti uashandise muhurukuro dzakasiyana. Mafaira akabatanidzwa muChat anoramba ari muhurukuro iyoyo; haawedzerwi muraibhurari ino.',
  nd: 'Faka imibhalo lapha ukuze uyisebenzise ezingxoxweni ezitshiyeneyo. Amafayela anamathiselwe kuChat ahlala kuleyongxoxo; awafakwa kuleli layibhurari.',
  zu: 'Engeza amadokhumenti lapha ukuze uwasebenzise ezingxoxweni ezahlukene. Amafayela anamathiselwe kuChat ahlala kuleyo ngxoxo; awafakwa kulo mtapo.',
};
