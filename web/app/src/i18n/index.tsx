import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { ar } from './locales/ar';
import { de } from './locales/de';
import { en } from './locales/en';
import { es } from './locales/es';
import { fr } from './locales/fr';
import { nd } from './locales/nd';
import { sn } from './locales/sn';
import { sw } from './locales/sw';
import { zu } from './locales/zu';
import type { LocaleDefinition, Messages } from './types';
import { readPreference, writePreference } from '../lib/preferences';

export const locales = {
  en: { code: 'en', label: 'English', direction: 'ltr', messages: en },
  fr: { code: 'fr', label: 'Français', direction: 'ltr', messages: fr },
  es: { code: 'es', label: 'Español', direction: 'ltr', messages: es },
  ar: { code: 'ar', label: 'العربية', direction: 'rtl', messages: ar },
  sw: { code: 'sw', label: 'Kiswahili', direction: 'ltr', messages: sw },
  sn: { code: 'sn', label: 'ChiShona', direction: 'ltr', messages: sn },
  nd: { code: 'nd', label: 'isiNdebele', direction: 'ltr', messages: nd },
  zu: { code: 'zu', label: 'isiZulu', direction: 'ltr', messages: zu },
  de: { code: 'de', label: 'Deutsch', direction: 'ltr', messages: de }
} as const satisfies Record<string, LocaleDefinition>;

export type LocaleCode = keyof typeof locales;
type I18nValue = { locale: LocaleCode; setLocale: (locale: LocaleCode) => void; messages: Messages; available: LocaleDefinition[] };
const I18nContext = createContext<I18nValue | null>(null);

function initialLocale(): LocaleCode {
  const saved = readPreference('offgrid.locale');
  if (saved && saved in locales) return saved as LocaleCode;
  const language = navigator.language.toLowerCase().split('-')[0];
  return language in locales ? language as LocaleCode : 'en';
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<LocaleCode>(initialLocale);
  const definition = locales[locale];
  useEffect(() => {
    writePreference('offgrid.locale', locale);
    document.documentElement.lang = locale;
    document.documentElement.dir = definition.direction;
  }, [definition.direction, locale]);
  const value = useMemo<I18nValue>(() => ({
    locale, setLocale, messages: definition.messages, available: Object.values(locales)
  }), [definition.messages, locale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) throw new Error('useI18n must be used inside I18nProvider');
  return context;
}
