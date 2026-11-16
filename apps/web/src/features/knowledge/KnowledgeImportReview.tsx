import {useEffect,useRef,useState} from 'react';
import {createPortal} from 'react-dom';
import type {CollectionPolicy} from '../../api/collections';
import {useI18n} from '../../i18n';
import {restoreFocus} from '../../lib/restore-focus';
import {CollectionDestination} from './CollectionDestination';
import {OCRLanguageChoice} from '../../components/OCRLanguageChoice';

// Files stay in this tab until the selected audience has been acknowledged.
// Closing this dialog sends neither bytes nor an import request to the service.
export function KnowledgeImportReview({files,disabled,close,confirm}:{files:File[];disabled:boolean;close:()=>void;confirm:(destination:CollectionPolicy,language?:string)=>void}) {
 const {messages}=useI18n(),dialog=useRef<HTMLDialogElement>(null),opener=useRef(document.activeElement as HTMLElement|null);
 const [destination,setDestination]=useState<CollectionPolicy>();
 const [language,setLanguage]=useState<string>();
 const submitted=useRef(false);
 useEffect(()=>{const node=dialog.current!,previous=opener.current;node.showModal();return()=>{node.close();restoreFocus(previous);};},[]);
 return createPortal(<dialog ref={dialog} className="history-delete-dialog" aria-label={messages.knowledge.add} onCancel={event=>{event.preventDefault();close();}}>
  <h2>{messages.knowledge.add}</h2>
  <ul className="knowledge-import-selection">{files.map((file,index)=><li key={index}><bdi>{file.name}</bdi></li>)}</ul>
  <CollectionDestination disabled={disabled} onChange={setDestination} files/>
  {files.some(file=>/\.(pdf|png|jpe?g|tiff?)$/i.test(file.name))&&<OCRLanguageChoice disabled={disabled} value={language} onChange={setLanguage}/>}
  <div className="history-dialog-actions">
   <button type="button" className="secondary-button" autoFocus onClick={close}>{messages.models.cancel}</button>
   <button type="button" className="primary-button" disabled={disabled||!destination} onClick={()=>{if(!disabled&&destination&&!submitted.current){submitted.current=true;confirm(destination,language);}}}>{messages.knowledge.add}</button>
  </div>
 </dialog>,document.body);
}
