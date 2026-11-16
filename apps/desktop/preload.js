const { contextBridge, ipcRenderer, webFrame } = require('electron');
// Isolated caption controls permit a frameless Quick Access presentation without
// replacing the trusted window or exposing window actions to service scripts.
let compactChrome = false, captionButtons = [];
function captionLabels(value) {
  const labels = value?.windowControls;
  if (Array.isArray(labels) && labels.length === 3) captionButtons.forEach((button,index) => {button.title=labels[index];button.setAttribute('aria-label',labels[index]);});
}
function fitNativeChrome() {
  if (process.platform !== 'win32') return;
  document.documentElement.style.setProperty('--native-chrome-height', `${compactChrome ? 0 : 32 / webFrame.getZoomFactor()}px`);
  document.documentElement.toggleAttribute('data-compact-chrome',compactChrome);
}
ipcRenderer.on('command-window-state',(_event,state)=>{compactChrome=state?.mode==='compact';fitNativeChrome();});
window.addEventListener('DOMContentLoaded', () => {
  if (process.platform !== 'win32') return;
  // Trusted preload-owned sheets also work on the strict-CSP startup page.
  // Inline <style> elements are blocked there, leaving bare caption buttons in
  // the document flow. Keep the page CSP and the closed control boundary intact.
  const style = new CSSStyleSheet();
  style.replaceSync(`:root { --workspace-viewport: calc(100dvh - var(--native-chrome-height, 0px)); }
    body { padding-top: var(--native-chrome-height) !important; box-sizing: border-box; }
    #offgrid-native-chrome { position: fixed; inset: 0 0 auto; height: var(--native-chrome-height); z-index: 2147483647; background: var(--surface-0, var(--bg, Canvas)); -webkit-app-region: drag; user-select: none; }
    :root[data-compact-chrome] #offgrid-native-chrome { display:none; }
    @media (forced-colors: active) { #offgrid-native-chrome { background: Canvas; } }`);
  document.adoptedStyleSheets = [...document.adoptedStyleSheets, style];
  const strip = document.createElement('div');
  strip.id = 'offgrid-native-chrome';
  const shadow=strip.attachShadow({mode:'closed'}),captionStyle=new CSSStyleSheet();
  captionStyle.replaceSync(':host{display:flex;justify-content:flex-end}button{width:46px;height:100%;border:0;background:transparent;color:var(--text,CanvasText);display:grid;place-items:center;-webkit-app-region:no-drag}button:hover{background:var(--surface-2,#8882)}button:last-child:hover{background:#c42b1c;color:white}button:focus-visible{outline:2px solid currentColor;outline-offset:-3px}svg{width:12px;height:12px;fill:none;stroke:currentColor;stroke-width:1}');
  shadow.adoptedStyleSheets = [captionStyle];
  ['minimize','maximize','close'].forEach((action,index)=>{
    const button=document.createElement('button');button.type='button';
    const paths=['M1 6h10','M1.5 1.5h9v9h-9z','M1 1l10 10M11 1L1 11'];
    button.innerHTML='<svg viewBox="0 0 12 12" aria-hidden="true"><path d="'+paths[index]+'"/></svg>';
    button.addEventListener('click',event=>{if(event.isTrusted)void ipcRenderer.invoke('window-chrome-action',action).catch(()=>{});});
    captionButtons.push(button);shadow.append(button);
  });
  captionLabels(presentation);
  document.body.append(strip);
  fitNativeChrome();
  void ipcRenderer.invoke('command-window-status').then(state=>{compactChrome=state.mode==='compact';fitNativeChrome();}).catch(()=>{});
});
// Chromium can deliver resize before webFrame exposes the new zoom factor.
// Read it on the next frame so the native caption strip never keeps old geometry.
let nativeChromeFrame=0;
window.addEventListener('resize',()=>{
  cancelAnimationFrame(nativeChromeFrame);
  nativeChromeFrame=requestAnimationFrame(fitNativeChrome);
});
// Geometry only: the service renderer never receives the local terminal bridge.
let terminalVisible=false,terminalFrame=0,terminalRect='',terminalOverlay=false,terminalBackdrop;
function measureTerminalWorkspace(){
  cancelAnimationFrame(terminalFrame);
  terminalFrame=requestAnimationFrame(()=>{
    const workspace=document.querySelector('main.workspace');if(!workspace)return;
    // Preserve CSS fractions at enlarged text sizes; round once in the native
    // layout instead of leaving a one-pixel seam under the caption strip.
    const r=workspace.getBoundingClientRect(),bounds={x:Math.max(0,r.x),y:Math.max(0,r.y),width:Math.max(1,r.width),height:Math.max(1,innerHeight-Math.max(0,r.y))};
    const key=JSON.stringify(bounds);
    if(key!==terminalRect){terminalRect=key;void ipcRenderer.invoke('terminal-layout',bounds).catch(()=>{});
      if(terminalBackdrop)Object.assign(terminalBackdrop.style,{left:`${bounds.x}px`,top:`${bounds.y}px`,width:`${bounds.width}px`,height:`${bounds.height}px`});}
    const overlay=terminalVisible&&Array.from(document.querySelectorAll(':popover-open,dialog[open],[role="dialog"][aria-modal="true"]')).some(el=>{
      const box=el.getBoundingClientRect();return box.width>0&&box.height>0&&getComputedStyle(el).visibility!=='hidden'&&box.left<r.right&&box.right>r.left&&box.top<innerHeight&&box.bottom>r.top;
    });
    if(terminalBackdrop)terminalBackdrop.hidden=!overlay;
    if(overlay!==terminalOverlay){terminalOverlay=overlay;void ipcRenderer.invoke('terminal-overlay',overlay).catch(()=>{});}
  });
}
ipcRenderer.on('terminal-visibility',(_event,visible)=>{
  if(typeof visible!=='boolean')return;terminalVisible=visible;
  document.documentElement.toggleAttribute('data-native-terminal-visible',visible);
  const workspace=document.querySelector('main.workspace');if(workspace)workspace.inert=visible;
  measureTerminalWorkspace();
});
window.addEventListener('DOMContentLoaded',()=>{
  // A larger web overlay temporarily covers the native view. Keep the inert Chat
  // underneath concealed, without copying private terminal output to the web app.
  terminalBackdrop=document.createElement('div');terminalBackdrop.hidden=true;terminalBackdrop.setAttribute('aria-hidden','true');
  terminalBackdrop.style.cssText='position:fixed;pointer-events:none;z-index:40;background:var(--surface-0,Canvas)';
  document.body.append(terminalBackdrop);
  const observer=new ResizeObserver(measureTerminalWorkspace);
  observer.observe(document.documentElement);
  let observed;
  new MutationObserver(records=>{
    if(records.every(record=>record.target===terminalBackdrop))return;
    const workspace=document.querySelector('main.workspace');
    if(workspace!==observed){if(observed)observer.unobserve(observed);observed=workspace;if(workspace)observer.observe(workspace);}
    measureTerminalWorkspace();
  }).observe(document.body,{childList:true,subtree:true,attributes:true,attributeFilter:['style','open','hidden','aria-modal']});
  document.addEventListener('toggle',measureTerminalWorkspace,true);
  document.addEventListener('close',measureTerminalWorkspace,true);
  measureTerminalWorkspace();
});
window.addEventListener('resize',measureTerminalWorkspace);
window.addEventListener('hashchange',()=>{if(terminalVisible)void ipcRenderer.invoke('terminal-hide').catch(()=>{});measureTerminalWorkspace();});
// Same-route navigation and New chat must also reveal the web workspace.
// Hiding the native view leaves terminal processes and their tabs intact.
document.addEventListener('click',event=>{
  if(!terminalVisible||event.defaultPrevented||event.button!==0||event.ctrlKey||event.metaKey||event.shiftKey||event.altKey)return;
  const target=event.target instanceof Element?event.target.closest('#workspace-sidebar a[href^="#/"], #workspace-sidebar .new-chat-button'):null;
  if(target&&!target.matches(':disabled,[aria-disabled="true"]'))void ipcRenderer.invoke('terminal-hide').catch(()=>{});
},true);
let presentation;
try { const value = process.argv.find(item => item.startsWith('--offgrid-presentation=')); if (value) presentation = JSON.parse(Buffer.from(value.split('=')[1], 'base64').toString('utf8')); } catch { /* Main provides validated display preferences. */ }

