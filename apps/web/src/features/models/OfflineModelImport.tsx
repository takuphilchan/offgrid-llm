import { useEffect, useRef, useState } from 'react';
import { api, APIError } from '../../api/client';
import { FilePicker } from '../../components/FilePicker';
import { useI18n } from '../../i18n';
import { modelImport } from '../../i18n/model-import';
import { useWorkspaceState } from '../../lib/workspace-context';
import { formatBytes } from './model-format';

// Retain outcome uncertainty across navigation, never a File or an upload queue.
export function OfflineModelImport({onRefresh}: {onRefresh:()=>Promise<void>}) {
 const {locale,messages}=useI18n(), copy=modelImport(locale);
 const [file,setFile]=useState<File>();
 const [invalid,setInvalid]=useState(false);
 const [state,setState]=useWorkspaceState<{phase:'idle'|'uploading'|'done'|'uncertain'|'rejected'; name?:string; error?:string}>('models.gguf-import',{phase:'idle'});
 const active=useRef<AbortController|null>(null);
 useEffect(()=>()=>{ if(active.current) { active.current.abort(); setState({phase:'uncertain'}); } },[]);
 const blocked=state.phase==='uploading'||state.phase==='uncertain';
 async function start() {
  if(!file||blocked||active.current) return;
  const controller=new AbortController(); active.current=controller; setState({phase:'uploading',name:file.name});
  try {
   const result=await api.importGGUF(file,controller.signal);
   if(controller.signal.aborted) return;
   if(result.filename!==file.name||result.size!==file.size) throw new Error('Mismatched import receipt');
   setState({phase:'done',name:result.filename}); setFile(undefined);
  } catch(error) {
   if(!controller.signal.aborted) {
    if(error instanceof APIError && ([401,403,409,422].includes(error.status)||error.data?.error?.code==='gguf_import_busy')) { setState({phase:'rejected',error:error.status===422?copy.invalid:error.message}); setFile(undefined); }
    else setState({phase:'uncertain',name:file.name});
   }
  }
  finally { if(active.current===controller) active.current=null; }
  if(!controller.signal.aborted) await onRefresh().catch(()=>{});
 }
 return <details className="offline-model-import">
  <summary>{copy.title}</summary>
  <p>{copy.body}</p>
  <FilePicker label={copy.choose} accept=".gguf" disabled={blocked} onFiles={files=>{
   const candidate=files[0]; const valid=!!candidate&&candidate.name.endsWith('.gguf')&&candidate.size>=1024**2&&candidate.size<=32*1024**3&&!/-\d{5}-of-\d{5}\.gguf$/i.test(candidate.name)&&!/^mmproj/i.test(candidate.name);
   setInvalid(!valid); setFile(valid?candidate:undefined); setState({phase:'idle'});
  }}/>
  {invalid&&<p role="alert">{copy.invalid}</p>}
  {file&&state.phase==='idle'&&<div className="model-actions"><span>{formatBytes(file.size)}</span><button type="button" className="primary-button" onClick={()=>void start()}>{copy.submit}</button></div>}
  {state.phase==='rejected'&&<p role="alert">{state.error}</p>}
  {state.phase!=='idle'&&state.phase!=='rejected'&&<p role="status">{state.phase==='uploading'&&<span className="offline-import-spinner" aria-hidden="true"/>}{state.phase==='uploading'?copy.busy:state.phase==='done'?copy.done:copy.uncertain}{state.name&&<> {state.name}</>}</p>}
  {state.phase==='uploading'&&<button type="button" className="text-button" onClick={()=>{active.current?.abort();active.current=null;setState({phase:'uncertain',name:file?.name});}}>{messages.models.cancel}</button>}
  {(state.phase==='uncertain'||state.phase==='done')&&<button type="button" className="text-button" onClick={()=>void onRefresh().then(()=>{setState({phase:'idle'});setFile(undefined);}).catch(()=>{})}>{copy.refresh}</button>}
 </details>;
}
