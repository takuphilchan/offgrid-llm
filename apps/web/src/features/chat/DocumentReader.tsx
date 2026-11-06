import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type DocumentPreview } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { documentReader } from '../../i18n/document-reader';
import type { ReaderSource } from './DocumentWorkspace';
import { knowledgeOriginal } from '../../i18n/knowledge-original';
import { knowledgeScope } from '../../i18n/knowledge-scope';
import { knowledgeUnitLabels } from '../../i18n/knowledge-locators';
import { DocumentPageInput } from './DocumentPageInput';

export function DocumentReader({ source, close }: { source: ReaderSource; close: () => void }) {
 const { locale, messages } = useI18n(), copy = documentReader(locale);
 const [preview, setPreview] = useState<DocumentPreview>();
 const [error, setError] = useState(''), [pageError, setPageError] = useState('');
 const [page, setPage] = useState(source.citation?.index ?? 1), [zoom, setZoom] = useState(100);
 const [query, setQuery] = useState(''), [textView, setTextView] = useState(!!(source.verifiedText || source.passage));
 const [passagePage, setPassagePage] = useState<number>();
 const [pixels, setPixels] = useState(''), [loadingPage, setLoadingPage] = useState(false);
 const [retryPage, setRetryPage] = useState(0);
 const [downloadError,setDownloadError]=useState(''),[downloading,setDownloading]=useState(false);
 const downloadLock=useRef(false);
 const closeButton = useRef<HTMLButtonElement>(null), surface = useRef<HTMLDivElement>(null);
 const pagePath = (sha: string) => `${source.path}${source.path.includes('?') ? '&' : '?'}sha256=${encodeURIComponent(sha)}`;
 useEffect(() => {
  const controller = new AbortController(); closeButton.current?.focus();
  void api.documentPreview(source.sha256 ? pagePath(source.sha256) : source.path, controller.signal).then(result => {
   if (controller.signal.aborted) return;
   if (source.sha256 && result.sha256 !== source.sha256) throw Error(messages.common.error);
   if (source.citation && (result.sha256 !== source.citation.source_sha256 || !result.units.some(unit => unit.index === source.citation?.index && unit.kind === source.citation?.kind))) throw Error(messages.common.error);
   setPreview(result); setPage(source.citation ? Math.max(1,result.units.findIndex(unit=>unit.index===source.citation?.index&&unit.kind===source.citation?.kind)+1) : 1);
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
 const unitLabel = knowledgeUnitLabels[locale][unit?.kind as keyof typeof knowledgeUnitLabels[typeof locale]] ?? copy.original;
 useEffect(() => {
  setPixels(''); setPageError(''); setLoadingPage(false);
  if (!preview || preview.format !== 'pdf' || textView) return;
  const controller = new AbortController(); let url = ''; setLoadingPage(true);
  void api.documentPage(`${pagePath(preview.sha256)}&page=${unit?.index ?? page}`, controller.signal).then(blob => {
   if (!controller.signal.aborted) { url = URL.createObjectURL(blob); setPixels(url); }
  }).catch(reason => { if (!controller.signal.aborted) setPageError(reason instanceof Error ? reason.message : messages.common.error); })
   .finally(() => { if (!controller.signal.aborted) setLoadingPage(false); });
  return () => { controller.abort(); if (url) URL.revokeObjectURL(url); };
 }, [preview, page, textView, retryPage]);
 const search=query.trim();
 const matches = useMemo(() => search ? preview?.units.filter(item => item.text.toLocaleLowerCase().includes(search.toLocaleLowerCase())) ?? [] : [], [search, preview]);
 const text = unit?.text ?? '';
 const highlight = source.verifiedText || (page === passagePage ? source.passage : '') || search;
 // Exact citation text must never be mapped through case folding: some Unicode
 // characters change length when folded, which would highlight different bytes.
 const folded = text.toLocaleLowerCase(), foldedHighlight = highlight.toLocaleLowerCase();
 const offset = !highlight ? -1 : source.verifiedText || source.passage && page === passagePage ? text.indexOf(highlight)
  : folded.length === text.length && foldedHighlight.length === highlight.length ? folded.indexOf(foldedHighlight) : text.indexOf(highlight);
 useEffect(() => { surface.current?.scrollTo(0, 0); }, [page, textView]);
 const download = async () => {
  if(downloadLock.current)return;
  downloadLock.current=true;setDownloading(true);setDownloadError('');
  try { const blob = await source.original(); const url = URL.createObjectURL(blob); const link = document.createElement('a'); link.href = url; link.download = source.name; link.click(); setTimeout(() => URL.revokeObjectURL(url), 30_000); }
  catch (reason) { setDownloadError(reason instanceof Error ? reason.message : messages.common.error); }
  finally { downloadLock.current=false;setDownloading(false); }
 };
 return <aside className="document-reader" aria-label={copy.document} onKeyDown={event => {
  if (event.key === 'Escape') { event.stopPropagation(); close(); }
  if ((event.target as HTMLElement).closest('input,textarea,select,[contenteditable=true]') || event.altKey || event.ctrlKey || event.metaKey) return;
  if (preview && ['PageUp','PageDown'].includes(event.key)) { event.preventDefault(); setPage(value => Math.max(1,Math.min(preview.units.length,value+(event.key==='PageDown'?1:-1)))); }
 }}>
  <header><span className="document-file-mark"><Icon name="file" size={22}/></span><div className="document-heading"><h2 dir="auto" title={source.name}>{source.name}</h2><span className="workspace-secondary">{preview?.format.toUpperCase() || copy.document}</span></div><div className="document-header-actions">
   <button className="icon-button document-download" disabled={downloading} aria-busy={downloading} title={knowledgeOriginal(locale).download} aria-label={knowledgeOriginal(locale).download} onClick={() => void download()}><Icon name="download" size={18}/></button>
   <button ref={closeButton} className="icon-button" aria-label={copy.close} onClick={close}><Icon name="close" size={18}/></button></div></header>
  {downloadError&&<p className="document-download-error" role="alert">{downloadError}</p>}
  {!preview && !error && <div className="document-scan" role="status">{messages.common.loading}</div>}
  {error && <p role="alert">{error}</p>}
  {preview && <>
   <div className="document-reader-tools">
    <label className="document-search"><Icon name="search" size={16}/><input type="search" aria-label={copy.search} placeholder={copy.search} value={query} onChange={event => setQuery(event.target.value)} /></label>
    <div className="document-page-tools">
     <div className="document-page-navigation">
     <button className="icon-button" aria-label={copy.previous} title={`${copy.previous} · Page Up`} disabled={page <= 1} onClick={() => setPage(page - 1)}><Icon name="arrow-up" size={17}/></button>
     <DocumentPageInput page={page} count={preview.units.length} label={unitLabel} change={setPage}/>
     <button className="icon-button" aria-label={copy.next} title={`${copy.next} · Page Down`} disabled={page >= preview.units.length} onClick={() => setPage(page + 1)}><Icon name="arrow-down" size={17}/></button>
     </div>
     <select aria-label={copy.zoom} value={zoom} onChange={event => setZoom(Number(event.target.value))}>{[75,100,125,150,200].map(value => <option key={value} value={value}>{value}%</option>)}</select>
     {preview.format === 'pdf' && <div className="document-view-mode" role="group" aria-label={copy.document}><button className="text-button" aria-pressed={!textView} onClick={() => setTextView(false)}>{copy.original}</button><button className="text-button" aria-pressed={textView} onClick={() => setTextView(true)}>{copy.extracted}</button></div>}
    </div>
   </div>
   {preview.state === 'partial' && <p className="document-coverage" role="status">{copy.partial}</p>}
   {preview.state === 'partial' && <nav className="document-coverage-pages" aria-label={copy.partial}>{preview.units.filter(item => !['ready','empty'].includes(item.state)).map(item => <button key={item.index} type="button" className="text-button" aria-current={unit?.index === item.index ? 'page' : undefined} onClick={() => {setPage(preview.units.indexOf(item)+1);setTextView(false);}}>{knowledgeUnitLabels[locale].page} {item.index}</button>)}</nav>}
   {preview.format !== 'pdf' && <p className="document-coverage">{copy.structured}</p>}
   {search && <div className="document-search-results" aria-label={copy.search}>{matches.length ? matches.map(item => <button className="text-button" key={item.index} aria-current={unit===item?'page':undefined} onClick={() => { setPage(preview.units.indexOf(item) + 1); setTextView(true); }}>{item.name || `${knowledgeUnitLabels[locale][item.kind as keyof typeof knowledgeUnitLabels[typeof locale]] ?? unitLabel} ${item.index}`}</button>) : <span role="status">{copy.noMatch}</span>}</div>}
   {pageError && <div className="document-page-error"><p role="alert">{pageError}</p><button className="icon-button" aria-label={messages.common.retry} title={messages.common.retry} onClick={() => setRetryPage(value=>value+1)}><Icon name="refresh" size={18}/></button></div>}
   {source.passage && !passagePage && <section className="document-verified-passage"><h3>{knowledgeScope[locale].passages}</h3><blockquote dir="auto">{source.passage}</blockquote></section>}
   {source.verifiedText && unit?.index === source.citation?.index && !text.includes(source.verifiedText) && <section className="document-verified-passage"><h3>{knowledgeScope[locale].passages}</h3><blockquote dir="auto">{source.verifiedText}</blockquote></section>}
   <div ref={surface} className="document-reader-surface" tabIndex={0} aria-busy={loadingPage}>
    {loadingPage && <div className="document-scan" role="status">{messages.common.loading}</div>}
    {preview.format === 'pdf' && !textView ? pixels && <img src={pixels} alt={`${source.name} · ${page}`} style={{ width: `${zoom}%` }} /> : <pre dir="auto" style={{ fontSize: `${zoom}%` }}>{offset >= 0 ? <>{text.slice(0, offset)}<mark>{text.slice(offset, offset + highlight.length)}</mark>{text.slice(offset + highlight.length)}</> : text}</pre>}
   </div>
  </>}
 </aside>;
}
