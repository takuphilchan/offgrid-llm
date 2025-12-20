const { contextBridge, ipcRenderer } = require('electron');

// Expose safe APIs to renderer process
contextBridge.exposeInMainWorld('electron', {
  // Directory selection for USB transfers and file operations
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  
  // Get the local server API URL
  getApiUrl: () => ipcRenderer.invoke('get-api-url'),
  
  // Get server status
  getServerStatus: () => ipcRenderer.invoke('get-server-status'),
  
  // Get app paths (config, models, data directories)
  getPaths: () => ipcRenderer.invoke('get-paths'),
  
  // Platform information
  platform: process.platform,
  
  // Flag to identify desktop environment
  isDesktop: true,
  
  // App version (exposed safely)
  getVersion: () => process.versions.electron
});
