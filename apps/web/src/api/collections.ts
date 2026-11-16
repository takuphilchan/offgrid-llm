import {collectionErrorMessage} from '../i18n/collection-errors';
import type {components} from './schema.generated';

type Schema=components['schemas'];
type Policy=Schema['KnowledgeCollection'];
export type CollectionPolicy=Policy;
type Request=<T>(path:string,init?:RequestInit)=>Promise<T>;
const policyPath=(id:string)=>`/v1/collections?${new URLSearchParams({id})}`;

export function validateCollection(p:Policy,id=''):Policy {
 if(!p||typeof p.id!=='string'||!p.id||(id&&p.id!==id)||p.version!==1||!Number.isSafeInteger(p.revision)||p.revision<1||typeof p.name!=='string'||!p.name
  ||(p.id!=='workspace'&&(typeof p.owner!=='string'||!p.owner))||!Array.isArray(p.readers)||!Array.isArray(p.editors)
  ||(p.id==='workspace'&&(!!p.owner||p.revision!==1||p.readers.length>0||p.editors.length>0))
  ||[p.readers,p.editors].some(items=>items.length>128||items.some(item=>typeof item!=='string'||!item)||new Set(items).size!==items.length))throw Error(collectionErrorMessage());
 return p;
}
const sameMembers=(a:string[],b:string[])=>JSON.stringify([...a].sort())===JSON.stringify([...b].sort());

// Every operation uses the authenticated transport once. No conflict or unknown
// mutation outcome is retried, and responses are checked before confirmation.
export function collectionAPI(request:Request) {
 return {
  people:async(signal?:AbortSignal)=>{
   const result=await request<{users:Schema['PublicUser'][]}>('/v1/users',{signal});
   if(!Array.isArray(result?.users)||result.users.length>10000||result.users.some(user=>!user||typeof user.id!=='string'||!user.id||typeof user.username!=='string'||!user.username)||new Set(result.users.map(user=>user.id)).size!==result.users.length)throw Error(collectionErrorMessage());
   return result.users;
  },
  acceptAttachment:async(id:string,revision:number,session:string,file:{id:string;sha256:string;revision?:string;extraction_revision?:string},signal?:AbortSignal)=>{
   if(!id||!Number.isSafeInteger(revision)||revision<1||!session||!file.id||!file.sha256)throw Error(collectionErrorMessage());
   if(file.revision){const identity=await request<Schema['SystemIdentity']>('/api/v2/system',{signal});if(!identity.capabilities?.includes('session-document-corrections-v1'))throw Error(collectionErrorMessage());}
   const result=await request<Schema['DocumentImport']>(`/v1/collections/imports?${new URLSearchParams({collection_id:id,collection_revision:String(revision)})}`,{method:'POST',body:JSON.stringify({attachment:{session,id:file.id,sha256:file.sha256,...(file.revision?{revision:file.revision}:{})}}),signal});
   if(!result?.id||result.collection_id!==id||result.collection_revision!==revision||result.sha256!==file.sha256||(file.revision&&file.extraction_revision&&result.extraction_revision!==file.extraction_revision))throw Error(collectionErrorMessage());
   return result;
  },
  audit:async(id:string,after=0,signal?:AbortSignal)=>{
   if(!id||!Number.isSafeInteger(after)||after<0)throw Error(collectionErrorMessage());
   const result=await request<{events:Schema['CollectionAuditEvent'][]}>(`/v1/collections?${new URLSearchParams({id,view:'audit',after:String(after)})}`,{signal});
   if(!Array.isArray(result?.events)||result.events.length>100)throw Error(collectionErrorMessage());
   for(const event of result.events){if(!event||!Number.isSafeInteger(event.sequence)||event.sequence<=after||!event.actor||!event.action||!event.subject||!event.created_at)throw Error(collectionErrorMessage());after=event.sequence;}
   return result.events;
  },
  acceptImport:async(id:string,revision:number,file:File,key:string,signal?:AbortSignal,language?:string)=>{
   if(!id||!Number.isSafeInteger(revision)||revision<1||!key)throw Error(collectionErrorMessage());
   if(language){const identity=await request<Schema['SystemIdentity']>('/api/v2/system',{signal});if(!identity.capabilities?.includes(language.includes('+')?'document-ocr-mixed-languages-v1':'document-ocr-language-selection-v1'))throw Error(collectionErrorMessage());}
   const body=new FormData();body.append('file',file);
   const query=new URLSearchParams({collection_id:id,collection_revision:String(revision)});if(language)query.set('ocr_language',language);
   const result=await request<Schema['DocumentImport']>(`/v1/collections/imports?${query}`,{method:'POST',body,headers:{'Idempotency-Key':key},signal});
   if(language&&result.ocr_language!==language)throw Error(collectionErrorMessage());
   if(!result?.id||result.collection_id!==id||result.collection_revision!==revision||!['accepted','ready','failed','needs_input','cancelled','interrupted','queued','extracting','indexing'].includes(result.state))throw Error(collectionErrorMessage());
   return result;
  },
  list:async(signal?:AbortSignal)=>{
   const result=await request<{collections:Policy[]}>('/v1/collections',{signal});
   if(!Array.isArray(result?.collections)||result.collections.length>1024)throw Error(collectionErrorMessage());
   const values=result.collections.map(p=>validateCollection(p));
   if(new Set(values.map(p=>p.id)).size!==values.length)throw Error(collectionErrorMessage());
   return values;
  },
  get:async(id:string,signal?:AbortSignal)=>validateCollection(await request<Policy>(policyPath(id),{signal}),id),
  create:async(name:string,signal?:AbortSignal)=>{
   const p=validateCollection(await request<Policy>('/v1/collections',{method:'POST',body:JSON.stringify({name}),signal}));
   if(p.id==='workspace'||p.revision!==1||p.name!==name||p.readers.length||p.editors.length)throw Error(collectionErrorMessage());
   return p;
  },
  update:async(id:string,body:Schema['CollectionUpdate'],signal?:AbortSignal)=>{
   const p=validateCollection(await request<Policy>(policyPath(id),{method:'PUT',body:JSON.stringify(body),signal}),id);
   if(p.revision!==body.expected_revision+1||p.name!==body.name||!sameMembers(p.readers,body.readers)||!sameMembers(p.editors,body.editors))throw Error(collectionErrorMessage());
   return p;
  },
  document:async(id:string,signal?:AbortSignal)=>{
   const value=await request<Schema['DocumentCollection']>(`/v1/documents/collection?${new URLSearchParams({id})}`,{signal});
   if(!value||value.document_id!==id||!Number.isSafeInteger(value.revision)||value.revision<1)throw Error(collectionErrorMessage());
   validateCollection(value.collection);return value;
  },
  move:async(body:Schema['CollectionMove'],signal?:AbortSignal)=>{
   const value=await request<Schema['DocumentCollection']>('/v1/documents/collection',{method:'PUT',body:JSON.stringify(body),signal});
   if(!value||value.document_id!==body.document_id||![body.expected_revision,body.expected_revision+1].includes(value.revision))throw Error(collectionErrorMessage());
   const p=validateCollection(value.collection,body.destination_id);
   if(p.revision!==body.expected_destination_revision)throw Error(collectionErrorMessage());
   return value;
  },
 };
}
