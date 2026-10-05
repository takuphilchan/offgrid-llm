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

const active = (operation: ModelOperation) => ['queued', 'downloading', 'verifying', 'activating', 'cancelling'].includes(operation.state);
const packageOf = (operation: ModelOperation) => operation.target.kind === 'package' ? operation.target.package : undefined;

export function PackageModels({ category }: { category: ModelCategory }) {
  const { messages: text, locale } = useI18n();
  const copy = modelAcquisition[locale], readiness = speechModels[locale];
  const { admin } = useWorkspace();
  const [catalog, setCatalog] = useState<TypedCatalogModel[]>([]);
  const [installed, setInstalled] = useState<InstalledTypedModel[]>([]);
  const [operations, setOperations] = useState<ModelOperation[]>([]);
  const [query, setQuery] = useWorkspaceState(`models.${category}.query`, '');
  const [repositories, setRepositories] = useState<{ id: string; size_bytes?: number }[] | null>(null);
  const [discovery, setDiscovery] = useState<PackageDiscovery | null>(null);
  const [variant, setVariant] = useState('');
  const [preview, setPreview] = useWorkspaceState<ModelResolution | null>(`models.${category}.preview`, null);
  const [installRequest, setInstallRequest] = useWorkspaceState(`models.${category}.request`, '');
  const [busy, setBusy] = useState(false), [loading, setLoading] = useState(true), [error, setError] = useState('');
  const [confirm, setConfirm] = useState<{ title: string; body: string; run: () => Promise<unknown> } | null>(null);
  const lock = useRef(false), mounted = useRef(true), metadata = useRef<AbortController | null>(null);
  const load = async (signal?: AbortSignal) => {
    const [items, work] = await Promise.all([api.typedModels(undefined, signal), api.modelOperations(signal)]);
    if (!mounted.current || signal?.aborted) return;
    // Unknown/corrupt packages remain visible so they can be removed/repaired.
    setInstalled(items.filter(item => item.kind === 'package' && (!item.category || item.category === category)));
    setOperations(work.filter(o => { const m = packageOf(o); return m && (category === 'speech_recognition' ? m.capabilities.includes('transcription') : m.capabilities.includes('speech_synthesis')); }));
  };
  useEffect(() => {
    mounted.current = true;
    const controller = new AbortController(); let polling = false;
    void Promise.all([load(controller.signal), api.typedCatalog(category, undefined, controller.signal).then(result => { if (!controller.signal.aborted) setCatalog(result.models); })])
      .catch(reason => { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : text.common.error); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    const timer = window.setInterval(async () => {
      if (polling || controller.signal.aborted) return;
      polling = true;
      try { await load(controller.signal); }
      catch (reason) { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : text.common.error); }
      finally { polling = false; }
    }, 2000);
    return () => { mounted.current = false; controller.abort(); metadata.current?.abort(); clearInterval(timer); };
  }, [category]);
  useWorkspaceRefresh(load);

  const perform = async (run: () => Promise<unknown>) => {
    if (lock.current) return;
    lock.current = true; setBusy(true); setError('');
    try { await run(); await load(); }
    catch (reason) { if (mounted.current) setError(reason instanceof Error ? reason.message : text.common.error); throw reason; }
    finally { lock.current = false; if (mounted.current) setBusy(false); }
  };
  const inspect = async (run: (signal: AbortSignal) => Promise<void>) => {
    metadata.current?.abort(); const controller = new AbortController(); metadata.current = controller;
    await perform(() => run(controller.signal)).catch(() => {});
  };
  const resolve = (input: Parameters<typeof api.resolvePackage>[0]) => inspect(async signal => {
    setPreview(null);
    const result = await api.resolvePackage(input, signal);
    if (!signal.aborted) { setPreview(result); setInstallRequest(crypto.randomUUID()); }
  });
  const choose = (repository: string) => inspect(async signal => {
    setDiscovery(null); setPreview(null);
    const found = await api.discoverPackage(repository, signal);
    const choices = found.choices.filter(c => c.supported);
    if (!choices.length || signal.aborted) {
      if (!signal.aborted) setError(copy.emptyVariants);
      return;
    }
    setDiscovery(found); setVariant('');
    if (choices.length !== 1) return;
    const choice = choices[0]; setVariant(choice.id);
    const result = await api.resolvePackage({ repository: found.repository, revision: found.revision, variant: choice.id, architecture: choice.architecture }, signal);
    if (!signal.aborted) { setPreview(result); setInstallRequest(crypto.randomUUID()); }
  });
  const accepted = operations.find(o => o.request_id === installRequest);
  const availableOperations = operations.filter(o => !o.discarded && (o.state !== 'complete' || o.retained_bytes > 0));

  const downloadDetails = preview && <div className="package-download-details">
    <dl className="package-facts"><dt>{copy.transfer}</dt><dd>{formatBytes(preview.preflight.transfer_bytes)}</dd><dt>{copy.space}</dt><dd>{formatBytes(preview.preflight.required_free_bytes)}</dd><dt>{copy.license}</dt><dd>{preview.resolution.manifest.license}</dd></dl>
    {(preview.preflight.warnings ?? []).map((warning, index) => <p className="model-readiness" key={index}>{warning}</p>)}
    <details><summary>{copy.files} ({preview.resolution.manifest.artifacts.length})</summary><ul className="package-files">{preview.resolution.manifest.artifacts.map(a => <li key={a.path}><span>{a.path} · {a.license}</span><span>{formatBytes(a.size)}</span></li>)}</ul></details>
    {(preview.resolution.source_notices ?? []).map((notice, index) => <details key={index}><summary>{copy.license}</summary><p>{notice}</p></details>)}
    <div className="model-actions"><button className="primary-button" disabled={!admin || busy || !!accepted} onClick={() => void perform(async () => { await api.installPackage(preview.id, installRequest); setPreview(null); }).catch(() => {})}>{accepted ? copy[accepted.state] : text.models.download}</button></div>
  </div>;

  return <>
    <div className="model-info-banner" role="status"><strong>{category === 'speech_recognition' ? copy.speech_recognition : copy.speech_generation}</strong><span>{copy.runtimeNotice ?? 'Choose a reviewed package to download it. Runtime readiness is shown before installation.'}</span></div>
    {error && <div className="inline-error" role="alert">{error}<button className="secondary-button" disabled={busy} onClick={() => void perform(() => load()).catch(() => {})}>{text.common.retry}</button></div>}
    <section className="model-section" aria-label={text.models.installed} aria-busy={loading}>
      <div className="section-heading"><h2>{text.models.installed}</h2><button className="secondary-button" disabled={busy} onClick={() => void perform(() => load()).catch(() => {})}>{text.common.refresh}</button></div>
      {!loading && !installed.length && <p className="compact-empty">{text.models.empty}</p>}
      <div className="catalog-grid">{installed.map(item => {
        const source = operations.find(o => o.state === 'complete' && !o.discarded && packageOf(o)?.id === item.id && packageOf(o)?.revision === item.revision);
        const state = item.package!;
        const inUse = !!state.in_use || !!state.operation;
        return <article className="catalog-card package-card" key={`${item.id}/${item.revision}`}>
          <h3>{item.name}</h3><p className="model-identity">{item.id} · {item.revision}</p>
          <p>{state.integrity === 'checked' ? readiness.checked : state.integrity === 'failed' ? readiness.failed : readiness.unchecked}</p>
          <p className="model-readiness">{state.issue || (state.runtime_compatible ? speechRuntimeText(locale)[state.smoke_tested ? 'tested' : 'available'] : speechRuntimeText(locale).unavailable)}</p>
          {state.manifest && <details><summary>{copy.source} · {copy.license}</summary><p className="model-identity">{source?.provenance.repository ?? item.provenance.kind} · {item.revision}</p><p>{state.manifest.license}</p><p>{state.manifest.runtime.adapter} · {state.manifest.runtime.revision}</p></details>}
          {!source && <p>{copy.localRepair}</p>}
          <div className="model-actions">
            <button className="secondary-button" disabled={!admin || busy || inUse} onClick={() => void perform(() => api.verifyModelPackage(item.id, item.revision!, new AbortController().signal)).catch(() => {})}>{text.models.verify}</button>
            {source && state.integrity === 'failed' && <button className="secondary-button" disabled={!admin || busy || inUse} onClick={() => { const requestID = crypto.randomUUID(); setConfirm({ title: `${copy.repair}: ${item.name}`, body: `${copy.transfer}: ${formatBytes(source.bytes_total)}. ${copy.runtime}`, run: () => api.repairPackage(source.id, requestID) }); }}>{copy.repair}</button>}
            <button className="danger-button" disabled={!admin || busy || inUse} onClick={() => setConfirm({ title: item.name, body: readiness.remove, run: () => api.removeModelPackage(item.id, item.revision!, new AbortController().signal) })}>{text.models.delete}</button>
          </div>
        </article>;
      })}</div>
    </section>
    {!!availableOperations.length && <section className="model-section" aria-label={copy.operations}>
      <h2>{copy.operations}</h2>
      {availableOperations.map(o => <article className="catalog-card package-card" key={o.id}>
        <h3>{packageOf(o)?.name}</h3><p role="status">{copy[o.state]} · {formatBytes(o.bytes_done)} / {formatBytes(o.bytes_total)}</p>
        <progress aria-label={`${packageOf(o)?.name}: ${copy[o.state]}`} value={o.bytes_done} max={o.bytes_total || 1} />
        {o.message && <p>{o.message}</p>}
        <p>{copy.retained}: {formatBytes(o.retained_bytes)}</p>
        <details><summary>{copy.files} ({o.artifacts.length})</summary><ul className="package-files">{o.artifacts.map(a => <li key={a.path}><span>{a.path}</span><span>{formatBytes(a.bytes_done)} / {formatBytes(a.bytes_total)} {a.verified ? '✓' : ''}</span></li>)}</ul></details>
        <div className="model-actions">{active(o) ? <button className="secondary-button" disabled={!admin || busy || o.state === 'cancelling'} onClick={() => void perform(() => api.controlModelOperation(o.id, 'cancel')).catch(() => {})}>{text.models.cancel}</button> : <>
          {o.state !== 'complete' && <button className="primary-button" disabled={!admin || busy} onClick={() => void perform(() => api.controlModelOperation(o.id, 'resume')).catch(() => {})}>{text.models.resume}</button>}
          <button className="secondary-button" disabled={!admin || busy} onClick={() => setConfirm({ title: copy.discard, body: copy.discardConfirm, run: () => api.controlModelOperation(o.id, 'discard') })}>{copy.discard}</button>
        </>}</div>
      </article>)}
    </section>}
    <section className="model-section" aria-label="Hugging Face">
      <div className="section-heading"><h2>Hugging Face</h2></div>
      <form className="model-search-form" onSubmit={event => { event.preventDefault(); void inspect(async signal => { setDiscovery(null); const result = await api.typedCatalog(category, query.trim(), signal); if (!signal.aborted) setRepositories(result.repositories); }); }}>
        <label className="field"><span>{text.modelSearch.query}</span><input type="search" maxLength={200} value={query} onChange={event => setQuery(event.target.value)} /></label>
        <button className="primary-button" disabled={!admin || busy || !query.trim()}>{busy ? text.common.loading : text.modelSearch.search}</button>
      </form>
      {repositories?.length === 0 && <p role="status">{text.modelSearch.empty}</p>}
      {repositories?.map(repo => <article className="model-search-result" key={repo.id}><div className="model-search-repo"><div><h3>{repo.id}</h3><p className="model-identity">{repo.size_bytes && repo.size_bytes > 0 ? formatBytes(repo.size_bytes) : text.modelSearch.unknownSize}</p></div><button className="secondary-button" disabled={!admin || busy} onClick={() => void choose(repo.id)}>{copy.preview}</button></div>
        {discovery?.repository === repo.id && discovery.choices.filter(c => c.supported).length > 1 && <label className="field"><span>{copy.variant}</span><select disabled={busy} value={variant} onChange={event => {
          const value = event.target.value; setVariant(value); setPreview(null);
          const choice = discovery.choices.find(c => c.id === value);
          if (choice) void resolve({ repository: discovery.repository, revision: discovery.revision, variant: choice.id, architecture: choice.architecture });
        }}><option value="">{copy.variant}</option>{discovery.choices.map(c => <option key={c.id} value={c.id} disabled={!c.supported}>{c.name} · {c.architecture}</option>)}</select></label>}
        {discovery?.repository === repo.id && preview?.resolution.provenance.repository === repo.id && downloadDetails}
      </article>)}
    </section>
    <section className="model-section" aria-label={text.models.catalog}>
      <h2>{text.models.catalog}</h2><div className="catalog-grid">{catalog.map(item => <article className="catalog-card package-card" key={item.id}>
        <h3>{item.name}</h3><p>{item.description}</p>
        <div className="model-meta"><span>{item.variants[0] && formatBytes(item.variants[0].size_bytes)}</span><span>{item.license}</span></div>
        <div className="catalog-actions"><button className="primary-button" disabled={!admin || busy} onClick={() => { setDiscovery(null); void resolve({ catalog_id: item.id }); }}>{copy.preview}</button></div>
        {!discovery && preview?.resolution.manifest.id === item.id && downloadDetails}
      </article>)}</div>
    </section>
    <SpeechPackages importOnly onChanged={load} />
    {confirm && <ConfirmDialog title={confirm.title} body={confirm.body} confirmLabel={confirm.title === copy.discard ? copy.discard : confirm.title.startsWith(copy.repair) ? copy.repair : undefined} close={() => setConfirm(null)} confirm={async () => { await perform(confirm.run); }} />}
  </>;
}
