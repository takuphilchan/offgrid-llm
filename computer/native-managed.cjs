// Owned Electron utility process. Native secrets and worker handles never enter
// the renderer. Target choices refer only to this worker's most recent list.
const {NativeWorker} = require('./native-worker.cjs');
let worker, session, selectedWorkspace, service, directory, stopped=false, busy=false;
const send=message=>process.parentPort.postMessage(message);
async function stop() {
  stopped=true;
  try { if(session)await session.stop();else await worker?.stop();send({state:'stopped'});process.exit(0); }
  catch {send({state:'error',code:'computer_stop_unconfirmed'});}
}
process.parentPort.on('message',async({data})=>{
  if(data?.type==='stop'){await stop();return;}
  if(stopped || busy)return;
  busy=true;
  try {
    if(data?.type==='discover' && !worker) {
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
