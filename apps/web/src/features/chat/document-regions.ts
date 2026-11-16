import type {DocumentPreview,DocumentStructurePage} from '../../api/client';
type Geometry=NonNullable<DocumentPreview['units'][number]['image_geometry']>;
export type SourceRegion={id:string;x:number;y:number;width:number;height:number};

/** Half-open encoded-pixel rectangles mapped to the same upright preview as OCR. */
export function uprightRegion(box:{x:number;y:number;width:number;height:number},g:Geometry):Omit<SourceRegion,'id'>|undefined{
 const {width:w,height:h,orientation:o}=g;
 const {x,y,width,height}=box,r=x+width,b=y+height;
 if(![w,h,o,x,y,width,height,r,b].every(Number.isFinite)||!Number.isInteger(o)||o<1||o>8||w<1||h<1||x<0||y<0||width<=0||height<=0||r>w||b>h)return;
 let bounds=[x,y,r,b];
 if(o===2)bounds=[w-r,y,w-x,b];
 if(o===3)bounds=[w-r,h-b,w-x,h-y];
 if(o===4)bounds=[x,h-b,r,h-y];
 if(o===5)bounds=[y,x,b,r];
 if(o===6)bounds=[h-b,x,h-y,r];
 if(o===7)bounds=[h-b,w-r,h-y,w-x];
 if(o===8)bounds=[y,w-r,b,w-x];
 const ow=o>=5?h:w,oh=o>=5?w:h;
 return {x:bounds[0]/ow,y:bounds[1]/oh,width:(bounds[2]-bounds[0])/ow,height:(bounds[3]-bounds[1])/oh};
}

// Exact UTF-8 agreement is required. Duplicate text, derived corrections,
// unknown frames and truncated blocks do not establish a visual location.
export function sourceRegions(preview:DocumentPreview,page:number,structure:DocumentStructurePage,highlight:string):SourceRegion[]{
 const unit=preview.units[page-1],g=unit?.image_geometry;
 if(!g||!highlight||structure.original_sha256!==preview.sha256||unit.source_sha256!==preview.sha256)return [];
 const match=structure.units.find(value=>value.kind===unit.kind&&value.index===unit.index);
 if(!match?.span||![match.span.start_byte,match.span.end_byte].every(Number.isSafeInteger)||match.span.start_byte<0||match.span.end_byte<match.span.start_byte)return [];
 const encoder=new TextEncoder(),bytes=encoder.encode(unit.text),start=unit.text.indexOf(highlight);
 if(start<0||start!==unit.text.lastIndexOf(highlight)||match.span.end_byte-match.span.start_byte!==bytes.length)return [];
 const a=encoder.encode(unit.text.slice(0,start)).length,b=a+encoder.encode(highlight).length,decoder=new TextDecoder('utf-8',{fatal:true});
 const regions:SourceRegion[]=[];
 for(const block of structure.blocks){
  if(!block.span||block.unit_id!==match.id||block.method!=='ocr'||block.source_part!=='original_encoded_pixels'||block.bounds?.unit!=='pixels'||block.truncated||block.metadata_omitted)continue;
  const x=block.span.start_byte-match.span.start_byte,y=block.span.end_byte-match.span.start_byte;
  if(!Number.isSafeInteger(x)||!Number.isSafeInteger(y)||x<0||y<=x||y>bytes.length||y<=a||x>=b)continue;
  try{if(decoder.decode(bytes.subarray(x,y))!==block.text)continue;}catch{continue;}
  const bounds=uprightRegion(block.bounds,g);if(bounds)regions.push({id:block.id,...bounds});
 }
 return regions;
}
