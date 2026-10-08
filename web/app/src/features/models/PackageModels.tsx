import { useEffect, useRef, useState } from 'react';
import { api, type InstalledTypedModel, type ModelCategory, type ModelOperation, type ModelResolution, type PackageDiscovery, type TypedCatalogModel } from '../../api/client';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useI18n } from '../../i18n';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { speechModels } from '../../i18n/speech-models';
import { speechRuntimeText } from '../../i18n/voice';
import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { formatBytes } from './model-format';
import { SpeechPackages } from './SpeechPackages';
import { operationFeedback } from '../../i18n/operation-feedback';
import { useRecoverableRead } from '../../lib/use-recoverable-read';
import { ModelIdentity } from './ModelIdentity';
import { packageCatalogDisplay, packageDisplay } from './model-display';
import { modelLibrary } from '../../i18n/model-library';
import type { LibraryView } from './LibraryNavigation';
import { PackageOperation } from './PackageOperation';

const packageOf = (operation: ModelOperation) => operation.target.kind === 'package' ? operation.target.package : undefined;

export function PackageModels({ category, view }: { category: ModelCategory; view: LibraryView }) {
  const { messages: text, locale } = useI18n();
  const copy = modelAcquisition[locale], readiness = speechModels[locale];
  const feedback = operationFeedback(locale);
  const metadata = useRecoverableRead(text.common.error);
  const { admin } = useWorkspace();
  const [catalog, setCatalog] = useState<TypedCatalogModel[]>([]);
  const [installed, setInstalled] = useState<InstalledTypedModel[]>([]);
  const [operations, setOperations] = useState<ModelOperation[]>([]);
  const [query, setQuery] = useWorkspaceState(`models.${category}.query`, '');
  const [repositories, setRepositories] = useWorkspaceState<{ id: string; size_bytes?: number }[] | null>(`models.${category}.results`, null);
  const [discovery, setDiscovery] = useWorkspaceState<PackageDiscovery | null>(`models.${category}.discovery`, null);
  const [variant, setVariant] = useWorkspaceState(`models.${category}.variant`, '');
  const [catalogReview, setCatalogReview] = useWorkspaceState(`models.${category}.catalog-review`, '');
  const [filter, setFilter] = useWorkspaceState(`models.${category}.filter-installed`, '');
  const [preview, setPreview] = useWorkspaceState<ModelResolution | null>(`models.${category}.preview`, null);
  const [installRequest, setInstallRequest] = useWorkspaceState(`models.${category}.request`, '');
  const [mutating, setMutating] = useState(false), [loading, setLoading] = useState(true), [error, setError] = useState('');
  const [inventoryError, setInventoryError] = useState(''), [catalogError, setCatalogError] = useState('');
  const [operationsError, setOperationsError] = useState('');
  const busy = mutating || metadata.pending;
  const [confirm, setConfirm] = useState<{ title: string; body: string; run: () => Promise<unknown> } | null>(null);
  const lock = useRef(false), mounted = useRef(true), inventoryRevision = useRef(0), catalogRevision = useRef(0);
  const load = async (signal?: AbortSignal) => {
    const revision = ++inventoryRevision.current;
    const [items, work] = await Promise.allSettled([api.typedModels(undefined, signal), admin ? api.modelOperations(signal) : Promise.resolve([])]);
    if (!mounted.current || signal?.aborted || revision !== inventoryRevision.current) return;
    // Unknown/corrupt packages remain visible so they can be removed/repaired.
    if (items.status === 'fulfilled') {
      setInstalled(items.value.filter(item => item.kind === 'package' && (!item.category || item.category === category))); setInventoryError('');
    } else setInventoryError(items.reason instanceof Error ? items.reason.message : text.common.error);
    if (work.status === 'fulfilled') {
      setOperations(work.value.filter(o => { const m = packageOf(o); return m && (category === 'speech_recognition' ? m.capabilities.includes('transcription') : m.capabilities.includes('speech_synthesis')); })); setOperationsError('');
    } else setOperationsError(work.reason instanceof Error ? work.reason.message : text.common.error);
  };
  const refreshInventory = async (signal?: AbortSignal) => {
    const revision = inventoryRevision.current + 1;
    try { await load(signal); }
    catch (reason) { if (mounted.current && !signal?.aborted && revision === inventoryRevision.current) setInventoryError(reason instanceof Error ? reason.message : text.common.error); }
  };
  const refreshCatalog = async (signal?: AbortSignal) => {
    const revision = ++catalogRevision.current;
    try {
      const result = await api.typedCatalog(category, undefined, signal);
      if (mounted.current && !signal?.aborted && revision === catalogRevision.current) { setCatalog(result.models); setCatalogError(''); }
    } catch (reason) { if (mounted.current && !signal?.aborted && revision === catalogRevision.current) setCatalogError(reason instanceof Error ? reason.message : text.common.error); }
  };
  useEffect(() => {
    mounted.current = true;
    const controller = new AbortController(); let polling = false;
    void refreshInventory(controller.signal)
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    void refreshCatalog(controller.signal);
    const timer = window.setInterval(async () => {
      if (polling || controller.signal.aborted) return;
      polling = true;
      try { await refreshInventory(controller.signal); }
      finally { polling = false; }
    }, 2000);
    return () => { mounted.current = false; controller.abort(); clearInterval(timer); };
  }, [category]);
  useWorkspaceRefresh(async () => { await Promise.all([refreshInventory(), refreshCatalog()]); });

  const perform = async (run: () => Promise<unknown>) => {
    if (lock.current) return;
    lock.current = true; setMutating(true); setError('');
    try { await run(); await refreshInventory(); }
    catch (reason) { if (mounted.current) setError(reason instanceof Error ? reason.message : text.common.error); throw reason; }
    finally { lock.current = false; if (mounted.current) setMutating(false); }
  };
  const inspect = (run: (signal: AbortSignal) => Promise<void>, label = feedback.retryInspection) => metadata.run({ run, label });
  const search = (query: string) => inspect(async signal => {
    setDiscovery(null); setPreview(null); setCatalogReview('');
    const result = await api.typedCatalog(category, query, signal);
    if (!signal.aborted) setRepositories(result.repositories);
  }, feedback.retrySearch);
  const resolve = (input: Parameters<typeof api.resolvePackage>[0]) => inspect(async signal => {
    setPreview(null);
    const result = await api.resolvePackage(input, signal);
    if (!signal.aborted) { setPreview(result); setInstallRequest(crypto.randomUUID()); }
  });
  const choose = (repository: string) => inspect(async signal => {
    setDiscovery(null); setPreview(null); setCatalogReview('');
    const found = await api.discoverPackage(repository, signal);
    const choices = found.choices.filter(c => c.supported);
    if (signal.aborted) return;
    if (!choices.length) throw new Error(copy.emptyVariants);
    setDiscovery(found); setVariant('');
    if (choices.length !== 1) return;
    const choice = choices[0]; setVariant(choice.id);
    const result = await api.resolvePackage({ repository: found.repository, revision: found.revision, variant: choice.id, architecture: choice.architecture }, signal);
    if (!signal.aborted) { setPreview(result); setInstallRequest(crypto.randomUUID()); }
  });
  const chooseCatalog = (item: TypedCatalogModel) => {
    metadata.cancel(); setDiscovery(null); setPreview(null); setCatalogReview(item.id); setVariant('');
    if (item.variants.length === 1) {
      setVariant(item.variants[0].id);
      void resolve({ catalog_id: item.id });
    }
  };
  const accepted = operations.find(o => o.request_id === installRequest);
  const availableOperations = operations.filter(o => !o.discarded && (o.state !== 'complete' || o.retained_bytes > 0));
  const filtered = installed.filter(item => `${item.id} ${item.name}`.toLocaleLowerCase(locale).includes(filter.trim().toLocaleLowerCase(locale)));

  const downloadDetails = preview && <div className="package-download-details">
    <p className="model-identity">{copy.source}: {preview.resolution.provenance.repository} · {preview.resolution.provenance.revision || preview.resolution.manifest.revision}</p>
    <dl className="package-facts"><dt>{copy.transfer}</dt><dd>{formatBytes(preview.preflight.transfer_bytes)}</dd><dt>{copy.space}</dt><dd>{formatBytes(preview.preflight.required_free_bytes)}</dd><dt>{copy.license}</dt><dd>{preview.resolution.manifest.license}</dd></dl>
    {(preview.preflight.warnings ?? []).map((warning, index) => <p className="model-readiness" key={index}>{warning}</p>)}
    <details><summary>{copy.files} ({preview.resolution.manifest.artifacts.length})</summary><ul className="package-files">{preview.resolution.manifest.artifacts.map(a => <li key={a.path}><span>{a.path} · {a.license}</span><span>{formatBytes(a.size)}</span></li>)}</ul></details>
    {(preview.resolution.source_notices ?? []).map((notice, index) => <details key={index}><summary>{copy.license}</summary><p>{notice}</p></details>)}
    <div className="model-actions"><button className="primary-button" disabled={!admin || busy || !!accepted} onClick={() => void perform(async () => { await api.installPackage(preview.id, installRequest); setPreview(null); }).catch(() => {})}>{accepted ? copy[accepted.state] : text.models.download}</button></div>
  </div>;

  return <>
    <div className="model-info-banner" role="status"><strong>{category === 'speech_recognition' ? copy.speech_recognition : copy.speech_generation}</strong><span>{copy.runtimeNotice ?? 'Choose a reviewed package to download it. Runtime readiness is shown before installation.'}</span></div>
    {error && <div className="inline-error" role="alert">{error}<button className="secondary-button" disabled={busy} onClick={() => void refreshInventory()}>{feedback.checkStatus}</button></div>}
    {inventoryError && <div className="inline-error" role="alert">{inventoryError}<button className="secondary-button" disabled={busy} onClick={() => void refreshInventory()}>{text.common.refresh}</button></div>}
    {operationsError && <div className="inline-error" role="alert">{copy.operations}: {operationsError}<button className="secondary-button" disabled={busy} onClick={() => void refreshInventory()}>{feedback.checkStatus}</button></div>}
    {metadata.failure && <div className="inline-error" role="alert">{metadata.failure.message}<button className="secondary-button" disabled={mutating} onClick={() => { if (metadata.failure) void metadata.run(metadata.failure.action); }}>{metadata.failure.action.label}</button></div>}
    {view === 'installed' && <section className="model-section" aria-label={text.models.installed} aria-busy={loading}>
      <div className="section-heading"><h2>{text.models.installed}</h2></div>
      {!loading && !inventoryError && !installed.length && <p className="compact-empty">{text.models.empty}</p>}
      {!!installed.length && <label className="field"><span>{modelLibrary(locale).filterInstalled}</span><input type="search" value={filter} onChange={event => setFilter(event.target.value)} /></label>}
      {!!installed.length && !filtered.length && <p role="status">{modelLibrary(locale).noMatches}</p>}
      <div className="catalog-grid">{filtered.map(item => {
        const source = operations.find(o => o.state === 'complete' && !o.discarded && packageOf(o)?.id === item.id && packageOf(o)?.revision === item.revision);
        const state = item.package ?? item.readiness;
        const inUse = !!item.package?.in_use || !!item.package?.operation;
        return <article className="catalog-card package-card" key={`${item.id}/${item.revision}`}>
          <ModelIdentity model={packageDisplay(item)} />
          <p>{state.integrity === 'checked' ? readiness.checked : state.integrity === 'failed' ? readiness.failed : readiness.unchecked}</p>
          <p className="model-readiness">{state.issue || (state.runtime_compatible ? speechRuntimeText(locale)[state.smoke_tested ? 'tested' : 'available'] : speechRuntimeText(locale).unavailable)}</p>
          {item.package?.manifest && <details><summary>{copy.source} · {copy.license}</summary><p>{item.package.manifest.runtime.adapter} · {item.package.manifest.runtime.revision}</p></details>}
          {!source && state.integrity === 'failed' && <p>{copy.localRepair}</p>}
          <div className="model-actions">
            <button className="secondary-button" disabled={!admin || busy || inUse} onClick={() => void perform(() => api.verifyModelPackage(item.id, item.revision!, new AbortController().signal)).catch(() => {})}>{text.models.verify}</button>
            {source && state.integrity === 'failed' && <button className="secondary-button" disabled={!admin || busy || inUse} onClick={() => { const requestID = crypto.randomUUID(); setConfirm({ title: `${copy.repair}: ${item.name}`, body: `${copy.transfer}: ${formatBytes(source.bytes_total)}. ${state.issue || ''}`, run: () => api.repairPackage(source.id, requestID) }); }}>{copy.repair}</button>}
            <button className="danger-button" disabled={!admin || busy || inUse} onClick={() => setConfirm({ title: item.name, body: readiness.remove, run: () => api.removeModelPackage(item.id, item.revision!, new AbortController().signal) })}>{text.models.delete}</button>
          </div>
        </article>;
      })}</div>
    </section>}
    {!!availableOperations.length && <section className="model-section" aria-label={copy.operations}>
      <h2>{copy.operations}</h2>
      {availableOperations.map(o => <PackageOperation key={o.id} operation={o} disabled={!admin || busy} control={action => void perform(() => api.controlModelOperation(o.id, action)).catch(() => {})} discard={() => setConfirm({ title: copy.discard, body: copy.discardConfirm, run: () => api.controlModelOperation(o.id, 'discard') })} />)}
    </section>}
    {view === 'discover' && <><section className="model-section" aria-label="Hugging Face">
      <div className="section-heading"><h2>Hugging Face</h2></div>
      <form className="model-search-form" aria-busy={metadata.pending} onSubmit={event => { event.preventDefault(); if (!mutating && admin && query.trim()) void search(query.trim()); }}>
        <label className="field"><span>{text.modelSearch.query}</span><input type="search" maxLength={200} value={query} onChange={event => { metadata.cancel(); setQuery(event.target.value); }} /></label>
        <button className="primary-button" disabled={!admin || mutating || !query.trim()}>{text.modelSearch.search}</button>
      </form>
      {metadata.pending && <p role="status">{text.common.loading}</p>}
      {repositories?.length === 0 && <p role="status">{text.modelSearch.empty}</p>}
      {repositories?.map(repo => <article className="model-search-result" key={repo.id}><div className="model-search-repo"><div><h3>{repo.id}</h3><p className="model-identity">{modelLibrary(locale).repositorySize}: {repo.size_bytes && repo.size_bytes > 0 ? formatBytes(repo.size_bytes) : text.modelSearch.unknownSize}</p></div><button className="secondary-button" disabled={!admin || busy} onClick={() => void choose(repo.id)}>{copy.preview}</button></div>
        {discovery?.repository === repo.id && discovery.choices.filter(c => c.supported).length > 1 && <label className="field"><span>{copy.variant}</span><select disabled={busy} value={variant} onChange={event => {
          const value = event.target.value; setVariant(value); setPreview(null);
          const choice = discovery.choices.find(c => c.id === value);
          if (choice) void resolve({ repository: discovery.repository, revision: discovery.revision, variant: choice.id, architecture: choice.architecture });
        }}><option value="">{copy.variant}</option>{discovery.choices.map(c => <option key={c.id} value={c.id} disabled={!c.supported}>{c.name} · {c.architecture}</option>)}</select></label>}
        {discovery?.repository === repo.id && preview?.resolution.provenance.repository === repo.id && downloadDetails}
      </article>)}
    </section>
    <section className="model-section" aria-label={text.models.catalog}>
      {catalogError && <div className="inline-error" role="alert">{catalogError}<button className="secondary-button" disabled={busy} onClick={() => void refreshCatalog()}>{feedback.refreshCatalog}</button></div>}
      <h2>{text.models.catalog}</h2>{!catalogError && !catalog.length && <p role="status">{modelLibrary(locale).emptyCatalog}</p>}<div className="catalog-grid">{catalog.map(item => <article className="catalog-card package-card" key={item.id}>
        <ModelIdentity model={packageCatalogDisplay(item)} /><p>{item.description}</p>
        <div className="catalog-actions"><button className="secondary-button" aria-expanded={catalogReview === item.id} disabled={!admin || busy || !item.variants.length} onClick={() => chooseCatalog(item)}>{copy.preview}</button></div>
        {catalogReview === item.id && item.variants.length > 1 && <label className="field"><span>{copy.variant}</span><select value={variant} disabled={busy} onChange={event => {
          const value = event.target.value; setVariant(value); setPreview(null);
          const choice = item.variants.find(v => v.id === value);
          // Each package target has its own existing curated resolution. Never
          // silently use the first variant or construct a new source authority.
          if (choice?.target.kind === 'package') void resolve({ catalog_id: choice.target.package.id });
        }}><option value="">{copy.variant}</option>{item.variants.map(choice => <option key={choice.id} value={choice.id} disabled={choice.target.kind !== 'package'}>{choice.name} · {choice.size_bytes > 0 ? formatBytes(choice.size_bytes) : modelLibrary(locale).unknown}</option>)}</select></label>}
        {!discovery && catalogReview === item.id && downloadDetails}
      </article>)}</div>
    </section>
    </>}
    <SpeechPackages importOnly onChanged={load} />
    {confirm && <ConfirmDialog title={confirm.title} body={confirm.body} confirmLabel={confirm.title === copy.discard ? copy.discard : confirm.title.startsWith(copy.repair) ? copy.repair : undefined} close={() => setConfirm(null)} confirm={async () => { await perform(confirm.run); }} />}
  </>;
}
