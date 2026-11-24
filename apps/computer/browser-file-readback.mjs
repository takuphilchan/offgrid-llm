import {open,constants} from 'node:fs/promises';
import {createHash,randomUUID} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';
import {selectors} from 'playwright';
import {fileReadbackEngine} from './browser-file-readback-engine.mjs';
import {browserFileLimit} from './browser-file-snapshot.mjs';

const engine='offgrid_file_readback_v1',chunkLimit=1024*1024;
let registered;
export function registerFileReadback(){
  return registered??=selectors.register(engine,fileReadbackEngine,{contentScript:true});
}

// The digest comes from complete byte comparison: each actual native File chunk
// must match the corresponding owned snapshot chunk, and the complete snapshot
// is hashed again against the selected grant. This proves file selection only;
// input/change handlers and remote receivers are separate effects.
export async function verifySelectedBrowserFile(handle,snapshot,{signal,timeoutMs=30000}={}){
  const metadata=snapshot?.metadata;
  if(snapshot?.state!=='ready'||!metadata||!Number.isSafeInteger(metadata.size)||metadata.size<0||metadata.size>browserFileLimit||! /^[a-f0-9]{64}$/.test(metadata.sha256??'')||!Number.isSafeInteger(timeoutMs)||timeoutMs<1||timeoutMs>120000)throw Error('File readback snapshot is unavailable.');
  const token=randomUUID().replaceAll('-',''),deadline=Date.now()+timeoutMs;
  const check=()=>{signal?.throwIfAborted();if(Date.now()>deadline)throw Error('File readback timed out.');};
  const request=async(op,fields={})=>{
    const result=await handle.$(`${engine}=${JSON.stringify({token,op,...fields})}`);
    if(result)await result.dispose();return !!result;
  };
  let input,started=false;
  const buffer=Buffer.allocUnsafe(chunkLimit),hash=createHash('sha256');
  try{
    check();input=await open(snapshot.path,constants.O_RDONLY|(constants.O_NOFOLLOW??0)|(constants.O_NONBLOCK??0));
    check();const initial=await input.stat();
    if(!initial.isFile()||initial.size!==metadata.size)throw Error('File readback snapshot changed.');
    started=true;await request('start',{name:metadata.name,size:metadata.size});
    let offset=0;
    while(offset<metadata.size){
      check();const length=Math.min(chunkLimit,metadata.size-offset);let read=0;
      while(read<length){const {bytesRead}=await input.read(buffer,read,length-read,offset+read);check();if(!bytesRead)throw Error('File readback snapshot changed.');read+=bytesRead;}
      hash.update(buffer.subarray(0,length));
      await request('chunk',{offset,length,sha256:createHash('sha256').update(buffer.subarray(0,length)).digest('hex')});
      buffer.fill(0);
      while(!await request('poll')){check();await delay(5,undefined,{signal});}
      offset+=length;
    }
    check();const final=await input.stat();
    if(final.dev!==initial.dev||final.ino!==initial.ino||final.size!==initial.size||final.mtimeMs!==initial.mtimeMs||hash.digest('hex')!==metadata.sha256)throw Error('File readback snapshot changed.');
    await request('finish');started=false;
    return {name:metadata.name,size:metadata.size,sha256:metadata.sha256};
  }finally{
    buffer.fill(0);
    try{if(started)await request('release');}finally{await input?.close();}
  }
}
