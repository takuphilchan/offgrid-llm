'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path'),os=require('node:os');
const {spawn}=require('node:child_process');
const {DesktopRuntime,availablePort}=require('../runtime');
const {createManifest}=require('../compatibility.cjs');
const {createWorkspaceRecovery}=require('../workspace-maintenance-host');
const binary=process.env.OFFGRID_RECOVERY_TEST_BINARY;

// Opt-in real production binary, disposable configuration/data and loopback
// service only. Native dialog answers are injected; this is not installed UI or
// participant qualification and never attaches to an existing service.
test('real owned service drains into host-framed encrypted recovery and resumes only its original data',{skip:!binary,timeout:90000},async t=>{
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'offgrid-recovery-host-'));
 const data=path.join(root,'data'),models=path.join(root,'models'),ui=path.join(root,'ui'),archive=path.join(root,'misleading.zip'),restored=path.join(root,'restored');
 for(const directory of [data,models,ui])await fs.mkdir(directory);
 const config=path.join(root,'config.json'),index=Buffer.from('<!doctype html><title>Isolated recovery fixture</title>'),manifest=createManifest(index),port=await availablePort();
 await fs.writeFile(path.join(ui,'index.html'),index);await fs.writeFile(path.join(ui,'desktop-compatibility.json'),JSON.stringify(manifest));
 await fs.writeFile(config,JSON.stringify({use_mock_engine:true,enable_p2p:false,prewarm_models:false,models_dir:models,data_dir:data,server_host:'127.0.0.1',server_port:port}));
 await fs.writeFile(path.join(data,'fixture.txt'),'isolated recovery content');
 const env={...process.env,HOME:root,USERPROFILE:root,APPDATA:root,LOCALAPPDATA:root,OFFGRID_CONFIG:config,OFFGRID_ENABLE_P2P:'false',OFFGRID_PREWARM_MODELS:'false',OFFGRID_REQUIRE_AUTH:'false',OFFGRID_INFERENCE_ENDPOINTS_FILE:''};
 const runtime=new DesktopRuntime({url:`http://127.0.0.1:${port}`,version:'desktop-recovery-fixture',binary:path.resolve(binary),uiDir:ui,uiBuildID:manifest.ui_build_id,
  workspace:{data,models},ownedStdio:true,retryMs:20,timeoutMs:15000,maintenanceGraceMs:15000,spawn:(file,args,options)=>spawn(file,args,{...options,env:{...options.env,...env,OFFGRID_PORT:String(port),OFFGRID_UI_DIR:ui}})});
 let recovery;
 t.after(async()=>{await recovery?.stop();await runtime.stop();await fs.rm(root,{recursive:true,force:true});});
 assert.equal((await runtime.connect()).state,'ready');
 const channels=new Map(),events=[];let destination=archive,source=archive,secret='isolated fixture passphrase';
 const win={isDestroyed:()=>false,webContents:{mainFrame:{routingId:1},session:{fetch},send:(_channel,state)=>events.push(state)}};
 recovery=createWorkspaceRecovery({runtime,window:()=>win,locale:()=> 'en',handle:(channel,handler)=>channels.set(channel,handler),computer:{stop:async()=>{}},execution:{stop:async()=>{}},terminal:()=>null,
  dialog:{showSaveDialog:async()=>({canceled:false,filePath:destination}),showOpenDialog:async()=>({canceled:false,filePaths:[source]})},showMessage:async(_win,options)=>options.secret?{response:1,secret}:{response:1}});
 recovery.options.environment=()=>env;
 const run=async(operation,encryption='passphrase')=>{const first=channels.get('workspace-recovery-start')({operation,encryption});await recovery.current.done;assert.equal(runtime.child,null);assert.equal(runtime.maintenanceDrained,true);return {first,state:channels.get('workspace-recovery-status')()};};
 const resume=async id=>{const state=await channels.get('workspace-recovery-resume')({id});assert.equal(state.phase,'idle');assert.equal(runtime.state.state,'ready');assert.equal(runtime.workspace.data,data);};
 const backup=await run('backup');assert.equal(backup.state.phase,'succeeded');assert.equal(backup.state.result.authenticated,true);assert.equal(backup.state.result.published,true);
 assert.equal((await fs.readFile(archive)).subarray(0,22).toString(),'age-encryption.org/v1\n');await resume(backup.first.id);
 secret='wrong fixture passphrase';const wrong=await run('verify');assert.equal(wrong.state.phase,'failed');assert.equal(wrong.state.result,undefined);await resume(wrong.first.id);
 secret='isolated fixture passphrase';destination=restored;const restore=await run('restore');assert.equal(restore.state.phase,'succeeded');assert.equal(restore.state.result.database_checked,true);assert.equal(await fs.readFile(path.join(restored,'fixture.txt'),'utf8'),'isolated recovery content');
 assert.equal(runtime.child,null);await resume(restore.first.id);
 const corrupt=path.join(root,'corrupt.age'),bytes=await fs.readFile(archive);bytes[Math.floor(bytes.length/2)]^=1;await fs.writeFile(corrupt,bytes);source=corrupt;
 const damaged=await run('verify');assert.equal(damaged.state.phase,'failed');await resume(damaged.first.id);
 assert.doesNotMatch(JSON.stringify(events),/isolated fixture passphrase|wrong fixture passphrase|fixture.txt|manifest|key_material/);
});
