const {contextBridge,ipcRenderer}=require('electron');
contextBridge.exposeInMainWorld('terminalDesktop',{
  request: value=>ipcRenderer.invoke('terminal-request',value),
  onEvent: callback=>{const handler=(_event,value)=>callback(value);ipcRenderer.on('terminal-event',handler);return()=>ipcRenderer.removeListener('terminal-event',handler);}
});
