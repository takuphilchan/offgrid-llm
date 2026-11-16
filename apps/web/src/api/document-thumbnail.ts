import {clientMessage} from '../i18n/client-errors';

export async function readDocumentThumbnail(response:Response,sha:string):Promise<Blob>{
 const invalid=()=>Error(clientMessage('response')),limit=256*1024;
 if(!/^[a-f0-9]{64}$/.test(sha)||response.headers.get('X-OffGrid-Source-SHA256')!==sha||response.headers.get('X-OffGrid-Preview')!=='thumbnail-v1'||response.headers.get('Content-Type')?.split(';')[0]!=='image/png'||!response.body){await response.body?.cancel().catch(()=>{});throw invalid();}
 const reader=response.body.getReader(),parts:Uint8Array[]=[];let size=0,finished=false;
 try{
  for(;;){const {value,done}=await reader.read();if(done){finished=true;break;}size+=value.byteLength;if(size>limit)throw invalid();parts.push(value);}
 }finally{if(!finished)await reader.cancel().catch(()=>{});reader.releaseLock();}
 const bytes=new Uint8Array(size);let offset=0;for(const part of parts){bytes.set(part,offset);offset+=part.length;}
 if(size<33||![137,80,78,71,13,10,26,10].every((value,index)=>bytes[index]===value)||String.fromCharCode(...bytes.slice(12,16))!=='IHDR')throw invalid();
 const data=new DataView(bytes.buffer),width=data.getUint32(16),height=data.getUint32(20);
 if(width<1||height<1||width>192||height>256)throw invalid();
 return new Blob([bytes],{type:'image/png'});
}
