import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { supportsChat } from '../../api/model-capabilities';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState } from 'react';
import { api, type CatalogModel, type DownloadProgress, type Model, type ModelCategory, type Verification } from '../../api/client';
import { Icon } from '../../components/Icon';
import { isActiveDownload, ModelDownloadProgress } from '../../components/ModelDownloadProgress';
import { useI18n } from '../../i18n';
import { ModelSearch } from './ModelSearch';
import { PackageModels } from './PackageModels';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { ModelIdentity } from './ModelIdentity';
import { catalogDisplay, legacyDisplay } from './model-display';
import { modelLibrary } from '../../i18n/model-library';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { workflow } from '../../i18n/workflow';
import { LibraryNavigation, type LibraryView } from './LibraryNavigation';
import { LegacyDownloadReview } from './LegacyDownloadReview';

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
  const { messages: text, locale } = useI18n();
  const { admin } = useWorkspace();
  const [category, setCategory] = useWorkspaceState<ModelCategory>('models.category', 'language');
  const [view, setView] = useWorkspaceState<LibraryView>('models.view', 'installed');
  const [filters, setFilters] = useWorkspaceState<Partial<Record<ModelCategory, string>>>('models.installed-filters', {});
  const filter = filters[category] ?? '';
  const [review, setReview] = useWorkspaceState('models.catalog-review', '');
  const packageMode = category === 'speech_recognition' || category === 'speech_generation';
  const categories: ModelCategory[] = ['language', 'embeddings', 'speech_recognition', 'speech_generation'];
  const categoryCopy = modelAcquisition[locale];
  const categoryModels = models.filter(model => category === 'embeddings' ? model.type === 'embedding' : supportsChat(model));
  const visibleModels = categoryModels.filter(model => model.id.toLocaleLowerCase(locale).includes(filter.trim().toLocaleLowerCase(locale)));
  const [catalog, setCatalog] = useState<CatalogModel[]>([]);
  const [progress, setProgress] = useState<Record<string, DownloadProgress>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [progressError, setProgressError] = useState('');
  const [operation, setOperation] = useState('');
  const [confirmDelete, setConfirmDelete] = useState('');
  const [verifications, setVerifications] = useWorkspaceState<Record<string, Verification>>('models.verifications', {});
  const completed = useRef(new Set<string>());
  const activeOperations = useRef(new Set<string>());
  const needsChatModel = onboardingPending && !models.some(supportsChat);
  const sortedCatalog = catalog.filter(item => category === 'embeddings' ? item.type === 'embedding' : supportsChat(item)).sort((a, b) => {
    const rank = (item: CatalogModel) => (supportsChat(item) ? 2 : 0) + (item.recommended ? 1 : 0);
    return rank(b) - rank(a);
  });

  const load = async () => {
    setLoading(true);
    setError('');
    const [catalogResult, progressResult] = await Promise.allSettled([api.catalog(), admin ? api.downloadProgress() : Promise.resolve({})]);
    if (catalogResult.status === 'fulfilled') setCatalog(catalogResult.value);
    else setError(catalogResult.reason instanceof Error ? catalogResult.reason.message : text.common.error);
    if (progressResult.status === 'fulfilled') { setProgress(progressResult.value); setProgressError(''); }
    else setProgressError(progressResult.reason instanceof Error ? progressResult.reason.message : text.common.error);
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);
  useWorkspaceRefresh(load);
  useEffect(() => {
    if (!admin) return;
    const controller = new AbortController(); let polling = false;
    const timer = window.setInterval(async () => {
      if (polling || controller.signal.aborted) return;
      polling = true;
      try {
        const next = await api.downloadProgress(controller.signal);
        if (controller.signal.aborted) return;
        setProgress(next);
        setProgressError('');
        const newlyComplete = Object.values(next).filter(item => item.status === 'complete' && !completed.current.has(item.file_name));
        if (newlyComplete.length > 0) {
          newlyComplete.forEach(item => completed.current.add(item.file_name));
          await onRefresh();
        }
      } catch (reason) { if (!controller.signal.aborted) setProgressError(reason instanceof Error ? reason.message : text.common.error); }
      finally { polling = false; }
    }, 1500);
    return () => { controller.abort(); clearInterval(timer); };
  }, [onRefresh, admin]);

  const download = async (model: Pick<CatalogModel, 'id' | 'repo' | 'file' | 'quant'>, enableKnowledge = false) => {
    const operationID = `download:${model.id}`;
    if (activeOperations.current.has(operationID)) return;
    activeOperations.current.add(operationID);
    setOperation(operationID);
    setError('');
    try {
      completed.current.delete(`${model.id}.gguf`);
      const accepted = await api.downloadModel(model, enableKnowledge);
      if (accepted.exists) await onRefresh();
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
    setOperation(`delete:${model.id}`);
    setError('');
    try {
      await api.deleteModel(model.id);
      setVerifications(old => { const next = { ...old }; delete next[model.id]; return next; });
      setConfirmDelete('');
      if (selected === model.id) setSelected('');
      await onRefresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
      throw reason;
    } finally { setOperation(''); }
  };

  const verify = async (model: Model) => {
    setOperation(`verify:${model.id}`);
    setError('');
    try { const result = await api.verifyModel(model.id); setVerifications(old => ({ ...old, [model.id]: result })); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setOperation(''); }
  };

  return <div className="stack models-workspace">
    <LibraryNavigation view={view} change={setView} />
    <div className="model-category-picker" role="group" aria-label={categoryCopy.category}>{categories.map(value => <button key={value} className="secondary-button" aria-pressed={category === value} onClick={() => setCategory(value)}>{categoryCopy[value]}</button>)}</div>
    <div id="model-library-panel" role="tabpanel" aria-labelledby={`model-view-${view}`} className="stack">
    {packageMode ? <PackageModels key={category} category={category} view={view} /> : <>
    {confirmDelete && <ConfirmDialog title={confirmDelete} body={workflow[locale].deleteModel} close={() => setConfirmDelete('')} confirm={async () => { const item = models.find(model => model.id === confirmDelete); if (item) await remove(item); }} />}
    {needsChatModel && <div className="setup-guidance" role="status"><strong>{text.onboarding.chooseModel}</strong><p>{text.onboarding.modelHint}</p></div>}
    {view === 'discover' && (admin ? <ModelSearch key={category} category={category} models={models} progress={progress} busy={operation !== ''} download={download} cancel={cancel} /> : <p className="permission-notice">{interaction[locale].adminOnly}</p>)}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {progressError && <div className="inline-error" role="alert">{progressError}<button onClick={() => void load()}>{text.common.retry}</button></div>}
    {Object.values(progress).filter(item => item.status !== 'complete' && !(view === 'discover' && sortedCatalog.some(model => `${model.id}.gguf` === item.file_name))).map(item => {
      const catalogItem = catalog.find(model => `${model.id}.gguf` === item.file_name);
      const source = item.repository && item.source_file && item.model_id ? { id: item.model_id, repo: item.repository, file: item.source_file, quant: item.quantization ?? '' } : catalogItem;
      return <article className="catalog-card" key={item.file_name}><h3>{catalogItem?.name ?? item.file_name}</h3><ModelDownloadProgress download={item} /><div className="catalog-actions">{isActiveDownload(item) ? <button className="danger-button" disabled={!admin || operation !== ''} onClick={() => void cancel(item)}>{text.models.cancel}</button> : <button className="primary-button" disabled={!admin || operation !== '' || !source} onClick={() => { if (source) void download(source, item.enable_knowledge === true); }}>{text.models.resume}</button>}</div></article>;
    })}
    {view === 'installed' && <section className="model-section" aria-label={text.models.installed}>
      <div className="section-heading"><div><span className="eyebrow">{text.models.installed}</span><h2>{visibleModels.length} {text.models.available}</h2></div></div>
      {!!categoryModels.length && <label className="field"><span>{modelLibrary(locale).filterInstalled}</span><input type="search" value={filter} onChange={event => setFilters(old => ({ ...old, [category]: event.target.value }))} /></label>}
      {visibleModels.length === 0 ? <div className="compact-empty">{categoryModels.length ? modelLibrary(locale).noMatches : text.models.empty}</div> : <div className="installed-models">{visibleModels.map(model => <article className={selected === model.id ? 'installed-model selected' : 'installed-model'} key={model.id}>
        <ModelIdentity model={legacyDisplay(model, catalog.find(item => installedCatalogModel(item, [model])))} select={supportsChat(model) ? () => setSelected(model.id) : undefined} selected={selected === model.id} />
        {verifications[model.id] && <div className="verification model-verification" role="status"><strong>{verifications[model.id].file_name}</strong><span>{verifications[model.id].message}</span><span>{verifications[model.id].sha256}</span></div>}
        <div className="model-actions"><button disabled={!admin || operation !== ''} onClick={() => void verify(model)}>{operation === `verify:${model.id}` ? text.common.loading : text.models.verify}</button><button className="danger-button" disabled={!admin || operation !== ''} onClick={() => setConfirmDelete(model.id)}>{text.models.delete}</button></div>
      </article>)}</div>}
    </section>}
    {view === 'discover' && <section className="model-section">
      <div className="section-heading"><div><span className="eyebrow">{text.models.catalog}</span><h2>{text.models.discover}</h2></div></div>
      {!loading && sortedCatalog.length === 0 && <p role="status">{modelLibrary(locale).emptyCatalog}</p>}
      {loading && catalog.length === 0 ? <div className="catalog-grid"><div className="catalog-card skeleton-card" /><div className="catalog-card skeleton-card" /></div> : <div className="catalog-grid">{sortedCatalog.map(model => {
        const current = downloadFor(model, progress);
        const installed = installedCatalogModel(model, models);
        return <article className="catalog-card" key={model.id}>
          <div className="catalog-card-top"><div className="model-glyph"><Icon name="models" /></div>{model.recommended && <span className="status-pill">{text.models.recommended}</span>}</div>
          <ModelIdentity model={catalogDisplay(model)} /><p>{model.description}</p>
          <div className="model-meta"><span>{model.parameters}</span>{model.min_ram_gb > 0 && <span>{model.min_ram_gb} GB RAM</span>}</div>
          {review === model.id && !installed && <LegacyDownloadReview repository={model.repo} file={model.file} bytes={model.size_bytes} license={model.license} />}
          {current && current.status !== 'complete' && <ModelDownloadProgress download={current} />}
          <div className="catalog-actions">{installed ? <span className="installed-status" role="status"><Icon name="check" size={15} />{text.models.installed}</span> : current && isActiveDownload(current) ? <button className="danger-button" disabled={!admin || operation !== ''} onClick={() => void cancel(current)}>{text.models.cancel}</button> : current && ['cancelled', 'failed'].includes(current.status) || review === model.id ? <button className="primary-button" disabled={!admin || !model.repo || operation !== ''} onClick={() => void download(model)}>{operation === `download:${model.id}` ? text.common.loading : current && ['cancelled', 'failed'].includes(current.status) ? text.models.resume : text.models.download}</button> : <button className="primary-button" aria-expanded="false" disabled={!admin || !model.repo || operation !== ''} onClick={() => setReview(model.id)}>{categoryCopy.preview}</button>}</div>
        </article>;
      })}</div>}
    </section>}
    </>}
    </div>
  </div>;
}
