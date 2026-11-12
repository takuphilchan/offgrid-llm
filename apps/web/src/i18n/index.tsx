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
import packs from './language-packs.json' with { type: 'json' };
import { resolveLanguage, dictionaryLocale, languageRegistry, type LanguageCode } from './language';
import {resolveMessages,featureTranslations} from './catalog';
export type { LanguageCode } from './language';

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

export type LocaleCode = LanguageCode;
// Both names identify the selected language. Feature catalogs resolve their own
// per-message fallbacks; a global English substitution must not bypass packs.
type I18nValue = { locale: LocaleCode; language: LanguageCode; setLocale: (language: LanguageCode) => void; messages: Messages; available: Array<{code:string;label:string;direction:'ltr'|'rtl';preview:boolean}>; translationNotice: string | null };
const I18nContext = createContext<I18nValue | null>(null);

function initialLocale(): LanguageCode {
  for (const explicit of [readPreference('offgrid.locale'),window.electron?.presentation?.locale]) {
    const language=resolveLanguage(explicit); if(language)return language;
  }
  const candidates = [...navigator.languages, navigator.language];
  for (const candidate of candidates) {
    const language = resolveLanguage(candidate);
    // Unreviewed packs are an explicit preview choice, never an automatic switch.
    if (language && languageRegistry[language].stage !== 'preview') return language;
  }
  return 'en';
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, updateLocale] = useState<LanguageCode>(initialLocale);
  const locale = language;
  const setLocale = (next: LanguageCode) => { const resolved=resolveLanguage(next); if (resolved) updateLocale(resolved); };
  const definition = locales[dictionaryLocale(language)];
  const pack = Object.hasOwn(packs,language) ? packs[language as keyof typeof packs] : null;
  const direction = languageRegistry[language].direction;
  useEffect(() => { const changed = (event: StorageEvent) => { if(event.key === 'offgrid.locale') updateLocale(initialLocale()); }; window.addEventListener('storage',changed);return()=>window.removeEventListener('storage',changed); }, []);
  useEffect(() => {
    writePreference('offgrid.locale', language);
    document.documentElement.lang = language;
    document.documentElement.dir = direction;
  }, [direction, language]);
  const value = useMemo<I18nValue>(() => {
    const messages = {...definition.messages};
    if (pack) for (const key of Object.keys(pack.messages) as (keyof typeof pack.messages)[]) {
      Object.assign(messages, {[key]:{...messages[key],...pack.messages[key]}});
    }
    return {locale,language,setLocale,messages:resolveMessages(messages,featureTranslations(language,'core')),translationNotice:pack?.notice ?? null,available:[
      ...Object.values(locales).map(item=>({...item,preview:false})),
      ...Object.entries(packs).map(([code,item])=>({code,label:item.label,direction:item.direction as 'ltr'|'rtl',preview:true}))
    ]};
  }, [definition.messages, locale, language, pack]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) throw new Error('useI18n must be used inside I18nProvider');
  return context;
}
