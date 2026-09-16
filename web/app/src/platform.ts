export type DesktopPaths = { config: string; models: string; data: string };

export type DesktopBridge = {
  isDesktop: true;
  platform: string;
  getApiUrl: () => Promise<string>;
  getServerStatus: () => Promise<boolean>;
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
