import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { I18nProvider } from './i18n';
import { initializeTheme } from './theme';
import { writePreference } from './lib/preferences';
import '@fontsource-variable/ibm-plex-sans/wght.css';
import '@fontsource/ibm-plex-sans-arabic/arabic-400.css';
import '@fontsource/ibm-plex-sans-arabic/arabic-500.css';
import '@fontsource/ibm-plex-sans-arabic/arabic-600.css';
import '@fontsource/ibm-plex-mono/latin-400.css';
import './styles.css';
import './workspace.css';
import './controls.css';
import './features/agents/task-workspace.css';

async function start() {
  // File-origin startup and HTTP renderer share native appearance preferences.
  // Read before first paint; a broken bridge cannot indefinitely block startup.
  if (window.electron?.getPresentation) {
    let timer: ReturnType<typeof setTimeout> | undefined;
    try {
      const value = await Promise.race([window.electron.getPresentation(), new Promise<never>((_, reject) => { timer = setTimeout(() => reject(new Error('Presentation timeout')), 1500); })]);
      writePreference('offgrid.locale', value.locale);
      writePreference('offgrid.theme', value.theme);
    } catch { /* Retain browser preferences if the desktop bridge is unavailable. */ }
    finally { clearTimeout(timer); }
  }
  initializeTheme();
  createRoot(document.getElementById('root')!).render(<StrictMode><I18nProvider><App /></I18nProvider></StrictMode>);
}
void start();
