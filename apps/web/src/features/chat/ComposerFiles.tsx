import { useEffect, useMemo, useRef, useState } from 'react';
import { api, APIError, type ChatSession, type SessionAttachment, type SessionAttachmentReference } from '../../api/client';
import { clearSubmittedDraft, draftKey, readDraft, useDraft, writeDraft } from '../../lib/drafts';
import { newSessionName } from './chat-presentation';
import { useI18n } from '../../i18n';
import { chatFiles, fileRecovery } from '../../i18n/chat-files';
import { SavedFilePicker } from './SavedFilePicker';
import { Icon } from '../../components/Icon';
import { documentRecovery } from '../../i18n/document-recovery';
import { FileDraftList } from './FileDraftList';
import './composer-files.css';
import { useDocumentReader } from './DocumentWorkspace';
import { documentReader } from '../../i18n/document-reader';
import { documentCoverage } from '../../i18n/document-coverage';

const accepted = '.txt,.md,.csv,.json,.xml,.html,.htm,.rtf,.pdf,.docx,.xlsx,.pptx';
export async function createFileConversation(options:{scope:string;prompt:string;model:string;draft:string;images:{key:string;value:string};valid:()=>boolean;accept:(session:ChatSession)=>void}){
 const created=await api.createSession(newSessionName(options.prompt||'Files'),options.model);
 if(!options.valid())throw new DOMException('Conversation selection changed','AbortError');
 writeDraft(draftKey(options.scope,'chat',created.name),readDraft(options.draft));writeDraft(options.draft,'');
 writeDraft(draftKey(options.scope,'chat-images',created.name),options.images.value);clearSubmittedDraft(options.images.key,options.images.value);
 options.accept(created);return created.name;
}
export function readFileDraft(value:string):SessionAttachment[]{
 try {const items=JSON.parse(value);return Array.isArray(items)&&items.length<=8&&items.every(a=>/^file-[a-f0-9]{32}$/.test(a?.id)&&typeof a.name==='string'&&['uploaded','ready','failed'].includes(a.status))?items:[];}catch{return [];}
}
export function clearCompletedFileDraft(scope:string,name:string,refs?:SessionAttachmentReference[]){
 if(!refs?.length)return;
 const key=draftKey(scope,'chat-files',name),value=readDraft(key),files=readFileDraft(value);
 if(files.length===refs.length&&files.every((a,i)=>a.id===refs[i].id&&a.revision===refs[i].revision))clearSubmittedDraft(key,value);
}
export function useComposerFiles(scope:string,name:string,disabled:boolean,service:boolean,agent:boolean,ensureSession:(select:(name:string)=>void)=>Promise<string>){
 const draft=useDraft(scope,'chat-files',name),files=useMemo(()=>readFileDraft(draft.value),[draft.value]);
 const {locale}=useI18n(),copy=chatFiles(locale),recovery=fileRecovery(locale);
 const [processing,setProcessing]=useState(false),[error,setError]=useState(''),[pendingName,setPendingName]=useState('');
 const failureMessage=(reason:unknown)=>{
  const code=reason instanceof APIError?reason.data?.error_code??reason.data?.error?.code:undefined;
  // A retained source owns its inline recovery message. Avoid a second error card.
  if(reason instanceof APIError&&reason.data?.attachment&&documentRecovery(locale,code))return '';
  return documentRecovery(locale,code)??(reason instanceof Error?reason.message:copy.blocked);
 };
 const input=useRef<HTMLInputElement>(null),operation=useRef<AbortController|null>(null),target=useRef(''),active=useRef(name);active.current=name;
 useEffect(()=>{setError('');if(operation.current&&name!==target.current)operation.current.abort();},[name]);
 useEffect(()=>()=>operation.current?.abort(),[scope]);
 const save=(session:string,item:SessionAttachment)=>{
  const key=draftKey(scope,'chat-files',session),previous=readFileDraft(readDraft(key));
  writeDraft(key,JSON.stringify([...previous.filter(a=>a.id!==item.id),item]));
 };
 const add=async(incoming:File[])=>{
  if(disabled||operation.current||!incoming.length)return;
  if(!service){setError(copy.blocked);return;}
  if(incoming.length+files.length>8||incoming.some(file=>!file.size||file.size>50*1024*1024||!accepted.split(',').includes('.'+file.name.split('.').at(-1)?.toLowerCase()))){setError(`${copy.files}: TXT, MD, CSV, JSON, XML, HTML, RTF, PDF, DOCX, XLSX, PPTX · ≤ 8 · ≤ 50 MiB`);return;}
  const controller=new AbortController();operation.current=controller;target.current=name;setProcessing(true);setError('');
  try {
   // Set the intended selection before React commits the newly created chat.
   // Otherwise the selection-change effect cancels this first upload itself.
   const session=await ensureSession(session=>{target.current=session;});target.current=session;
   for(const file of incoming){
    if(controller.signal.aborted)break;
    setPendingName(file.name);
    try {const result=await api.uploadSessionAttachment(session,file,controller.signal);if(result.attachment)save(session,result.attachment);else throw Error(copy.blocked);}
    catch(reason){if(reason instanceof APIError&&reason.data?.attachment)save(session,reason.data.attachment);throw reason;}
   }
  }catch(reason){if(!controller.signal.aborted)setError(failureMessage(reason));}
  finally{if(operation.current===controller){operation.current=null;setProcessing(false);}}
 };
 const retry=async(item:SessionAttachment,textOnly=false)=>{
  if(disabled||operation.current||!name)return;
  const controller=new AbortController();operation.current=controller;target.current=name;setProcessing(true);setError('');
  setPendingName(item.name);
  try {const result=await api.prepareSessionAttachment(name,item.id,controller.signal,textOnly);if(result.attachment)save(name,result.attachment);}
  catch(reason){if(reason instanceof APIError&&reason.data?.attachment)save(name,reason.data.attachment);if(!controller.signal.aborted)setError(failureMessage(reason));}
  finally{if(operation.current===controller){operation.current=null;setProcessing(false);}}
 };
 const blocker=files.length?(agent?copy.agent:!service||files.some(a=>a.status!=='ready'||!a.revision)?copy.blocked:''):'';
 return {...draft,files,processing,blocker,add,
  saved:service&&<SavedFilePicker name={name} disabled={disabled||processing} selected={files.map(a=>a.id)} choose={item=>save(name,item)}/>,
  recovery:service&&draft.unsaved&&<p role="alert" className="file-input-notice">{recovery.unsaved}</p>,
  references:files.map(a=>({id:a.id,revision:a.revision??''})),
  control:service&&<><input hidden ref={input} data-attachment="files" type="file" accept={accepted} multiple onChange={event=>{void add([...event.target.files??[]]);event.target.value='';}}/><button type="button" className="utility-button" disabled={disabled||processing||files.length>=8} onClick={()=>input.current?.click()}><Icon name="file" size={17}/><span>{copy.add}</span></button></>,
  preview:<><FileDraftList session={name} files={files} disabled={disabled||processing} retry={item=>void retry(item)} textOnly={item=>void retry(item,true)} remove={item=>{draft.setValue(JSON.stringify(files.filter(a=>a.id!==item.id)));requestAnimationFrame(()=>{input.current?.closest('.composer')?.querySelector<HTMLButtonElement>('.composer-attachments-menu > button')?.focus();});}}/>{processing&&<div role="status" className="document-scan file-input-notice"><Icon name="file" size={20}/> <bdi>{pendingName}</bdi><p>{copy.preparing}</p></div>}{(error||((agent||!service)&&blocker))&&<p role="alert" className="file-input-notice">{error||blocker}</p>}</>,
 };
}

