import { useEffect, useId, useRef, useState } from 'react';
import { api, type AgentSkill, type AgentSkillReference, type AgentSkillInspection, type AgentMemoryEntry } from '../../api/client';
import { useI18n } from '../../i18n';
import { agentContextText } from '../../i18n/agent-context';
import { SkillLibrary, sameSkill } from './SkillLibrary';
import { skillLibraryText } from '../../i18n/skill-library';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { FilePicker } from '../../components/FilePicker';

function decoded(content:string) {
  try {const text=new TextDecoder('utf-8',{fatal:true}).decode(Uint8Array.from(atob(content),char=>char.charCodeAt(0)));return text.includes('\0')?`base64: ${content}`:text;}
  catch {return `base64: ${content}`;}
}

export function AgentContextResources({profile,selected,onChange,disabled=false}:{profile?:string;selected:AgentSkillReference[];onChange:(refs:AgentSkillReference[])=>void;disabled?:boolean}) {
  const {locale,messages}=useI18n(),copy=agentContextText[locale],library=skillLibraryText[locale];
  const formId=useId();
  const [supported,setSupported]=useState<boolean>(),[skills,setSkills]=useState<AgentSkill[]>([]),[inspection,setInspection]=useState<AgentSkillInspection>();
  const [memory,setMemory]=useState<Awaited<ReturnType<typeof api.agentMemory>>>(),[editing,setEditing]=useState<AgentMemoryEntry>();
  const [label,setLabel]=useState(''),[content,setContent]=useState(''),[deleting,setDeleting]=useState<AgentMemoryEntry>();
  const [error,setError]=useState(''),[busy,setBusy]=useState(false);
  const alive=useRef(true),lock=useRef(false),generation=useRef(0),writeIdentity=useRef<{payload:string;operation:string}|undefined>(undefined);
  useEffect(()=>{
    alive.current=true;const abort=new AbortController();
    void api.systemIdentity(abort.signal).then(async identity=>{
      if(abort.signal.aborted)return;
      const available=identity.capabilities?.includes('agent-context-resources-v1')===true;setSupported(available);
      if(available){const result=await api.agentSkills(abort.signal);if(!abort.signal.aborted)setSkills(result.skills);}
    }).catch(reason=>{if(!abort.signal.aborted)setError(reason instanceof Error?reason.message:messages.common.error);});
    return()=>{alive.current=false;generation.current++;abort.abort();};
  },[]);
  const run=async(action:()=>Promise<void>)=>{
    if(lock.current || disabled)return;lock.current=true;setBusy(true);setError('');
    try{await action();}catch(reason){if(alive.current)setError(reason instanceof Error?reason.message:messages.common.error);}
    finally{lock.current=false;if(alive.current)setBusy(false);}
  };
  const refreshSkills=async()=>{const result=await api.agentSkills();if(alive.current)setSkills(result.skills);};
  const refreshMemory=async()=>{
    if(!profile || !supported)return;const current=++generation.current;
    try{const result=await api.agentMemory(profile);if(alive.current && current===generation.current)setMemory(result);}
    catch(reason){if(alive.current && current===generation.current)setError(reason instanceof Error?reason.message:messages.common.error);}
  };
  const importFiles=async(files:File[])=>{
    let size=0;
    if(!files.length || files.length>64 || files.some(file=>{size+=file.size;return file.size>65536;}) || size>524288)throw new Error(copy.invalid);
    const payload=[];
    for(const file of files){
      const relative=file.webkitRelativePath || file.name,path=file.webkitRelativePath?relative.split('/').slice(1).join('/'):relative;
      const bytes=new Uint8Array(await file.arrayBuffer());let binary='';for(const byte of bytes)binary+=String.fromCharCode(byte);
      payload.push({path,content:btoa(binary)});
    }
    await api.importAgentSkill(payload);await refreshSkills();
  };
  const inspect=async(ref:AgentSkillReference)=>{setInspection(undefined);const value=await api.inspectAgentSkill(ref);if(alive.current)setInspection(value);};
  const edit=(entry?:AgentMemoryEntry)=>{setEditing(entry);setLabel(entry?.label??'');setContent(entry?.content??'');writeIdentity.current=undefined;};
  const saveMemory=async()=>{
    if(!profile)return;
    const value={id:editing?.id,expected_revision:editing?.revision??0,label,content};const payload=JSON.stringify(value);
    if(writeIdentity.current?.payload!==payload)writeIdentity.current={payload,operation:crypto.randomUUID()};
    await api.writeAgentMemory(profile,{...value,operation:writeIdentity.current.operation});
    if(alive.current)edit();await refreshMemory();
  };
  if(supported===false)return <p className="profile-hint">{copy.unsupported}</p>;
  return <section className="profile-context-resources" aria-label={`${copy.skills} / ${copy.memory}`} onKeyDown={event=>{if(event.key==='Enter'&&!event.nativeEvent.isComposing&&event.target instanceof HTMLInputElement&&['text','search'].includes(event.target.type))event.preventDefault();}}>
    {error&&<p role="alert">{error}</p>}
    <details><summary>{copy.skills} · {selected.length}</summary>
      <p className="profile-hint">{copy.skillHint}</p>
      <fieldset className="skill-controls" disabled={disabled||busy||!supported}><legend className="sr-only">{copy.skills}</legend>
        <div className="skill-toolbar">
        <FilePicker label={copy.import} directory disabled={disabled||busy||!supported} onFiles={files=>run(()=>importFiles(files))}/>
        <button type="button" className="secondary-button" onClick={()=>void run(refreshSkills)}>{copy.refresh}</button>
        </div>
        <SkillLibrary skills={skills} selected={selected} onChange={onChange} inspect={ref=>void run(()=>inspect(ref))}/>
        {inspection&&<section className="skill-inspection" aria-label={copy.inspect}><h4>{inspection.skill.name} · v{inspection.skill.revision}</h4><p>{copy.inert}</p><details className="skill-revision"><summary>{library.details}</summary><p>{inspection.skill.id} · v{inspection.skill.revision}</p><p>{inspection.skill.digest}</p></details>
          <div className="skill-example"><strong>{library.example}</strong><p>{library.prompt.replace('{name}',inspection.skill.name)}</p></div>
          {!inspection.skill.draft&&(selected.some(ref=>sameSkill(ref,inspection.skill))?<p role="status">{library.attached}</p>:<button type="button" className="secondary-button" disabled={selected.length>=16} onClick={()=>onChange([...selected.filter(ref=>ref.id!==inspection.skill.id),{id:inspection.skill.id,revision:inspection.skill.revision,digest:inspection.skill.digest}])}>{library.attach}</button>)}
          <details open><summary>{copy.files}</summary>{inspection.bundle.files.map(file=><details key={file.path} open={file.path==='SKILL.md'}><summary>{file.path}</summary><pre className="tool-output">{decoded(file.content)}</pre></details>)}</details>
          {inspection.skill.draft&&<button type="button" className="secondary-button" onClick={()=>void run(async()=>{const reviewed=await api.reviewAgentSkill(inspection.skill);await refreshSkills();await inspect(reviewed);})}>{copy.review}</button>}
        </section>}
      </fieldset>
    </details>
    <details onToggle={event=>{if(event.currentTarget.open)void refreshMemory();}}><summary>{copy.memory}</summary><p className="profile-hint">{copy.memoryHint}</p><p className="profile-hint">{copy.retention}</p>
      {!profile?<p>{copy.saveFirst}</p>:<fieldset disabled={disabled||busy||!supported}><legend>{copy.memory}</legend>
        <label className="profile-option"><input type="checkbox" checked={memory?.consent.enabled??false} disabled={!memory} onChange={event=>{const enabled=event.target.checked,revision=memory!.consent.revision;void run(async()=>{const consent=await api.changeAgentMemoryConsent(profile,revision,enabled);if(alive.current)setMemory(current=>current&&({...current,consent}));});}}/><span>{copy.enabled}</span></label>
        <button type="button" className="secondary-button" onClick={()=>void run(refreshMemory)}>{copy.refresh}</button>
        {memory&&!memory.entries.length&&<p>{copy.empty}</p>}
        {memory?.entries.map(entry=><article key={entry.id}><h4>{entry.label}</h4><pre className="tool-output">{entry.content}</pre><div className="profile-actions"><button type="button" className="utility-button" onClick={()=>edit(entry)}>{copy.edit}</button><button type="button" className="danger-button" onClick={()=>setDeleting(entry)}>{copy.delete}</button></div></article>)}
        <div className="profile-context-field"><label htmlFor={`${formId}-label`}>{copy.label}</label><input id={`${formId}-label`} value={label} maxLength={128} onChange={event=>setLabel(event.target.value)}/></div>
        <div className="profile-context-field"><label htmlFor={`${formId}-content`}>{copy.content}</label><textarea id={`${formId}-content`} value={content} maxLength={8192} onChange={event=>setContent(event.target.value)}/></div>
        <div className="profile-actions"><button type="button" className="secondary-button" disabled={!label.trim()||!content.trim()||!memory} onClick={()=>void run(saveMemory)}>{copy.save}</button>{editing&&<button type="button" className="utility-button" onClick={()=>edit()}>{copy.cancel}</button>}</div>
      </fieldset>}
    </details>
    {(busy||supported===undefined)&&<p role="status">{messages.common.loading}</p>}
    {deleting&&profile&&<ConfirmDialog title={copy.delete} body={copy.retention} confirmLabel={copy.delete} close={()=>setDeleting(undefined)} confirm={async()=>{await api.deleteAgentMemory(profile,deleting);if(alive.current&&editing?.id===deleting.id)edit();await refreshMemory();}}/>}
  </section>;
}
