import { useEffect, useRef, useState } from 'react';
import { api, APIError, type RAGStatus } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { knowledgeImport } from '../../i18n/knowledge-import';
import './knowledge-import.css';
import { KnowledgeImportRecovery } from './KnowledgeImportRecovery';
import { OCRLanguageSetup } from './OCRLanguageSetup';
import { documentRecovery } from '../../i18n/document-recovery';
import { documentProgress } from '../../i18n/document-progress';
import { runDocumentImport } from './import-progress';
import { knowledgeOptions } from '../../i18n/knowledge-options';

type Result = { name: string; state: 'pending' | 'importing' | 'added' | 'failed' | 'unknown'; detail?: string };
type Props = { status: RAGStatus | null; disabled: boolean; setupDisabled?: boolean; refresh: () => Promise<void>; durable?: boolean };

// These are acknowledgments for this selection, not a second durable job store.
// A lost response is never retried automatically: the service may have committed.
export function KnowledgeImport({ status, disabled, setupDisabled = disabled, refresh, durable = false }: Props) {
  const { locale, messages } = useI18n();
  const copy = knowledgeImport(locale);
  const input = useRef<HTMLInputElement>(null);
  const locked = useRef(false);
  const blocked = useRef(disabled);
  blocked.current = disabled;
  const mounted = useRef(true);
  const operation = useRef<AbortController | null>(null);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; operation.current?.abort(); }; }, []);
  const [busy, setBusy] = useState(false);
  const [results, setResults] = useState<Result[]>([]);
  const [notice, setNotice] = useState('');
  const [revision, setRevision] = useState(0);
  const formats = status?.document_formats;
  const accept = formats?.filter(format => ['available', 'installed_unchecked'].includes(format.runtime_state)).flatMap(format => format.extensions).join(',');

  const add = async (files: File[]) => {
    if (disabled || locked.current || !files.length) return;
    if (files.length > 20) { setNotice(copy.batchLimit); return; }
    locked.current = true;
    const abort = new AbortController(); operation.current = abort;
    setBusy(true); setNotice('');
    setResults(files.map(file => ({ name: file.name, state: 'pending' })));
    const update = (index: number, state: Result['state'], detail?: string) => setResults(items => items.map((item, i) => i === index ? { ...item, state, detail } : item));
    try {
      for (let index = 0; index < files.length; index++) {
        if (!mounted.current || blocked.current || abort.signal.aborted) break;
        const file = files[index];
        const suffix = file.name.slice(file.name.lastIndexOf('.')).toLowerCase();
        const format = formats?.find(item => item.extensions.includes(suffix));
        if (formats && (!format || !['available', 'installed_unchecked'].includes(format.runtime_state))) {
          update(index, 'failed', copy.missing); continue;
        }
        if (file.size > (format?.max_file_bytes ?? 50 * 1024 * 1024)) {
          update(index, 'failed', copy.sizeLimit); continue;
        }
        update(index, 'importing');
        try {
          if (durable) {
            const accepted = await api.acceptDocumentImport(file, crypto.randomUUID(), abort.signal);
            if (!accepted.id) { update(index, 'unknown', copy.unknown); break; }
            // Navigation/cancellation after acceptance must not start extraction.
            if (!mounted.current || abort.signal.aborted) { update(index, 'unknown', copy.unknown); break; }
            const completed = await runDocumentImport(accepted.id, abort.signal, state => {
              if (mounted.current) update(index, 'importing', documentProgress(locale, state));
            });
            if (completed.state !== 'ready' || !completed.document_id) { update(index, 'unknown', copy.unknown); break; }
            update(index, 'added'); continue;
          }
          const response = await api.ingest(file);
          if (!response.success || !response.document?.id) {
            update(index, 'unknown', copy.unknown); break;
          }
          // A legacy acknowledgment may omit readiness. Never label that indexed.
          if (response.document.index_status !== 'ready') {
            update(index, 'unknown', copy.unknown); break;
          }
          update(index, 'added');
        } catch (reason) {
          const code = reason instanceof APIError ? reason.data?.error_code ?? reason.data?.error?.code : undefined;
          // Only explicit pre-extraction/input rejection proves no publication.
          const rejected = ['document_invalid', 'document_empty', 'document_unsupported', 'document_unsupported_format', 'document_encrypted', 'document_limit', 'document_storage_limit', 'document_parser_unavailable', 'document_ocr_required', 'document_ocr_language_missing', 'document_ocr_language_invalid', 'document_ocr_language_storage'].includes(code);
          if (code === 'document_busy') { update(index, 'failed', documentRecovery(locale, code)); break; }
          if (!rejected) { update(index, 'unknown', copy.unknown); break; }
          update(index, 'failed', documentRecovery(locale, code) ?? (reason instanceof Error ? reason.message : copy.failed));
        }
      }
      if (mounted.current) await refresh();
    } finally { locked.current = false; operation.current = null; if (mounted.current) { setBusy(false); setRevision(value => value + 1); } }
  };

  return <section className="knowledge-import" aria-label={messages.knowledge.add}>
    {(status?.enabled || status === null) && <div className="knowledge-import-actions">
      <input hidden ref={input} type="file" multiple disabled={disabled || busy} accept={accept} data-knowledge-import onChange={event => { const files = [...event.target.files ?? []]; event.target.value = ''; void add(files); }} />
      <button type="button" className="primary-button" disabled={disabled || busy} onClick={() => input.current?.click()}><Icon name="upload" size={17} />{busy ? copy.importing : messages.knowledge.add}</button>
      {durable && busy && <button type="button" className="text-button" onClick={() => operation.current?.abort()}>{messages.models.cancel}</button>}
    </div>}
    {notice && <p role="alert">{notice}</p>}
    {!!results.length && <div className="knowledge-import-results">
      <div className="knowledge-import-heading"><h3>{copy.recent}</h3>{!busy && <button type="button" className="text-button" onClick={() => { setResults([]); setNotice(''); }}>{copy.clear}</button>}</div>
      <ul aria-label={copy.recent}>{results.map((result, index) => <li key={index} data-state={result.state}><bdi title={result.name}>{result.name}</bdi><span>{copy[result.state]}</span>{result.detail && <p>{result.detail}</p>}</li>)}</ul>
      <p role="status" className="workspace-secondary">{busy ? copy.importing : `${copy.added}: ${results.filter(result => result.state === 'added').length} / ${results.length}`}</p>
    </div>}
    {durable && !busy && <KnowledgeImportRecovery revision={revision} disabled={disabled} refresh={refresh} ocrSetupNeeded={formats?.some(format => ['language_missing', 'language_invalid'].includes(format.ocr_state ?? ''))} />}
    {!!formats?.length && <details className="knowledge-document-options">
      <summary>{knowledgeOptions(locale)}</summary>
      <div>
        <details className="knowledge-format-details"><summary>{copy.formats}</summary><ul>{formats.map(format => <li key={format.id}><strong>{format.extensions.join(', ')}</strong><span>{format.runtime_state === 'unavailable' ? copy.unavailable : format.runtime_state === 'installed_unchecked' ? copy.unchecked : copy.available}</span>{format.id === 'pdf' && !format.ocr_available && format.ocr_state !== 'installed_unchecked' && <span>{['language_missing', 'language_invalid'].includes(format.ocr_state ?? '') ? documentRecovery(locale, 'document_ocr_language_missing') : copy.ocr}</span>}</li>)}</ul></details>
        {formats.some(format => format.ocr_state && format.ocr_state !== 'unavailable') && <OCRLanguageSetup disabled={setupDisabled || busy} refresh={refresh} />}
      </div>
    </details>}
  </section>;
}
