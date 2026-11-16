import {DocumentRevisionAlternatives} from './DocumentRevisionAlternatives';
import { useEffect, useRef, useState } from 'react';
import type {components} from '../../api/schema.generated';
import { api, type DocumentBlock, type DocumentStructurePage } from '../../api/client';
import { useI18n } from '../../i18n';
import {workSurface} from '../../i18n/work-surface';
import { documentReview } from '../../i18n/document-review';
import { documentReader } from '../../i18n/document-reader';
import { knowledgeUnitLabels } from '../../i18n/knowledge-locators';
import { DocumentEvidenceNotice } from '../../components/DocumentEvidenceNotice';
import './document-review.css';
import {DocumentStructureNavigation} from './DocumentStructureNavigation';
import {documentNavigation} from '../../i18n/document-navigation';
import {useWorkspaceState} from '../../lib/workspace-context';
import {useDraftScope} from '../../lib/draft-scope';

type Draft = { block: DocumentBlock; revision: string; attachmentRevision?:string; text: string; uncertain: boolean; latest?: { block: DocumentBlock; revision: string; attachmentRevision?:string } };

/** Inspect immutable extracted text. A failed write is never automatically replayed. */
export function DocumentReview({ id, revision = '', highlight = '', attachment, readOnly = false, selectBlock }: { id: string; revision?: string; highlight?: string; attachment?: {session: string; revision: string}; readOnly?: boolean; selectBlock?:(revision:string,block:DocumentBlock)=>void }) {
 const { locale, messages, language } = useI18n(), copy = documentReview(locale), reader = documentReader(locale);
 const [result, setResult] = useState<DocumentStructurePage>();
 const [selection, setSelection] = useState({ revision, unit: '', offset: 0, retry: 0, navigation:0 });
 const [attachmentRevision,setAttachmentRevision]=useState(attachment?.revision??'');
 const [history,setHistory]=useState<components['schemas']['AttachmentRevisionHistory']>();
 const [targetBlock,setTargetBlock]=useState('');
 const [expanded,setExpanded]=useState({coverage:false,outline:false});
 const root=useRef<HTMLElement>(null),navigationCopy=documentNavigation(locale);
 const [error, setError] = useState(false), [loading, setLoading] = useState(false);
 const draftScope=useDraftScope();
 const [draft, setDraft] = useWorkspaceState<Draft|undefined>(`document-correction:${JSON.stringify([draftScope,id,readOnly?revision:'editable',attachment?.session,attachment?.revision,readOnly])}`,undefined);
 const [busy, setBusy] = useState(false), [saved, setSaved] = useState(false);
 const canWrite=!!result?.can_correct&&!readOnly;
 const baseChanged=!!draft&&!!result&&(draft.revision!==result.revision||!!attachment&&draft.attachmentRevision!==attachmentRevision);
 const lock = useRef(false), mounted = useRef(true);
 useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
 useEffect(() => {
  const controller = new AbortController(); setLoading(true); setError(false); setResult(undefined);
  const load = attachment ? api.sessionAttachmentStructure(attachment.session, id, attachmentRevision, selection.revision, selection.unit, selection.offset, controller.signal,selection.navigation,!readOnly) : api.documentStructure(id, selection.revision, selection.unit, selection.offset, controller.signal,0,selection.navigation);
  void load.then(value => {
   if (!controller.signal.aborted) setResult(value);
  }).catch(() => { if (!controller.signal.aborted) setError(true); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
  return () => controller.abort();
 }, [id, selection, attachment?.session, attachmentRevision,readOnly]);
 useEffect(()=>{
  if(!attachment)return;
  const controller=new AbortController();
  void api.sessionAttachmentRevisions(attachment.session,id,controller.signal).then(value=>{if(!controller.signal.aborted)setHistory(value);}).catch(()=>{if(!controller.signal.aborted)setHistory(undefined);});
  return ()=>controller.abort();
 },[id,attachment?.session,attachmentRevision]);
 useEffect(()=>{if(result&&targetBlock){const node=Array.from(root.current?.querySelectorAll<HTMLElement>('[data-block-id]')??[]).find(item=>item.dataset.blockId===targetBlock);node?.scrollIntoView({block:'nearest'});node?.focus({preventScroll:true});}},[result,targetBlock]);
 const save = async () => {
  if (!canWrite || baseChanged || !draft || draft.uncertain || lock.current || draft.text === draft.block.text) return;
  lock.current = true; setBusy(true); setSaved(false);
  // Persist uncertainty before dispatch. Navigating away must never turn an
  // in-flight mutation back into an apparently unsent, replayable draft.
  setDraft({...draft,uncertain:true,latest:undefined});
  try {
   const edits=[{ block_id: draft.block.id, expected_text_sha256: draft.block.text_sha256, text: draft.text }];
   let nextRevision:string;
   if(attachment){
    const response=await api.correctSessionAttachment(attachment.session,id,draft.attachmentRevision!,draft.revision,edits);
    if(!mounted.current)return;
    setAttachmentRevision(response.attachment.revision!);nextRevision=response.attachment.extraction_revision!;
   }else nextRevision=(await api.correctDocument(id,draft.revision,edits)).document.extraction_revision!;
   if (!mounted.current) return;
   setDraft(undefined); setSaved(true);
   setSelection(value => ({ ...value, revision: nextRevision, retry: value.retry + 1 }));
  } catch { if (mounted.current) setDraft(value => value && ({ ...value, uncertain: true, latest: undefined })); }
  finally { lock.current = false; if (mounted.current) setBusy(false); }
 };
 const inspectLatest = async () => {
  if (!draft || lock.current) return;
  lock.current = true; setBusy(true);
  try {
   // A fresh read never changes the draft's base implicitly. Pagination stays
   // pinned to the revision returned by the first request.
   const latest=attachment?await api.sessionAttachmentRevisions(attachment.session,id):undefined;
   const current=latest?.revisions[0];
   const read=(revision:string,offset=0)=>attachment?api.sessionAttachmentStructure(attachment.session,id,current!.revision,revision,draft.block.unit_id,offset,new AbortController().signal,0,true):api.documentStructure(id,revision,draft.block.unit_id,offset);
   let page = await read(current?.extraction_revision??'');
   let block = page.blocks.find(value => value.id === draft.block.id);
   for (let n = 0; !block && page.next_offset !== null && n < 20; n++) {
    page = await read(page.revision, page.next_offset);
    block = page.blocks.find(value => value.id === draft.block.id);
   }
   if (mounted.current) setDraft(value => value && ({ ...value, latest: block && page.can_correct && block.editable ? { block, revision: page.revision, attachmentRevision:current?.revision } : undefined }));
  } catch { if (mounted.current) setDraft(value => value && ({ ...value, latest: undefined })); }
  finally { lock.current = false; if (mounted.current) setBusy(false); }
 };
 const number = (value: number) => new Intl.NumberFormat(language).format(value);
 return <section ref={root} className="document-review" aria-label={copy.title} aria-busy={busy || loading}>
  <p className="workspace-secondary">{copy.note}</p>
  {(saved || loading) && <p role="status">{loading ? messages.common.loading : copy.saved}</p>}
  {error && <div><p role="alert">{copy.unavailable}</p><button className="text-button" onClick={() => setSelection(value => ({ ...value, retry: value.retry + 1 }))}>{messages.common.retry}</button></div>}
  {result && <>
   {attachment&&history&&history.revisions.length>1&&<label className="document-revision-picker">{copy.revision}<select value={attachmentRevision} disabled={!!draft||busy} onChange={event=>{const selected=history.revisions.find(r=>r.revision===event.target.value);if(selected){setAttachmentRevision(selected.revision);setSelection({revision:selected.extraction_revision,unit:'',offset:0,retry:0,navigation:0});setSaved(false);}}}>{history.revisions.map((r,index)=><option key={r.revision} value={r.revision}>{number(history.revisions.length-index)}{r.created_at?' · '+new Date(r.created_at).toLocaleString(language):''}{r.author?' · '+r.author:''}</option>)}</select></label>}
   <DocumentEvidenceNotice evidence={result.coverage}/>
   {(!result.can_correct || readOnly) && <p role="status">{copy.readOnly}</p>}
   <label>{copy.units}<select value={selection.unit} disabled={!!draft} onChange={event => setSelection(value => ({ ...value, revision: result.revision, unit: event.target.value, offset: 0, navigation:0 }))}>
    <option value="">{copy.all}</option>{result.units.map(unit => <option key={unit.id} value={unit.id}>{unit.name || `${knowledgeUnitLabels[locale][unit.kind as keyof typeof knowledgeUnitLabels[typeof locale]] ?? copy.units} ${number(unit.index)}`}</option>)}
   </select></label>
   {result.units.some(unit => unit.metadata_omitted) && <p className="workspace-secondary">{copy.incomplete}</p>}
   <DocumentStructureNavigation page={result} unit={selection.unit} disabled={!!draft} expanded={expanded} toggle={(kind,value)=>setExpanded(current=>current[kind]===value?current:{...current,[kind]:value})} select={(unit,offset,block)=>{setTargetBlock(block??'');setSelection(value=>({...value,revision:result.revision,unit,offset,navigation:0}));}} paginate={navigation=>setSelection(value=>({...value,revision:result.revision,navigation}))}/>
   <ol className="document-review-blocks" role="list" start={result.offset + 1}>{result.blocks.map(block => <li key={block.id} data-block-id={block.id} data-selected={targetBlock===block.id||undefined} tabIndex={-1}>
    {block.cell&&<p className="document-block-location"><bdi>{result.units.find(unit=>unit.id===block.unit_id)?.name}</bdi> · <bdi>{block.cell.reference}</bdi></p>}
    {block.kind==='figure'&&<p>{navigationCopy.figures}</p>}{((!!block.review_state&&block.review_state!=='corrected'&&block.review_state!=='reviewed')||!!block.warnings?.length)&&<p className="workspace-secondary">{navigationCopy.needsReview}{!!block.warnings?.length&&<> · {number(block.warnings.length)}</>}</p>}
    <pre dir="auto">{highlight && block.text.includes(highlight) ? <>{block.text.slice(0, block.text.indexOf(highlight))}<mark>{highlight}</mark>{block.text.slice(block.text.indexOf(highlight) + highlight.length)}</> : block.text}</pre>
    <DocumentRevisionAlternatives block={block}/>
    {selectBlock&&block.text&&<button className="text-button" onClick={()=>selectBlock(result.revision,block)}>{workSurface(locale).askSelection}</button>}
    {(block.truncated || block.metadata_omitted) && <p className="workspace-secondary">{copy.incomplete}</p>}
    {block.editable && result.can_correct && !readOnly && <button className="text-button" disabled={!!draft} onClick={() => { setSaved(false); setDraft({ block, revision: result.revision, attachmentRevision:attachment?attachmentRevision:undefined, text: block.text, uncertain: false }); }}>{copy.edit}</button>}
   </li>)}</ol>
   <nav aria-label={copy.title}><button className="text-button" disabled={!!draft || result.offset === 0} onClick={() => setSelection(value => ({ ...value, revision: result.revision, offset: Math.max(0, result.offset - 100) }))}>{reader.previous}</button><bdi dir="ltr">{number(result.offset + (result.blocks.length ? 1 : 0))}–{number(result.offset + result.blocks.length)} / {number(result.total_blocks)}</bdi><button className="text-button" disabled={!!draft || result.next_offset === null} onClick={() => setSelection(value => ({ ...value, revision: result.revision, offset: result.next_offset! }))}>{reader.next}</button></nav>
   <details className="document-review-revision"><summary>{copy.revision}</summary><p dir="ltr">{result.revision}</p>{result.author && <p dir="auto">{result.author}</p>}{result.created_at && <time dateTime={result.created_at}>{new Date(result.created_at).toLocaleString(language)}</time>}</details>
  </>}
  {result && draft && <form className="document-correction" onSubmit={event => { event.preventDefault(); void save(); }}>
   <label>{copy.draft}<textarea autoFocus dir="auto" value={draft.text} disabled={busy} readOnly={!canWrite||baseChanged} onChange={event => setDraft({ ...draft, text: event.target.value })}/></label>
   {baseChanged&&<p role="status">{copy.readOnly}</p>}
   {(draft.uncertain||baseChanged||!canWrite) && !busy && <>{draft.uncertain&&<p role="alert">{copy.outcome}</p>}<button type="button" className="text-button" disabled={busy||readOnly} onClick={() => void inspectLatest()}>{copy.load}</button></>}
   {draft.latest && <><h4>{copy.base}</h4><pre dir="auto">{draft.latest.block.text}</pre>{draft.latest.block.text === draft.text ? <p role="status">{copy.same}</p> : <button type="button" className="text-button" disabled={busy||readOnly} onClick={() => {setDraft({ ...draft, block: draft.latest!.block, revision: draft.latest!.revision, attachmentRevision:draft.latest!.attachmentRevision, uncertain: false, latest: undefined });if(attachment)setAttachmentRevision(draft.latest!.attachmentRevision!);setSelection(value=>({...value,revision:draft.latest!.revision,unit:draft.latest!.block.unit_id,offset:0,navigation:0}));}}>{copy.rebase}</button>}</>}
   <div><button type="submit" className="primary-button" disabled={!canWrite || baseChanged || busy || draft.uncertain || draft.text === draft.block.text || new TextEncoder().encode(draft.text).length > 1 << 20}>{busy ? messages.common.loading : copy.save}</button><button type="button" className="text-button" disabled={busy} onClick={() => setDraft(undefined)}>{copy.cancel}</button></div>
  </form>}
 </section>;
}
