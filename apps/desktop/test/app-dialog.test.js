const test=require('node:test'),assert=require('node:assert/strict'),{EventEmitter}=require('node:events');
const {showAppDialog}=require('../app-dialog');
function fixture(){
 const ipcMain=new EventEmitter(),windows=[];
 class Window extends EventEmitter {
  constructor(options={}){super();this.options=options;this.dead=false;this.webContents=new EventEmitter();this.webContents.mainFrame={};this.webContents.send=(channel,value)=>{this.content={channel,value};};this.webContents.session={setPermissionRequestHandler(){},setPermissionCheckHandler(){}};this.webContents.setWindowOpenHandler=()=>{};windows.push(this);}
  getBounds(){return {x:0,y:0,width:1000,height:800};}isDestroyed(){return this.dead;}show(){this.shown=true;}destroy(){this.dead=true;this.emit('closed');}loadFile(){return Promise.resolve();}
 }
 const parent=new Window(),electron={BrowserWindow:Window,ipcMain,screen:{getDisplayMatching:()=>({workArea:{width:1000,height:800}})}};
 return {ipcMain,parent,windows,electron,event:w=>({sender:w.webContents,senderFrame:w.webContents.mainFrame}),tick:()=>new Promise(resolve=>setImmediate(resolve))};
}
test('only the local dialog main frame can explicitly confirm; preserve checkbox opt-in',async()=>{
 const f=fixture(),result=showAppDialog(f.electron,f.parent,{buttons:['Cancel','Allow'],cancelId:0,defaultId:0,checkboxLabel:'Remember'},()=>({theme:'dark'}));await f.tick();const win=f.windows[1];
 assert.equal(win.options.webPreferences.sandbox,true);assert.equal(win.options.webPreferences.nodeIntegration,false);assert.equal(win.options.modal,true);
 f.ipcMain.emit('offgrid-dialog-ready',f.event(f.parent));assert.equal(win.shown,undefined);
 f.ipcMain.emit('offgrid-dialog-ready',f.event(win));assert.equal(win.shown,true);assert.equal(win.content.value.cancelId,0);
 f.ipcMain.emit('offgrid-dialog-answer',f.event(f.parent),{response:1,checkboxChecked:true});assert.equal(win.dead,false);
 f.ipcMain.emit('offgrid-dialog-answer',{sender:win.webContents,senderFrame:{}},{response:1});assert.equal(win.dead,false);
 f.ipcMain.emit('offgrid-dialog-answer',f.event(win),{response:90});assert.equal(win.dead,false);
 f.ipcMain.emit('offgrid-dialog-answer',f.event(win),{response:1,checkboxChecked:true});assert.deepEqual(await result,{response:1,checkboxChecked:true});assert.equal(win.dead,true);
 assert.equal(f.ipcMain.listenerCount('offgrid-dialog-answer'),0);
});
for(const reason of ['closed','hide','reload','crash'])test('dialog cancels safely on '+reason,async()=>{
 const f=fixture(),result=showAppDialog(f.electron,f.parent,{buttons:['Cancel','Stop'],cancelId:0},()=>({}));await f.tick();const win=f.windows[1];
 if(reason==='reload')f.parent.webContents.emit('did-start-loading');else if(reason==='crash')win.webContents.emit('render-process-gone');else f.parent.emit(reason);
 assert.deepEqual(await result,{response:0,checkboxChecked:false});assert.equal(f.ipcMain.listenerCount('offgrid-dialog-ready'),0);
});
test('queued dialogs do not stack or grant on close',async()=>{
 const f=fixture(),show=()=>showAppDialog(f.electron,f.parent,{buttons:['Cancel','Allow']},()=>({}));const first=show(),second=show();await f.tick();assert.equal(f.windows.length,2);
 f.windows[1].destroy();assert.equal((await first).response,0);await f.tick();assert.equal(f.windows.length,3);f.windows[2].destroy();assert.equal((await second).response,0);
});

test('navigation invalidates queued prompts as well as the visible prompt',async()=>{
 const f=fixture(),show=()=>showAppDialog(f.electron,f.parent,{buttons:['Cancel','Allow']},()=>({}));const first=show(),second=show();await f.tick();
 f.parent.webContents.emit('did-start-loading');assert.equal((await first).response,0);assert.equal((await second).response,0);assert.equal(f.windows.length,2);
});
