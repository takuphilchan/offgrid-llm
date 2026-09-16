import { useEffect, useRef, useState } from 'react';
import { api, type CatalogModel, type DownloadProgress, type Model, type Verification } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';

function fileName(path: string): string {
  return path.split('/').pop() ?? path;
}

function installedCatalogModel(model: CatalogModel, installed: Model[]): boolean {
  const stem = fileName(model.file).replace(/\.gguf$/i, '').toLowerCase();
  return installed.some(item => item.id.toLowerCase() === stem || item.id.toLowerCase() === model.id.toLowerCase());
}

function downloadFor(model: CatalogModel, progress: Record<string, DownloadProgress>): DownloadProgress | undefined {
  return progress[`${model.id}.gguf`];
}

export function ModelsPage({ models, selected, setSelected, onRefresh, onboardingPending }: { models: Model[]; selected: string; setSelected: (model: string) => void; onRefresh: () => Promise<void>; onboardingPending: boolean }) {
  const { messages: text } = useI18n();
  const [catalog, setCatalog] = useState<CatalogModel[]>([]);
  const [progress, setProgress] = useState<Record<string, DownloadProgress>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [operation, setOperation] = useState('');
  const [confirmDelete, setConfirmDelete] = useState('');
  const [verification, setVerification] = useState<Verification | null>(null);
  const completed = useRef(new Set<string>());
  const activeOperations = useRef(new Set<string>());
  const needsChatModel = onboardingPending && !models.some(item => item.type !== 'embedding');
  const sortedCatalog = [...catalog].sort((a, b) => {
    const rank = (item: CatalogModel) => (item.type !== 'embedding' ? 2 : 0) + (item.recommended ? 1 : 0);
    return rank(b) - rank(a);
  });

  const load = async () => {
    setLoading(true);
    setError('');
    const [catalogResult, progressResult] = await Promise.allSettled([api.catalog(), api.downloadProgress()]);
    if (catalogResult.status === 'fulfilled') setCatalog(catalogResult.value);
    else setError(catalogResult.reason instanceof Error ? catalogResult.reason.message : text.common.error);
    if (progressResult.status === 'fulfilled') setProgress(progressResult.value);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);
  useEffect(() => {
    const timer = window.setInterval(async () => {
      try {
        const next = await api.downloadProgress();
        setProgress(next);
        const newlyComplete = Object.values(next).filter(item => item.status === 'complete' && !completed.current.has(item.file_name));
        if (newlyComplete.length > 0) {
          newlyComplete.forEach(item => completed.current.add(item.file_name));
          await onRefresh();
        }
      } catch { /* The last known state remains visible; explicit actions surface errors. */ }
    }, 1500);
    return () => clearInterval(timer);
  }, [onRefresh]);

  const download = async (model: CatalogModel) => {
    const operationID = `download:${model.id}`;
    if (activeOperations.current.has(operationID)) return;
    activeOperations.current.add(operationID);
    setOperation(operationID);
    setError('');
    try {
      await api.downloadModel(model);
      setProgress(await api.downloadProgress());
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      activeOperations.current.delete(operationID);
      setOperation('');
    }
  };

  const cancel = async (download: DownloadProgress) => {
    setOperation(`cancel:${download.file_name}`);
    setError('');
    try {
      await api.cancelDownload(download.file_name);
      setProgress(await api.downloadProgress());
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally { setOperation(''); }
  };

  const remove = async (model: Model) => {
    if (confirmDelete !== model.id) {
      setConfirmDelete(model.id);
      return;
    }
    setOperation(`delete:${model.id}`);
    setError('');
    try {
      await api.deleteModel(model.id);
      setConfirmDelete('');
      if (selected === model.id) setSelected('');
      await onRefresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally { setOperation(''); }
  };

  const verify = async (model: Model) => {
    setOperation(`verify:${model.id}`);
    setVerification(null);
    setError('');
    try { setVerification(await api.verifyModel(model.id)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setOperation(''); }
  };

  return <div className="stack models-workspace">
    {needsChatModel && <div className="setup-guidance" role="status"><strong>{text.onboarding.chooseModel}</strong><p>{text.onboarding.modelHint}</p></div>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {verification && <div className={verification.verified ? 'verification success' : 'verification'}><Icon name={verification.verified ? 'check' : 'models'} size={17} /><div><strong>{verification.file_name}</strong><span>{verification.message} {verification.sha256 && `· SHA-256 ${verification.sha256}`}</span></div></div>}
    <section className="model-section">
      <div className="section-heading"><div><span className="eyebrow">{text.models.installed}</span><h2>{models.length} {text.models.available}</h2></div></div>
      {models.length === 0 ? <div className="compact-empty">{text.models.empty}</div> : <div className="installed-models">{models.map(model => <article className={selected === model.id ? 'installed-model selected' : 'installed-model'} key={model.id}>
        <button className="installed-model-main" onClick={() => model.type !== 'embedding' && setSelected(model.id)}><div className="model-glyph"><Icon name="models" /></div><div><strong>{model.id}</strong><span>{model.type === 'embedding' ? text.models.embedding : text.models.local}</span></div><small>{model.size_gb || formatBytes(model.size ?? 0)}</small></button>
        <div className="model-actions"><button disabled={operation !== ''} onClick={() => void verify(model)}>{operation === `verify:${model.id}` ? text.common.loading : text.models.verify}</button><button className={confirmDelete === model.id ? 'danger-button armed' : 'danger-button'} disabled={operation !== ''} onClick={() => void remove(model)}>{confirmDelete === model.id ? text.models.confirmDelete : text.models.delete}</button></div>
      </article>)}</div>}
    </section>
    <section className="model-section">
      <div className="section-heading"><div><span className="eyebrow">{text.models.catalog}</span><h2>{text.models.discover}</h2></div><button className="secondary-button" onClick={() => void load()}>{text.common.refresh}</button></div>
      {loading && catalog.length === 0 ? <div className="catalog-grid"><div className="catalog-card skeleton-card" /><div className="catalog-card skeleton-card" /></div> : <div className="catalog-grid">{sortedCatalog.map(model => {
        const current = downloadFor(model, progress);
        const installed = installedCatalogModel(model, models);
        return <article className="catalog-card" key={model.id}>
          <div className="catalog-card-top"><div className="model-glyph"><Icon name="models" /></div>{model.recommended && <span className="status-pill">{text.models.recommended}</span>}</div>
          <h3>{model.name}</h3><p>{model.description}</p>
          <div className="model-meta"><span>{model.parameters}</span><span>{model.quant}</span><span>{formatBytes(model.size_bytes)}</span><span>{model.min_ram_gb} GB RAM</span></div>
          {current && <div className={`download-state ${current.status}`}><div><span style={{ width: `${Math.max(0, Math.min(100, current.percent))}%` }} /></div><small>{current.status} · {current.percent.toFixed(1)}% · {formatBytes(current.speed)}/s</small>{current.error && <p>{current.error}</p>}</div>}
          <div className="catalog-actions">{current?.status === 'downloading' ? <button className="danger-button" disabled={operation !== ''} onClick={() => void cancel(current)}>{text.models.cancel}</button> : <button className="primary-button" disabled={installed || !model.repo || operation !== ''} onClick={() => void download(model)}>{installed ? text.models.installed : operation === `download:${model.id}` ? text.common.loading : current?.status === 'cancelled' ? text.models.resume : text.models.download}</button>}</div>
        </article>;
      })}</div>}
    </section>
  </div>;
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`;
}
