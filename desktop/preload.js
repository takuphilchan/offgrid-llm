const { contextBridge, ipcRenderer } = require('electron');
let presentation;
try { const value = process.argv.find(item => item.startsWith('--offgrid-presentation=')); if (value) presentation = JSON.parse(Buffer.from(value.split('=')[1], 'base64').toString('utf8')); } catch { /* Main provides validated display preferences. */ }

// Expose safe APIs to renderer process
contextBridge.exposeInMainWorld('electron', {
  presentation,
  getPresentation: () => ipcRenderer.invoke('get-presentation'),
  setPresentation: preferences => ipcRenderer.invoke('set-presentation', preferences),
  // Directory selection for USB transfers and file operations
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  
  // Get the local server API URL
  getApiUrl: () => ipcRenderer.invoke('get-api-url'),
  
  // Get server status
  getServerStatus: () => ipcRenderer.invoke('get-server-status'),
  getBackendInfo: () => ipcRenderer.invoke('get-backend-info'),
  getComputerStatus: () => ipcRenderer.invoke('computer-status'),
  startComputerBrowser: request => ipcRenderer.invoke('computer-start', request),
  stopComputerBrowser: () => ipcRenderer.invoke('computer-stop'),
  retryStartup: () => ipcRenderer.invoke('startup-retry'),
  startDesktopWorkspace: () => ipcRenderer.invoke('startup-local'),
  openExistingWorkspace: () => ipcRenderer.invoke('startup-browser'),
  openStartupHelp: () => ipcRenderer.invoke('startup-help'),
  onStartupState: (callback) => {
    const listener = (_event, state) => callback(state);
    ipcRenderer.on('startup-state', listener);
    return () => ipcRenderer.removeListener('startup-state', listener);
  },
  
  // Get app paths (config, models, data directories)
  getPaths: () => ipcRenderer.invoke('get-paths'),
  
  // Platform information
  platform: process.platform,
  
  // Flag to identify desktop environment
  isDesktop: true,
  
  // App version (exposed safely)
  getVersion: () => ipcRenderer.invoke('get-app-version'),
  
  // System theme support
  getSystemTheme: () => ipcRenderer.invoke('get-system-theme'),
  onThemeChange: (callback) => {
    const listener = (_event, theme) => callback(theme);
    ipcRenderer.on('system-theme-changed', listener);
    return () => ipcRenderer.removeListener('system-theme-changed', listener);
  }
});
