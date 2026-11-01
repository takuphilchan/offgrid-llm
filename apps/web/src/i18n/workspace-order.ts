import type { LocaleCode } from './index';

const labels: Record<LocaleCode, readonly [string, string, string]> = {
  en: ['Model actions', 'Select model', 'Full filename'],
  fr: ['Actions du modèle', 'Sélectionner le modèle', 'Nom complet du fichier'],
  es: ['Acciones del modelo', 'Seleccionar modelo', 'Nombre completo del archivo'],
  de: ['Modellaktionen', 'Modell auswählen', 'Vollständiger Dateiname'],
  ar: ['إجراءات النموذج', 'اختيار النموذج', 'اسم الملف الكامل'],
  sw: ['Vitendo vya modeli', 'Chagua modeli', 'Jina kamili la faili'],
  sn: ['Zviito zvemodheru', 'Sarudza modheru', 'Zita rose refaira'],
  nd: ['Izenzo zemodeli', 'Khetha imodeli', 'Ibizo lefayela elipheleleyo'],
  zu: ['Izenzo zemodeli', 'Khetha imodeli', 'Igama lefayela eligcwele'],
};
export function workspaceOrder(locale: LocaleCode) {
  const [modelActions, selectModel, fullName] = labels[locale];
  return { modelActions, selectModel, fullName };
}
