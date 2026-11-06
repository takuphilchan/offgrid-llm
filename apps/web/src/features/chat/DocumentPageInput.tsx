import { useEffect, useState } from 'react';

/** Editing a page number must not request every intermediate digit. */
export function DocumentPageInput({ page, count, label, change }: {page:number;count:number;label:string;change:(page:number)=>void}) {
 const [draft,setDraft]=useState(String(page));
 useEffect(()=>setDraft(String(page)),[page]);
 const commit=()=>{
  const value=Number(draft);
  const next=draft.trim()&&Number.isInteger(value)?Math.max(1,Math.min(count,value)):page;
  setDraft(String(next));
  if(next!==page)change(next);
 };
 return <label><span className="sr-only">{label}</span><input aria-label={label} type="number" min={1} max={count} value={draft}
  onChange={event=>setDraft(event.target.value)} onBlur={commit} onKeyDown={event=>{if(event.key==='Enter'){event.preventDefault();commit();}}}/><span> / {count}</span></label>;
}
