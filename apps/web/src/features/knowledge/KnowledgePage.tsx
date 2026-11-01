import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { presentation } from '../../i18n/presentation';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useMemo, useRef, useState } from 'react';
import { api, type CatalogModel, type Document, type DownloadProgress, type Model, type RAGStatus } from '../../api/client';
import { Icon } from '../../components/Icon';
import { isActiveDownload, ModelDownloadProgress } from '../../components/ModelDownloadProgress';
import { useI18n } from '../../i18n';
import { workflow } from '../../i18n/workflow';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { DocumentSourceDialog } from './DocumentSourceDialog';
import { SectionHeading, ListSearch, EmptyState } from '../../components/WorkspacePresentation';
import { UtilityPopover } from '../../components/UtilityPopover';
import { workspaceManagement } from '../../i18n/workspace-management';
import { useSetupHandoff } from '../../lib/setup-handoff';
import { knowledgeScope } from '../../i18n/knowledge-scope';
import { ModelChoice } from '../../components/ModelChoice';
import { modelLibrary } from '../../i18n/model-library';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { PermissionNotice } from '../../components/PermissionNotice';
import { readableModelName } from '../models/model-display';
import '../chat/knowledge-scope.css';

type Props = {
  models: Model[];
  onModelsChanged: () => Promise<void>;
  onOpenModels: () => void;
};

export function KnowledgePage(props: Props) {
  const { knowledge } = useWorkspace();
  return knowledge ? <KnowledgeWorkspace {...props} /> : <PermissionNotice />;
}