export function FileAttachments({name,references,active=false}:{name:string;references?:SessionAttachmentReference[];active?:boolean}){
 const open=useDocumentReader();
 const {locale}=useI18n(),copy=chatFiles(locale),[files,setFiles]=useState<SessionAttachment[]>([]),[error,setError]=useState('');
 const ids=(references??[]).map(r=>r.id).join(',');
 useEffect(()=>{const abort=new AbortController();setFiles([]);setError('');if(name&&ids)void api.sessionAttachments(name,abort.signal).then(result=>{if(!abort.signal.aborted)setFiles(result.attachments);}).catch(reason=>{if(!abort.signal.aborted)setError(reason instanceof Error?reason.message:copy.blocked);});return()=>abort.abort();},[name,ids]);
 if(!references?.length)return null;
 const download=async(item:SessionAttachment)=>{try{const blob=await api.sessionAttachmentOriginal(name,item.id),url=URL.createObjectURL(blob),link=document.createElement('a');link.href=url;link.download=item.name;link.click();setTimeout(()=>URL.revokeObjectURL(url),30_000);}catch(reason){setError(reason instanceof Error?reason.message:copy.blocked);}};
 return <div className={`saved-file-attachments ${active?'document-source-active':''}`} aria-label={copy.files}>{references.map(ref=>{const item=files.find(a=>a.id===ref.id);return <span key={ref.id}><button type="button" className="secondary-button" disabled={!item} title={open?documentReader(locale).open:copy.download} onClick={()=>item&&(open?open({name:item.name,sha256:item.sha256,path:`/v1/sessions/${encodeURIComponent(name)}/attachments/${item.id}/preview`,original:()=>api.sessionAttachmentOriginal(name,item.id)}):void download(item))}><Icon name="file" size={16}/>{item?.name??copy.files}</button>{item?.coverage==='native_text_only'&&<small>{documentCoverage(locale)}</small>}</span>;})}{active&&<small className="sr-only" role="status">{documentReader(locale).attached}</small>}{error&&<p role="alert">{error}</p>}</div>;
}
