export type DesktopPaths = { config: string; models: string; data: string };
export type DesktopBackend = { state: 'checking' | 'starting' | 'ready' | 'offline' | 'incompatible' | 'unavailable' | 'error'; url: string; managedByDesktop: boolean; desktopVersion: string; version?: string; revision?: string; uiBuildID?: string; apiVersion?: number; reason?: string };

export type DesktopBridge = {
  discoverComputerApps?: (request:{workspace:string;requestId?:string}) => Promise<{state:string;targets?:{id:string;title:string;driver:string}[];code?:string}>;
  startComputerApp?: (request:{workspace:string;requestId?:string;target:string;approvalMode:'ask_every_time'|'scoped_changes'|'full_task'}) => Promise<{state:string;target?:{id:string;origin:string};code?:string}>;
  listComputerApplications?: () => Promise<{state:string;targets?:{id:string;title:string;driver:string}[];code?:string}>;
  launchComputerApplication?: (request:{id:string}) => Promise<{id:string;title:string;driver:string;code?:string}>;
  getComputerStatus?: () => Promise<{state:string; code?:string; installed:boolean; targets?:{id:string;title:string;driver:string}[]; target?:{id:string;origin:string}}>;
  startComputerBrowser?: (request:{origin:string;workspace:string;requestId?:string;networkMode?:'direct'|'trusted-vpn';uploadGrant?:string;approvalMode:'ask_every_time'|'scoped_changes'|'full_task'}) => Promise<{state:string;code?:string;target?:{id:string;origin:string}}>;
  selectComputerUpload?: () => Promise<{id:string;name:string;size:number;sha256:string}|null>;
  stopComputerBrowser?: () => Promise<{state:string;code?:string}>;
  stopComputerAccess?: (request:{requestId:string}|{session:string}) => Promise<{state:string;code?:string}>;
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
