'use strict';
// Explicit hidden, disposable Electron qualification fixture. It never reads
// an installed profile, registers protocols/shortcuts, or opens personal apps.
const {app,BrowserWindow,ipcMain,screen}=require('electron');
const fs=require('node:fs/promises'),path=require('node:path'),{spawn}=require('node:child_process'),{randomBytes}=require('node:crypto');
const {DesktopRuntime,availablePort}=require('../../runtime');
const {fingerprintUI}=require('../../compatibility.cjs');
const {isTrustedSender,workspaceChannelAllowed}=require('../../backend');
const {createWorkspaceRecovery}=require('../../workspace-maintenance-host');
const {showAppDialog}=require('../../app-dialog');
const root=process.env.OFFGRID_RECOVERY_FIXTURE_ROOT,binary=process.env.OFFGRID_RECOVERY_TEST_BINARY;
if(!root||!path.isAbsolute(root)||!binary||process.env.OFFGRID_RECOVERY_ELECTRON_TEST!=='1')throw Error('Explicit isolated fixture required');
app.setPath('userData',path.join(root,'electron'));
let runtime,recovery,win,cleaned=false,closing=false;
app.whenReady().then(async()=>{
 const data=path.join(root,'data'),models=path.join(root,'models'),ui=path.resolve(__dirname,'../../../web/dist'),config=path.join(root,'config.json'),port=await availablePort();
 for(const directory of [data,models])await fs.mkdir(directory,{recursive:true});
 await fs.writeFile(path.join(data,'fixture.txt'),'isolated Electron recovery content');
 // Real incompressible data keeps cancellation exercisable during authentication
 // and extraction instead of racing an already-completed tiny archive.
 await fs.writeFile(path.join(data,'payload.bin'),randomBytes(32*1024*1024));
 await require('./recovery-agent-state.cjs').seed(data);
 await fs.writeFile(config,JSON.stringify({use_mock_engine:true,enable_p2p:false,prewarm_models:false,models_dir:models,data_dir:data,server_host:'127.0.0.1',server_port:port}));
 const env={...process.env,HOME:root,USERPROFILE:root,APPDATA:root,LOCALAPPDATA:root,OFFGRID_CONFIG:config,OFFGRID_ENABLE_P2P:'false',OFFGRID_PREWARM_MODELS:'false',OFFGRID_REQUIRE_AUTH:'false',OFFGRID_INFERENCE_ENDPOINTS_FILE:''};
 runtime=new DesktopRuntime({url:`http://127.0.0.1:${port}`,version:'desktop-recovery-fixture',binary,uiDir:ui,uiBuildID:fingerprintUI(await fs.readFile(path.join(ui,'index.html'))),workspace:{data,models},ownedStdio:true,retryMs:20,timeoutMs:15000,maintenanceGraceMs:15000,
  spawn:(file,args,options)=>spawn(file,args,{...options,env:{...options.env,...env,OFFGRID_PORT:String(port),OFFGRID_UI_DIR:ui}})});
 if((await runtime.connect()).state!=='ready')throw Error('Isolated service did not become ready');
 win=new BrowserWindow({show:false,width:980,height:700,webPreferences:{preload:path.resolve(__dirname,'../../preload.js'),sandbox:true,contextIsolation:true,nodeIntegration:false}});
 const handle=(channel,handler)=>ipcMain.handle(channel,(event,...args)=>{
  if(!isTrustedSender(event,win.webContents,runtime.url,'')||!workspaceChannelAllowed(channel,runtime.state.state==='ready'&&!runtime.maintenanceAdmissionHold,false))throw Error('Untrusted fixture IPC');return handler(...args);
 });
 for(const [channel,value] of Object.entries({'get-api-url':()=>runtime.url,'get-app-version':()=> 'desktop-recovery-fixture','get-server-status':()=>runtime.state.state==='ready','get-backend-info':()=>runtime.snapshot(),'get-system-theme':()=> 'light','command-window-status':()=>({mode:'full',shortcut:'',registered:false}),'terminal-status':()=>({available:false}),'get-paths':()=>runtime.workspace}))handle(channel,value);
 // Keep real host-owned DOM/isolated preload and IPC, but never show a native
 // window on the user's desktop. Native picker answers are the only injected UI.
 function HiddenDialog(options){const child=new BrowserWindow({...options,show:false});child.show=()=>{};return child;}
 const selection={source:path.join(root,'backup.age'),destination:path.join(root,'backup.age')};
 recovery=createWorkspaceRecovery({runtime,window:()=>win,locale:()=> 'en',handle,computer:{stop:async()=>{}},execution:{stop:async()=>{}},terminal:()=>null,
  dialog:{showOpenDialog:async()=>({canceled:false,filePaths:[selection.source]}),showSaveDialog:async()=>({canceled:false,filePath:selection.destination})},
  showMessage:(parent,options)=>showAppDialog({BrowserWindow:HiddenDialog,ipcMain,screen},parent,options,()=>({theme:'light',locale:'en',direction:'ltr'}))});
 recovery.options.environment=()=>env;
 win.webContents.on('did-start-navigation',(_event,_url,inPlace,main)=>{if(main&&!inPlace)recovery.invalidate();});
 win.webContents.on('render-process-gone',()=>recovery.invalidate());
 global.recoveryFixture={runtime,recovery,selection,root};
 await win.loadURL(runtime.url+'/ui/#/settings/recovery');
}).catch(()=>{process.stderr.write('Isolated recovery fixture startup failed\n');app.exit(1);});
app.on('window-all-closed',()=>{if(!closing)app.quit();});
app.on('before-quit',event=>{
 if(cleaned)return;event.preventDefault();if(closing)return;closing=true;
 void (async()=>{await recovery?.stop();await runtime?.stop();await fs.writeFile(path.join(root,'cleanup-complete'),'owned children drained');cleaned=true;app.quit();})().catch(()=>{closing=false;process.stderr.write('Isolated recovery fixture cleanup unconfirmed\n');});
});
