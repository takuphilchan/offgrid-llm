'use strict';
const path=require('node:path'),fs=require('node:fs'),{pathToFileURL}=require('node:url');
const {TerminalSessions}=require('./terminal-sessions');
const {createTerminalProtocol}=require('./terminal-protocol');
const {validateWorkspaceBounds,terminalBounds}=require('./terminal-layout');

function createTerminalDock({electron,window,host,cli,service,presentation}){
  const {WebContentsView,ipcMain,dialog,clipboard}=electron;
  const url=pathToFileURL(path.join(__dirname,'terminal.html')).href;
  let view,visible=false,overlay=false,workspaceBounds,loaded=false,loading=null,stopping=false,cleanup=null,visibilityRevision=0;
  const emit=value=>{if(view&&!view.webContents.isDestroyed())view.webContents.send('terminal-event',value);};
  const sessions=new TerminalSessions({host,runner:process.execPath,worker:path.join(__dirname,'terminal-worker.js'),cli,service,
    choose:async()=>{const result=await dialog.showOpenDialog(window,{title:'Choose a local terminal folder',properties:['openDirectory']});return result.canceled?null:result.filePaths[0];},
    confirm:async r=>(await dialog.showMessageBox(window,{type:'question',title:'Close terminal?',message:'Stop this terminal and its running commands?',detail:r.cwd,buttons:['Cancel','Stop and close'],defaultId:0,cancelId:0})).response===1,
    send:emit});
  function layout(){
    if(window.isDestroyed())return {};
    const [width,height]=window.getContentSize();
    const bounds=terminalBounds(workspaceBounds,width,height,window.webContents.getZoomFactor());
    view?.setBounds(bounds);view?.setVisible(visible&&!overlay);
    window.webContents.send('terminal-visibility',visible);emit({type:'layout',visible});return bounds;
  }
  function hide(){visibilityRevision++;visible=false;emit({type:'visibility',visible:false});layout();window.webContents.focus();return {visible:false};}
  const protocol=createTerminalProtocol({trustedURL:url,contents:()=>view?.webContents,available:()=>sessions.available()&&!cleanup,operations:{
    create:request=>sessions.create(request),attach:id=>sessions.attach(id),input:(id,text)=>sessions.input(id,text),resize:(id,cols,rows)=>sessions.resize(id,cols,rows),ack:(id,sequence)=>sessions.ack(id,sequence),close:id=>sessions.close(id),layout,hide,copy:text=>clipboard.writeText(text),presentation:()=>({...presentation(),visible:visible&&!overlay})
  }});
  function cleanupRenderer(){
    protocol.invalidate();
    if(!cleanup)cleanup=sessions.stop().then(()=>{protocol.releaseStopped();cleanup=null;emit({type:'availability',available:sessions.available()});}).catch(()=>{emit({type:'availability',available:false});emit({type:'error',message:'Terminal cleanup could not be confirmed. Quit and retry stopping before opening another terminal.'});});
  }
  ipcMain.removeHandler('terminal-request');
  ipcMain.handle('terminal-request',(event,request)=>protocol.dispatch(event,request));
  window.on('closed',()=>{protocol.invalidate();void sessions.stop().catch(()=>{});});
  window.on('resize',()=>layout());window.webContents.on('did-finish-load',()=>layout());
  return {
    async open(){
      if(stopping)throw Error('terminal_stopping');
      const intent=visibilityRevision;
      if(!view){
        view=new WebContentsView({webPreferences:{preload:path.join(__dirname,'terminal-preload.js'),sandbox:true,contextIsolation:true,nodeIntegration:false,partition:'offgrid-terminal',spellcheck:false}});
        view.setBackgroundColor(presentation().theme==='dark'?'#191919':'#ffffff');
        window.contentView.addChildView(view);
        view.webContents.setWindowOpenHandler(()=>({action:'deny'}));
        view.webContents.on('will-navigate',event=>event.preventDefault());
        view.webContents.session.setPermissionRequestHandler((_wc,_permission,callback)=>callback(false));
        view.webContents.on('render-process-gone',()=>{loaded=false;cleanupRenderer();});
        view.webContents.on('did-start-loading',()=>{if(loaded)cleanupRenderer();});
      }
      const target=view;
      // Several entry points can request the terminal during its first load.
      // A second loadURL aborts the first navigation even when the second succeeds.
      if(!loaded){
        if(!loading)loading=(async()=>{
          await cleanup;
          if(stopping||view!==target)throw Error('terminal_stopping');
          await target.webContents.loadURL(url);
          if(stopping||view!==target||target.webContents.isDestroyed())throw Error('terminal_stopping');
          loaded=true;
        })().finally(()=>{loading=null;});
        await loading;
      }
      if(stopping||view!==target||target.webContents.isDestroyed())throw Error('terminal_stopping');
      if(intent!==visibilityRevision)return {visible:false};
      visible=true;layout();if(!overlay)view.webContents.focus();emit({type:'visibility',visible:!overlay});return {visible:true};
    },
    setWorkspaceBounds(value){workspaceBounds=validateWorkspaceBounds(value);return layout();},
    setOverlayActive(value){
      if(typeof value!=='boolean')throw Error('terminal_invalid_overlay');
      if(overlay===value)return;overlay=value;
      // Native views sit above every DOM z-index, including the HTML top layer.
      // Suspend only their presentation/input; never resize, restart or replay a shell.
      view?.setVisible(visible&&!overlay);
      emit({type:'visibility',visible:visible&&!overlay,focus:false});
    },
    hide,
    toggle(){return visible?Promise.resolve(hide()):this.open();},
    appearance(){emit({type:'appearance',...presentation()});},
    async stop(){stopping=true;protocol.invalidate();try{await sessions.stop();if(view&&!view.webContents.isDestroyed())view.webContents.close();view=undefined;loaded=false;visible=false;layout();}finally{stopping=false;}},
    get sessions(){return sessions;}
  };
}
module.exports={createTerminalDock};
