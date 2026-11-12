import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

const labels = defineMessages<string>("i18n/composer:labels", {
  en: 'Add files or images', es: 'Añadir archivos o imágenes', fr: 'Ajouter des fichiers ou des images',
  de: 'Dateien oder Bilder hinzufügen', ar: 'إضافة ملفات أو صور', sw: 'Ongeza faili au picha',
  sn: 'Wedzera mafaira kana mifananidzo', nd: 'Faka amafayela kumbe imifanekiso', zu: 'Engeza amafayela noma izithombe',
});
export const composerAttachmentLabel = (locale: LocaleCode) => labels[locale];
