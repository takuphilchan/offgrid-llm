// Inspect primary-image metadata before pixel allocation. Do not let an animated
// preview or conflicting orientation stand in for the still image sent to a model.
export class ImageMetadataError extends Error {}
const invalid=():never=>{throw new ImageMetadataError();};

// Saved originals from an older service can also need the newer projection.
// Invalid legacy data must not be sent on the assumption that orientation is 1.
export function needsImageOrientation(url:string):boolean {
 try{
  if(url.length>Math.ceil(2*1024*1024/3)*4+32)return true;
  const comma=url.indexOf(','),type=url.slice(5,comma).split(';')[0];
  return imageOrientation(Uint8Array.from(atob(url.slice(comma+1)),c=>c.charCodeAt(0)),type)!==1;
 }catch{return true;}
}

function exif(data:Uint8Array):number {
 if(data.length<8)invalid();
 const order=String.fromCharCode(data[0],data[1]);
 if(order!=='II'&&order!=='MM')invalid();
 const little=order==='II',view=new DataView(data.buffer,data.byteOffset,data.byteLength);
 if(view.getUint16(2,little)!==42)invalid();
 const offset=view.getUint32(4,little);
 if(offset<8||offset+2>data.length)invalid();
 const count=view.getUint16(offset,little);
 if(count>512||offset+2+count*12+4>data.length)invalid();
 let orientation=1,seen=false;
 for(let i=0;i<count;i++){
  const at=offset+2+i*12;if(view.getUint16(at,little)!==0x112)continue;
  if(seen||view.getUint16(at+2,little)!==3||view.getUint32(at+4,little)!==1)invalid();
  seen=true;orientation=view.getUint16(at+8,little);
  if(orientation<1||orientation>8)invalid();
 }
 return orientation;
}

export function imageOrientation(data:Uint8Array,type:string):number {
 const view=new DataView(data.buffer,data.byteOffset,data.byteLength);
 let orientation=1,seen=false;
 const read=(value:Uint8Array)=>{if(seen)invalid();seen=true;orientation=exif(value);};
 if(type==='image/png'){
  for(let at=8;at+12<=data.length;){
   const length=view.getUint32(at),end=at+length+12;
   if(end>data.length)invalid();
   const kind=String.fromCharCode(...data.subarray(at+4,at+8));
   if(['acTL','fcTL','fdAT'].includes(kind))invalid();
   if(kind==='eXIf')read(data.subarray(at+8,end-4));
   if(kind==='IEND'){if(length!==0||end!==data.length)invalid();return orientation;}
   at=end;
  }
  return invalid();
 }
 if(type!=='image/jpeg'||data[0]!==255||data[1]!==216)invalid();
 for(let at=2;at<data.length;){
  if(data[at]!==255)invalid();while(at<data.length&&data[at]===255)at++;
  if(at>=data.length)invalid();const marker=data[at++];
  if(marker===0xda||marker===0xd9)return orientation;
  if(marker===1||(marker>=0xd0&&marker<=0xd7))continue;
  if(at+2>data.length)invalid();const length=view.getUint16(at);
  if(length<2||at+length>data.length)invalid();
  const prefix=String.fromCharCode(...data.subarray(at+2,Math.min(at+8,at+length)));
  if(marker===0xe2&&prefix.startsWith('MPF\0'))invalid();
  if(marker===0xe1&&prefix==='Exif\0\0')read(data.subarray(at+8,at+length));
  at+=length;
 }
 return invalid();
}
