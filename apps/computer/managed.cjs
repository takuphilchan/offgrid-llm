// Electron utility entry. All credentials arrive over private IPC, never argv.
let session;
let started = false;
let stopping = false;
let startingFinished;
let finishStarting;
const send = message => process.parentPort.postMessage(message);
process.parentPort.on('message', async ({data}) => {
  if (data?.type === 'stop') { stopping = true; await session?.stop(); await startingFinished; send({state:'stopped'}); process.exit(0); }
  if (data?.type !== 'start' || started || stopping) return;
  started = true;
  startingFinished = new Promise(resolve=>{finishStarting=resolve;});
  try {
    const {CompanionSession} = await import('./session.mjs');
    session = new CompanionSession({service:data.service, directory:data.directory, emit:send, upload:data.upload});
    if (stopping) { finishStarting(); return; }
    const target = await session.start({origin:data.origin, code:data.code, workspace:data.workspace, networkMode:data.networkMode, approvalMode:data.approvalMode});
    finishStarting();
    send({state:'ready', target});
    await session.running;
    process.exit(0);
  } catch (error) {
    await session?.stop();
    finishStarting();
    if(stopping) return;
    const code = ['workspace_changed','service_invalid','target_invalid','network_blocked','network_unavailable','network_mode_invalid'].includes(error.message) ? error.message : 'browser_unavailable';
    send({state:'error',code}); process.exit(1);
  }
});
send({state:'booted'});
