import {spawn} from 'node:child_process';
import {isAbsolute,join} from 'node:path';
import {verifyNativePack} from './pack.cjs';
import {assertUniqueJSONKeys} from './native-json-keys.mjs';

const unavailable=()=>Error('computer_storage_unavailable');
const decoder=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true});
const keys=(value,wanted)=>value&&Object.getPrototypeOf(value)===Object.prototype&&Object.keys(value).sort().join(',')===wanted;
function requestFrame(roots){
 if(!Array.isArray(roots)||roots.length>16||roots.some(root=>typeof root!=='string'||!isAbsolute(root)||Buffer.byteLength(root)>4096||root.includes('\0')))throw unavailable();
 const body=Buffer.from(JSON.stringify({protocol:1,kind:'prepare_browser_storage',workspace_roots:roots}));
 if(body.length>16384)throw unavailable();
 const header=Buffer.alloc(4);header.writeUInt32BE(body.length);return Buffer.concat([header,body]);
}
function reply(bytes){
 if(bytes.length<4||bytes.length>516||bytes.readUInt32BE()!==bytes.length-4)throw unavailable();
 const text=decoder.decode(bytes.subarray(4));assertUniqueJSONKeys(text);const value=JSON.parse(text);
 if(!keys(value,'id,protocol,result')||value.protocol!==1||value.id!=='browser-storage'||!keys(value.result,'browser_storage,ready')||value.result.ready!==true||value.result.browser_storage!==1)throw unavailable();
 return true;
}

// Runs only the verified pack's storage-only mode. No UI/native input owner is
// started, and neither private roots nor provider diagnostics leave this owner.
export class BrowserStorageCheck {
 constructor({root,directory,workspaceRoots,verify=verifyNativePack,spawnWorker=spawn,platform=process.platform,arch=process.arch,timeoutMs=12000,stopMs=3000}){
  Object.assign(this,{root,directory,workspaceRoots:Array.isArray(workspaceRoots)?Object.freeze([...workspaceRoots]):workspaceRoots,verify,spawnWorker,platform,arch,timeoutMs,stopMs});this.stopped=false;this.abort=new AbortController();
 }
 verifyProfile(){
  return new Promise((resolve,reject)=>{
   let settled=false;
   const finish=(error,value)=>{if(settled)return;settled=true;clearTimeout(timer);this.abort.signal.removeEventListener('abort',cancel);error?reject(error):resolve(value);};
   const cancel=()=>finish(Error('computer_stopped'));
   const timer=setTimeout(()=>finish(unavailable()),this.timeoutMs);
   this.abort.signal.addEventListener('abort',cancel,{once:true});
   if(this.stopped){cancel();return;}
   Promise.resolve().then(()=>this.verify(this.root,this.platform,this.arch)).then(value=>finish(null,value),error=>finish(error));
  });
 }
 prepare(){
  if(this.stopped||this.pending||this.child)return Promise.reject(unavailable());
  if(this.workspaceRoots===null||this.workspaceRoots===undefined)return Promise.resolve(false);
  const pending=this.check();this.pending=pending;
  return pending.finally(()=>{if(this.pending===pending)this.pending=null;});
 }
 async check(){
  if(!isAbsolute(this.directory??''))throw unavailable();
  const frame=requestFrame(this.workspaceRoots);
  let profile;
  try{profile=await this.verifyProfile();}
  catch(error){if(error.message==='computer_driver_unavailable')return false;throw error;}
  if(this.stopped)throw Error('computer_stopped');
  if(profile?.browser_storage!==1)return false;
  const env={...process.env};delete env.NODE_OPTIONS;delete env.NODE_PATH;delete env.ELECTRON_RUN_AS_NODE;
  const executable=join(this.root,'native',this.platform==='win32'?'offgrid-computer.exe':'offgrid-computer');
  let child;
  try{child=this.spawnWorker(executable,['--browser-storage','--state',this.directory],{windowsHide:true,shell:false,stdio:['pipe','pipe','pipe'],env});}
  catch{throw unavailable();}
  this.child=child;
  return new Promise((resolve,reject)=>{
   let bytes=Buffer.alloc(0),failure,settled=false,timer,killTimer;
   const finish=(error,value)=>{if(settled)return;settled=true;clearTimeout(timer);clearTimeout(killTimer);error?reject(error):resolve(value);};
   const fail=code=>{
    failure??=Error(code);if(killTimer)return;
    killTimer=setTimeout(()=>finish(Error('computer_stop_unconfirmed')),this.stopMs);
    try{child.kill();}catch{/* Keep ownership until close or explicit Stop retry. */}
   };
   this.cancel=()=>fail('computer_stopped');
   child.stderr?.resume();
   child.stdin.on('error',()=>fail('computer_storage_unavailable'));
   child.stdout.on('error',()=>fail('computer_storage_unavailable'));
   child.stdout.on('data',data=>{
    if(failure)return;
    if(!Buffer.isBuffer(data)||bytes.length+data.length>516){fail('computer_storage_unavailable');return;}
    bytes=Buffer.concat([bytes,data]);
    if(bytes.length>=4&&(!bytes.readUInt32BE()||bytes.readUInt32BE()>512))fail('computer_storage_unavailable');
   });
   child.once('error',()=>fail('computer_storage_unavailable'));
   child.once('close',code=>{
    if(this.child===child){this.child=null;this.cancel=null;}
    if(failure||code!==0||this.stopped){finish(failure??unavailable());return;}
    try{finish(null,reply(bytes));}catch{finish(unavailable());}
   });
   timer=setTimeout(()=>fail('computer_storage_unavailable'),this.timeoutMs);
   try{child.stdin.end(frame);}catch{fail('computer_storage_unavailable');}
  });
 }
 async stop(){
  this.stopped=true;this.abort.abort();const child=this.child;if(!child)return;
  await new Promise((resolve,reject)=>{
   let settled=false;
   const finish=error=>{if(settled)return;settled=true;clearTimeout(timer);child.off('close',closed);error?reject(error):resolve();};
   const closed=()=>finish();child.once('close',closed);
   const timer=setTimeout(()=>finish(Error('computer_stop_unconfirmed')),this.stopMs);
   this.cancel?.();try{child.kill();}catch{/* A response or kill request is not confirmed process exit. */}
  });
 }
}
