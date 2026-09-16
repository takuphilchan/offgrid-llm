import { useEffect, useState } from 'react';
import './platform';

export type ThemeChoice = 'system' | 'dark' | 'light';
const themeKey = 'offgrid.theme';

function readThemeChoice(): ThemeChoice {
  const saved = localStorage.getItem(themeKey);
  return saved === 'dark' || saved === 'light' ? saved : 'system';
}

function preferredTheme(): 'dark' | 'light' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme: 'dark' | 'light') {
  document.documentElement.dataset.theme = theme;
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', theme === 'dark' ? '#101010' : '#f7f7f6');
}

export function initializeTheme() {
  const choice = readThemeChoice();
  applyTheme(choice === 'system' ? preferredTheme() : choice);
}

export function useTheme() {
  const [choice, setChoice] = useState<ThemeChoice>(readThemeChoice);
  const [systemTheme, setSystemTheme] = useState<'dark' | 'light'>(preferredTheme);

  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const update = () => setSystemTheme(media.matches ? 'dark' : 'light');
    media.addEventListener('change', update);
    const bridge = window.electron;
    let removeDesktopListener: (() => void) | undefined;
    if (bridge) {
      void bridge.getSystemTheme().then(setSystemTheme).catch(update);
      removeDesktopListener = bridge.onThemeChange(setSystemTheme);
    }
    return () => { media.removeEventListener('change', update); removeDesktopListener?.(); };
  }, []);

  useEffect(() => {
    localStorage.setItem(themeKey, choice);
    applyTheme(choice === 'system' ? systemTheme : choice);
  }, [choice, systemTheme]);

  return { choice, setChoice, effective: choice === 'system' ? systemTheme : choice };
}
