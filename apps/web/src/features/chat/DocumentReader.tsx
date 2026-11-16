import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { api, type DocumentPreview, type DocumentStructurePage } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { documentReader } from '../../i18n/document-reader';
import type { ReaderSource } from './DocumentWorkspace';
import { knowledgeOriginal } from '../../i18n/knowledge-original';
import { knowledgeScope } from '../../i18n/knowledge-scope';
import { knowledgeUnitLabels } from '../../i18n/knowledge-locators';
import { DocumentPageInput } from './DocumentPageInput';
import { ReaderCoverage } from './ReaderCoverage';
import { DocumentReview } from './DocumentReview';
import { documentReview } from '../../i18n/document-review';
import {documentNavigation} from '../../i18n/document-navigation';
import {rememberReader,useReaderMemory,type ReaderPosition} from '../../lib/reader-memory';
import {sourceRegions} from './document-regions';
import './document-reader.css';
import './document-workspace-layout.css';
import {DocumentThumbnails} from './DocumentThumbnails';
import {workSurface} from '../../i18n/work-surface';
import {UtilityPopover} from '../../components/UtilityPopover';
import {DocumentFileIcon} from './DocumentFileIcon';
import {documentChrome} from '../../i18n/document-chrome';
import {useFocusScope} from '../../lib/focus-scope';

