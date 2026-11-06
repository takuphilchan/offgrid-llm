const test=require('node:test'),assert=require('node:assert/strict');
const {createTerminalDock}=require('../terminal-dock');

function loadingDock(loadURL){
  let instance;
  class View {
    constructor(){instance=this;this.closed=false;this.webContents={isDestroyed:()=>this.closed,send(){},setWindowOpenHandler(){},on(){},session:{setPermissionRequestHandler(){}},loadURL,focus(){},close:()=>{this.closed=true;}};}
    setBackgroundColor(){} setBounds(){} setVisible(value){this.visible=value;}
  }
  const window={isDestroyed:()=>false,getContentSize:()=>[1200,800],webContents:{getZoomFactor:()=>1,send(){},on(){},focus(){}},contentView:{addChildView(){}},on(){}};
  const dock=createTerminalDock({electron:{WebContentsView:View,ipcMain:{removeHandler(){},handle(){}},dialog:{},clipboard:{}},window,presentation:()=>({theme:'dark'})});
  return {dock,view:()=>instance};
}

test('concurrent terminal opens share one navigation and failures remain retryable',async()=>{
  let resolveLoad,rejectLoad,loads=0;
  const {dock}=loadingDock(()=>{loads++;return new Promise((resolve,reject)=>{resolveLoad=resolve;rejectLoad=reject;});});
  const first=dock.open(),second=dock.open();
  await Promise.resolve();assert.equal(loads,1);
  rejectLoad(Error('real navigation failure'));
  const failures=await Promise.allSettled([first,second]);
  assert.ok(failures.every(result=>result.status==='rejected'));
  const retry=dock.open();await Promise.resolve();assert.equal(loads,2);
  resolveLoad();assert.deepEqual(await retry,{visible:true});
  await dock.stop();
});

test('closing during terminal loading never reopens the destroyed view',async()=>{
  let resolveLoad;
  const {dock,view}=loadingDock(()=>new Promise(resolve=>{resolveLoad=resolve;}));
  const opening=dock.open();await Promise.resolve();
  await dock.stop();resolveLoad();
  await assert.rejects(opening,/terminal_stopping/);
  assert.equal(view().closed,true);
  assert.notEqual(view().visible,true);
});

test('web overlays suspend native presentation without resizing or recreating shells',async()=>{
  let instance,loads=0,focus=0;const events=[],bounds=[];
  class View {
    constructor(){instance=this;this.webContents={isDestroyed:()=>false,send:(_channel,value)=>events.push(value),setWindowOpenHandler(){},on(){},session:{setPermissionRequestHandler(){}},loadURL:async()=>{loads++;},focus:()=>{focus++;},close(){}};}
    setBackgroundColor(){} setBounds(value){bounds.push(value);} setVisible(value){this.visible=value;}
  }
  const window={isDestroyed:()=>false,getContentSize:()=>[1200,800],webContents:{getZoomFactor:()=>1,send(){},on(){},focus(){}},contentView:{addChildView(){}},on(){}};
  const dock=createTerminalDock({electron:{WebContentsView:View,ipcMain:{removeHandler(){},handle(){}},dialog:{},clipboard:{}},window,presentation:()=>({theme:'dark'})});
  dock.setWorkspaceBounds({x:248,y:0,width:952,height:800});await dock.open();
  assert.equal(instance.visible,true);const count=bounds.length;
  dock.setOverlayActive(true);assert.equal(instance.visible,false);
  dock.setOverlayActive(true);dock.setOverlayActive(false);
  assert.equal(instance.visible,true);assert.equal(bounds.length,count);
  assert.equal(loads,1);assert.equal(focus,1,'closing a menu must not steal focus from its trigger');
  assert.deepEqual(events.filter(e=>e.type==='visibility').slice(-2),[{type:'visibility',visible:false,focus:false},{type:'visibility',visible:true,focus:false}]);
  for(const value of [null,{},'true',1])assert.throws(()=>dock.setOverlayActive(value),/invalid_overlay/);
  dock.hide();dock.setOverlayActive(true);dock.setOverlayActive(false);
  assert.equal(instance.visible,false,'closing an overlay must not reopen a hidden terminal');
});


test('returning to Chat during terminal loading does not reopen the terminal later',async()=>{
  let complete,loads=0;
  const {dock,view}=loadingDock(()=>{loads++;return new Promise(resolve=>{complete=resolve;});});
  const opening=dock.open();await Promise.resolve();
  dock.hide();complete();
  assert.deepEqual(await opening,{visible:false});
  assert.equal(view().visible,false);
  assert.deepEqual(await dock.open(),{visible:true});
  assert.equal(loads,1,'returning to the terminal reuses its ready renderer');
});
