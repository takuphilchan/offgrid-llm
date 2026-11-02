import { useEffect, useRef, useState } from 'react';
import { api, type DocumentImport } from '../../api/client';
import { useI18n } from '../../i18n';
import { knowledgePartial } from '../../i18n/knowledge-partial';
import { sourceClose } from '../../i18n/knowledge-original';

export function ImportExtractionDialog({ item, close }: { item: DocumentImport; close: () => void }) {
 const { locale, messages } = useI18n(), copy = knowledgePartial(locale);
 const ref = useRef<HTMLDialogElement>(null);
 const [result, setResult] = useState<Awaited<ReturnType<typeof api.documentImportExtraction>>>();
 const [error, setError] = useState(''), [page, setPage] = useState(0);
 useEffect(() => {
  const controller = new AbortController(), previous = document.activeElement as HTMLElement | null;
  const dialog = ref.current!; dialog.showModal();
  void api.documentImportExtraction(item.id, controller.signal).then(value => {
   if (value.version !== 1 || value.state !== 'partial' || value.source_sha256 !== item.sha256 || !Array.isArray(value.source_units) || !value.source_units.length) throw new Error(messages.common.error);
   if (!controller.signal.aborted) setResult(value);
  }).catch(reason => { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); });
  return () => { controller.abort(); dialog.close(); previous?.focus(); };
 }, [item.id]);
 const unit = result?.source_units[page];
 return <dialog ref={ref} className="document-source-dialog" aria-label={copy.title} onCancel={event => { event.preventDefault(); close(); }}>
  <header><h2><bdi>{item.name}</bdi></h2><button autoFocus className="secondary-button" onClick={close}>{sourceClose[locale]}</button></header>
  <strong>{copy.title}</strong><p>{copy.help}</p>
  {error ? <p role="alert">{error}</p> : !result ? <p role="status">{messages.common.loading}</p> : unit && <>
   <label>{copy.page} <select value={page} onChange={event => setPage(Number(event.target.value))}>{result.source_units.map((u, i) => <option key={u.index} value={i}>{u.index} · {u.state === 'ready' ? copy.ready : u.state === 'empty' ? copy.empty : copy.review}</option>)}</select></label>
   <p role="status">{unit.state === 'ready' ? copy.ready : unit.state === 'empty' ? copy.empty : copy.review}</p>
   <pre tabIndex={0} dir="auto">{unit.text || copy.empty}</pre>
  </>}
 </dialog>;
}
