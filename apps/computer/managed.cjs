// Electron utility entry. All credentials arrive over private IPC, never argv.
const controller=import('./managed-controller.mjs').then(({ManagedBrowserController})=>{
  const owner=new ManagedBrowserController({send:message=>process.parentPort.postMessage(message),exit:code=>process.exit(code)});
  owner.boot();return owner;
});
process.parentPort.on('message',async({data})=>{
  try{await (await controller).command(data);}
  catch{process.parentPort.postMessage({state:'error',code:'computer_stop_unconfirmed'});}
});
