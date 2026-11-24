import {endianness} from 'node:os';
import {assertUniqueJSONKeys} from './native-json-keys.mjs';

export const nativeMessageLimit=64*1024;
const littleEndian=endianness()==='LE';
const decoder=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true});
export class NativeMessageError extends Error {
  constructor(code){super(code);this.name='NativeMessageError';this.code=code;}
}
const invalid=()=>{throw new NativeMessageError('native_message_invalid');};

// Framing validates bounded JSON data, not sender identity or action authority.
function validateJSON(value){
  if(!value||typeof value!=='object'||Array.isArray(value))invalid();
  const queue=[[value,0]],seen=new WeakSet();let count=0,bytes=0;
  const reserve=size=>{bytes+=size;if(bytes>nativeMessageLimit)throw new NativeMessageError('native_message_oversize');};
  while(queue.length){
    const [item,depth]=queue.pop();
    if(++count>4096||depth>32)invalid();
    if(item===null||typeof item==='boolean'){reserve(item===null?4:item?4:5);continue;}
    if(typeof item==='string'){
      if(!item.isWellFormed()||Buffer.byteLength(item)>nativeMessageLimit)invalid();
      reserve(Buffer.byteLength(JSON.stringify(item)));continue;
    }
    if(typeof item==='number'){if(!Number.isFinite(item))invalid();reserve(String(item).length);continue;}
    if(!item||typeof item!=='object'||seen.has(item))invalid();
    seen.add(item);
    if(!Array.isArray(item)&&Object.getPrototypeOf(item)!==Object.prototype)invalid();
    const keys=Object.keys(item);
    if(keys.length>4096||Object.getOwnPropertySymbols(item).length)invalid();
    // JSON.stringify consults even a non-enumerable toJSON getter. Accept only
    // data properties from a plain JSON tree, plus arrays' intrinsic length.
    if(Object.getOwnPropertyNames(item).length!==keys.length+(Array.isArray(item)?1:0))invalid();
    if(Array.isArray(item)&&(keys.length!==item.length||keys.some((key,index)=>key!==String(index))))invalid();
    reserve(2+Math.max(0,keys.length-1));
    for(const key of keys){
      if(!key.isWellFormed()||Buffer.byteLength(key)>nativeMessageLimit)invalid();
      if(!Array.isArray(item))reserve(Buffer.byteLength(JSON.stringify(key))+1);
      const descriptor=Object.getOwnPropertyDescriptor(item,key);
      if(!descriptor||!Object.hasOwn(descriptor,'value'))invalid();
      queue.push([descriptor.value,depth+1]);
    }
  }
}

export function encodeNativeMessage(message){
  try {
    validateJSON(message);
    const data=Buffer.from(JSON.stringify(message),'utf8');
    if(!data.length||data.length>nativeMessageLimit)throw new NativeMessageError('native_message_oversize');
    const frame=Buffer.allocUnsafe(4+data.length);
    if(littleEndian)frame.writeUInt32LE(data.length);else frame.writeUInt32BE(data.length);
    data.copy(frame,4);data.fill(0);return frame;
  }catch(error){if(error instanceof NativeMessageError)throw error;invalid();}
}

export async function* readNativeMessages(readable,{signal}={}){
  const header=Buffer.alloc(4);let headerBytes=0,payload,payloadBytes=0;
  const cancel=()=>readable.destroy(new NativeMessageError('native_message_cancelled'));
  signal?.addEventListener('abort',cancel,{once:true});
  try {
    if(signal?.aborted)throw new NativeMessageError('native_message_cancelled');
    for await(const chunk of readable){
      if(signal?.aborted)throw new NativeMessageError('native_message_cancelled');
      if(!Buffer.isBuffer(chunk))invalid();
      let offset=0;
      while(offset<chunk.length){
        if(!payload){
          const bytes=Math.min(4-headerBytes,chunk.length-offset);
          chunk.copy(header,headerBytes,offset,offset+bytes);headerBytes+=bytes;offset+=bytes;
          if(headerBytes<4)continue;
          const length=littleEndian?header.readUInt32LE():header.readUInt32BE();
          if(!length)invalid();
          if(length>nativeMessageLimit)throw new NativeMessageError('native_message_oversize');
          payload=Buffer.allocUnsafe(length);payloadBytes=0;
        }
        const bytes=Math.min(payload.length-payloadBytes,chunk.length-offset);
        chunk.copy(payload,payloadBytes,offset,offset+bytes);payloadBytes+=bytes;offset+=bytes;
        if(payloadBytes===payload.length){
          let message;
          try {const text=decoder.decode(payload);assertUniqueJSONKeys(text);message=JSON.parse(text);validateJSON(message);}
          catch(error){if(error instanceof NativeMessageError)throw error;invalid();}
          payload.fill(0);payload=undefined;headerBytes=0;
          yield message;
          if(signal?.aborted)throw new NativeMessageError('native_message_cancelled');
        }
      }
    }
    if(headerBytes||payload)throw new NativeMessageError('native_message_truncated');
  }catch(error){
    if(error instanceof NativeMessageError)throw error;
    throw new NativeMessageError(signal?.aborted?'native_message_cancelled':'native_message_io');
  }finally{
    signal?.removeEventListener('abort',cancel);header.fill(0);payload?.fill(0);
  }
}

export async function writeNativeMessage(writable,message,{signal}={}){
  if(signal?.aborted)throw new NativeMessageError('native_message_cancelled');
  if(writable.destroyed||writable.errored)throw new NativeMessageError('native_message_io');
  const frame=encodeNativeMessage(message);
  try {
    await new Promise((resolve,reject)=>{
      let settled=false;
      const finish=error=>{
        if(settled)return;settled=true;
        writable.off('error',failed);writable.off('close',failed);signal?.removeEventListener('abort',cancel);
        if(error)reject(error);else resolve();
      };
      const failed=()=>finish(new NativeMessageError('native_message_io'));
      const cancel=()=>{
        // Keep an error listener until destroy has emitted; no raw error leaves.
        writable.once('error',()=>{});writable.destroy();
        finish(new NativeMessageError('native_message_cancelled'));
      };
      writable.once('error',failed);writable.once('close',failed);signal?.addEventListener('abort',cancel,{once:true});
      // Node emits 'error' after the write callback receives an error. Retain
      // the listener until that event (or close), so raw sink errors cannot
      // escape as uncaught exceptions after we have already rejected safely.
      try {writable.write(frame,error=>{if(!error)finish();});}catch{failed();}
    });
  }finally{frame.fill(0);}
}
