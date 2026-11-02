import { useEffect, useRef, useState } from 'react';
import { api, type DocumentImport } from '../../api/client';
import { useI18n } from '../../i18n';
import { knowledgeImport } from '../../i18n/knowledge-import';
import { knowledgeRecovery } from '../../i18n/knowledge-recovery';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { openOCRLanguageSetup } from './OCRLanguageSetup';
import { ocrSetup } from '../../i18n/ocr-setup';
import { ImportExtractionDialog } from './ImportExtractionDialog';
import { workflow } from '../../i18n/workflow';
import { documentRecovery } from '../../i18n/document-recovery';
import { documentProgress } from '../../i18n/document-progress';
import { runDocumentImport } from './import-progress';

const active = (item: DocumentImport) => ['queued', 'extracting', 'indexing'].includes(item.state);

export function KnowledgeImportRecovery({ revision, disabled, refresh, ocrSetupNeeded = false }: { revision: number; disabled: boolean; refresh: () => Promise<void>; ocrSetupNeeded?: boolean }) {
  const { locale, messages } = useI18n(), copy = knowledgeRecovery(locale), imports = knowledgeImport(locale);
  const [records, setRecords] = useState<DocumentImport[]>([]), [error, setError] = useState('');
  const [working, setWorking] = useState(''), [removing, setRemoving] = useState<DocumentImport | null>(null);
  const [generation, setGeneration] = useState(0);
  const [review, setReview] = useState<DocumentImport | null>(null);
  const mounted = useRef(true), locked = useRef(false), operation = useRef<AbortController | null>(null);
  const refreshLibrary = useRef(refresh);
  refreshLibrary.current = refresh;
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; operation.current?.abort(); }; }, []);
  useEffect(() => {
    const abort = new AbortController(); let timer: ReturnType<typeof setTimeout> | undefined;
    let pending = new Set<string>();
    const load = async () => {
      try {
        const result = await api.documentImports(abort.signal);
        if (abort.signal.aborted) return;
        const finished = result.imports.some(item => pending.has(item.id) && item.state === 'ready');
        pending = new Set(result.imports.filter(active).map(item => item.id));
        setRecords(result.imports.filter(item => !['ready', 'deleted'].includes(item.state)));
        // A returning user may be observing work started in another view or
        // client. Completion must update the library, not just hide progress.
        // This refresh only reads state; it never replays an import.
        if (finished) await refreshLibrary.current();
        if (!abort.signal.aborted && pending.size) timer = setTimeout(() => void load(), 2000);
      } catch (reason) { if (!abort.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error); }
    };
    void load();
    return () => { abort.abort(); clearTimeout(timer); };
  }, [revision, generation]);

  const run = async (item: DocumentImport, action: 'retry' | 'cancel' | 'remove') => {
    if (locked.current) return;
    locked.current = true; setWorking(item.id); setError('');
    const abort = new AbortController(); operation.current = abort;
    try {
      if (action === 'retry') await runDocumentImport(item.id, abort.signal, state => {
        if (mounted.current) setRecords(items => items.map(saved => saved.id === item.id ? { ...saved, state } : saved));
      });
      else if (action === 'cancel') await api.cancelDocumentImport(item.id);
      else await api.removeDocumentImport(item.id);
      if (mounted.current) await refresh();
    } catch (reason) {
      if (action === 'remove') throw reason; // Keep the confirmation open with its actionable error.
      if (mounted.current && !abort.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error);
    }
    finally { locked.current = false; operation.current = null; if (mounted.current) { setWorking(''); setGeneration(value => value + 1); } }
  };
  if (!records.length && !error) return null;
  return <section className="knowledge-import-recovery" aria-label={copy.title}>
    {review && <ImportExtractionDialog item={review} close={() => setReview(null)} />}
    {removing && <ConfirmDialog title={removing.name} body={copy.removeHelp} close={() => setRemoving(null)} confirm={async () => { await run(removing, 'remove'); }} />}
    <div className="knowledge-import-heading"><h3>{copy.title}</h3><button type="button" className="text-button" onClick={() => { setError(''); setGeneration(value => value + 1); }}>{messages.common.refresh}</button></div>
    <p className="workspace-secondary">{copy.body}</p>
    {error && <p role="alert">{error}</p>}
    <ul>{records.map(item => <li key={item.id}>
      <div><strong><bdi>{item.name}</bdi></strong><span>{documentProgress(locale, item.state) ?? (working === item.id ? imports.importing : item.state === 'interrupted' ? copy.interrupted : imports.failed)}</span>
        {documentRecovery(locale, item.error_code) && <p>{documentRecovery(locale, item.error_code)}</p>}
        {ocrSetupNeeded && ['document_ocr_language_missing', 'document_ocr_language_invalid'].includes(item.error_code ?? '') && <button type="button" className="text-button" onClick={openOCRLanguageSetup}>{ocrSetup(locale).title}</button>}
      </div>
      <div className="knowledge-recovery-actions">
        {item.review_available && <button type="button" className="text-button" onClick={() => setReview(item)}>{workflow[locale].viewSource}</button>}
        {!active(item) && <button type="button" className="secondary-button" disabled={disabled || !!working} onClick={() => void run(item, 'retry')}>{messages.common.retry}</button>}
        {(active(item) || working === item.id) && <button type="button" className="text-button" onClick={() => { if (working === item.id) operation.current?.abort(); else void run(item, 'cancel'); }}>{messages.models.cancel}</button>}
        <button type="button" className="text-button" disabled={!!working} onClick={() => setRemoving(item)}>{copy.remove}</button>
      </div>
    </li>)}</ul>
  </section>;
}
