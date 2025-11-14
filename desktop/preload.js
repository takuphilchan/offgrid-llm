const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electron', {
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  getApiUrl: () => ipcRenderer.invoke('get-api-url'),
  platform: process.platform,
  isDesktop: true
});
