import {useEffect,useRef} from 'react';
import {createPortal} from 'react-dom';
import {useI18n} from '../i18n';
import {ocrSetup} from '../i18n/ocr-setup';
import {ocrChoice} from '../i18n/ocr-choice';
import {restoreFocus} from '../lib/restore-focus';
import {OCRLanguageChoice} from './OCRLanguageChoice';
import './ocr-language.css';

export function OCRLanguageDialog({value,onChange,close}:{value:string|undefined;onChange:(value:string)=>void;close:()=>void}) {
 const {locale}=useI18n(),copy=ocrSetup(locale),choice=ocrChoice(locale),dialog=useRef<HTMLDialogElement>(null),opener=useRef(document.activeElement as HTMLElement|null);
 useEffect(()=>{const node=dialog.current!,previous=opener.current;node.showModal();return()=>{node.close();restoreFocus(previous);};},[]);
 return createPortal(<dialog ref={dialog} className="history-delete-dialog ocr-language-dialog" aria-label={copy.title} onCancel={event=>{event.preventDefault();close();}}>
  <h2>{copy.title}</h2>
  <OCRLanguageChoice value={value} onChange={onChange} chat/>
  <p className="workspace-secondary">{copy.guidance}</p>
  <p className="workspace-secondary">{choice.hint}</p>
  <div className="history-dialog-actions"><button autoFocus type="button" className="secondary-button" onClick={close}>{choice.close}</button></div>
 </dialog>,document.body);
}
