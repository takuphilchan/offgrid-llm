const test=require('node:test'),assert=require('node:assert/strict');
const {createTerminalDock}=require('../terminal-dock');

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
