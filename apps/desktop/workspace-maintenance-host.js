'use strict';
const {WorkspaceMaintenance}=require('./workspace-maintenance');
const {localizedCopy}=require('./locale-messages');
const path=require('node:path'),fs=require('node:fs');
const base=JSON.parse(fs.readFileSync(fs.existsSync(path.join(__dirname,'workspace-recovery.json'))?path.join(__dirname,'workspace-recovery.json'):path.join(__dirname,'../web/src/i18n/workspace-recovery.json'),'utf8'));
async function boundedJSON(response){
 if(!response.ok)throw Error('maintenance_authority_changed');
 const reader=response.body.getReader();let length=0;const chunks=[];
 try{for(;;){const {done,value}=await reader.read();if(done)break;length+=value.length;if(length>65536)throw Error('maintenance_authority_changed');chunks.push(value);}return JSON.parse(Buffer.concat(chunks).toString('utf8'));}
 finally{await reader.cancel().catch(()=>{});}
}
function createWorkspaceRecovery(options){
 const {runtime,dialog,window,showMessage,locale,computer,execution,terminal}=options;
 const text=()=>localizedCopy({en:base},locale(),'i18n/workspace-recovery:copy');
 const open=async title=>{const result=await dialog.showOpenDialog(window(),{title,properties:['openFile','dontAddToRecent']});return result.canceled?null:result.filePaths[0];};
 const save=async title=>{const result=await dialog.showSaveDialog(window(),{title,properties:['dontAddToRecent']});return result.canceled?null:result.filePath;};
 const recovery=new WorkspaceMaintenance({runtime,
  authority:async()=>{
   const win=window(),state=runtime.snapshot();
   if(!win||win.isDestroyed()||state.state!=='ready'||!runtime.child?.__ownedStdio||!state.workspaceID)throw Error('maintenance_owned_service_required');
   const get=endpoint=>win.webContents.session.fetch(runtime.url+endpoint,{credentials:'include',redirect:'error',signal:AbortSignal.timeout(5000)}).then(boundedJSON);
   const account=await get('/v1/users/me'),identity=await get('/api/v2/system');
   let actor;if(account.auth_required===false)actor='local-admin';else if(account.auth_required===true&&account.authenticated===true&&account.user?.role==='admin'&&typeof account.user.id==='string'&&account.user.id)actor=account.user.id;
   if(!actor||identity.product!=='offgrid'||identity.workspace_id!==state.workspaceID||identity.version!==runtime.options.version||identity.ui_build_id!==runtime.options.uiBuildID)throw Error('maintenance_authority_changed');
   return {actor,workspace:identity.workspace_id,version:identity.version,ui:identity.ui_build_id,url:runtime.url,document:win.webContents.mainFrame.routingId};
  },
  select:async request=>{
   const copy=text(),selection={};
   if(request.operation!=='backup'){selection.source=await open(copy.archive);if(!selection.source||request.signal.aborted)return null;}
   if(request.operation!=='verify'){selection.destination=await save(request.operation==='backup'?copy.destination:copy.restoreDestination);if(!selection.destination||request.signal.aborted)return null;}
   if(request.encryption==='recipient'){selection.key=await open(request.operation==='backup'?copy.recipients:copy.identities);if(!selection.key||request.signal.aborted)return null;}
   return selection;
  },
  secret:async(operation,signal)=>{
   const copy=text(),answer=await showMessage(window(),{title:copy.title,message:copy.passphrase,detail:copy.secretDetail,buttons:[copy.cancel,copy.continue],cancelId:0,defaultId:0,signal,
    secret:{label:copy.passphrase,confirm:operation==='backup',confirmLabel:copy.confirmPassphrase,invalid:copy.invalidPassphrase}});
   return answer.response===1?answer.secret:null;
  },
  confirm:async(selection,signal)=>{
   const copy=text(),detail=[copy[selection.operation],copy[selection.encryption==='passphrase'?'passphraseMode':selection.encryption],`${copy.workspace}: ${selection.workspace}`,
    selection.source?`${copy.source}: ${selection.source}`:null,selection.destination?`${copy.target}: ${selection.destination}`:null,copy.scope,copy.confirmDetail].filter(Boolean).join('\n\n');
   return (await showMessage(window(),{title:copy.title,message:copy.confirmTitle,detail,buttons:[copy.cancel,copy.continue],defaultId:0,cancelId:0,signal})).response===1;
  },
  quiesce:async()=>{
   const results=await Promise.allSettled([computer.stop(),execution.stop(),terminal()?.stop()]);
   if(results.some(result=>result.status==='rejected')||computer.child||computer.pending||computer.stopping||execution.child||execution.pending||execution.state==='uncertain')throw Error('maintenance_stop_unconfirmed');
  },environment:()=>({...process.env})
 });
 recovery.on('status',state=>{const win=window();if(win&&!win.isDestroyed())win.webContents.send('workspace-recovery-state',state);});
 options.handle('workspace-recovery-status',()=>recovery.snapshot());
 options.handle('workspace-recovery-start',request=>recovery.start(request));
 options.handle('workspace-recovery-cancel',request=>recovery.cancel(request));
 options.handle('workspace-recovery-resume',request=>recovery.resume(request));
 return recovery;
}
module.exports={createWorkspaceRecovery,base,boundedJSON};