function KnowledgeWorkspace({ models, onModelsChanged, onOpenModels }: Props) {
  const { messages: text, locale } = useI18n();
  const copy = workflow[locale];
  const management = workspaceManagement(locale);
  const scopeCopy = knowledgeScope[locale];
  const handoff = useSetupHandoff();
  const permissions = useWorkspace();
  const [deleting, setDeleting] = useState<Document | null>(null);
  const [sourceID, setSourceID] = useState('');
  const [query, setQuery] = useWorkspaceState('knowledge.query', '');
  const [documents, setDocuments] = useState<Document[]>([]);
  const [selection, setSelection] = useWorkspaceState<string[]>('knowledge.selection', []);
  const [scopedReady, setScopedReady] = useState(false);
  useEffect(() => { let active = true; void api.systemIdentity().then(identity => { if (active) setScopedReady(identity.capabilities?.includes('scoped-knowledge-chat-v1') === true); }).catch(() => {}); return () => { active = false; }; }, []);
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
  const refreshRevision = useRef(0);
  const preparing = useRef(false);

  const installedEmbeddings = useMemo(() => models.filter(model => model.type === 'embedding'), [models]);
  const embeddingCatalog = useMemo(() => catalog.filter(model => model.type === 'embedding'), [catalog]);
  const selectedInstalled = installedEmbeddings.some(model => model.id === embeddingModel);

  const refresh = async () => {
    const revision = ++refreshRevision.current;
    setBusy(true);
    setError('');
    try {
      const [list, state, catalogResult, transfers] = await Promise.allSettled([api.documents(), api.ragStatus(), api.catalog(), permissions.admin ? api.downloadProgress() : Promise.resolve({} as Record<string, DownloadProgress>)]);
      if (revision !== refreshRevision.current) return;
      if (list.status === 'fulfilled') setDocuments(list.value.documents);
      const nextStatus = state.status === 'fulfilled' ? state.value : null;
      setStatus(nextStatus);
      const nextCatalog = catalogResult.status === 'fulfilled' ? catalogResult.value : catalog;
      setCatalog(nextCatalog);
      const selected = embeddingModel || nextStatus?.embedding_model || installedEmbeddings[0]?.id || nextCatalog.find(model => model.id === 'bge-m3')?.id || nextCatalog.find(model => model.type === 'embedding')?.id || '';
      setEmbeddingModel(selected);
      if (transfers.status === 'fulfilled' && !preparing.current) {
        const setup = Object.values(transfers.value).filter(item => item.enable_knowledge && item.status !== 'complete' && (item.model_id === selected || item.file_name === `${selected}.gguf`)).sort((a,b) => b.started_at - a.started_at)[0];
        setDownload(setup ?? null);
        setDownloadKey(setup && isActiveDownload(setup) ? setup.file_name : '');
        setSetupBusy(isActiveDownload(setup));
      }
      const failed = [list, state, catalogResult, transfers].find(item => item.status === 'rejected');
      if (failed?.status === 'rejected') setError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      if (revision === refreshRevision.current) setBusy(false);
    }
  };

  useEffect(() => { void refresh(); }, []);
  useWorkspaceRefresh(refresh);
  useEffect(() => {
    if (!downloadKey) return;
    let stopped = false;
    let polling = false;
    const poll = async () => {
      if (polling) return;
      polling = true;
      try {
        const progress = (await api.downloadProgress())[downloadKey];
        if (stopped) return;
        if (!progress) {
          setDownloadKey(''); setSetupBusy(false); setDownload(null);
          setError(text.knowledgeSetup.failed);
          return;
        }
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
      } finally { polling = false; }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), 1200);
    return () => { stopped = true; window.clearInterval(timer); };
  }, [downloadKey, onModelsChanged]);

  const prepare = async () => {
    if (!embeddingModel || setupBusy || preparing.current) return;
    preparing.current = true;
    setSetupBusy(true);
    setError('');
    try {
      const model = embeddingCatalog.find(item => item.id === embeddingModel) ?? (selectedInstalled ? { id: embeddingModel, repo: '', file: `${embeddingModel}.gguf`, quant: '', size_bytes: 0 } : undefined);
      if (!model) throw new Error('Selected embedding model is not available in the catalog.');
      const accepted = await api.downloadModel(model, true);
      preparing.current = false;
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
    } finally { preparing.current = false; }
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
  const readiness = (document: Document) => document.index_status === 'ready' ? management.indexed : ['indexing', 'building'].includes(document.index_status ?? '') ? management.indexing : document.index_status === 'failed' ? management.indexFailed : presentation[locale].unknown;
  return <div className="stack knowledge-workspace">
    {!permissions.admin && <p className="permission-notice">{interaction[locale].adminOnly}</p>}
	{typeof status?.stats?.index_error === 'string' && <div className="error-banner" role="alert">{status.stats.index_error}</div>}
    {sourceID && <DocumentSourceDialog id={sourceID} close={() => setSourceID('')} />}
    {deleting && <ConfirmDialog title={deleting.name} body={copy.deleteDocument} close={() => setDeleting(null)} confirm={async () => { await api.deleteDocument(deleting.id); await refresh(); }} />}
    <SectionHeading title={`${management.documents} · ${documents.length}`} description={text.guidance.knowledge} actions={<>
      {enabled && <UtilityPopover label={management.manageKnowledge} triggerClassName="utility-button">
        <p className="knowledge-model">{text.knowledgeSetup.active}: {status?.embedding_model || presentation[locale].unknown}</p>
        {permissions.admin && <button className="secondary-button" disabled={busy || reindexing !== ''} onClick={() => { setBusy(true); void api.disableRAG().then(refresh).catch(reason => { setError(reason instanceof Error ? reason.message : text.common.error); setBusy(false); }); }}>{copy.disableKnowledge}</button>}
      </UtilityPopover>}
      <button className="primary-button" onClick={() => input.current?.click()} disabled={busy || !enabled || !permissions.admin}><Icon name="upload" size={17} />{busy ? text.common.loading : text.knowledge.add}</button>
    </>} />
    <input ref={input} hidden type="file" onChange={event => void upload(event.target.files?.[0])} />
    {permissions.admin && !enabled && !busy && status !== null && <section className="knowledge-setup">
      <div className="knowledge-setup-copy"><div className="model-glyph"><Icon name="knowledge" /></div><div><span className="eyebrow">{text.knowledgeSetup.title}</span><h2>{text.knowledge.disabled}</h2><p>{text.knowledgeSetup.body}</p></div></div>
      <div className="knowledge-setup-controls">
        <ModelChoice label={text.knowledgeSetup.model} placeholder={text.knowledgeSetup.model} missing={modelLibrary(locale).unknown} value={embeddingModel} disabled={setupBusy} onChange={value => { setEmbeddingModel(value); setDownload(null); setError(''); }}
          options={[
            ...installedEmbeddings.map(model => ({ id: model.id, name: readableModelName(model.id), details: `${text.models.installed} · ${model.loaded ? modelLibrary(locale).loaded : modelLibrary(locale).unchecked}` })),
            ...embeddingCatalog.filter(model => !installedEmbeddings.some(local => local.id === model.id)).map(model => ({ id: model.id, name: model.name, details: `${text.models.catalog} · ${model.size_bytes > 0 ? formatBytes(model.size_bytes) : modelLibrary(locale).unknown}${model.license ? ` · ${model.license}` : ''}` })),
          ]} />
        <button className="primary-button" onClick={() => void prepare()} disabled={(!selectedInstalled && !embeddingCatalog.some(model => model.id === embeddingModel)) || setupBusy}>{setupBusy ? text.knowledgeSetup.preparing : selectedInstalled ? text.knowledgeSetup.enable : text.knowledgeSetup.installEnable}</button>
        <button className="text-button" onClick={onOpenModels}>{text.knowledgeSetup.openModels}</button>
        {download && download.status !== 'complete' && <ModelDownloadProgress download={download} />}
      </div>
    </section>}

    {!enabled && documents.length > 0 && <p>{copy.offlineLibrary}</p>}
    {error && <div className="inline-error" role="alert">{error}<button onClick={() => void refresh()}>{text.common.retry}</button></div>}
    <div className="knowledge-library-toolbar">
    {documents.length > 0 && <ListSearch id="knowledge-document-search" label={management.searchDocuments} value={query} onChange={setQuery} />}
    {enabled && !selection.length && <button className="secondary-button" aria-describedby="knowledge-retrieval-scope" disabled={busy || !documents.length || !!status?.stats?.index_error} onClick={() => handoff.askKnowledge()}>{management.askKnowledge}</button>}
    {selection.length > 0 && <div className="knowledge-selection">
      <button className="primary-button" disabled={busy || !enabled || !scopedReady || !!status?.stats?.index_error || selection.some(id => !documents.some(doc => doc.id === id && doc.index_status === 'ready'))} onClick={() => handoff.askKnowledge(selection.map(id => ({ id, name: documents.find(doc => doc.id === id)!.name })))}>{scopeCopy.ask} · {selection.length}/16</button>
      <button className="text-button" onClick={() => setSelection([])}>{text.models.cancel}</button>
      {!scopedReady && <p role="status">{scopeCopy.unsupported}</p>}
    </div>}
    </div>
    {enabled && !selection.length && <p id="knowledge-retrieval-scope" className="workspace-secondary knowledge-scope-hint">{management.retrievalScope}</p>}
    {busy && documents.length === 0 ? <SkeletonCards /> : shownDocuments.length === 0 ? !error && <EmptyState title={query ? management.noDocuments : text.knowledge.empty} action={query ? <button type="button" className="secondary-button" onClick={() => { setQuery(''); document.getElementById('knowledge-document-search')?.focus(); }}>{workspaceRefinement[locale].clear}</button> : undefined} /> : <div className="knowledge-documents">{shownDocuments.map(document => <article className="resource-card" key={document.id}><div className="file-icon"><Icon name="knowledge" /></div><div className="resource-body"><h3>{document.name}</h3><p>{document.content_type || text.common.document} · {formatBytes(document.size)}</p><p className="document-readiness">{readiness(document)}</p>{document.last_error && <p role="status">{document.last_error}</p>}<div className="resource-actions"><label className="knowledge-source-select"><input type="checkbox" checked={selection.includes(document.id)} disabled={!selection.includes(document.id) && (document.index_status !== 'ready' || selection.length >= 16)} onChange={event => setSelection(current => event.target.checked ? [...current, document.id] : current.filter(id => id !== document.id))} aria-label={scopeCopy.select + ': ' + document.name} />{scopeCopy.select}</label><span className={document.source_retained ? 'source-state retained' : 'source-state'}>{document.source_retained ? text.knowledge.retained : text.knowledge.legacy}</span><button className="text-button" disabled={!document.source_retained} title={!document.source_retained ? copy.sourceMissing : undefined} onClick={() => setSourceID(document.id)}>{copy.viewSource}</button><UtilityPopover label={`${management.manageDocument}: ${document.name}`} trigger={management.manageDocument} triggerClassName="utility-button"><p>{document.chunk_count} {text.knowledge.chunks}</p><button className="secondary-button" disabled={!permissions.admin || !document.source_retained || !enabled || reindexing !== ''} onClick={() => void reindex(document)}>{reindexing === document.id ? text.knowledge.reindexing : text.knowledge.reindex}</button><button className="danger-button" disabled={!permissions.admin || busy || reindexing !== ''} onClick={() => setDeleting(document)}>{text.models.delete}</button></UtilityPopover></div>{!document.source_retained && <p className="workspace-secondary">{copy.sourceMissing}</p>}</div></article>)}</div>}
  </div>;
}

function SkeletonCards() { return <div className="card-grid">{[1, 2, 3].map(item => <div className="skeleton-card" key={item}><i /><div><span /><span /></div></div>)}</div>; }
function formatBytes(bytes: number) { if (!bytes) return '0 B'; const units = ['B', 'KB', 'MB', 'GB', 'TB']; const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`; }
