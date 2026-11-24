import {LibraryPagination} from './LibraryPagination';
import './knowledge-library.css';
import {DocumentComparison} from './DocumentComparison';
import {DocumentSynthesis} from './DocumentSynthesis';
import {documentSynthesis} from '../../i18n/document-synthesis';
import {documentComparison} from '../../i18n/document-comparison';
import {CollectionManager} from './CollectionManager';
import {DocumentMove} from './DocumentMove';
import {collectionManagement} from '../../i18n/collection-management';
import {collectionErrorMessage} from '../../i18n/collection-errors';
import type {CollectionPolicy} from '../../api/collections';
import { DocumentEvidenceNotice } from '../../components/DocumentEvidenceNotice';
import {clientMessage} from '../../i18n/client-errors';
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
import { DocumentRetentionDialog } from './DocumentRetentionDialog';
import { documentRetention } from '../../i18n/document-retention';
import { DocumentSourceDialog } from './DocumentSourceDialog';
import { SectionHeading, ListSearch, EmptyState } from '../../components/WorkspacePresentation';
import { UtilityPopover } from '../../components/UtilityPopover';
import { workspaceManagement } from '../../i18n/workspace-management';
import { useSetupHandoff } from '../../lib/setup-handoff';
import { knowledgeScope } from '../../i18n/knowledge-scope';
import { ModelChoice } from '../../components/ModelChoice';
import { ActionCard } from '../../components/ActionCard';
import { modelLibrary } from '../../i18n/model-library';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { PermissionNotice } from '../../components/PermissionNotice';
import { readableModelName } from '../models/model-display';
import { KnowledgeImport } from './KnowledgeImport';
import { knowledgeImport } from '../../i18n/knowledge-import';
import { knowledgeStart } from '../../i18n/knowledge-start';
import { knowledgeGuidance } from '../../i18n/knowledge-options';
import { libraryScope } from '../../i18n/workspace-audit';
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
  const { messages: text, locale , language} = useI18n();
  const copy = workflow[locale];
  const start = knowledgeStart(locale);
  const guidance = knowledgeGuidance(locale);
  const management = workspaceManagement(locale);
  const scopeCopy = knowledgeScope[locale];
  const handoff = useSetupHandoff();
  const permissions = useWorkspace();
  const [deleting, setDeleting] = useState<Document | null>(null);
  const [sourceID, setSourceID] = useState('');
  const [retentionDocument, setRetentionDocument] = useState<Document | null>(null);
  const [collectionsReady,setCollectionsReady]=useState(false),[collections,setCollections]=useState<CollectionPolicy[]>([]),[collectionError,setCollectionError]=useState('');
  const [collection,setCollection]=useWorkspaceState('knowledge.collection',''),[managing,setManaging]=useState(false),[moving,setMoving]=useState<Document|null>(null);
  const collectionCopy=collectionManagement(locale);
  const [retentionReady, setRetentionReady] = useState(false);
  const [libraryPage,setLibraryPage]=useWorkspaceState('knowledge.page',1);
  const [query, setQuery] = useWorkspaceState('knowledge.query', '');
  const previousFilter = useRef({query,collection});
  const [documents, setDocuments] = useState<Document[]>([]);
  const [importActionTarget, setImportActionTarget] = useState<HTMLDivElement | null>(null);
  const [selection, setSelection] = useWorkspaceState<string[]>('knowledge.selection', []);
  const [scopedReady, setScopedReady] = useState(false);
  const [durableImports, setDurableImports] = useState(false);
  const [comparisonReady,setComparisonReady]=useState(false),[comparing,setComparing]=useState(false);
  const [synthesisReady,setSynthesisReady]=useState(false),[analysing,setAnalysing]=useState(false);
  useEffect(()=>{ const route=()=>{setComparing(comparisonReady&&location.hash==='#/knowledge/compare');setAnalysing(synthesisReady&&['#/knowledge/summary','#/knowledge/requirements'].includes(location.hash));};route();window.addEventListener('hashchange',route);return()=>window.removeEventListener('hashchange',route);},[comparisonReady,synthesisReady]);
  useEffect(()=>{let active=true;void api.systemIdentity().then(identity=>{if(active)setSynthesisReady(identity.capabilities?.includes('document-synthesis-candidates-v1')===true);}).catch(()=>{});return()=>{active=false;};},[]);
  useEffect(()=>{let active=true;void api.systemIdentity().then(identity=>{if(active)setComparisonReady(identity.capabilities?.includes('document-comparison-jobs-v1')===true&&identity.capabilities?.includes('document-analysis-history-v1')===true);}).catch(()=>{});return()=>{active=false;};},[]);
  useEffect(() => { let active = true; void api.systemIdentity().then(identity => { if (active) { setCollectionsReady(identity.capabilities?.includes('knowledge-collections-v1')===true); setRetentionReady(identity.capabilities?.includes('knowledge-retention-v1') === true); setScopedReady(identity.capabilities?.includes('scoped-knowledge-chat-v1') === true); setDurableImports(identity.capabilities?.includes('document-imports-v1') === true); } }).catch(() => {}); return () => { active = false; }; }, []);
  const [status, setStatus] = useState<RAGStatus | null>(null);
  const [catalog, setCatalog] = useState<CatalogModel[]>([]);
  const [embeddingModel, setEmbeddingModel] = useState('');
  const [busy, setBusy] = useState(true);
  const [setupBusy, setSetupBusy] = useState(false);
  const [error, setError] = useState('');
  const [reindexing, setReindexing] = useState('');
  const [downloadKey, setDownloadKey] = useState('');
  const [download, setDownload] = useState<DownloadProgress | null>(null);
  const refreshRevision = useRef(0);
  const preparing = useRef(false);

  const collectionRevision=useRef(0);
  const refreshCollections=async()=>{const revision=++collectionRevision.current;try{const list=await api.collections.list();if(revision!==collectionRevision.current)return;setCollections(list);setCollectionError('');setCollection(current=>list.some(item=>item.id===current)?current:'');}catch{if(revision===collectionRevision.current){setCollections([]);setCollectionError(collectionErrorMessage());}}};
  useEffect(()=>{if(collectionsReady)void refreshCollections();return()=>{collectionRevision.current++;};},[collectionsReady]);
  const collectionsChanged=()=>{void refreshCollections();void refresh();};
  const installedEmbeddings = useMemo(() => models.filter(model => model.type === 'embedding'), [models]);
  const embeddingCatalog = useMemo(() => catalog.filter(model => model.type === 'embedding'), [catalog]);
  const selectedInstalled = installedEmbeddings.some(model => model.id === embeddingModel);

  const refresh = async () => {
    const revision = ++refreshRevision.current;
    setBusy(true);
    setError('');
    try {
      const [list, state] = await Promise.allSettled([api.documents(), api.ragStatus()]);
      if (revision !== refreshRevision.current) return;
      if (list.status === 'fulfilled') {
        setDocuments(list.value.documents);
        setSelection(current => current.filter(id => list.value.documents.some(doc => doc.id === id)));
      } else {
        // Failed refresh must not keep inaccessible titles or actionable stale
        // source selections visible from a previous successful request.
        setDocuments([]);
        setSelection([]);
      }
      const nextStatus = state.status === 'fulfilled' ? state.value : null;
      setStatus(nextStatus);
      setEmbeddingModel(current => current || nextStatus?.embedding_model || installedEmbeddings[0]?.id || '');
      if (nextStatus && installedEmbeddings.length) setBusy(false);
      // A working library does not depend on catalog/download administration.
      // Fetch setup choices only when setup is actually needed, and never let
      // an optional catalog failure hide an installed search model.
      const needsSetup = permissions.admin && nextStatus?.enabled === false;
      const [catalogResult, transfers] = await Promise.allSettled([
        needsSetup ? api.catalog() : Promise.resolve(catalog),
        needsSetup ? api.downloadProgress() : Promise.resolve({} as Record<string, DownloadProgress>),
      ]);
      if (revision !== refreshRevision.current) return;
      const nextCatalog = catalogResult.status === 'fulfilled' ? catalogResult.value : catalog;
      setCatalog(nextCatalog);
      const selected = embeddingModel || nextStatus?.embedding_model || installedEmbeddings[0]?.id || nextCatalog.find(model => model.id === 'bge-m3')?.id || nextCatalog.find(model => model.type === 'embedding')?.id || '';
      setEmbeddingModel(current => current || selected);
      if (transfers.status === 'fulfilled' && !preparing.current) {
        const setup = Object.values(transfers.value).filter(item => item.enable_knowledge && item.status !== 'complete' && (item.model_id === selected || item.file_name === `${selected}.gguf`)).sort((a,b) => b.started_at - a.started_at)[0];
        setDownload(setup ?? null);
        setDownloadKey(setup && isActiveDownload(setup) ? setup.file_name : '');
        setSetupBusy(isActiveDownload(setup));
      }
      const catalogRequired = needsSetup && !installedEmbeddings.length && !nextCatalog.some(model => model.type === 'embedding');
      const failed = [list, state, ...(catalogRequired ? [catalogResult] : [])].find(item => item.status === 'rejected');
      if (failed?.status === 'rejected') setError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      if (revision === refreshRevision.current) setBusy(false);
    }
  };

  useEffect(() => { void refresh(); }, []);
  useWorkspaceRefresh(async()=>{await Promise.all([refresh(),collectionsReady?refreshCollections():Promise.resolve()]);});
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
      if (!model) throw new Error(clientMessage('embedding'));
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

  const reindex = async (document: Document) => {
    if (reindexing) return;
    setReindexing(document.id);
    setError('');
    try { await api.reindexDocument(document.id); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setReindexing(''); }
  };

  const enabled = status?.enabled === true;
  const shownDocuments = documents.filter(document => (!collection||document.collection_id===collection)&&document.name.toLocaleLowerCase(language).includes(query.trim().toLocaleLowerCase(language)));
  useEffect(()=>{
    if(previousFilter.current.query!==query||previousFilter.current.collection!==collection)setLibraryPage(1);
    previousFilter.current={query,collection};
  },[query,collection]);
  const pageCount=Math.max(1,Math.ceil(shownDocuments.length/30)),currentPage=Math.min(libraryPage,pageCount);
  const visibleDocuments=shownDocuments.slice((currentPage-1)*30,currentPage*30);
  const readiness = (document: Document) => document.index_status === 'ready' ? management.indexed : ['indexing', 'building'].includes(document.index_status ?? '') ? management.indexing : document.index_status === 'failed' ? management.indexFailed : presentation[locale].unknown;
  return <><div className="stack knowledge-workspace" hidden={comparing||analysing} style={comparing||analysing?{display:"none"}:undefined}>
    {!permissions.admin && status?.enabled === false && <p className="permission-notice">{interaction[locale].adminOnly}</p>}
	{typeof status?.stats?.index_error === 'string' && <div className="error-banner" role="alert">{status.stats.index_error}</div>}
    {managing&&<CollectionManager key={JSON.stringify(["collections",permissions.scope,permissions.workspace])} close={()=>setManaging(false)} changed={collectionsChanged} initial={collection}/>}
    {moving&&<DocumentMove key={JSON.stringify([permissions.scope,permissions.workspace,moving.id])} source={moving} close={()=>setMoving(null)} changed={collectionsChanged}/>}
    {retentionDocument && <DocumentRetentionDialog id={retentionDocument.id} name={retentionDocument.name} close={() => setRetentionDocument(null)} />}
    {sourceID && <DocumentSourceDialog id={sourceID} close={() => setSourceID('')} />}
    {deleting && <ConfirmDialog title={deleting.name} body={copy.deleteDocument} close={() => setDeleting(null)} confirm={async () => { await api.deleteDocument(deleting.id); await refresh(); }} />}
    <SectionHeading title={documents.length ? `${management.documents} · ${documents.length}` : management.documents} description={documents.length ? guidance.intro : libraryScope[locale]} actions={<>
      {permissions.admin && <div ref={setImportActionTarget} className="knowledge-heading-import" />}
      {enabled && <UtilityPopover compact label={management.manageKnowledge} trigger={<Icon name="more" />} triggerClassName="utility-button icon-button" panelClassName="knowledge-management-menu" align="end">
        <div className="knowledge-menu-status"><span>{text.knowledgeSetup.active}</span><strong>{status?.embedding_model || presentation[locale].unknown}</strong></div>
        {permissions.admin && <button className="secondary-button" disabled={busy || reindexing !== ''} onClick={() => { setBusy(true); void api.disableRAG().then(refresh).catch(reason => { setError(reason instanceof Error ? reason.message : text.common.error); setBusy(false); }); }}><Icon name="power" size={16} />{copy.disableKnowledge}</button>}
      </UtilityPopover>}
    </>} />
    {permissions.admin && !enabled && !busy && status !== null && <section className="knowledge-setup">
      <div className="knowledge-setup-copy"><div className="model-glyph"><Icon name="knowledge" /></div><div><h2>{start.title}</h2><p>{start.body}</p></div></div>
      <div className="knowledge-setup-controls">
        {selectedInstalled || embeddingCatalog.some(model => model.id === embeddingModel) ? <>
        {!selectedInstalled && <p>{start.model}: <strong>{readableModelName(embeddingModel) || modelLibrary(locale).unknown}</strong></p>}
        {!selectedInstalled && <p>{text.models.catalog} · {(embeddingCatalog.find(model => model.id === embeddingModel)?.size_bytes ?? 0) > 0 ? formatBytes(embeddingCatalog.find(model => model.id === embeddingModel)!.size_bytes) : modelLibrary(locale).unknown}</p>}
        <button className="primary-button" onClick={() => void prepare()} disabled={(!selectedInstalled && !embeddingCatalog.some(model => model.id === embeddingModel)) || setupBusy}>{setupBusy ? text.knowledgeSetup.preparing : selectedInstalled ? text.knowledgeSetup.enable : text.knowledgeSetup.installEnable}</button>
        </> : <button className="primary-button" onClick={onOpenModels}>{text.knowledgeSetup.openModels}</button>}
        <details className="knowledge-search-options"><summary>{knowledgeImport(locale).changeModel}</summary>
        {selectedInstalled && <p>{start.model}: <strong>{readableModelName(embeddingModel)}</strong></p>}
        <ModelChoice label={start.model} placeholder={start.model} missing={modelLibrary(locale).unknown} value={embeddingModel} disabled={setupBusy} onChange={value => { setEmbeddingModel(value); setDownload(null); setError(''); }}
          options={[
            ...installedEmbeddings.map(model => ({ id: model.id, name: readableModelName(model.id), details: `${text.models.installed} · ${model.loaded ? modelLibrary(locale).loaded : modelLibrary(locale).unchecked}` })),
            ...embeddingCatalog.filter(model => !installedEmbeddings.some(local => local.id === model.id)).map(model => ({ id: model.id, name: model.name, details: `${text.models.catalog} · ${model.size_bytes > 0 ? formatBytes(model.size_bytes) : modelLibrary(locale).unknown}${model.license ? ` · ${model.license}` : ''}` })),
          ]} />
        <button className="text-button" onClick={onOpenModels}>{text.knowledgeSetup.openModels}</button>
        </details>
        {download && download.status !== 'complete' && <ModelDownloadProgress download={download} />}
      </div>
    </section>}

    {!enabled && documents.length > 0 && <p>{copy.offlineLibrary}</p>}
    {error && <div className="inline-error" role="alert">{error}<button onClick={() => void refresh()}>{text.common.retry}</button></div>}
    {permissions.admin && <KnowledgeImport key={JSON.stringify([permissions.scope,permissions.workspace])} status={status} disabled={busy || !enabled || reindexing !== ''} setupDisabled={busy || reindexing !== ''} refresh={refresh} durable={durableImports} actionTarget={importActionTarget} />}
    <div className="action-card-grid knowledge-workflows">
      {comparisonReady&&<ActionCard icon="compare" title={documentComparison(locale).title} description={documentComparison(locale).choose} onClick={()=>setComparing(true)}/>}
      {synthesisReady&&<ActionCard icon="file-search" title={documentSynthesis(locale).title} description={`${documentSynthesis(locale).summary} · ${documentSynthesis(locale).requirements}`} onClick={()=>setAnalysing(true)}/>}
    </div>

    <div className="knowledge-library-toolbar">
    {collectionsReady&&<div className="collection-library-controls"><label>{collectionCopy.title}<select value={collection} onChange={event=>setCollection(event.target.value)}><option value="">{collectionCopy.all}</option>{collections.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select></label>{permissions.admin&&<button className="icon-button" aria-label={collectionCopy.title} title={collectionCopy.title} onClick={()=>setManaging(true)}><Icon name="collections" size={19}/></button>}{collectionError&&<p role="status">{collectionError}<button className="text-button" onClick={()=>void refreshCollections()}>{text.common.retry}</button></p>}</div>}
    {documents.length > 0 && <ListSearch id="knowledge-document-search" label={management.searchDocuments} value={query} onChange={setQuery} />}
    {enabled && !!documents.length && !selection.length && <button className="secondary-button" aria-describedby="knowledge-retrieval-scope" disabled={busy || !!status?.stats?.index_error} onClick={() => handoff.askKnowledge()}>{management.askKnowledge}</button>}
    {selection.length > 0 && <div className="knowledge-selection">
      <button className="primary-button" disabled={busy || !enabled || !scopedReady || !!status?.stats?.index_error || selection.some(id => !documents.some(doc => doc.id === id && doc.index_status === 'ready'))} onClick={() => handoff.askKnowledge(selection.map(id => ({ id, name: documents.find(doc => doc.id === id)!.name })))}>{scopeCopy.ask} · {selection.length}/16</button>
      <button className="text-button" onClick={() => setSelection([])}>{text.models.cancel}</button>
      {!scopedReady && <p role="status">{scopeCopy.unsupported}</p>}
    </div>}
    </div>
    {enabled && documents.length > 0 && !selection.length && <p id="knowledge-retrieval-scope" className="workspace-secondary knowledge-scope-hint">{guidance.scope}</p>}
    {busy && documents.length === 0 ? <SkeletonCards /> : shownDocuments.length === 0 ? !error && enabled && <EmptyState title={query||collection ? management.noDocuments : text.knowledge.empty} action={query||collection ? <button type="button" className="secondary-button" onClick={() => { setQuery(''); setCollection(''); document.getElementById('knowledge-document-search')?.focus(); }}>{workspaceRefinement[locale].clear}</button> : undefined} /> : <div className="knowledge-documents">{visibleDocuments.map(document => <article className="resource-card" data-selected={selection.includes(document.id)} key={document.id}><div className="file-icon"><Icon name="knowledge" /></div><div className="resource-body"><h3>{document.name}</h3><div className="knowledge-document-meta"><p>{document.content_type || text.common.document} · {formatBytes(document.size)}</p><p className="document-readiness">{readiness(document)}</p></div><DocumentEvidenceNotice evidence={document.evidence}/>{document.last_error && <p role="status">{document.last_error}</p>}<div className="resource-actions"><label className="knowledge-source-select"><input type="checkbox" checked={selection.includes(document.id)} disabled={!selection.includes(document.id) && (!document.extraction_revision && document.index_status !== 'ready' || selection.length >= 16)} onChange={event => setSelection(current => event.target.checked ? [...current, document.id] : current.filter(id => id !== document.id))} aria-label={scopeCopy.select + ': ' + document.name} />{scopeCopy.select}</label><button className="text-button" disabled={!document.source_retained} title={!document.source_retained ? copy.sourceMissing : undefined} onClick={() => setSourceID(document.id)}>{copy.viewSource}</button><UtilityPopover compact align="end" label={`${management.manageDocument}: ${document.name}`} trigger={<Icon name="more" size={18} />} triggerClassName="utility-button icon-button"><span className={document.source_retained ? 'source-state retained' : 'source-state'}>{document.source_retained ? text.knowledge.retained : text.knowledge.legacy}</span><p>{document.chunk_count} {text.knowledge.chunks}</p><button className="secondary-button" disabled={!permissions.admin || !document.source_retained || !enabled || reindexing !== ''} onClick={() => void reindex(document)}>{reindexing === document.id ? text.knowledge.reindexing : text.knowledge.reindex}</button>{permissions.admin&&collectionsReady&&<button className="text-button" disabled={busy} onClick={()=>setMoving(document)}>{collectionCopy.move}</button>}{permissions.admin && retentionReady && <button className="text-button" onClick={() => setRetentionDocument(document)}>{documentRetention(locale).title}</button>}<button className="danger-button" disabled={!permissions.admin || busy || reindexing !== ''} onClick={() => setDeleting(document)}>{text.models.delete}</button></UtilityPopover></div>{!document.source_retained && <p className="workspace-secondary">{copy.sourceMissing}</p>}</div></article>)}</div>}
    <LibraryPagination page={currentPage} pages={pageCount} onChange={setLibraryPage}/>
  </div>{comparing&&<DocumentComparison key={JSON.stringify(['comparison',permissions.scope,permissions.workspace])} initial={selection.slice(0,2)} close={()=>{setComparing(false);location.hash='#/knowledge';}}/>}
{analysing&&<DocumentSynthesis key={JSON.stringify(['synthesis',permissions.scope,permissions.workspace])} models={models} initial={selection} close={()=>{setAnalysing(false);location.hash='#/knowledge';}}/>}
</>;
}

function SkeletonCards() { return <div className="card-grid">{[1, 2, 3].map(item => <div className="skeleton-card" key={item}><i /><div><span /><span /></div></div>)}</div>; }function formatBytes(bytes: number) { if (!bytes) return '0 B'; const units = ['B', 'KB', 'MB', 'GB', 'TB']; const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1); return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`; }
