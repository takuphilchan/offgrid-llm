import {test,expect} from '@playwright/test';
import {sourceRegions,uprightRegion} from '../src/features/chat/document-regions';
import {validatePreview} from '../src/api/document-structure';
import type {DocumentPreview,DocumentStructurePage} from '../src/api/client';
import {rememberReader} from '../src/lib/reader-memory';

test('encoded source rectangles follow all eight EXIF orientations',()=>{
 const expected=[[.1,.2,.2,.3],[.7,.2,.2,.3],[.7,.5,.2,.3],[.1,.5,.2,.3],[.2,.1,.3,.2],[.5,.1,.3,.2],[.5,.7,.3,.2],[.2,.7,.3,.2]];
 for(let orientation=1;orientation<=8;orientation++){
  const region=uprightRegion({x:10,y:40,width:20,height:60},{width:100,height:200,orientation})!;
  [region.x,region.y,region.width,region.height].forEach((value,i)=>expect(value).toBeCloseTo(expected[orientation-1][i]));
 }
 expect(uprightRegion({x:90,y:0,width:20,height:10},{width:100,height:200,orientation:1})).toBeUndefined();
});

const text='税额 42\nException 17',length=new TextEncoder().encode(text).length;
const preview={sha256:'a'.repeat(64),format:'tiff',units:[{kind:'page',index:2,text,state:'ready',source_sha256:'a'.repeat(64),image_geometry:{width:100,height:200,orientation:6}}]} as DocumentPreview;
const structure={original_sha256:preview.sha256,units:[{id:'page-2',kind:'page',index:2,span:{start_byte:100,end_byte:100+length}}],blocks:[{id:'line',unit_id:'page-2',method:'ocr',source_part:'original_encoded_pixels',span:{start_byte:100,end_byte:109},text:'税额 42',bounds:{x:10,y:40,width:20,height:60,unit:'pixels'},truncated:false,metadata_omitted:false}]} as DocumentStructurePage;
test('source regions require exact byte text, source identity and retained OCR provenance',()=>{
 expect(sourceRegions(preview,1,structure,'42')).toHaveLength(1);
 expect(sourceRegions(preview,1,{...structure,original_sha256:'b'.repeat(64)},'42')).toEqual([]);
 for(const mutation of [{method:'review'},{truncated:true},{source_part:'unknown'},{text:'税额 43'},{span:undefined},{span:{start_byte:101,end_byte:109}}])expect(sourceRegions(preview,1,{...structure,blocks:[{...structure.blocks[0],...mutation}]} as DocumentStructurePage,'42')).toEqual([]);
 expect(sourceRegions(preview,1,{...structure,units:[{...structure.units[0],span:undefined}]} as unknown as DocumentStructurePage,'42')).toEqual([]);
 expect(sourceRegions({...preview,units:[{...preview.units[0],text:text+' 42'}]},1,structure,'42')).toEqual([]);
 expect(sourceRegions(preview,1,structure,'not present')).toEqual([]);
 for(const geometry of [{width:0,height:1,orientation:1},{width:8193,height:1,orientation:1},{width:100,height:100,orientation:9}])expect(()=>validatePreview({...preview,units:[{...preview.units[0],image_geometry:geometry}]})).toThrow();
});

test('reader positions are bounded and scoped to their provider, with no content cache',()=>{
 const a=new Map(),b=new Map(),position={page:3,zoom:150,query:'42',textView:true,compare:false,top:80,left:0};
 for(let i=0;i<45;i++)rememberReader(a,String(i),position);
 expect(a.size).toBe(40);expect(a.has('0')).toBe(false);expect(b.size).toBe(0);
 position.page=9;expect(a.get('44').page).toBe(3);
});
