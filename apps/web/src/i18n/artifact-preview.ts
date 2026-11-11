import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

const labels = defineMessages<readonly [string, string, string]>("i18n/artifact-preview:labels", {
  en: ['Preview', 'Close preview', 'Preview unavailable. Download the file to inspect it.'],
  fr: ['Aperçu', 'Fermer l’aperçu', 'Aperçu indisponible. Téléchargez le fichier pour le consulter.'],
  es: ['Vista previa', 'Cerrar vista previa', 'Vista previa no disponible. Descarga el archivo para examinarlo.'],
  de: ['Vorschau', 'Vorschau schließen', 'Vorschau nicht verfügbar. Datei zur Prüfung herunterladen.'],
  ar: ['معاينة', 'إغلاق المعاينة', 'المعاينة غير متاحة. نزّل الملف لفحصه.'],
  sw: ['Hakiki', 'Funga hakikisho', 'Hakikisho halipatikani. Pakua faili ili kuikagua.'],
  sn: ['Tarisa faira', 'Vhara tarisiro', 'Tarisiro haipo. Dhaunirodha faira kuti uriongorore.'],
  nd: ['Buka ifayela', 'Vala ukubuka', 'Ukubuka akutholakali. Dawuniloda ifayela ulihlole.'],
  zu: ['Buka kuqala', 'Vala ukubuka kuqala', 'Ukubuka kuqala akutholakali. Landa ifayela ukuze ulihlole.'],
});

export function artifactPreviewText(locale: LocaleCode) {
  const [preview, close, unavailable] = labels[locale];
  return { preview, close, unavailable };
}
