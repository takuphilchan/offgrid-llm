import {imageOrientation} from './image-metadata';

/** Browser-only projection: Chromium ignores PNG Exif after IDAT. Move the
 * unchanged metadata chunk before pixels; do not re-encode pixels or save this
 * projection into the draft/history. The service reads the original metadata. */
export function imageDisplayURL(url:string):string|undefined {
 if(!/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/=]+$/.test(url)||url.length>Math.ceil(2*1024*1024/3)*4+32)return undefined;
 if(!url.startsWith('data:image/png;'))return url;
 try{
  const bytes=Uint8Array.from(atob(url.slice(url.indexOf(',')+1)),c=>c.charCodeAt(0));
  if(imageOrientation(bytes,'image/png')===1)return url;
  const view=new DataView(bytes.buffer);let pixels=-1,start=-1,end=-1;
  for(let at=8;at+12<=bytes.length;){
   const next=at+view.getUint32(at)+12,kind=String.fromCharCode(...bytes.subarray(at+4,at+8));
   if(next>bytes.length)return undefined;
   if(kind==='IDAT'&&pixels<0)pixels=at;
   if(kind==='eXIf'){start=at;end=next;}
   at=next;
  }
  if(pixels<0||start<pixels)return url;
  const display=new Uint8Array(bytes.length),length=end-start;
  display.set(bytes.subarray(0,pixels));display.set(bytes.subarray(start,end),pixels);
  display.set(bytes.subarray(pixels,start),pixels+length);display.set(bytes.subarray(end),end);
  const parts:string[]=[];
  for(let at=0;at<display.length;at+=8192)parts.push(String.fromCharCode(...display.subarray(at,at+8192)));
  return 'data:image/png;base64,'+btoa(parts.join(''));
 }catch{return undefined;}
}
