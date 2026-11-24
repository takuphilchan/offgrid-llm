'use strict';
const path = require('node:path');
const queues = new WeakMap();

// Host-owned local content: no service renderer can confirm these dialogs.
function showAppDialog(electron, parent, options, appearance) {
  if (!parent || parent.isDestroyed()) return Promise.resolve({response:options.cancelId ?? 0,checkboxChecked:false});
  let invalidated=options.signal?.aborted===true;
  const invalidate=()=>{invalidated=true;};
  parent.on('closed',invalidate);parent.on('hide',invalidate);parent.webContents.on('did-start-loading',invalidate);
  options.signal?.addEventListener('abort',invalidate,{once:true});
  const previous = queues.get(parent) || Promise.resolve();
  const pending = previous.catch(() => {}).then(() => {
    parent.removeListener('closed',invalidate);parent.removeListener('hide',invalidate);parent.webContents.removeListener('did-start-loading',invalidate);
    options.signal?.removeEventListener('abort',invalidate);
    return invalidated ? {response:options.cancelId ?? 0,checkboxChecked:false} : openDialog(electron, parent, options, appearance);
  });
  queues.set(parent, pending);
  pending.finally(() => { if (queues.get(parent) === pending) queues.delete(parent); }).catch(() => {});
  return pending;
}
function openDialog({BrowserWindow, ipcMain, screen}, parent, options, appearance) {
  const buttons = options.buttons?.length ? options.buttons.map(String) : ['OK'];
  const cancelId = Number.isInteger(options.cancelId) && options.cancelId >= 0 && options.cancelId < buttons.length ? options.cancelId : 0;
  const canceled = {response:cancelId, checkboxChecked:false};
  if (!parent || parent.isDestroyed() || options.signal?.aborted) return Promise.resolve(canceled);
  return new Promise(resolve => {
    const theme = appearance();
    const area = screen.getDisplayMatching(parent.getBounds()).workArea;
    const width = Math.min(460, Math.max(240, area.width - 32)), height = Math.min(280, area.height);
    const child = new BrowserWindow({parent, modal:true, show:false, frame:false, resizable:false, minimizable:false, maximizable:false, fullscreenable:false, skipTaskbar:true,
      width, height, backgroundColor:theme.palette?.['surface-1'] || (theme.theme==='dark' ? '#181819' : '#ffffff'),
      title:options.title || options.message || 'OffGrid',
      webPreferences:{preload:path.join(__dirname,'app-dialog-preload.js'), sandbox:true, contextIsolation:true, nodeIntegration:false, partition:'offgrid-app-dialog', spellcheck:false}});
    if(options.secret)child.setContentProtection?.(true);
    let settled=false, initialized=false;
    const trusted = event => event.sender === child.webContents && event.senderFrame === child.webContents.mainFrame;
    const cleanup = () => {
      ipcMain.removeListener('offgrid-dialog-layout', layout);ipcMain.removeListener('offgrid-dialog-ready', ready);ipcMain.removeListener('offgrid-dialog-answer', answer);
      parent.removeListener('closed', cancel);parent.removeListener('hide', cancel);parent.webContents.removeListener('did-start-loading', cancel);
      options.signal?.removeEventListener('abort',cancel);
    };
    const finish = value => { if(settled)return;settled=true;cleanup();resolve(value);if(!child.isDestroyed())child.destroy(); };
    const cancel = () => finish(canceled);
    const ready = event => {
      if(!trusted(event) || initialized || settled)return;
      initialized=true;
      child.webContents.send('offgrid-dialog-content',{title:options.title,message:options.message,detail:options.detail,buttons,cancelId,
        defaultId:Number.isInteger(options.defaultId)?options.defaultId:cancelId,checkboxLabel:options.checkboxLabel,checkboxChecked:options.checkboxChecked===true,
        secret:options.secret,theme:theme.theme,locale:theme.locale,direction:theme.direction,palette:theme.palette,reduceMotion:theme.reduceMotion});

    };
    // Only the isolated local dialog may report its content height. This grants
    // no additional action authority; clamp geometry to the parent's display.
    const layout = (event, value) => {
      if(!trusted(event) || !initialized || settled || !Number.isFinite(value) || value <= 0)return;
      const zoom = child.webContents.getZoomFactor();
      const height = Math.min(Math.max(120, Math.ceil(value * zoom)), Math.max(120, area.height - 32));
      const bounds = parent.getBounds();
      const x = Math.round(Math.max(area.x, Math.min(bounds.x + (bounds.width-width)/2, area.x+area.width-width)));
      const y = Math.round(Math.max(area.y, Math.min(bounds.y + (bounds.height-height)/2, area.y+area.height-height)));
      child.setBounds({x,y,width,height});
      if(!child.isVisible())child.show();
    };
    const answer = (event,value) => {
      if(!trusted(event) || !initialized || !value || !Number.isInteger(value.response) || value.response<0 || value.response>=buttons.length)return;
      if(options.secret && value.response!==cancelId && (typeof value.secret!=='string'||!value.secret||Buffer.byteLength(value.secret)>4096||/[\x00\r\n]/.test(value.secret)))return;
      finish({response:value.response,checkboxChecked:!!options.checkboxLabel && value.checkboxChecked===true,...(options.secret&&value.response!==cancelId?{secret:value.secret}:{})});
    };
    ipcMain.on('offgrid-dialog-layout',layout);ipcMain.on('offgrid-dialog-ready',ready);ipcMain.on('offgrid-dialog-answer',answer);
    parent.on('closed',cancel);parent.on('hide',cancel);parent.webContents.on('did-start-loading',cancel);
    options.signal?.addEventListener('abort',cancel,{once:true});
    child.on('closed',cancel);child.webContents.on('render-process-gone',cancel);
    child.webContents.setWindowOpenHandler(()=>({action:'deny'}));
    child.webContents.on('will-navigate',event=>event.preventDefault());
    child.webContents.session.setPermissionRequestHandler((_contents,_permission,callback)=>callback(false));
    child.webContents.session.setPermissionCheckHandler(()=>false);
    child.loadFile(path.join(__dirname,'app-dialog.html')).catch(cancel);
  });
}
module.exports={showAppDialog};
