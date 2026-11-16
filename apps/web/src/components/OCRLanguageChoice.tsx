import {useEffect,useId,useState} from 'react';
import {api} from '../api/client';
import {useI18n} from '../i18n';
import {ocrSetup} from '../i18n/ocr-setup';
import {ocrChoice} from '../i18n/ocr-choice';
import {ocrMixed} from '../i18n/ocr-mixed';
import {documentRecovery} from '../i18n/document-recovery';
import {clientMessage} from '../i18n/client-errors';
import {Icon} from './Icon';
import './ocr-language.css';

// Selection applies to document recognition, independently of interface locale.
// Older services keep their existing English behavior; explicit choices require
// the new capability and matching selection in the import receipt.
export function OCRLanguageChoice({value,onChange,disabled=false,chat=false}:{value:string|undefined;onChange:(value:string)=>void;disabled?:boolean;chat?:boolean}) {
 const {locale,available}=useI18n(),copy=ocrSetup(locale),choice=ocrChoice(locale),mixed=ocrMixed(locale),id=useId();
 const [readiness,setReadiness]=useState<Awaited<ReturnType<typeof api.ocrLanguages>>>();
 const [notice,setNotice]=useState('');
 const [adding,setAdding]=useState(false);
 useEffect(()=>{const abort=new AbortController();void api.systemIdentity(abort.signal).then(async identity=>{
  if(!identity.capabilities?.includes('document-ocr-language-selection-v1')){if(chat&&!abort.signal.aborted)setNotice(clientMessage('fileUpgrade'));return;}
  const result=await (chat?api.chatOCRLanguages:api.ocrLanguages)(abort.signal);
  if(!['single_language','ordered_languages'].includes(result.selection_mode??'')||!Array.isArray(result.languages)||result.languages.length>100||!result.languages.some(p=>p.language==='eng'))throw Error(copy.unknown);
  if(result.selection_mode==='ordered_languages'&&(!identity.capabilities?.includes('document-ocr-mixed-languages-v1')||result.max_languages!==3))throw Error(copy.unknown);
  if(!abort.signal.aborted)setReadiness(result);
 }).catch(()=>{if(!abort.signal.aborted)setNotice(copy.unknown);});return()=>abort.abort();},[chat,copy.unknown]);
 if(!readiness)return notice?<p className="workspace-secondary" role="status">{notice}</p>:null;
 const ids=(value??'eng').split('+'),canMix=readiness.selection_mode==='ordered_languages';
 const missing=ids.map(language=>readiness.languages.find(p=>p.language===language)).filter(p=>p?.state!=='installed_verified');
 const setPrimary=(language:string)=>onChange([language,...ids.slice(1).filter(id=>id!==language)].join('+'));
 const focusPrimary=()=>document.getElementById(id)?.focus();
 const remove=(index:number)=>{onChange(ids.filter((_,i)=>i!==index+1).join('+'));focusPrimary();};
 return <fieldset className="ocr-language-choice" disabled={disabled}>
  {!chat&&<legend>{copy.title}</legend>}
  <label htmlFor={id}>{choice.language}</label>
  <select id={id} value={ids[0]} onChange={event=>setPrimary(event.target.value)}>{readiness.languages.map(p=><option key={p.language} value={p.language}>{p.name}</option>)}</select>
  {canMix&&<>
   {ids.slice(1).map((language,index)=><div className="ocr-additional-language" key={index}>
    <label htmlFor={`${id}-${index}`}>{mixed.additional} {new Intl.NumberFormat(locale).format(index+1)}</label>
    <div><select id={`${id}-${index}`} value={language} onChange={event=>onChange(ids.map((item,i)=>i===index+1?event.target.value:item).join('+'))}>{readiness.languages.map(p=><option key={p.language} value={p.language} disabled={ids.includes(p.language)&&p.language!==language}>{p.name}</option>)}</select>
     <button type="button" className="text-button" aria-label={`${mixed.remove}: ${readiness.languages.find(p=>p.language===language)?.name??language}`} onClick={()=>remove(index)}><Icon name="close" size={16}/></button></div>
   </div>)}
   {ids.length<3&&(adding?<div className="ocr-additional-language">
    <label htmlFor={`${id}-new`}>{mixed.additional} {new Intl.NumberFormat(locale).format(ids.length)}</label>
    <div><select id={`${id}-new`} autoFocus value="" onChange={event=>{if(event.target.value){onChange([...ids,event.target.value].join('+'));setAdding(false);focusPrimary();}}}><option value="" disabled>{mixed.additional}</option>{readiness.languages.filter(p=>!ids.includes(p.language)).map(p=><option key={p.language} value={p.language}>{p.name}</option>)}</select>
     <button type="button" className="text-button" aria-label={mixed.remove} onClick={()=>{setAdding(false);focusPrimary();}}><Icon name="close" size={16}/></button></div>
   </div>:<button type="button" className="text-button ocr-add-language" onClick={()=>setAdding(true)}>{mixed.add}</button>)}
   <p className="workspace-secondary">{mixed.hint}</p>
  </>}
  <p className="workspace-secondary" role="status">{missing.length===0?(ids.length>1?mixed.installed:copy.installed):documentRecovery(locale,'document_ocr_language_missing')}{missing.length>0&&` · ${missing.map(p=>p?.name??'').join(' · ')}`}</p>
  {!!readiness.unavailable_interface_locales?.length&&<p className="workspace-secondary">{choice.unavailable}: {readiness.unavailable_interface_locales.map(code=>available.find(language=>language.code===code)?.label??code).join(' · ')}</p>}
 </fieldset>;
}