// Expose safe APIs to renderer process
contextBridge.exposeInMainWorld('electron', {
  commandWindowProtocol: 1,
  getCommandWindow: () => ipcRenderer.invoke('command-window-status'),
  openCommandWindow: () => ipcRenderer.invoke('command-window-open'),
  expandCommandWindow: () => ipcRenderer.invoke('command-window-expand'),
  dismissCommandWindow: () => ipcRenderer.invoke('command-window-dismiss'),
  setCommandLayout: (layout, motion) => ipcRenderer.invoke('command-window-layout', {layout, prepared:motion?.prepared === true, reduceMotion:motion?.reduceMotion === true}),
  onCommandTransition: callback => {
    const listener = (_event, value) => callback(value);
    ipcRenderer.on('command-window-transition', listener);
    return () => ipcRenderer.removeListener('command-window-transition', listener);
  },
  acknowledgeCommandTransition: value => ipcRenderer.invoke('command-window-transition-ready', {id:value?.id, reduceMotion:value?.reduceMotion === true}),
  setCommandShortcut: shortcut => ipcRenderer.invoke('command-window-configure', {shortcut}),
  onCommandWindow: callback => {
    const listener = (_event, value) => callback(value);
    ipcRenderer.on('command-window-state', listener);
    return () => ipcRenderer.removeListener('command-window-state', listener);
  },
  bridgeProtocol: 1,
  terminalProtocol: 1,
  onTerminalContext: callback => {
    const listener=(_event,value)=>{
      if(!value||typeof value.id!=='string')return;
      let accepted=false;
      try {accepted=callback(value.context)===true;} catch {}
      void ipcRenderer.invoke('terminal-context-response',value.id,accepted).catch(()=>{});
    };
    ipcRenderer.on('terminal-context',listener);
    return ()=>ipcRenderer.removeListener('terminal-context',listener);
  },
  openTerminal: () => ipcRenderer.invoke('terminal-open'),
  getTerminalStatus: () => ipcRenderer.invoke('terminal-status'),
  presentation,
  getPresentation: () => ipcRenderer.invoke('get-presentation'),
  setPresentation: preferences => ipcRenderer.invoke('set-presentation', preferences).then(value=>{void ipcRenderer.invoke('get-presentation').then(captionLabels).catch(()=>{});return value;}),
  // Directory selection for USB transfers and file operations
  selectDirectory: () => ipcRenderer.invoke('select-directory'),
  
  // Get the local server API URL
  getApiUrl: () => ipcRenderer.invoke('get-api-url'),
  
  // Get server status
  getServerStatus: () => ipcRenderer.invoke('get-server-status'),
  getBackendInfo: () => ipcRenderer.invoke('get-backend-info'),
  startExecutionWorkspace: request => ipcRenderer.invoke('execution-start', request),
  getExecutionWorkspace: () => ipcRenderer.invoke('execution-status'),
  stopExecutionWorkspace: () => ipcRenderer.invoke('execution-stop'),
  getComputerStatus: () => ipcRenderer.invoke('computer-status'),
  startComputerBrowser: request => ipcRenderer.invoke('computer-start', request),
  selectComputerUpload: () => ipcRenderer.invoke('computer-select-upload'),
  discoverComputerApps: request => ipcRenderer.invoke('computer-targets', request),
  startComputerApp: request => ipcRenderer.invoke('computer-native-start', request),
  desktopComputerProtocol: process.platform === 'win32' && process.arch === 'x64' ? 3 : 0,
  startComputerDesktop: request => ipcRenderer.invoke('computer-desktop-start', request),
  enableDesktopAccess: request => ipcRenderer.invoke('computer-desktop-enable', request),
  getDesktopAccess: () => ipcRenderer.invoke('computer-desktop-access'),
  listComputerApplications: () => ipcRenderer.invoke('computer-launchable-apps'),
  launchComputerApplication: request => ipcRenderer.invoke('computer-launch-app', request),
  stopComputerBrowser: () => ipcRenderer.invoke('computer-stop'),
  stopComputerAccess: request => ipcRenderer.invoke('computer-stop-access', request),
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
ipcRenderer.on('command-window-dismiss', () => window.dispatchEvent(new Event('offgrid-desktop-dismiss')));
