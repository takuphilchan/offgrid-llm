// Read bounded PNG/JPEG headers before asking the browser to allocate pixels.
// This is a preflight only: createImageBitmap still validates the complete file.
export function imageDimensions(bytes: Uint8Array, type: string): {width:number;height:number} {
 const view=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength);
 if(type==='image/png'){
  if(bytes.length<33||![137,80,78,71,13,10,26,10].every((v,i)=>bytes[i]===v)||view.getUint32(8)!==13||String.fromCharCode(...bytes.slice(12,16))!=='IHDR')throw Error('Invalid PNG header');
  return bounded(view.getUint32(16),view.getUint32(20));
 }
 if(type!=='image/jpeg'||bytes.length<4||bytes[0]!==255||bytes[1]!==216)throw Error('Invalid JPEG header');
 let at=2;
 while(at<bytes.length){
  if(bytes[at++]!==255)throw Error('Invalid JPEG marker');
  while(at<bytes.length&&bytes[at]===255)at++;
  if(at>=bytes.length)break;
  const marker=bytes[at++];
  if(marker===0xda||marker===0xd9)break; // No frame dimensions before pixels/end.
  if(marker===0x01||(marker>=0xd0&&marker<=0xd7))continue;
  if(at+2>bytes.length)break;
  const length=view.getUint16(at);
  if(length<2||at+length>bytes.length)throw Error('Truncated JPEG segment');
  if(marker>=0xc0&&marker<=0xcf&&![0xc4,0xc8,0xcc].includes(marker)){
   if(length<8||length!==8+3*bytes[at+7])throw Error('Invalid JPEG frame');
   return bounded(view.getUint16(at+5),view.getUint16(at+3));
  }
  at+=length;
 }
 throw Error('Missing JPEG frame');
}

function bounded(width:number,height:number){
 if(width<1||height<1||width>8192||height>8192||width*height>32*1024*1024)throw Error('Image pixel limit');
 return {width,height};
}
