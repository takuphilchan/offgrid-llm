import type {components} from './schema.generated';
import {clientMessage} from '../i18n/client-errors';

type Schema=components['schemas'];
type Request=<T>(path:string,init?:RequestInit,timeout?:number)=>Promise<T>;
const base=(name:string,id:string)=>`/v1/sessions/${encodeURIComponent(name)}/attachments/${encodeURIComponent(id)}`;
export function attachmentCorrectionAPI(request:Request){return {
 sessionAttachmentRevisions:async(name:string,id:string,signal?:AbortSignal)=>{
  const result=await request<Schema['AttachmentRevisionHistory']>(`${base(name,id)}/revisions`,{signal});
  if(result?.id!==id||!result.current_revision||!Array.isArray(result.revisions)||!result.revisions.length||result.revisions.length>65||result.revisions[0].revision!==result.current_revision||new Set(result.revisions.map(r=>r.revision)).size!==result.revisions.length||result.revisions.some(r=>!/^[a-f0-9]{64}$/.test(r.revision)||!r.extraction_revision))throw Error(clientMessage('response'));
  return result;
 },
 correctSessionAttachment:async(name:string,id:string,revision:string,extraction:string,edits:{block_id:string;expected_text_sha256:string;text:string}[])=>{
  const identity=await request<Schema['SystemIdentity']>('/api/v2/system');
  if(!identity.capabilities?.includes('session-document-corrections-v1'))throw Error(clientMessage('response'));
  const result=await request<Schema['SessionAttachmentResult']>(`${base(name,id)}/corrections`,{method:'POST',body:JSON.stringify({expected_revision:revision,extraction_revision:extraction,edits})},120_000);
  const a=result?.attachment;
  if(a?.id!==id||a.status!=='ready'||a.parent_revision!==revision||!a.revision||a.revision===revision||!a.extraction_revision||a.extraction_revision===extraction)throw Error(clientMessage('response'));
  return {...result,attachment:a};
 }
};}
