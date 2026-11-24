export type DesktopPaths = { config: string; models: string; data: string };
export type DesktopBackend = { state: 'checking' | 'starting' | 'ready' | 'offline' | 'incompatible' | 'unavailable' | 'error'; url: string; managedByDesktop: boolean; desktopVersion: string; version?: string; revision?: string; uiBuildID?: string; apiVersion?: number; reason?: string; code?: string; bridgeProtocol?: number; compatibilityBasis?: 'declared-contract' | 'reviewed-legacy' };

export type DesktopBridge = {
  workspaceRecoveryProtocol?: number;
  getWorkspaceRecovery?: () => Promise<WorkspaceRecoveryState>;
  startWorkspaceRecovery?: (request:{operation:WorkspaceRecoveryOperation;encryption:WorkspaceRecoveryEncryption}) => Promise<WorkspaceRecoveryState>;
  cancelWorkspaceRecovery?: (request:{id:string}) => Promise<WorkspaceRecoveryState>;
  resumeWorkspaceAfterRecovery?: (request:{id:string}) => Promise<WorkspaceRecoveryState>;
  onWorkspaceRecovery?: (callback:(state:WorkspaceRecoveryState)=>void) => () => void;
  commandWindowProtocol?: number;
  getCommandWindow?: () => Promise<CommandWindowState>;
  openCommandWindow?: () => Promise<CommandWindowState>;
  expandCommandWindow?: () => Promise<CommandWindowState>;
  dismissCommandWindow?: () => Promise<CommandWindowState>;
  setCommandLayout?: (layout:'input'|'work'|'voice', motion?:{prepared:boolean;reduceMotion:boolean}) => Promise<CommandWindowState>;
  onCommandTransition?: (callback:(request:{id:number})=>void) => () => void;
  acknowledgeCommandTransition?: (request:{id:number;reduceMotion:boolean}) => Promise<void>;
  setCommandShortcut?: (shortcut: string) => Promise<CommandWindowState>;
  onCommandWindow?: (callback: (state: CommandWindowState) => void) => () => void;
  bridgeProtocol?: number;
  terminalProtocol?: number;
  onTerminalContext?: (callback:(context:unknown)=>boolean)=>()=>void;
  openTerminal?: () => Promise<{visible:boolean}>;
  getTerminalStatus?: () => Promise<{available:boolean}>;
  startExecutionWorkspace?: (request:{access:'read'|'edit'|'trusted'}) => Promise<{state:string;resource?:import('./api/client').ExecutionWorkspace}>;
  getExecutionWorkspace?: () => Promise<{state:string;resource?:import('./api/client').ExecutionWorkspace}>;
  stopExecutionWorkspace?: () => Promise<{state:string}>;
  desktopComputerProtocol?: number;
  browserManualProtocol?: number;
  browserTaskBindingProtocol?: number;
  takeoverComputerAccess?: (request:{session:string})=>Promise<{state:string;code?:string;browserManualProtocol?:number;target?:{id:string;origin:string}}>;
  resumeComputerBrowser?: (request:{workspace:string;runId:string;requestId:string;session:string})=>Promise<{state:string;code?:string;browserManualProtocol?:number;target?:{id:string;origin:string}}>;
  enableDesktopAccess?: (request:{workspace:string})=>Promise<{enabled:boolean;workspace?:string}>;
  getDesktopAccess?: ()=>Promise<{enabled:boolean;workspace?:string}>;
  startComputerDesktop?: (request:{workspace:string;runId:string;requestId:string}) => Promise<{state:string;target?:{id:string;origin:string;scope?:string};code?:string}>;
  discoverComputerApps?: (request:{workspace:string;requestId?:string}) => Promise<{state:string;targets?:{id:string;title:string;driver:string}[];code?:string}>;
  startComputerApp?: (request:{workspace:string;requestId?:string;target:string;approvalMode:'ask_every_time'|'scoped_changes'|'full_task'}) => Promise<{state:string;target?:{id:string;origin:string};code?:string}>;
  listComputerApplications?: () => Promise<{state:string;targets?:{id:string;title:string;driver:string}[];code?:string}>;
  launchComputerApplication?: (request:{id:string}) => Promise<{id:string;title:string;driver:string;code?:string}>;
  getComputerStatus?: () => Promise<{state:string; active?:boolean; code?:string; installed:boolean; browserManualProtocol?:number; targets?:{id:string;title:string;driver:string}[]; target?:{id:string;origin:string}}>;
  startComputerBrowser?: (request:{origin:string;additionalOrigins?:string[];workspace:string;requestId?:string;runId?:string;networkMode?:'direct'|'trusted-vpn';uploadGrant?:string;approvalMode:'ask_every_time'|'scoped_changes'|'full_task'}) => Promise<{state:string;code?:string;target?:{id:string;origin:string}}>;
  selectComputerUpload?: () => Promise<{id:string;name:string;size:number;sha256:string}|null>;
  stopComputerBrowser?: () => Promise<{state:string;code?:string}>;
  stopComputerAccess?: (request:{requestId:string}|{session:string}) => Promise<{state:string;code?:string}>;
  isDesktop: true;
  presentation?: { locale: string; theme: 'system' | 'dark' | 'light'; appearance?: unknown };
  getPresentation?: () => Promise<{ locale: string; theme: 'system' | 'dark' | 'light'; appearance?: unknown }>;
  setPresentation?: (preferences: { locale: string; theme: 'system' | 'dark' | 'light'; appearance?: unknown }) => Promise<void>;
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

export type CommandWindowState = { mode: 'full' | 'compact'; shortcut: string; registered: boolean };

export type WorkspaceRecoveryOperation = 'backup'|'verify'|'restore';
export type WorkspaceRecoveryEncryption = 'recipient'|'passphrase'|'legacy';
export type WorkspaceRecoveryPhase = 'idle'|'selecting'|'confirming'|'stopping'|'running'|'succeeded'|'cancelled'|'failed'|'uncertain'|'resuming';
export type WorkspaceRecoveryState = {
  version:1; revision:number; available:boolean; phase:WorkspaceRecoveryPhase; active:boolean; serviceStopped:boolean; canResume:boolean;
  id?:string; operation?:WorkspaceRecoveryOperation; encryption?:WorkspaceRecoveryEncryption; code?:string;
  result?:{files:number;bytes:number;application_version:string;encryption:'none'|'age-v1';authenticated:boolean;published:boolean;database_checked:boolean};
  retainedPaths?:{path:string;directory:boolean;contains_plaintext:boolean}[];
};

declare global {
  interface Window {
    electron?: DesktopBridge;
  }
}
