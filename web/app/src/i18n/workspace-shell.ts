import type { LocaleCode } from './index';

type ShellMessages = { options: string; collapse: string; expand: string; navigation: string };
export const workspaceShell = {
  en: { options: 'Workspace options', collapse: 'Collapse navigation', expand: 'Expand navigation', navigation: 'Primary navigation' },
  fr: { options: 'Options de l’espace de travail', collapse: 'Réduire la navigation', expand: 'Développer la navigation', navigation: 'Navigation principale' },
  es: { options: 'Opciones del espacio de trabajo', collapse: 'Contraer navegación', expand: 'Expandir navegación', navigation: 'Navegación principal' },
  de: { options: 'Arbeitsbereich-Optionen', collapse: 'Navigation einklappen', expand: 'Navigation ausklappen', navigation: 'Hauptnavigation' },
  ar: { options: 'خيارات مساحة العمل', collapse: 'طي التنقل', expand: 'توسيع التنقل', navigation: 'التنقل الرئيسي' },
  sw: { options: 'Chaguo za eneo la kazi', collapse: 'Kunja urambazaji', expand: 'Panua urambazaji', navigation: 'Urambazaji mkuu' },
  sn: { options: 'Sarudzo dzenzvimbo yekushandira', collapse: 'Peta nzira dzekufamba', expand: 'Vhura nzira dzekufamba', navigation: 'Nzira huru dzekufamba' },
  nd: { options: 'Okukhethwa kwendawo yokusebenza', collapse: 'Fingqa ukuzulazula', expand: 'Nweba ukuzulazula', navigation: 'Ukuzulazula okuyinhloko' },
  zu: { options: 'Izinketho zendawo yokusebenza', collapse: 'Goqa ukuzulazula', expand: 'Nweba ukuzulazula', navigation: 'Ukuzulazula okuyinhloko' },
} satisfies Record<LocaleCode, ShellMessages>;
