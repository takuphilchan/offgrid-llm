import type { LocaleCode } from './index';
const labels: Record<LocaleCode, readonly [string, string, string, string, string]> = {
  en: ['Desktop compatibility', 'Declared contract', 'Reviewed legacy build', 'Desktop-managed service', 'Separately managed service'],
  fr: ['Compatibilité du bureau', 'Contrat déclaré', 'Version antérieure vérifiée', 'Service géré par le bureau', 'Service géré séparément'],
  es: ['Compatibilidad de escritorio', 'Contrato declarado', 'Versión anterior revisada', 'Servicio gestionado por escritorio', 'Servicio gestionado por separado'],
  de: ['Desktop-Kompatibilität', 'Deklarierter Vertrag', 'Geprüfter älterer Build', 'Vom Desktop verwalteter Dienst', 'Separat verwalteter Dienst'],
  ar: ['توافق سطح المكتب', 'عقد توافق معلن', 'إصدار سابق تمت مراجعته', 'خدمة يديرها تطبيق سطح المكتب', 'خدمة تُدار بشكل منفصل'],
  sw: ['Uoanifu wa programu', 'Mkataba uliotangazwa', 'Toleo la zamani lililokaguliwa', 'Huduma inayosimamiwa na programu', 'Huduma inayosimamiwa kando'],
  sn: ['Kuenderana kweapp', 'Chibvumirano chakaziviswa', 'Shanduro yekare yakaongororwa', 'Sevhisi inotongwa neapp', 'Sevhisi inotongwa yakazvimirira'],
  nd: ['Ukuhambelana kohlelo', 'Isivumelwano esimenyezelweyo', 'Inguqulo endala ehloliweyo', 'Isevisi ephathwa luhlelo', 'Isevisi ephathwa ngokwehlukana'],
  zu: ['Ukuhambisana kohlelo', 'Isivumelwano esimenyezelwe', 'Inguqulo endala ehloliwe', 'Isevisi ephathwa uhlelo', 'Isevisi ephathwa ngokwehlukana']
};
export function desktopCompatibility(locale: LocaleCode) {
  const [title, declared, legacy, owned, external] = labels[locale];
  return { title, declared, legacy, owned, external };
}
