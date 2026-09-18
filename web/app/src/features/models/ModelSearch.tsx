import { useEffect, useRef, useState } from 'react';
import { api, type CatalogModel, type DiscoveredModel, type DiscoveredFile, type DownloadProgress, type Model } from '../../api/client';
import { useI18n } from '../../i18n';
import { isActiveDownload, ModelDownloadProgress } from '../../components/ModelDownloadProgress';
import { formatBytes } from './model-format';

type Props = {
  models: Model[];
  progress: Record<string, DownloadProgress>;
  busy: boolean;
  download: (model: Pick<CatalogModel, 'id' | 'repo' | 'file' | 'quant'>) => Promise<void>;
  cancel: (progress: DownloadProgress) => Promise<void>;
};

export function ModelSearch({ models, progress, busy, download, cancel }: Props) {
  const { messages: text } = useI18n();
  const copy = text.modelSearch;
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<DiscoveredModel[] | null>(null);
  const [repo, setRepo] = useState('');
  const [files, setFiles] = useState<DiscoveredFile[]>([]);
  const [selected, setSelected] = useState('');
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
      const response = await api.searchModels(query.trim(), controller.signal);
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
      <label className="sr-only" htmlFor="model-search-query">{copy.query}</label>
      <input id="model-search-query" type="search" maxLength={200} value={query} onChange={event => setQuery(event.target.value)} placeholder={copy.query} />
      <button className="primary-button" disabled={loading || !query.trim()}>{loading ? text.common.loading : copy.search}</button>
    </form>
    {error && <div role="alert" className="inline-error">{error}</div>}
    {results?.length === 0 && <p role="status">{copy.empty}</p>}
    <div className="model-search-results" aria-busy={loading}>
      {results?.map(result => <article className="model-search-result" key={result.id}>
        <div className="model-search-repo"><div><h3>{result.id}</h3><a href={`https://huggingface.co/${encodeURI(result.id)}`} target="_blank" rel="noreferrer">{copy.modelCard}</a></div>
          <button className="secondary-button" aria-expanded={repo === result.id} disabled={repo === result.id && filesLoading} onClick={() => void chooseRepo(result.id)}>{repo === result.id && filesLoading ? text.common.loading : copy.choose}</button></div>
        {repo === result.id && !filesLoading && <div className="model-search-files">
          {files.length === 0 ? <p role="status">{copy.noFiles}</p> : <>
            <label htmlFor="model-search-file">{copy.file}</label>
            <select id="model-search-file" value={selected} onChange={event => setSelected(event.target.value)}>
              {!selected && <option value="">{copy.noFiles}</option>}
              {files.map(file => <option key={file.id} value={file.id} disabled={!file.supported}>{file.file} · {file.size_bytes > 0 ? formatBytes(file.size_bytes) : copy.unknownSize}{!file.supported ? ` · ${copy.unsupported}` : ''}</option>)}
            </select>
            <p className="model-search-hint">{copy.compatibility}</p>
            {current && <ModelDownloadProgress download={current} />}
            <div className="catalog-actions">{current && isActiveDownload(current)
              ? <button className="danger-button" disabled={busy} onClick={() => void cancel(current)}>{text.models.cancel}</button>
              : <button className="primary-button" disabled={busy || !choice?.supported || installed} onClick={() => { if (choice) void download({ id: choice.id, repo, file: choice.file, quant: choice.quant }); }}>{installed ? text.models.installed : current && current.bytes_done > 0 && ['failed', 'cancelled'].includes(current.status) ? text.models.resume : text.models.download}</button>}
            </div>
          </>}
        </div>}
      </article>)}
    </div>
  </section>;
}
