import {useEffect,useState} from 'react';
import {api,type SessionAttachment} from '../../api/client';
import {useI18n} from '../../i18n';
import {chatFiles,fileRecovery} from '../../i18n/chat-files';
import { documentRecovery } from '../../i18n/document-recovery';
import { useWorkspace } from '../../lib/workspace-context';
import { FileKnowledgeImport } from './FileKnowledgeImport';
import { fileKnowledge } from '../../i18n/file-knowledge';

// Inspection is explicit. Listing a retained source never sends it to a model,
// changes history, republishes it, or retries an uncertain upload.
export function SavedFilePicker({name,disabled,selected,choose}:{name:string;disabled:boolean;selected:string[];choose:(file:SessionAttachment)=>void}){
 const {locale}=useI18n(),copy=chatFiles(locale),labels=fileRecovery(locale);
 const [open,setOpen]=useState(false),[files,setFiles]=useState<SessionAttachment[]>([]),[error,setError]=useState(''),[loading,setLoading]=useState(false);
 const permissions=useWorkspace(),[canImport,setCanImport]=useState(false),[publishing,setPublishing]=useState<SessionAttachment>();
 useEffect(()=>{setPublishing(undefined);setCanImport(false);const abort=new AbortController();if(open&&permissions.admin)void api.systemIdentity(abort.signal).then(identity=>{if(!abort.signal.aborted)setCanImport(identity.capabilities?.includes('document-attachment-imports-v1')===true);}).catch(()=>{});return()=>abort.abort();},[name,open,permissions.admin]);
 useEffect(()=>{setOpen(false);setFiles([]);setError('');},[name]);
 useEffect(()=>{
  if(!open||!name)return;
  const abort=new AbortController();setLoading(true);setError('');
  void api.sessionAttachments(name,abort.signal).then(result=>{if(!abort.signal.aborted)setFiles(result.attachments);}).catch(reason=>{if(!abort.signal.aborted)setError(reason instanceof Error?reason.message:copy.blocked);}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>abort.abort();
 },[name,open]);
 if(!name)return null;
 return <div className="saved-file-picker">{publishing&&<FileKnowledgeImport session={name} file={publishing} close={()=>setPublishing(undefined)}/>}<button type="button" className="text-button" disabled={disabled} aria-expanded={open} onClick={()=>setOpen(value=>!value)}>{labels.saved}</button>{open&&<div className="saved-file-list" aria-label={labels.saved}>
  {loading?<p role="status">{copy.preparing}</p>:error?<p role="alert">{error}</p>:!files.length?<p>{labels.empty}</p>:<ul>{files.map(file=><li key={file.id}><span><strong>{file.name}</strong><small>{file.status==='ready'?copy.ready:documentRecovery(locale,file.error_code)??copy.blocked}</small></span><div className="saved-file-actions"><button type="button" className="text-button" disabled={disabled||selected.includes(file.id)||selected.length>=8} onClick={()=>choose(file)}>{selected.includes(file.id)?labels.selected:labels.use}</button>{canImport&&<button type="button" className="text-button" disabled={disabled} onClick={()=>setPublishing(file)}>{fileKnowledge(locale).title}</button>}</div></li>)}</ul>}
 </div>}</div>;
}
