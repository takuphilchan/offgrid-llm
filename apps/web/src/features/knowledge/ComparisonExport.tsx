import {useEffect,useRef,useState} from 'react';
import {api} from '../../api/client';
import {useI18n} from '../../i18n';
import {documentExport} from '../../i18n/document-export';
import {synthesisReviewCopy} from '../../i18n/document-synthesis-review';
import {Icon} from '../../components/Icon';
import {UtilityPopover} from '../../components/UtilityPopover';

export function ComparisonExport({id,synthesis=false,composition=false}:{id:string;synthesis?:boolean;composition?:boolean}) {
 const {locale,messages}=useI18n(),copy=documentExport(locale),failure=messages.common.error;
 const title=synthesis||composition?synthesisReviewCopy(locale).exportDraft:copy.title;
 const [ready,setReady]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(false);
 const operation=useRef<AbortController|null>(null),lock=useRef(false);
 useEffect(()=>{let active=true;void api.systemIdentity().then(v=>{if(active)setReady(v.capabilities?.includes(composition?'document-composition-v1':synthesis?'document-synthesis-export-v1':'document-analysis-export-v1')===true);}).catch(()=>{});return()=>{active=false;operation.current?.abort();};},[synthesis,composition]);
 const download=async(format:'json'|'csv'|'markdown')=>{
  if(lock.current)return;lock.current=true;setBusy(true);setError(false);const abort=new AbortController();operation.current=abort;
  try{
   const result=await (composition?api.composition:synthesis?api.synthesis:api.analyses).export(id,format,abort.signal);if(abort.signal.aborted)return;
   const blob=new Blob([result.content],{type:format==='json'?'application/json;charset=utf-8':format==='csv'?'text/csv;charset=utf-8':'text/markdown;charset=utf-8'}),url=URL.createObjectURL(blob);
   const link=document.createElement('a');link.href=url;link.download=`offgrid-${composition?'draft-summary':synthesis?'draft-analysis':'comparison'}.${format==='markdown'?'md':format}`;link.click();setTimeout(()=>URL.revokeObjectURL(url),30000);
  }catch{if(!abort.signal.aborted)setError(true);}finally{lock.current=false;if(!abort.signal.aborted)setBusy(false);}
 };
 if(!ready)return null;
 return <UtilityPopover className="comparison-export" align="end" label={title} trigger={<><Icon name="download" size={16}/>{title}</>}><div><p>{copy.retention}</p><div className="comparison-actions">{(['json','csv','markdown'] as const).map(format=><button className="secondary-button" disabled={busy} key={format} onClick={()=>void download(format)}>{format==='markdown'?'Markdown':format.toUpperCase()}</button>)}</div>{busy&&<p role="status">{messages.common.loading}</p>}{error&&<p role="alert">{failure}</p>}</div></UtilityPopover>;
}
