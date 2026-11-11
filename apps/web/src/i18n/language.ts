import registry from '../../../desktop/language-registry.json' with { type: 'json' };

export type LanguageCode = keyof typeof registry;
export type DictionaryLocale = 'en' | 'fr' | 'es' | 'ar' | 'sw' | 'sn' | 'nd' | 'zu' | 'de';
export const languageRegistry = registry;

// Preserve explicit scripts/regions before applying language fallbacks.
export function resolveLanguage(value: unknown): LanguageCode | undefined {
 if (typeof value !== 'string' || !value.trim()) return undefined;
 try {
  const tag = new Intl.Locale(value.trim().replaceAll('_','-'));
  if (Object.hasOwn(registry,tag.baseName)) return tag.baseName as LanguageCode;
  if (tag.language === 'zh') return tag.maximize().script === 'Hant' ? 'zh-Hant' : 'zh-Hans';
  if (tag.language === 'pt') return tag.region === 'BR' ? 'pt-BR' : 'pt-PT';
  return Object.hasOwn(registry,tag.language) ? tag.language as LanguageCode : undefined;
 } catch { return undefined; }
}

export const dictionaryLocale = (language: LanguageCode) => registry[language].fallback as DictionaryLocale;
