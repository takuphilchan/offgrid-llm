import {useEffect,useMemo,useRef,useState,type DragEvent,type ClipboardEvent} from 'react';
import type {SessionImage,Model} from '../../api/client';
import {useDraft,readDraft,writeDraft} from '../../lib/drafts';
import {Icon} from '../../components/Icon';
import {useI18n} from '../../i18n';
import {chatImages,imagePreview} from '../../i18n/chat-images';
import {ImagePreview} from './ImagePreview';
import './composer-images.css';

const limit=2*1024*1024;
export function readImages(value:string):SessionImage[]{
 try{const images=JSON.parse(value);return Array.isArray(images)&&images.length<=4&&images.every(image=>typeof image.name==='string'&&typeof image.data_url==='string'&&/^data:image\/(png|jpeg);base64,[A-Za-z0-9+/=]+$/.test(image.data_url)&&image.data_url.length<Math.ceil(limit/3)*4+32)?images:[];}catch{return [];}
}
export function useComposerImages(scope:string,id:string,disabled:boolean,availability:{service:boolean;agent:boolean;model?:Model;history:boolean}){
 const draft=useDraft(scope,'chat-images',id),images=useMemo(()=>readImages(draft.value),[draft.value]);
 const {locale,messages}=useI18n(),copy=chatImages(locale);
 const blocker=!(images.length||availability.history)?'':availability.agent?copy.agent:!availability.service?copy.service:!['unknown','tested'].includes(availability.model?.capability_status?.vision??'')?copy.model:'';
 const blockerCode=blocker===copy.model?'vision_projector_unavailable':'';
 const [error,setError]=useState(''),[processing,setProcessing]=useState(false);
 const current=useRef(draft.key);current.current=draft.key;
 const input=useRef<HTMLInputElement>(null),pending=useRef(false);
 useEffect(()=>{setError('');},[draft.key]);
 useEffect(()=>()=>{current.current='';},[]);
 const add=async(files:File[])=>{
  if(disabled||pending.current||!files.length)return;
  const key=draft.key;pending.current=true;setProcessing(true);setError('');
  try{
   if(files.length+readImages(readDraft(key)).length>4)throw Error(copy.limits);
   const incoming=await Promise.all(files.map(async file=>{
    if(!['image/png','image/jpeg'].includes(file.type)||!file.size||file.size>limit||new TextEncoder().encode(file.name).length>200)throw Error(copy.limits);
    const bytes=new Uint8Array(await file.slice(0,8).arrayBuffer());
    if(file.type==='image/png'?![137,80,78,71,13,10,26,10].every((v,i)=>bytes[i]===v):bytes[0]!==255||bytes[1]!==216||bytes[2]!==255)throw Error(copy.limits);
    const bitmap=await createImageBitmap(file);const valid=bitmap.width<=8192&&bitmap.height<=8192&&bitmap.width*bitmap.height<=32*1024*1024;bitmap.close();if(!valid)throw Error(copy.limits);
    const data_url=await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result));reader.onerror=()=>reject(Error(copy.limits));reader.readAsDataURL(file);});
    return {name:file.name,data_url};
   }));
   if(current.current!==key)return;
   const previous=readImages(readDraft(key));if(previous.length+incoming.length>4)throw Error(copy.limits);
   writeDraft(key,JSON.stringify([...previous,...incoming]));
  }catch{if(current.current===key)setError(copy.limits);}
  finally{pending.current=false;setProcessing(false);}
 };
 return {...draft,images,error,processing,copy,blocker,blockerCode,
  paste:(event:ClipboardEvent)=>{const files=[...event.clipboardData.files];if(files.length){event.preventDefault();void add(files);}},
  dragOver:(event:DragEvent)=>{if(event.dataTransfer.types.includes('Files')){event.preventDefault();event.dataTransfer.dropEffect=disabled?'none':'copy';}},
  drop:(event:DragEvent)=>{if(event.dataTransfer.types.includes('Files')){event.preventDefault();void add([...event.dataTransfer.files]);}},
  control:<><input hidden ref={input} type="file" accept="image/png,image/jpeg" multiple onChange={event=>{void add([...event.target.files??[]]);event.target.value='';}}/><button type="button" className="utility-button composer-image-add" disabled={disabled||processing||images.length>=4} aria-label={copy.add} title={copy.limits} onClick={()=>input.current?.click()}><Icon name="image" size={17}/><span>{copy.add}</span></button></>,
  preview:<>{images.length>0&&<ImageAttachments images={images} remove={index=>draft.setValue(JSON.stringify(images.filter((_,i)=>i!==index)))} disabled={disabled||processing}/ >}{processing&&<div className="document-scan file-input-notice" role="status"><Icon name="image" size={20}/><span>{copy.images}</span><p>{messages.common.loading}</p></div>}{error&&<p role="alert" className="image-input-notice">{error}</p>}{draft.unsaved&&<p role="alert" className="image-input-notice">{copy.unsaved}</p>}</>,
 };
}
export function ImageAttachments({images,remove,disabled}:{images?:SessionImage[];remove?:(index:number)=>void;disabled?:boolean}){
 const {locale}=useI18n(),copy=chatImages(locale);
 const [selected,setSelected]=useState<SessionImage>();
 if(!images?.length)return null;
 return <div className="image-attachments" aria-label={copy.images}>
  {images.map((image,index)=><figure key={index}>
   <button type="button" className="image-attachment-open" aria-label={`${imagePreview[locale].open}: ${image.name}`} onClick={()=>setSelected(image)}>
    <img src={/^data:image\/(png|jpeg);base64,/.test(image.data_url)?image.data_url:undefined} alt={image.name}/>
    <span title={image.name}><bdi>{image.name}</bdi></span>
   </button>
   {remove&&<button type="button" className="icon-button image-attachment-remove" disabled={disabled} aria-label={`${copy.remove}: ${image.name}`} onClick={event=>{
    const composer=event.currentTarget.closest('.composer');remove(index);
    requestAnimationFrame(()=>{const next=composer?.querySelectorAll<HTMLButtonElement>('.image-attachment-remove');(next?.[Math.min(index,next.length-1)]??composer?.querySelector<HTMLButtonElement>('.composer-attachments-menu > button'))?.focus();});
   }}><Icon name="close" size={16}/></button>}
  </figure>)}
  {selected&&images.includes(selected)&&<ImagePreview image={selected} close={()=>setSelected(undefined)}/>}
 </div>;
}
