import { useWorkspaceState } from '../../lib/workspace-context';
import { useEffect, useRef, useState } from 'react';
import { api, type CatalogModel, type DiscoveredModel, type DiscoveredFile, type DownloadProgress, type Model, type ModelCategory } from '../../api/client';
import { useI18n } from '../../i18n';
import { isActiveDownload, ModelDownloadProgress } from '../../components/ModelDownloadProgress';
import { formatBytes } from './model-format';
import { LegacyDownloadReview } from './LegacyDownloadReview';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { modelLibrary } from '../../i18n/model-library';

type Props = {
  category?: ModelCategory;
  models: Model[];
  progress: Record<string, DownloadProgress>;
  busy: boolean;
  download: (model: Pick<CatalogModel, 'id' | 'repo' | 'file' | 'quant'>) => Promise<void>;
  cancel: (progress: DownloadProgress) => Promise<void>;
};

export function ModelSearch({ category, models, progress, busy, download, cancel }: Props) {
  const { messages: text, locale } = useI18n();
  const copy = text.modelSearch;
  const [query, setQuery] = useWorkspaceState(`models.${category}.query`, '');
  const [results, setResults] = useWorkspaceState<DiscoveredModel[] | null>(`models.${category}.results`, null);
  const [repo, setRepo] = useWorkspaceState(`models.${category}.repo`, '');
  const [files, setFiles] = useWorkspaceState<DiscoveredFile[]>(`models.${category}.files`, []);
  const [selected, setSelected] = useWorkspaceState(`models.${category}.file`, '');
  const [loading, setLoading] = useState(false);
  const [filesLoading, setFilesLoading] = useState(false);
  const [error, setError] = useState('');
  const request = useRef<AbortController | null>(null);
  const fileRequest = useRef<AbortController | null>(null);
  useEffect(() => () => { request.current?.abort(); fileRequest.current?.abort(); }, []);

  const search = async () => {
    if (!query.trim()) return;
    request.current?.abort(); fileRequest.current?.abort();
    const controller = new AbortController(); request.current = controller;
    setLoading(true); setFilesLoading(false); setResults(null); setRepo(''); setFiles([]); setError('');
    try {
      const response = await api.searchModels(query.trim(), controller.signal, category);
      if (!controller.signal.aborted) setResults(response.results ?? []);
    } catch {
      if (!controller.signal.aborted) setError(copy.unavailable);
    } finally { if (!controller.signal.aborted) setLoading(false); }
  };
  const chooseRepo = async (id: string) => {
    fileRequest.current?.abort(); const controller = new AbortController(); fileRequest.current = controller;
    setRepo(id); setFiles([]); setSelected(''); setFilesLoading(true); setError('');
    try {
      const response = await api.modelFiles(id, controller.signal);
      if (!controller.signal.aborted) {
        const choices = response.files ?? [];
        setFiles(choices);
        setSelected(choices.find(file => file.supported && file.quant === 'Q4_K_M')?.id ?? choices.find(file => file.supported)?.id ?? '');
      }
    } catch { if (!controller.signal.aborted) setError(copy.unavailable); }
    finally { if (!controller.signal.aborted) setFilesLoading(false); }
  };
  const choice = files.find(file => file.id === selected);
  const current = choice ? progress[`${choice.id}.gguf`] : undefined;
  const installed = choice && models.some(model => model.id === choice.id);

  return <section className="model-section model-search" aria-labelledby="model-search-title">
    <div className="section-heading"><div><span className="eyebrow">Hugging Face</span><h2 id="model-search-title">{copy.title}</h2></div></div>
    <p className="model-search-hint">{copy.hint}</p>
    <form className="model-search-form" onSubmit={event => { event.preventDefault(); void search(); }}>
      <label className="field" htmlFor="model-search-query"><span>{copy.query}</span>
        <input id="model-search-query" type="search" maxLength={200} value={query} onChange={event => { request.current?.abort(); fileRequest.current?.abort(); setLoading(false); setFilesLoading(false); setQuery(event.target.value); }} placeholder={copy.query} />
      </label>
      <button className="primary-button" disabled={loading || !query.trim()}>{loading ? text.common.loading : copy.search}</button>
    </form>
    {error && <div role="alert" className="inline-error">{error}</div>}
    {results?.length === 0 && <p role="status">{copy.empty}</p>}
    <div className="model-search-results" aria-busy={loading}>
      {results?.map(result => <article className="model-search-result" key={result.id}>
        <div className="model-search-repo"><div><h3>{result.id}</h3><p>{modelLibrary(locale).repositorySize}: {modelLibrary(locale).unknown}</p></div>
          <button className="secondary-button" aria-expanded={repo === result.id} disabled={repo === result.id && filesLoading} onClick={() => void chooseRepo(result.id)}>{repo === result.id && filesLoading ? text.common.loading : modelAcquisition[locale].preview}</button></div>
        {repo === result.id && !filesLoading && <div className="model-search-files">
          {!files.some(file => file.supported) ? <p role="status">{copy.noFiles}</p> : <>
            {files.filter(file => file.supported).length > 1 && <><label htmlFor="model-search-file">{copy.file}</label>
            <select id="model-search-file" value={selected} onChange={event => setSelected(event.target.value)}>
              {!selected && <option value="">{copy.noFiles}</option>}
              {files.map(file => <option key={file.id} value={file.id} disabled={!file.supported}>{file.file} · {file.size_bytes > 0 ? formatBytes(file.size_bytes) : copy.unknownSize}{!file.supported ? ` · ${copy.unsupported}` : ''}</option>)}
            </select></>}
            {choice && <LegacyDownloadReview repository={repo} file={choice.file} bytes={choice.size_bytes} />}
            {current && current.status !== 'complete' && <ModelDownloadProgress download={current} />}
            <div className="catalog-actions">{installed ? <span className="installed-status" role="status">{text.models.installed}</span> : current && isActiveDownload(current)
              ? <button className="danger-button" disabled={busy} onClick={() => void cancel(current)}>{text.models.cancel}</button>
              : <button className="primary-button" disabled={busy || !choice?.supported || installed} onClick={() => { if (choice) void download({ id: choice.id, repo, file: choice.file, quant: choice.quant }); }}>{installed ? text.models.installed : current && current.bytes_done > 0 && ['failed', 'cancelled'].includes(current.status) ? text.models.resume : text.models.download}</button>}
            </div>
          </>}
        </div>}
      </article>)}
    </div>
  </section>;
}
