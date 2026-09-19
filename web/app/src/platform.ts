export type DesktopPaths = { config: string; models: string; data: string };
export type DesktopBackend = { state: 'checking' | 'starting' | 'ready' | 'offline' | 'incompatible' | 'unavailable' | 'error'; url: string; managedByDesktop: boolean; desktopVersion: string; version?: string; revision?: string; uiBuildID?: string; apiVersion?: number; reason?: string };

export type DesktopBridge = {
  isDesktop: true;
  presentation?: { locale: string; theme: 'system' | 'dark' | 'light' };
  getPresentation?: () => Promise<{ locale: string; theme: 'system' | 'dark' | 'light' }>;
  setPresentation?: (preferences: { locale: string; theme: 'system' | 'dark' | 'light' }) => Promise<void>;
  platform: string;
  getApiUrl: () => Promise<string>;
  getServerStatus: () => Promise<boolean>;
  getBackendInfo: () => Promise<DesktopBackend>;
  getVersion: () => Promise<string>;
  selectDirectory: () => Promise<string | null>;
  getSystemTheme: () => Promise<'dark' | 'light'>;
  onThemeChange: (callback: (theme: 'dark' | 'light') => void) => () => void;
  getPaths: () => Promise<DesktopPaths>;
};

declare global {
  interface Window {
    electron?: DesktopBridge;
  }
}
