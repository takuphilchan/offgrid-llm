// Owned Electron utility process. Native secrets and worker handles never enter
// the renderer. Target choices refer only to this worker's most recent list.
const {NativeWorker} = require('./native-worker.cjs');
let worker, session, permission, selectedWorkspace, service, directory, stopped=false, busy=false;
const send=message=>process.parentPort.postMessage(message);
async function stop() {
  stopped=true;
  try { if(session)await session.stop();await worker?.stop();send({state:'stopped'});process.exit(0); }
  catch {send({state:'error',code:'computer_stop_unconfirmed'});}
}
process.parentPort.on('message',async({data})=>{
  if(data?.type==='stop'){await stop();return;}
  if(stopped || busy)return;
  busy=true;
  try {
    if(data?.type==='enable-desktop' && !worker) {
      worker=new NativeWorker({root:__dirname,directory:require('node:path').join(data.directory,'native-worker'),protocol:3});
      await worker.start();if(stopped){await stop();return;}
      worker.on('stopped',()=>{if(!session&&!stopped){stopped=true;send({state:'stopped'});process.exit(0);}});
      const candidate={service:data.service,workspace:data.workspace,actor:data.actor};
      send({state:'consent'});
      const result=await worker.request('enable_desktop',{permission:candidate});
      if(stopped){await stop();return;}
      if(result?.enabled!==true)throw Error('computer_scope_violation');
      permission=candidate;service=data.service;directory=data.directory;
      send({state:'access_ready'});
    } else if(data?.type==='start-desktop' && (!worker || permission && !session)) {
      if(permission && (data.service!==permission.service||data.workspace!==permission.workspace||data.actor!==permission.actor||data.approvalMode!=='full_task'))throw Error('computer_scope_violation');
      if(!worker){worker=new NativeWorker({root:__dirname,directory:require('node:path').join(data.directory,'native-worker'),protocol:3});await worker.start();}
      if(stopped){await stop();return;}
      const {DesktopSession}=await import('./desktop-session.mjs');
      const releaseWorker=permission?async()=>{
        if(stopped)return worker.stop();
        try{const result=await worker.request('release_desktop');if(result?.released!==true)throw Error('computer_stop_unconfirmed');}
        catch(error){await worker.stop();throw error;}
      }:undefined;
      session=new DesktopSession({service:data.service,directory:data.directory,worker,emit:send,releaseWorker});
      await session.start({code:data.code,workspace:data.workspace,run:data.run,input:data.input,prompt:data.prompt,approvalMode:data.approvalMode});
      if(stopped)return;
      await session.running;
      if(!permission||stopped||!worker.child||worker.revoked)process.exit(0);
      session=null;send({state:'access_ready'});
    } else if(data?.type==='discover' && !worker) {
      service=data.service;directory=data.directory;selectedWorkspace=data.workspace;
      worker=new NativeWorker({root:__dirname,directory:require('node:path').join(directory,'native-worker')});
      await worker.start();if(stopped){await stop();return;}
      const targets=await worker.request('targets');
      if(stopped)return;
      send({state:'selecting',targets:targets.map(t=>({id:t.identity.id,title:t.title,driver:t.identity.driver}))});
    } else if(data?.type==='start-native' && worker && !session) {
      const target=worker.targets?.find(t=>t.identity.id===data.target);
      if(!target)throw Error('computer_stale_target');
      const {NativeSession}=await import('./native-session.mjs');
      session=new NativeSession({service,directory,worker,emit:send});
      const paired=await session.start({code:data.code,workspace:selectedWorkspace,target,approvalMode:data.approvalMode});
      if(stopped)return;
      send({state:'ready',target:paired});
      await session.running;process.exit(0);
    } else throw Error('computer_invalid_action');
  } catch(error) {
    if(!stopped)send({state:'error',code:/^computer_[a-z_]+$/.test(error.message)?error.message:'computer_worker_unavailable'});
    try {await session?.stop();await worker?.stop();}catch{}
    process.exit(1);
  } finally {busy=false;}
});
send({state:'booted'});
