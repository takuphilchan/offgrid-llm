import desktopCopy from '../../public/desktop-presentation.json' with { type: 'json' };
import { dictionaryLocale, type LanguageCode } from '../i18n/language';
import { featureTranslations } from '../i18n/catalog';

export function startupCopy(locale: LanguageCode) {
  return {...desktopCopy[dictionaryLocale(locale)], ...featureTranslations(locale, 'desktop/presentation')};
}

/** Reveal only once; later refreshes and running jobs must not reopen startup. */
export function updateWorkspaceStartup(state: { locale?: string; copy?: Record<string,string>; error?: string; onRetry?: () => void; ready?: boolean }) {
  window.dispatchEvent(new CustomEvent('offgrid-workspace-startup', { detail: state }));
}
