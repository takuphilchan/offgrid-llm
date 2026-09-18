import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type CatalogModel, type Document, type DownloadProgress, type Model, type RAGStatus } from '../../api/client';
import { Icon } from '../../components/Icon';
import { ModelDownloadProgress } from '../../components/ModelDownloadProgress';
import { useI18n } from '../../i18n';
import { workflow } from '../../i18n/workflow';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DocumentSourceDialog } from './DocumentSourceDialog';

type Props = {
  models: Model[];
  onModelsChanged: () => Promise<void>;
  onOpenModels: () => void;
};

export function KnowledgePage({ models, onModelsChanged, onOpenModels }: Props) {
  const { messages: text, locale } = useI18n();
  const copy = workflow[locale];
  const [deleting, setDeleting] = useState<Document | null>(null);
  const [sourceID, setSourceID] = useState('');
  const [query, setQuery] = useState('');
  const [documents, setDocuments] = useState<Document[]>([]);
  const [status, setStatus] = useState<RAGStatus | null>(null);
  const [catalog, setCatalog] = useState<CatalogModel[]>([]);
  const [embeddingModel, setEmbeddingModel] = useState('');
  const [busy, setBusy] = useState(true);
  const [setupBusy, setSetupBusy] = useState(false);
  const [error, setError] = useState('');
  const [reindexing, setReindexing] = useState('');
  const [downloadKey, setDownloadKey] = useState('');
  const [download, setDownload] = useState<DownloadProgress | null>(null);
  const input = useRef<HTMLInputElement | null>(null);

  const installedEmbeddings = useMemo(() => models.filter(model => model.type === 'embedding'), [models]);
  const embeddingCatalog = useMemo(() => catalog.filter(model => model.type === 'embedding'), [catalog]);
  const selectedInstalled = installedEmbeddings.some(model => model.id === embeddingModel);

  const refresh = async () => {
    setBusy(true);
    setError('');
    try {
      const [list, nextStatus, nextCatalog, downloads] = await Promise.all([api.documents(), api.ragStatus(), api.catalog(), api.downloadProgress()]);
      setDocuments(list.documents);
      setStatus(nextStatus);
      setCatalog(nextCatalog);
      const setup = Object.values(downloads).filter(item => item.enable_knowledge).sort((a,b) => b.started_at - a.started_at)[0];
      if (setup) {
        setDownload(setup);
        if (setup.status === 'downloading' || setup.status === 'finalizing') { setDownloadKey(setup.file_name); setSetupBusy(true); }
      }
      setEmbeddingModel(current => current || nextStatus.embedding_model || installedEmbeddings[0]?.id || nextCatalog.find(model => model.id === 'bge-m3')?.id || nextCatalog.find(model => model.type === 'embedding')?.id || '');
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => { void refresh(); }, []);
  useEffect(() => {
    if (!downloadKey) return;
    let stopped = false;
    const poll = async () => {
      try {
        const progress = (await api.downloadProgress())[downloadKey];
        if (stopped || !progress) return;
        setDownload(progress);
        if (progress.status === 'complete') {
          setDownloadKey('');
          await onModelsChanged();
          await refresh();
          setSetupBusy(false);
        } else if (progress.status === 'failed' || progress.status === 'cancelled') {
          setError(progress.error || text.knowledgeSetup.failed);
          setDownloadKey('');
          setSetupBusy(false);
        }
      } catch (reason) {
        if (!stopped) {
          setError(reason instanceof Error ? reason.message : text.knowledgeSetup.failed);
          setSetupBusy(false);
          setDownloadKey('');
        }
      }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), 1200);
    return () => { stopped = true; window.clearInterval(timer); };
  }, [downloadKey, onModelsChanged]);

  const prepare = async () => {
    if (!embeddingModel || setupBusy) return;
    setSetupBusy(true);
    setError('');
    try {
      const model = embeddingCatalog.find(item => item.id === embeddingModel) ?? (selectedInstalled ? { id: embeddingModel, repo: '', file: `${embeddingModel}.gguf`, quant: '', size_bytes: 0 } : undefined);
      if (!model) throw new Error('Selected embedding model is not available in the catalog.');
      const accepted = await api.downloadModel(model, true);
      if (accepted.exists) {
        await onModelsChanged();
        await refresh();
        setSetupBusy(false);
        return;
      }
      setDownloadKey(accepted.file_name);
      setDownload({ file_name: accepted.file_name, status: 'downloading', percent: 0, bytes_done: 0, bytes_total: model.size_bytes, speed: 0, started_at: Math.floor(Date.now() / 1000) });
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.knowledgeSetup.failed);
      setSetupBusy(false);
    }
  };

  const upload = async (file?: File) => {
    if (!file) return;
    setBusy(true);
    setError('');
    try { await api.ingest(file); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); setBusy(false); }
    finally { if (input.current) input.current.value = ''; }
  };

  const reindex = async (document: Document) => {
    if (reindexing) return;
    setReindexing(document.id);
    setError('');
    try { await api.reindexDocument(document.id); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setReindexing(''); }
  };

  const enabled = status?.enabled === true;
  const shownDocuments = documents.filter(document => document.name.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  return <div className="stack knowledge-workspace">
    {enabled && <button className="secondary-button" disabled={busy || reindexing !== ''} onClick={() => { setBusy(true); void api.disableRAG().then(refresh).catch(reason => { setError(reason instanceof Error ? reason.message : text.common.error); setBusy(false); }); }}>{copy.disableKnowledge}</button>}
    {sourceID && <DocumentSourceDialog id={sourceID} close={() => setSourceID('')} />}
    {deleting && <ConfirmDialog title={deleting.name} body={copy.deleteDocument} close={() => setDeleting(null)} confirm={async () => { await api.deleteDocument(deleting.id); await refresh(); }} />}
    {!enabled && !busy && <section className="knowledge-setup">
      <div className="knowledge-setup-copy"><div className="model-glyph"><Icon name="knowledge" /></div><div><span className="eyebrow">{text.knowledgeSetup.title}</span><h2>{text.knowledge.disabled}</h2><p>{text.knowledgeSetup.body}</p></div></div>
      <div className="knowledge-setup-controls">
        <label><span>{text.knowledgeSetup.model}</span><select value={embeddingModel} onChange={event => setEmbeddingModel(event.target.value)} disabled={setupBusy}>
          {installedEmbeddings.map(model => <option key={model.id} value={model.id}>{model.id} · {text.models.installed}</option>)}
          {embeddingCatalog.filter(model => !installedEmbeddings.some(local => local.id === model.id)).map(model => <option key={model.id} value={model.id}>{model.name} · {formatBytes(model.size_bytes)}</option>)}
        </select></label>
        <button className="primary-button" onClick={() => void prepare()} disabled={!embeddingModel || setupBusy}>{setupBusy ? text.knowledgeSetup.preparing : selectedInstalled ? text.knowledgeSetup.enable : text.knowledgeSetup.installEnable}</button>
        <button className="secondary-button" onClick={onOpenModels}>{text.knowledgeSetup.openModels}</button>
      </div>
      {download && <ModelDownloadProgress download={download} />}
    </section>}

    <div className="section-actions"><div className="notice"><span>{documents.length} · {text.knowledge.title}</span>{enabled && <span className="knowledge-model">{text.knowledgeSetup.active}: {status?.embedding_model}</span>}</div><input ref={input} hidden type="file" onChange={event => void upload(event.target.files?.[0])} /><button className="primary-button" onClick={() => input.current?.click()} disabled={busy || !enabled}><Icon name="upload" size={17} />{busy ? text.common.loading : text.knowledge.add}</button><button className="secondary-button" disabled={busy} onClick={() => void refresh()}>{text.common.refresh}</button></div>
    {!enabled && documents.length > 0 && <p>{copy.offlineLibrary}</p>}
    {error && <div className="inline-error" role="alert">{error}<button onClick={() => void refresh()}>{text.common.retry}</button></div>}
    {documents.length > 0 && <input type="search" aria-label={text.knowledge.title} placeholder={text.knowledge.title} value={query} onChange={event => setQuery(event.target.value)} />}
    {busy && documents.length === 0 ? <SkeletonCards /> : shownDocuments.length === 0 ? <EmptyPanel text={query ? text.history.noMatches : text.knowledge.empty} /> : <div className="card-grid">{shownDocuments.map(document => <article className="resource-card" key={document.id}><div className="file-icon"><Icon name="knowledge" /></div><div className="resource-body"><h3>{document.name}</h3><p>{document.content_type || text.common.document} · {formatBytes(document.size)}</p><small>{document.chunk_count} {text.knowledge.chunks} · {document.index_status ?? 'ready'}</small><div className="resource-actions"><span className={document.source_retained ? 'source-state retained' : 'source-state'}>{document.source_retained ? text.knowledge.retained : text.knowledge.legacy}</span><button disabled={!document.source_retained || !enabled || reindexing !== ''} onClick={() => void reindex(document)}>{reindexing === document.id ? text.knowledge.reindexing : text.knowledge.reindex}</button><button disabled={!document.source_retained} title={!document.source_retained ? copy.sourceMissing : undefined} onClick={() => setSourceID(document.id)}>{copy.viewSource}</button><button className="danger-button" disabled={busy || reindexing !== ''} onClick={() => setDeleting(document)}>{text.models.delete}</button></div></div></article>)}</div>}
  </div>;
}

function EmptyPanel({ text }: { text: string }) { return <div className="empty-panel"><div className="empty-lines"><i /><i /><i /></div><p>{text}</p></div>; }
function SkeletonCards() { return <div className="card-grid">{[1, 2, 3].map(item => <div className="skeleton-card" key={item}><i /><div><span /><span /></div></div>)}</div>; }
function formatBytes(bytes: number) { if (!bytes) return '0 B'; const units = ['B', 'KB', 'MB', 'GB', 'TB']; const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`; }
