'use strict';
const path = require('node:path');
const queues = new WeakMap();

// Host-owned local content: no service renderer can confirm these dialogs.
function showAppDialog(electron, parent, options, appearance) {
  if (!parent || parent.isDestroyed()) return Promise.resolve({response:options.cancelId ?? 0,checkboxChecked:false});
  let invalidated=false;
  const invalidate=()=>{invalidated=true;};
  parent.on('closed',invalidate);parent.on('hide',invalidate);parent.webContents.on('did-start-loading',invalidate);
  const previous = queues.get(parent) || Promise.resolve();
  const pending = previous.catch(() => {}).then(() => {
    parent.removeListener('closed',invalidate);parent.removeListener('hide',invalidate);parent.webContents.removeListener('did-start-loading',invalidate);
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
  if (!parent || parent.isDestroyed()) return Promise.resolve(canceled);
  return new Promise(resolve => {
    const theme = appearance();
    const area = screen.getDisplayMatching(parent.getBounds()).workArea;
    const width = Math.min(460, area.width), height = Math.min(options.checkboxLabel || (options.detail || '').length>200 ? 430 : 280, area.height);
    const child = new BrowserWindow({parent, modal:true, show:false, frame:false, resizable:false, minimizable:false, maximizable:false, fullscreenable:false, skipTaskbar:true,
      width, height, backgroundColor:theme.palette?.['surface-1'] || (theme.theme==='dark' ? '#181819' : '#ffffff'),
      title:options.title || options.message || 'OffGrid',
      webPreferences:{preload:path.join(__dirname,'app-dialog-preload.js'), sandbox:true, contextIsolation:true, nodeIntegration:false, partition:'offgrid-app-dialog', spellcheck:false}});
    let settled=false, initialized=false;
    const trusted = event => event.sender === child.webContents && event.senderFrame === child.webContents.mainFrame;
    const cleanup = () => {
      ipcMain.removeListener('offgrid-dialog-ready', ready);ipcMain.removeListener('offgrid-dialog-answer', answer);
      parent.removeListener('closed', cancel);parent.removeListener('hide', cancel);parent.webContents.removeListener('did-start-loading', cancel);
    };
    const finish = value => { if(settled)return;settled=true;cleanup();resolve(value);if(!child.isDestroyed())child.destroy(); };
    const cancel = () => finish(canceled);
    const ready = event => {
      if(!trusted(event) || initialized || settled)return;
      initialized=true;
      child.webContents.send('offgrid-dialog-content',{title:options.title,message:options.message,detail:options.detail,buttons,cancelId,
        defaultId:Number.isInteger(options.defaultId)?options.defaultId:cancelId,checkboxLabel:options.checkboxLabel,checkboxChecked:options.checkboxChecked===true,
        theme:theme.theme,locale:theme.locale,direction:theme.direction,palette:theme.palette,reduceMotion:theme.reduceMotion});
      child.show();
    };
    const answer = (event,value) => {
      if(!trusted(event) || !initialized || !value || !Number.isInteger(value.response) || value.response<0 || value.response>=buttons.length)return;
      finish({response:value.response,checkboxChecked:!!options.checkboxLabel && value.checkboxChecked===true});
    };
    ipcMain.on('offgrid-dialog-ready',ready);ipcMain.on('offgrid-dialog-answer',answer);
    parent.on('closed',cancel);parent.on('hide',cancel);parent.webContents.on('did-start-loading',cancel);
    child.on('closed',cancel);child.webContents.on('render-process-gone',cancel);
    child.webContents.setWindowOpenHandler(()=>({action:'deny'}));
    child.webContents.on('will-navigate',event=>event.preventDefault());
    child.webContents.session.setPermissionRequestHandler((_contents,_permission,callback)=>callback(false));
    child.webContents.session.setPermissionCheckHandler(()=>false);
    child.loadFile(path.join(__dirname,'app-dialog.html')).catch(cancel);
  });
}
module.exports={showAppDialog};
