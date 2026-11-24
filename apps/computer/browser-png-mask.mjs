import {PNG} from 'pngjs';

// Only an owned Chromium screenshot enters this function. Validate its declared
// dimensions before allocating decoded pixels; raw pixels never leave the
// companion, enter a log/journal, or get written to disk.
export function maskBrowserPNG(content,viewport,rectangles){
  if(content.length<33||!content.subarray(0,8).equals(Buffer.from([137,80,78,71,13,10,26,10]))||content.toString('ascii',12,16)!=='IHDR'||content.readUInt32BE(16)!==viewport.width||content.readUInt32BE(20)!==viewport.height)throw Error('Capture dimensions changed before masking.');
  const png=PNG.sync.read(content,{checkCRC:true});
  try{
    for(const rectangle of rectangles){
      const left=Math.max(0,Math.floor(rectangle.x)-2),top=Math.max(0,Math.floor(rectangle.y)-2);
      const right=Math.min(png.width,Math.ceil(rectangle.x+rectangle.width)+2),bottom=Math.min(png.height,Math.ceil(rectangle.y+rectangle.height)+2);
      for(let y=top;y<bottom;y++)for(let x=left;x<right;x++){
        const offset=(y*png.width+x)*4;png.data[offset]=0;png.data[offset+1]=0;png.data[offset+2]=0;png.data[offset+3]=255;
      }
    }
    return PNG.sync.write(png,{colorType:6,bitDepth:8,deflateLevel:6});
  }finally{png.data.fill(0);}
}

// Include overflowing descendant boxes and direct text, including text in a
// display:contents node. A null border box does not mean there is no paint.
function protectedPaint(element){
  const style=getComputedStyle(element);
  if(style.display==='none'||style.visibility==='hidden')return [];
  for(let node=element;node;node=node.parentElement??node.getRootNode().host){
    const current=getComputedStyle(node);
    if(current.filter!=='none'||current.textShadow!=='none'||current.boxShadow!=='none'||current.webkitBoxReflect&&current.webkitBoxReflect!=='none')throw Error('Protected paint effects cannot be bounded for capture.');
  }
  for(const pseudo of ['::before','::after']){
    const content=getComputedStyle(element,pseudo).content;
    if(content&&content!=='none'&&content!=='normal'&&content!=='""')throw Error('Protected generated paint cannot be bounded for capture.');
  }
  const rectangles=Array.from(element.getClientRects());
  for(const node of element.childNodes)if(node.nodeType===Node.TEXT_NODE&&node.textContent.trim()){
    const range=document.createRange();range.selectNode(node);rectangles.push(...range.getClientRects());
    if(rectangles.length>2000)throw Error('Protected capture exceeds rectangle bounds.');
  }
  return rectangles.map(({x,y,width,height})=>({x,y,width,height}));
}

async function frameOffset(frame){
  for(let current=frame;current.parentFrame();current=current.parentFrame()){
    const element=await current.frameElement();
    try{
      const safe=await element.evaluate(element=>{
        for(let node=element;node;node=node.parentElement??node.getRootNode().host){
          const style=getComputedStyle(node);
          if(style.transform!=='none'||style.perspective!=='none'||style.zoom!=='1'||style.translate!=='none'||style.rotate!=='none'||style.scale!=='none'||style.offsetPath!=='none'||style.filter!=='none'||style.webkitBoxReflect&&style.webkitBoxReflect!=='none')return false;
        }
        return true;
      });
      if(!safe)throw Error('Transformed frame paint cannot be bounded for capture.');
    }finally{await element.dispose();}
  }
  const root=frame.locator(':root'),global=await root.boundingBox();
  const local=await root.evaluate(element=>{const {x,y}=element.getBoundingClientRect();return {x,y};});
  if(!global)throw Error('Protected frame geometry is unavailable.');
  return {x:global.x-local.x,y:global.y-local.y};
}

export async function maskRectangles(masks){
  const rectangles=[],offsets=new Map();let count=0;
  for(const {locator,frame} of masks){
    const length=await locator.count();count+=length;
    if(count>2000)throw Error('Protected capture exceeds rectangle bounds.');
    if(!length)continue;
    if(!offsets.has(frame))offsets.set(frame,await frameOffset(frame));
    const offset=offsets.get(frame);
    for(let i=0;i<length;i++){
      for(const rectangle of await locator.nth(i).evaluate(protectedPaint)){
        if(Object.values(rectangle).some(value=>!Number.isFinite(value))||rectangle.width<0||rectangle.height<0)throw Error('Protected capture geometry is unavailable.');
        rectangles.push({...rectangle,x:rectangle.x+offset.x,y:rectangle.y+offset.y});
        if(rectangles.length>2000)throw Error('Protected capture exceeds rectangle bounds.');
      }
    }
  }
  return rectangles;
}
