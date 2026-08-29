import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type CatalogModel, type Document, type DownloadProgress, type Model, type RAGStatus } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';

type Props = {
  models: Model[];
  onModelsChanged: () => Promise<void>;
  onOpenModels: () => void;
};

export function KnowledgePage({ models, onModelsChanged, onOpenModels }: Props) {
  const { messages: text } = useI18n();
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
  const setupModel = useRef('');
  const input = useRef<HTMLInputElement | null>(null);

  const installedEmbeddings = useMemo(() => models.filter(model => model.type === 'embedding'), [models]);
  const embeddingCatalog = useMemo(() => catalog.filter(model => model.type === 'embedding'), [catalog]);
  const selectedInstalled = installedEmbeddings.some(model => model.id === embeddingModel);

  const refresh = async () => {
    setBusy(true);
    setError('');
    try {
      const [list, nextStatus, nextCatalog] = await Promise.all([api.documents(), api.ragStatus(), api.catalog()]);
      setDocuments(list.documents);
      setStatus(nextStatus);
      setCatalog(nextCatalog);
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
          await api.enableRAG(setupModel.current);
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
    setupModel.current = embeddingModel;
    try {
      if (selectedInstalled) {
        await api.enableRAG(embeddingModel);
        await refresh();
        setSetupBusy(false);
        return;
      }
      const model = embeddingCatalog.find(item => item.id === embeddingModel);
      if (!model) throw new Error('Selected embedding model is not available in the catalog.');
      const accepted = await api.downloadModel(model);
      if (accepted.exists) {
        await onModelsChanged();
        await api.enableRAG(embeddingModel);
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
    setReindexing(document.id);
    setError('');
    try { await api.reindexDocument(document.id); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setReindexing(''); }
  };

  const enabled = status?.enabled === true;
  return <div className="stack knowledge-workspace">
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
      {download && setupBusy && <div className={`download-state ${download.status}`}><div><span style={{ width: `${Math.max(0, Math.min(100, download.percent))}%` }} /></div><small>{download.status} · {download.percent.toFixed(1)}% · {formatBytes(download.speed)}/s</small></div>}
    </section>}

    {enabled && <div className="section-actions"><div className="notice success"><i />{documents.length} {text.knowledge.chunks}<span className="knowledge-model">{text.knowledgeSetup.active}: {status?.embedding_model}</span></div><input ref={input} hidden type="file" onChange={event => void upload(event.target.files?.[0])} /><button className="primary-button" onClick={() => input.current?.click()} disabled={busy}><Icon name="upload" size={17} />{text.knowledge.add}</button></div>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {enabled && (busy && documents.length === 0 ? <SkeletonCards /> : documents.length === 0 ? <EmptyPanel text={text.knowledge.empty} /> : <div className="card-grid">{documents.map(document => <article className="resource-card" key={document.id}><div className="file-icon"><Icon name="knowledge" /></div><div className="resource-body"><h3>{document.name}</h3><p>{document.content_type || text.common.document} · {formatBytes(document.size)}</p><small>{document.chunk_count} {text.knowledge.chunks} · {document.index_status ?? 'ready'}</small><div className="resource-actions"><span className={document.source_retained ? 'source-state retained' : 'source-state'}>{document.source_retained ? text.knowledge.retained : text.knowledge.legacy}</span><button disabled={!document.source_retained || reindexing === document.id} onClick={() => void reindex(document)}>{reindexing === document.id ? text.knowledge.reindexing : text.knowledge.reindex}</button></div></div></article>)}</div>)}
  </div>;
}

function EmptyPanel({ text }: { text: string }) { return <div className="empty-panel"><div className="empty-lines"><i /><i /><i /></div><p>{text}</p></div>; }
function SkeletonCards() { return <div className="card-grid">{[1, 2, 3].map(item => <div className="skeleton-card" key={item}><i /><div><span /><span /></div></div>)}</div>; }
function formatBytes(bytes: number) { if (!bytes) return '0 B'; const units = ['B', 'KB', 'MB', 'GB', 'TB']; const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`; }