export function DocumentReader({ source, close, actions, tabs, selectPage, selectBlock }: { source: ReaderSource; close: () => void; actions?: ReactNode; tabs?:ReactNode; selectBlock?: (revision:string,block:import('../../api/client').DocumentBlock)=>void; selectPage?: (page:number)=>void }) {
 const { locale, messages } = useI18n(), copy = documentReader(locale);
 const [preview, setPreview] = useState<DocumentPreview>();
 const memory=useReaderMemory(),memoryKey=useRef(''),restoreScroll=useRef<{top:number;left:number}|undefined>(undefined);
 const position=useRef<ReaderPosition>({page:1,zoom:100,query:'',textView:false,compare:false,top:0,left:0});
 const [geometry,setGeometry]=useState<DocumentStructurePage>();
 const [error, setError] = useState(''), [pageError, setPageError] = useState('');
 const [page, setPage] = useState(source.citation?.index ?? 1), [zoom, setZoom] = useState(100);
 const [query, setQuery] = useState(''), [textView, setTextView] = useState(!!(source.verifiedText || source.passage));
 const [passagePage, setPassagePage] = useState<number>();
 const [compare,setCompare]=useState(false);
 const [expanded,setExpanded]=useState(false);
 const [searchOpen,setSearchOpen]=useState(false);
 const [pixels, setPixels] = useState(''), [loadingPage, setLoadingPage] = useState(false);
 const [rasterKey,setRasterKey]=useState('');
 const [rasterVerified,setRasterVerified]=useState(false),[frameVerified,setFrameVerified]=useState(false);
 const [retryPage, setRetryPage] = useState(0);
 const [review, setReview] = useState(false), [reviewOpened, setReviewOpened] = useState(false);
 const [downloadError,setDownloadError]=useState(''),[downloading,setDownloading]=useState(false);
 const downloadLock=useRef(false);
 const closeButton = useRef<HTMLButtonElement>(null), surface = useRef<HTMLDivElement>(null);
 const readerRoot=useRef<HTMLElement>(null),header=useRef<HTMLElement>(null);
 const restoreView=()=>{setExpanded(false);requestAnimationFrame(()=>header.current?.querySelector<HTMLButtonElement>('.document-expand')?.focus());};
 useFocusScope(readerRoot,expanded,restoreView);
 const pinnedPath=source.attachment?`${source.path}${source.path.includes('?')?'&':'?'}revision=${encodeURIComponent(source.attachment.revision)}`:source.path;
 const pagePath = (sha: string) => `${pinnedPath}${pinnedPath.includes('?') ? '&' : '?'}sha256=${encodeURIComponent(sha)}`;
 useEffect(() => {
  const controller = new AbortController(); (closeButton.current??header.current?.querySelector<HTMLElement>('[role=tab][aria-selected=true]'))?.focus();
  memoryKey.current='';setPreview(undefined);setError('');setGeometry(undefined);
  void api.documentPreview(source.sha256 ? pagePath(source.sha256) : pinnedPath, controller.signal).then(result => {
   if (controller.signal.aborted) return;
   if (source.sha256 && result.sha256 !== source.sha256) throw Error(messages.common.error);
   if (source.citation && (result.sha256 !== source.citation.source_sha256 || !result.units.some(unit => unit.index === source.citation?.index && unit.kind === source.citation?.kind))) throw Error(messages.common.error);
   const key=JSON.stringify([source.path,result.sha256,source.extractionRevision,source.attachment?.revision]);
   const saved=memory?.get(key),targeted=!!(source.citation||source.passage||source.verifiedText);
   memoryKey.current=key;
   setPreview(result); setPage(source.citation ? Math.max(1,result.units.findIndex(unit=>unit.index===source.citation?.index&&unit.kind===source.citation?.kind)+1) : saved&&!targeted?Math.max(1,Math.min(result.units.length,saved.page)):1);
   setZoom(saved?.zoom??100);setQuery(targeted?'':saved?.query??'');
   setTextView(targeted?true:saved?.textView??false);setCompare(targeted?false:saved?.compare??false);
   restoreScroll.current=saved&&!targeted?{top:saved.top,left:saved.left}:undefined;
   if (source.passage) {
    // Only one exact occurrence in one native unit establishes a page location.
    // Cross-page passages and repeated text remain a verified excerpt, not a guessed page.
    const matches = result.units.flatMap((unit, index) => unit.text.includes(source.passage!) ? [{unit,index}] : []);
    if (matches.length === 1 && matches[0].unit.text.indexOf(source.passage) === matches[0].unit.text.lastIndexOf(source.passage)) { setPassagePage(matches[0].index+1); setPage(matches[0].index+1); }
   }
  }).catch(reason => { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); });
  return () => controller.abort();
 }, [source]);
 const unit = preview?.units[page - 1];
 const expectedRasterKey=preview?`${preview.sha256}:${unit?.index??page}`:'';
 const hasOriginalPages = !!preview && ['pdf','png','jpg','jpeg','tif','tiff'].includes(preview.format);
 const unitLabel = knowledgeUnitLabels[locale][unit?.kind as keyof typeof knowledgeUnitLabels[typeof locale]] ?? copy.original;
 useEffect(() => {
  setPixels('');setRasterKey(''); setPageError(''); setLoadingPage(false);setRasterVerified(false);setFrameVerified(false);
  if (!preview || !hasOriginalPages || textView&&!compare) return;
  const controller = new AbortController(); let url = ''; setLoadingPage(true);
  void api.documentPage(`${pagePath(preview.sha256)}&page=${unit?.index ?? page}`, controller.signal).then(result => {
   if (!controller.signal.aborted) { url = URL.createObjectURL(result.blob); setPixels(url);setRasterKey(expectedRasterKey);setRasterVerified(result.sourceVerified); }
  }).catch(reason => { if (!controller.signal.aborted) setPageError(reason instanceof Error ? reason.message : messages.common.error); })
   .finally(() => { if (!controller.signal.aborted) setLoadingPage(false); });
  return () => { controller.abort(); if (url) URL.revokeObjectURL(url); };
 }, [preview, page, textView, compare, retryPage]);
 const search=query.trim();
 const matches = useMemo(() => search ? preview?.units.filter(item => item.text.toLocaleLowerCase().includes(search.toLocaleLowerCase())) ?? [] : [], [search, preview]);
 const text = unit?.text ?? '';
 const highlight = source.verifiedText || (page === passagePage ? source.passage : '') || search;
 const wantsRegions=!!highlight&&(!textView||compare);
 // Geometry is optional. Retrieve only bounded, revision-pinned structure for
 // the current page; never guess coordinates from text length or model output.
 useEffect(()=>{
  setGeometry(undefined);
  if(!wantsRegions||!preview||!unit?.image_geometry||!source.extractionRevision||(!source.documentId&&!source.attachment))return;
  const controller=new AbortController();
  const load=(unitId:string,offset:number)=>source.attachment?api.sessionAttachmentStructure(source.attachment.session,source.attachment.id,source.attachment.revision,source.extractionRevision!,unitId,offset,controller.signal):api.documentStructure(source.documentId!,source.extractionRevision!,unitId,offset,controller.signal);
  void (async()=>{
   const first=await load('',0);if(first.original_sha256!==preview.sha256)return;
   const selected=first.units.find(value=>value.kind===unit.kind&&value.index===unit.index);if(!selected)return;
   let part=await load(selected.id,0);const blocks=[...part.blocks];
   for(let n=1;part.next_offset!==null&&n<20;n++){part=await load(selected.id,part.next_offset);if(part.original_sha256!==preview.sha256)return;blocks.push(...part.blocks);}
   if(!controller.signal.aborted&&part.original_sha256===preview.sha256)setGeometry({...part,blocks});
  })().catch(()=>{/* Original/text inspection remains usable without optional geometry. */});
  return ()=>controller.abort();
 },[wantsRegions,preview,page,source.extractionRevision,source.documentId,source.attachment?.id,source.attachment?.revision]);
 const regions=useMemo(()=>preview&&geometry?sourceRegions(preview,page,geometry,highlight):[],[preview,page,geometry,highlight]);
 // Exact citation text must never be mapped through case folding: some Unicode
 // characters change length when folded, which would highlight different bytes.
 const folded = text.toLocaleLowerCase(), foldedHighlight = highlight.toLocaleLowerCase();
 const offset = !highlight ? -1 : source.verifiedText || source.passage && page === passagePage ? text.indexOf(highlight)
  : folded.length === text.length && foldedHighlight.length === highlight.length ? folded.indexOf(foldedHighlight) : text.indexOf(highlight);
 const restorePosition=()=>{const saved=restoreScroll.current;if(saved&&surface.current)surface.current.scrollTo(saved.left,saved.top);};
 useEffect(() => { if(restoreScroll.current)restorePosition();else surface.current?.scrollTo(0, 0); }, [page, textView,compare,preview]);
 useEffect(()=>{
  position.current={page,zoom,query,textView,compare,top:restoreScroll.current?.top??surface.current?.scrollTop??0,left:restoreScroll.current?.left??surface.current?.scrollLeft??0};
  if(preview)rememberReader(memory,memoryKey.current,position.current);
 },[page,zoom,query,textView,compare,preview,memory]);
 const scrollPosition=()=>{
  // An image may load after the initial layout. Keep the pending position until
  // it can be restored; pointer/keyboard input cancels it immediately.
  if(restoreScroll.current)return;
  position.current={...position.current,top:surface.current?.scrollTop??0,left:surface.current?.scrollLeft??0};
  rememberReader(memory,memoryKey.current,position.current);
 };
 const download = async () => {
  if(downloadLock.current)return;
  downloadLock.current=true;setDownloading(true);setDownloadError('');
  try { const blob = await source.original(); const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = source.name; link.click(); setTimeout(() => URL.revokeObjectURL(url), 30_000); }
  catch (reason) { setDownloadError(reason instanceof Error ? reason.message : messages.common.error); }
  finally { downloadLock.current=false;setDownloading(false); }
 };
 return <aside ref={readerRoot} className="document-reader" data-expanded={expanded} role={expanded?'dialog':undefined} aria-modal={expanded||undefined} aria-label={copy.document} onPointerDownCapture={()=>{restoreScroll.current=undefined;}} onKeyDown={event => {
  restoreScroll.current=undefined;
  if((event.target as HTMLElement).closest('[popover]')||event.currentTarget.querySelector('[popover]:popover-open'))return;
  if (event.key === 'Escape') { event.stopPropagation(); if(expanded)restoreView();else close(); }
  if ((event.target as HTMLElement).closest('input,textarea,select,[contenteditable=true]') || event.altKey || event.ctrlKey || event.metaKey) return;
  if (preview && ['PageUp','PageDown'].includes(event.key)) { event.preventDefault(); setPage(value => Math.max(1,Math.min(preview.units.length,value+(event.key==='PageDown'?1:-1)))); }
 }}>
  <header ref={header}>{tabs ?? <div className="document-file-tab"><DocumentFileIcon name={source.name}/><div className="document-heading"><h2 dir="auto" title={source.name}>{source.name}</h2></div>
   <button ref={closeButton} className="icon-button" aria-label={copy.close} onClick={close}><Icon name="close" size={16}/></button></div>}<div className="document-header-actions"><button type="button" className="icon-button document-expand" aria-label={expanded?documentChrome(locale).restore:documentChrome(locale).expand} title={expanded?documentChrome(locale).restore:documentChrome(locale).expand} aria-pressed={expanded} onClick={()=>setExpanded(value=>!value)}><Icon name={expanded?'contract':'expand'} size={17}/></button>{actions}</div></header>
  {downloadError&&<p className="document-download-error" role="alert">{downloadError}</p>}
  {review&&<button className="text-button document-review-back" onClick={()=>setReview(false)}><Icon name="chevron-left" size={16}/>{copy.document}</button>}
  {reviewOpened && (source.documentId || source.attachment) && <div className="document-review-scroll" hidden={!review}><DocumentReview key={(source.documentId ?? source.attachment!.id) + source.extractionRevision} id={source.documentId ?? source.attachment!.id} revision={source.extractionRevision} attachment={source.attachment} highlight={source.verifiedText || source.passage} selectBlock={selectBlock}/></div>}
  <div className="document-reader-content" hidden={review}>
  {!preview && !error && <div className="document-scan" role="status">{messages.common.loading}</div>}
  {error && <p role="alert">{error}</p>}
  {preview && <>
   <div className="document-reader-tools">
    <div className="document-page-tools">
     <DocumentThumbnails key={preview.sha256} name={source.name} preview={preview} path={pinnedPath} page={page} select={setPage}/>
     {selectPage && (source.attachment||source.documentId) && unit?.kind==='page' && <button type="button" className="document-ask-page" aria-label={workSurface(locale).askPage} title={workSurface(locale).askPage} onClick={()=>selectPage(unit.index)}><Icon name="chat" size={17}/><span>{workSurface(locale).askPage}</span></button>}
     <span className="document-toolbar-spacer"/>
     <select className="document-zoom" aria-label={copy.zoom} value={zoom} onChange={event => setZoom(Number(event.target.value))}>{[75,100,125,150,200].map(value => <option key={value} value={value}>{value}%</option>)}</select>
     <button className="icon-button document-download" disabled={downloading} aria-busy={downloading} title={knowledgeOriginal(locale).download} aria-label={knowledgeOriginal(locale).download} onClick={() => void download()}><Icon name="download" size={18}/></button>
     <UtilityPopover label={copy.document} className="document-options" trigger={<Icon name="more" size={18}/>} triggerClassName="icon-button" align="end" panelClassName="document-options-panel">
      <button type="button" className="utility-button document-search-toggle" aria-expanded={searchOpen||!!query} data-popover-dismiss onClick={()=>{if(searchOpen||query){setSearchOpen(false);setQuery('');}else {setSearchOpen(true);requestAnimationFrame(()=>surface.current?.closest('.document-reader')?.querySelector<HTMLInputElement>('.document-search input')?.focus());}}}><Icon name="search" size={17}/>{copy.search}</button>
      {hasOriginalPages && <label className="document-view-choice"><span>{copy.document}</span><select className="document-view-select" aria-label={copy.document} value={compare?'compare':textView?'text':'original'} onChange={event=>{setCompare(event.target.value==='compare');setTextView(event.target.value==='text');event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover();surface.current?.closest('.document-reader')?.querySelector<HTMLButtonElement>('.document-options > button')?.focus();}}><option value="original">{copy.original}</option><option value="text">{copy.extracted}</option><option value="compare">{documentNavigation(locale).sideBySide}</option></select></label>}
      {(source.documentId||source.attachment)&&source.extractionRevision&&<button type="button" className="utility-button" data-popover-dismiss onClick={()=>{setReview(true);setReviewOpened(true);}}><Icon name="file-search" size={17}/>{documentReview(locale).title}</button>}
     </UtilityPopover>
    </div>
   </div>
   {(searchOpen||!!query)&&<label className="document-search"><Icon name="search" size={16}/><input autoFocus={searchOpen} type="search" aria-label={copy.search} placeholder={copy.search} value={query} onChange={event=>setQuery(event.target.value)} onKeyDown={event=>{if(event.key==='Escape'){event.stopPropagation();setQuery('');setSearchOpen(false);surface.current?.closest('.document-reader')?.querySelector<HTMLButtonElement>('.document-options > button')?.focus();}}}/></label>}
   {search && <div className="document-search-results" aria-label={copy.search}>{matches.length ? matches.map(item => <button className="text-button" key={item.index} aria-current={unit===item?'page':undefined} onClick={() => { setPage(preview.units.indexOf(item) + 1); setTextView(true); }}>{item.name || `${knowledgeUnitLabels[locale][item.kind as keyof typeof knowledgeUnitLabels[typeof locale]] ?? unitLabel} ${item.index}`}</button>) : <span role="status">{copy.noMatch}</span>}</div>}
   {pageError && <div className="document-page-error"><p role="alert">{pageError}</p><button className="icon-button" aria-label={messages.common.retry} title={messages.common.retry} onClick={() => setRetryPage(value=>value+1)}><Icon name="refresh" size={18}/></button></div>}
   {source.passage && !passagePage && <section className="document-verified-passage"><h3>{knowledgeScope[locale].passages}</h3><blockquote dir="auto">{source.passage}</blockquote></section>}
   {source.verifiedText && unit?.index === source.citation?.index && !text.includes(source.verifiedText) && <section className="document-verified-passage"><h3>{knowledgeScope[locale].passages}</h3><blockquote dir="auto">{source.verifiedText}</blockquote></section>}
   <div ref={surface} className={`document-reader-surface${compare?' document-compare':''}`} tabIndex={0} aria-busy={loadingPage} onScroll={scrollPosition} onWheel={()=>{restoreScroll.current=undefined;}} onPointerDown={()=>{restoreScroll.current=undefined;}} onKeyDown={()=>{restoreScroll.current=undefined;}}>
    {loadingPage && <div className="document-scan" role="status">{messages.common.loading}</div>}
    {hasOriginalPages && (!textView||compare) && pixels && rasterKey===expectedRasterKey && <div className="document-original-pane"><div className="document-page-image" style={{width:`${zoom}%`}}><img src={pixels} alt={`${source.name} · ${page}`} onLoad={event=>{restorePosition();const g=unit?.image_geometry;setFrameVerified(!!g&&event.currentTarget.naturalWidth===(g.orientation>=5?g.height:g.width)&&event.currentTarget.naturalHeight===(g.orientation>=5?g.width:g.height));}}/>{rasterVerified&&frameVerified&&regions.length>0&&<svg className="document-source-regions" viewBox="0 0 1 1" preserveAspectRatio="none" aria-hidden="true">{regions.map(region=><rect key={region.id} x={region.x} y={region.y} width={region.width} height={region.height}/>)}</svg>}</div></div>}{(!hasOriginalPages||textView||compare)&&<pre dir="auto" style={{ fontSize: `${zoom}%` }}>{offset >= 0 ? <>{text.slice(0, offset)}<mark>{text.slice(offset, offset + highlight.length)}</mark>{text.slice(offset + highlight.length)}</> : text}</pre>}
   </div>
   <footer className="document-reader-footer">
    <ReaderCoverage preview={preview} page={page} original={hasOriginalPages} select={value=>{setPage(value);setTextView(false);}}/>
    <div className="document-page-navigation">
     <button className="icon-button" aria-label={copy.previous} title={`${copy.previous} · Page Up`} disabled={page <= 1} onClick={() => setPage(page - 1)}><Icon name="arrow-up" size={16}/></button>
     <DocumentPageInput page={page} count={preview.units.length} label={unitLabel} change={setPage}/>
     <button className="icon-button" aria-label={copy.next} title={`${copy.next} · Page Down`} disabled={page >= preview.units.length} onClick={() => setPage(page + 1)}><Icon name="arrow-down" size={16}/></button>
    </div>
   </footer>
  </>}
  </div>
 </aside>;
}
