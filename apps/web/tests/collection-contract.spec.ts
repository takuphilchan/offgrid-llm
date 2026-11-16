import {expect,test} from '@playwright/test';
import {collectionAPI,validateCollection} from '../src/api/collections';
import type {components} from '../src/api/schema.generated';
type Policy=components['schemas']['KnowledgeCollection'];
const base:Policy={id:'collection-fixture',version:1,revision:1,name:'Department',owner:'owner',readers:[],editors:[]};

test('collection updates reject stale or different audience receipts without replay',async()=>{
 for(const returned of [{...base,revision:1},{...base,revision:2,readers:['unexpected']},{...base,revision:2,id:'other'}]){
  let calls=0;
  const api=collectionAPI(async<T>(path:string,init?:RequestInit)=>{calls++;expect(path).toBe('/v1/collections?id=collection-fixture');expect(init?.method).toBe('PUT');return returned as T;});
  await expect(api.update(base.id,{expected_revision:1,name:base.name,readers:[],editors:[]})).rejects.toThrow();
  expect(calls).toBe(1);
 }
});

test('collection creation and listing cannot accept unrelated projections',async()=>{
 expect(()=>validateCollection({...base,readers:undefined} as unknown as Policy)).toThrow();
 expect(()=>validateCollection({...base,name:{text:'invalid'}} as unknown as Policy)).toThrow();
 expect(()=>validateCollection({...base,id:'workspace',owner:'unexpected'})).toThrow();
 const api=collectionAPI(async<T>()=>({...base,readers:['unexpected']} as T));
 await expect(api.create(base.name)).rejects.toThrow();
 const list=collectionAPI(async<T>()=>({collections:[base,base]} as T));
 await expect(list.list()).rejects.toThrow();
});

test('private import uses the distinct endpoint and never falls back on older services',async()=>{
 let calls=0;
 const api=collectionAPI(async<T>(path:string)=>{calls++;expect(path.startsWith('/v1/collections/imports?')).toBe(true);throw Error('404 unsupported');return {} as T;});
 await expect(api.acceptImport(base.id,1,new File(['private'],'source.txt'),'request')).rejects.toThrow('404');
 expect(calls).toBe(1);
});

test('collection audit rejects repeated cursors instead of inventing progress',async()=>{
 const event={sequence:2,actor:'owner',action:'collection_created',subject:base.id,revision:1,created_at:'2026-11-13T00:00:00Z'};
 const api=collectionAPI(async<T>()=>({events:[event,event]} as T));
 await expect(api.audit(base.id,1)).rejects.toThrow();
});

test('move and directory receipts reject mismatched identities without replay',async()=>{
 const body={document_id:'document',expected_revision:8,expected_source_revision:3,destination_id:base.id,expected_destination_revision:1};
 for(const receipt of [{document_id:'wrong',revision:9,collection:base},{document_id:'document',revision:10,collection:base},{document_id:'document',revision:9,collection:{...base,revision:2}}]){
  let calls=0;const api=collectionAPI(async<T>()=>{calls++;return receipt as T;});
  await expect(api.move(body)).rejects.toThrow();expect(calls).toBe(1);
 }
 const people=collectionAPI(async<T>()=>({users:[{id:'a',username:'one'},{id:'a',username:'two'}]} as T));await expect(people.people()).rejects.toThrow();
});

test('attachment publication rejects wrong source, destination and audience receipts without replay',async()=>{
 const source={id:'file-fixture',sha256:'b'.repeat(64)};
 const receipt={id:'import-fixture',state:'accepted',collection_id:base.id,collection_revision:1,sha256:source.sha256};
 for(const invalid of [{...receipt,collection_id:'workspace'},{...receipt,collection_revision:2},{...receipt,sha256:'other'}]){
  let calls=0;
  const api=collectionAPI(async<T>(path:string,init?:RequestInit)=>{
   calls++;expect(path).toBe('/v1/collections/imports?collection_id=collection-fixture&collection_revision=1');
   expect(JSON.parse(init?.body as string)).toEqual({attachment:{session:'private',...source}});return invalid as T;
  });
  await expect(api.acceptAttachment(base.id,1,'private',source)).rejects.toThrow();expect(calls).toBe(1);
 }
});

test('versioned attachment publication requires service support and an exact extraction receipt',async()=>{
 const source={id:'file-source',sha256:'b'.repeat(64),revision:'c'.repeat(64),extraction_revision:'extraction-selected'};
 for(const mode of ['legacy','wrong','missing','ok']){
  let writes=0;
  const api=collectionAPI(async<T>(path:string,init?:RequestInit)=>{
   if(path==='/api/v2/system')return {capabilities:mode==='legacy'?[]:['session-document-corrections-v1']} as T;
   writes++;expect(JSON.parse(init?.body as string).attachment.revision).toBe(source.revision);
   return {id:'import',collection_id:base.id,collection_revision:1,sha256:source.sha256,extraction_revision:mode==='ok'?source.extraction_revision:mode==='wrong'?'other':undefined} as T;
  });
  if(mode==='ok')expect((await api.acceptAttachment(base.id,1,'chat',source)).extraction_revision).toBe(source.extraction_revision);
  else await expect(api.acceptAttachment(base.id,1,'chat',source)).rejects.toThrow();
  expect(writes).toBe(mode==='legacy'?0:1);
 }
});
