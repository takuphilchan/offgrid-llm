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
import { EmptyState, SectionHeading } from '../../components/WorkspacePresentation';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { LegacyDownloadReview } from './LegacyDownloadReview';
import { SearchField } from '../../components/SearchField';
import { LanguagePackageModels } from './LanguagePackageModels';
import { CatalogComparison } from './CatalogComparison';
import { modelComparison } from '../../i18n/model-comparison';
import { UtilityPopover } from '../../components/UtilityPopover';
import { workspaceOrder } from '../../i18n/workspace-order';
import { modelBundles } from '../../i18n/model-bundles';

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
  const [bundles, setBundles] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    void api.systemIdentity(controller.signal).then(value => { if (!controller.signal.aborted) setBundles(value.capabilities?.includes('gguf-bundles-v1') === true); }).catch(() => {});
    return () => controller.abort();
  }, []);
  const [category, setCategory] = useWorkspaceState<ModelCategory>('models.category', 'language');
  const [view, setView] = useWorkspaceState<LibraryView>('models.view', 'installed');
  useEffect(() => {
    if (window.location.hash.startsWith('#/models/discover')) {
      setCategory('language');
      setView('discover');
    }
  }, []);
  const discover = () => { setView('discover'); document.getElementById('model-view-discover')?.focus(); };
  const [filters, setFilters] = useWorkspaceState<Partial<Record<ModelCategory, string>>>('models.installed-filters', {});
  const filter = filters[category] ?? '';
  const [review, setReview] = useWorkspaceState('models.catalog-review', '');
  const [catalogFilters, setCatalogFilters] = useWorkspaceState<Partial<Record<ModelCategory, string>>>('models.catalog-filters', {});
  const [comparisons, setComparisons] = useWorkspaceState<Partial<Record<ModelCategory, string[]>>>('models.catalog-comparisons', {});
  const [discoverySources, setDiscoverySources] = useWorkspaceState<Partial<Record<ModelCategory, 'curated' | 'search'>>>('models.discovery-sources', {});
  const discoverySource = discoverySources[category] ?? 'curated';
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
  const compared = sortedCatalog.filter(item => comparisons[category]?.includes(item.id)).slice(0, 3);
  const catalogQuery = (catalogFilters[category] ?? '').trim().toLocaleLowerCase(locale);
  const filteredCatalog = sortedCatalog.filter(item => [item.name, item.id, item.description, item.parameters, item.quant].some(value => value?.toLocaleLowerCase(locale).includes(catalogQuery)));

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
    <div className="model-library-toolbar"><LibraryNavigation view={view} change={setView} />
    <div className="model-category-picker" role="group" aria-label={categoryCopy.category}>{categories.map(value => <button key={value} className="secondary-button" aria-pressed={category === value} onClick={() => setCategory(value)}>{categoryCopy[value]}</button>)}</div></div>
    <p className="model-search-hint">{view === 'discover' ? text.guidance.discovery : category === 'language' ? text.guidance.languageModels : category === 'embeddings' ? text.guidance.embeddingModels : text.guidance.speechModels}</p>
    <div id="model-library-panel" role="tabpanel" aria-labelledby={`model-view-${view}`} className="stack">
    {packageMode ? <PackageModels key={category} category={category} view={view} onDiscover={discover} /> : <>
    {confirmDelete && <ConfirmDialog title={confirmDelete} body={workflow[locale].deleteModel} close={() => setConfirmDelete('')} confirm={async () => { const item = models.find(model => model.id === confirmDelete); if (item) await remove(item); }} />}
    {needsChatModel && <div className="setup-guidance" role="status"><strong>{text.onboarding.chooseModel}</strong><p>{text.onboarding.modelHint}</p></div>}
    {view === 'discover' && <div className="model-discovery-sources" role="group" aria-label={text.models.discover}>
      {(['curated', 'search'] as const).map(source => <button key={source} type="button" className="secondary-button" aria-pressed={discoverySource === source} onClick={() => setDiscoverySources(old => ({ ...old, [category]: source }))}>{modelLibrary(locale)[source]}</button>)}
    </div>}
    {view === 'discover' && discoverySource === 'search' && (admin ? <ModelSearch key={category} bundles={bundles} category={category} models={models} progress={progress} busy={operation !== ''} download={download} cancel={cancel} /> : <p className="permission-notice">{interaction[locale].adminOnly}</p>)}
    {error && <div className="inline-error" role="alert">{error}<button disabled={loading} onClick={() => void load()}>{text.common.refresh}</button></div>}
    {progressError && <div className="inline-error" role="alert">{progressError}<button onClick={() => void load()}>{text.common.retry}</button></div>}
    {Object.values(progress).filter(item => item.status !== 'complete' && !(view === 'discover' && discoverySource === 'curated' && filteredCatalog.some(model => `${model.id}.gguf` === item.file_name))).map(item => {
      const catalogItem = catalog.find(model => `${model.id}.gguf` === item.file_name);
      const source = item.repository && item.source_file && item.model_id ? { id: item.model_id, repo: item.repository, file: item.source_file, quant: item.quantization ?? '' } : catalogItem;
      return <article className="catalog-card" key={item.file_name}><h3>{catalogItem?.name ?? item.file_name}</h3><ModelDownloadProgress download={item} /><div className="catalog-actions">{isActiveDownload(item) ? <button className="danger-button" disabled={!admin || operation !== ''} onClick={() => void cancel(item)}>{text.models.cancel}</button> : <button className="primary-button" disabled={!admin || operation !== '' || !source} onClick={() => { if (source) void download(source, item.enable_knowledge === true); }}>{text.models.resume}</button>}</div></article>;
    })}
    {view === 'installed' && <section className="model-section" aria-label={text.models.installed}>
      <SectionHeading title={`${text.models.installed} · ${categoryModels.length}`} />
    {!!categoryModels.length && <SearchField id="installed-model-search" className="model-installed-search" label={modelLibrary(locale).filterInstalled} value={filter} onChange={value => setFilters(old => ({ ...old, [category]: value }))} />}
      {visibleModels.length === 0 ? <EmptyState title={categoryModels.length ? modelLibrary(locale).noMatches : text.models.empty} action={<button type="button" className="secondary-button" onClick={() => { if (categoryModels.length) { setFilters(old => ({ ...old, [category]: '' })); document.getElementById('installed-model-search')?.focus(); } else discover(); }}>{categoryModels.length ? workspaceRefinement[locale].clear : text.models.discover}</button>} /> : <div className="installed-models">{visibleModels.map(model => <article className={selected === model.id ? 'installed-model selected' : 'installed-model'} key={model.id}>
        <ModelIdentity model={legacyDisplay(model, catalog.find(item => installedCatalogModel(item, [model])))} />
        {verifications[model.id] && <div className="verification model-verification" role="status"><strong>{verifications[model.id].file_name}</strong><span>{verifications[model.id].message}</span><span>{verifications[model.id].sha256}</span></div>}
        <div className="model-actions">
          {supportsChat(model) && <button className="text-button" aria-pressed={selected === model.id} onClick={() => setSelected(model.id)}>{selected === model.id && <Icon name="check" size={16} />}{workspaceOrder(locale).selectModel}</button>}
          <UtilityPopover compact align="end" label={`${workspaceOrder(locale).modelActions}: ${model.id}`} trigger={<Icon name="more" size={18} />} triggerClassName="utility-button icon-button" className="model-maintenance">
            {bundles && progress[`${model.id}.gguf`]?.repository && progress[`${model.id}.gguf`]?.source_file && <button className="text-button" disabled={!admin || operation !== '' || isActiveDownload(progress[`${model.id}.gguf`])} onClick={event => {
              event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover();
              const source = progress[`${model.id}.gguf`];
              void download({ id: model.id, repo: source.repository!, file: source.source_file!, quant: source.quantization ?? '' });
            }}>{modelBundles(locale).repair}</button>}
            <button className="text-button" disabled={!admin || operation !== ''} onClick={event => { event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover(); void verify(model); }}><Icon name="check" size={16} />{operation === `verify:${model.id}` ? text.common.loading : text.models.verify}</button><button className="danger-button" disabled={!admin || operation !== ''} onClick={event => { event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover(); setConfirmDelete(model.id); }}><Icon name="trash" size={16} />{text.models.delete}</button>
          </UtilityPopover>
        </div>
      </article>)}</div>}
    </section>}
    {view === 'installed' && category === 'language' && <LanguagePackageModels />}
    {view === 'discover' && discoverySource === 'curated' && <section className="model-section">
      <div className="section-heading"><div><span className="eyebrow">{text.models.catalog}</span><h2>{text.models.discover}</h2></div></div>
      {!!sortedCatalog.length && <>
        <SearchField className="model-installed-search" label={modelComparison[locale].filter} value={catalogFilters[category] ?? ''} onChange={value => setCatalogFilters(old => ({ ...old, [category]: value }))} />
        <CatalogComparison models={compared} clear={() => setComparisons(old => ({ ...old, [category]: [] }))} />
        {!filteredCatalog.length && <p role="status">{modelLibrary(locale).noMatches}</p>}
      </>}
      {!loading && sortedCatalog.length === 0 && <p role="status">{modelLibrary(locale).emptyCatalog}</p>}
      {loading && catalog.length === 0 ? <div className="catalog-grid"><div className="catalog-card skeleton-card" /><div className="catalog-card skeleton-card" /></div> : <div className="catalog-grid">{filteredCatalog.map(model => {
        const current = downloadFor(model, progress);
        const installed = installedCatalogModel(model, models);
        return <article className="catalog-card" key={model.id}>
          <div className="catalog-card-top"><div className="model-glyph"><Icon name="models" /></div>{model.recommended && <span className="status-pill">{text.models.recommended}</span>}</div>
          <ModelIdentity model={catalogDisplay(model)} /><p>{model.description}</p>
          <label className="catalog-compare-choice"><input type="checkbox" checked={compared.some(item => item.id === model.id)} disabled={compared.length >= 3 && !compared.some(item => item.id === model.id)} onChange={event => setComparisons(old => ({ ...old, [category]: event.target.checked ? [...compared.map(item => item.id), model.id] : compared.filter(item => item.id !== model.id).map(item => item.id) }))} />{modelComparison[locale].choose}<span className="sr-only"> {model.name}</span></label>
          <div className="model-meta"><span>{model.parameters}</span>{model.min_ram_gb > 0 && <span>{model.min_ram_gb} GB RAM</span>}</div>
          {review === model.id && !installed && <LegacyDownloadReview bundles={bundles} repository={model.repo} file={model.file} bytes={model.size_bytes} license={model.license} />}
          {current && current.status !== 'complete' && <ModelDownloadProgress download={current} />}
          <div className="catalog-actions">{installed ? <span className="installed-status" role="status"><Icon name="check" size={15} />{text.models.installed}</span> : current && isActiveDownload(current) ? <button className="danger-button" disabled={!admin || operation !== ''} onClick={() => void cancel(current)}>{text.models.cancel}</button> : current && ['cancelled', 'failed'].includes(current.status) || review === model.id ? <button className="primary-button" disabled={!admin || !model.repo || operation !== ''} onClick={() => void download(model)}>{operation === `download:${model.id}` ? text.common.loading : current && ['cancelled', 'failed'].includes(current.status) ? text.models.resume : text.models.download}</button> : <button className="primary-button" aria-expanded="false" disabled={!admin || !model.repo || operation !== ''} onClick={() => setReview(model.id)}>{categoryCopy.preview}</button>}</div>
        </article>;
      })}</div>}
    </section>}
    </>}
    </div>
  </div>;
}
