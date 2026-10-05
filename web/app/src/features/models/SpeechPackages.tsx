import { useEffect, useRef, useState } from 'react';
import { api, type ModelPackageState } from '../../api/client';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useI18n } from '../../i18n';
import { speechModels } from '../../i18n/speech-models';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { useWorkspace } from '../../lib/workspace-context';
import { formatBytes } from './model-format';
import { preparePackageImport } from './package-import';

export function SpeechPackages({ importOnly = false, onChanged }: { importOnly?: boolean; onChanged?: () => Promise<void> }) {
  const { locale, messages: text } = useI18n();
  const copy = speechModels[locale];
  const { admin } = useWorkspace();
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<ModelPackageState[]>([]);
  const [prepared, setPrepared] = useState<Awaited<ReturnType<typeof preparePackageImport>> | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [remove, setRemove] = useState<ModelPackageState | null>(null);
  const active = useRef<AbortController | null>(null);
  const selection = useRef(0);
  const load = async (signal?: AbortSignal) => setItems(await api.modelPackages(signal));
  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    void load(controller.signal).catch(reason => { if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : text.common.error); });
    return () => controller.abort();
  }, [open]);
  useEffect(() => () => { selection.current++; active.current?.abort(); }, []);

  const perform = async (operation: (signal: AbortSignal) => Promise<unknown>) => {
    if (active.current) return;
    const controller = new AbortController(); active.current = controller; setBusy(true); setError('');
    try { await operation(controller.signal); await load(controller.signal); await onChanged?.(); }
    catch (reason) { if (!controller.signal.aborted) { setError(reason instanceof Error ? reason.message : text.common.error); await load(controller.signal).catch(() => {}); throw reason; } }
    finally { active.current = null; setBusy(false); }
  };

  return <details className="model-section" open={open} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>{importOnly ? modelAcquisition[locale].advanced : copy.title}</summary>
    <div className="stack">
      <p>{copy.hint}</p>
      {error && <div role="alert" className="inline-error">{error}</div>}
      {!importOnly && <button className="secondary-button" disabled={busy} onClick={() => void perform(signal => load(signal)).catch(() => {})}>{text.common.refresh}</button>}
      {!importOnly && items.map(item => <article className="catalog-card" key={`${item.id}/${item.revision}`}>
        <h3>{item.manifest?.name ?? item.id}</h3>
        <p>{item.id} · {item.revision}</p>
        <p>{item.integrity === 'checked' ? copy.checked : item.integrity === 'failed' ? copy.failed : copy.unchecked}</p>
        <p>{copy.runtime}</p>
        {!!item.in_use && <p>{copy.inUse}: {item.in_use}</p>}
        <div className="model-actions">
          <button disabled={!admin || busy} onClick={() => void perform(signal => api.verifyModelPackage(item.id, item.revision, signal)).catch(() => {})}>{text.models.verify}</button>
          <button disabled={!admin || busy || item.in_use > 0} onClick={() => setRemove(item)}>{text.models.delete}</button>
        </div>
      </article>)}
      {admin && <>
        <label className="field"><span>{copy.folder}</span><input type="file" multiple disabled={busy} ref={node => { node?.setAttribute('webkitdirectory', ''); }} onChange={async event => {
          const version = ++selection.current; const files = Array.from(event.target.files ?? []); setPrepared(null); setError('');
          if (!files.length) return;
          try { const result = await preparePackageImport(files); if (version === selection.current) setPrepared(result); }
          catch { if (version === selection.current) setError(copy.invalid); }
        }} /></label>
        {prepared && <p>{prepared.manifest.name} · {prepared.manifest.revision} · {formatBytes(prepared.manifest.artifacts.reduce((n, artifact) => n + artifact.size, 0))}</p>}
        <div className="model-actions"><button className="primary-button" disabled={!prepared || busy} onClick={() => void perform(async signal => { if (prepared) { await api.importModelPackage(prepared.body, signal); setPrepared(null); } }).catch(() => {})}>{busy ? text.common.loading : copy.import}</button>
        {busy && <button onClick={() => active.current?.abort()}>{text.models.cancel}</button>}</div>
      </>}
      {remove && <ConfirmDialog title={remove.manifest?.name ?? remove.id} body={copy.remove} close={() => setRemove(null)} confirm={async () => { await perform(signal => api.removeModelPackage(remove.id, remove.revision, signal)); setRemove(null); }} />}
    </div>
  </details>;
}
