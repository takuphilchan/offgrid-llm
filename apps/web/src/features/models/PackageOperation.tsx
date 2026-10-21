import type { ModelOperation } from '../../api/client';
import { useI18n } from '../../i18n';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { formatBytes } from './model-format';

export function PackageOperation({ operation, disabled, control, discard }: { operation: ModelOperation; disabled: boolean; control: (action: 'cancel' | 'resume') => void; discard: () => void }) {
  const { messages: text, locale } = useI18n(), copy = modelAcquisition[locale];
  if (operation.target.kind !== 'package') return null;
  const model = operation.target.package;
  const active = ['queued', 'downloading', 'verifying', 'activating', 'cancelling'].includes(operation.state);
  return <article className="catalog-card package-card" data-operation-id={operation.id}>
    <h3>{model.name}</h3><p className="model-identity">{model.id} · {model.revision}</p>
    <p role="status">{copy[operation.state]} · {formatBytes(operation.bytes_done)} / {formatBytes(operation.bytes_total)}</p>
    <progress aria-label={`${model.name}: ${copy[operation.state]}`} value={operation.bytes_done} max={operation.bytes_total || 1} />
    {operation.message && <p>{operation.message}</p>}
    <p>{copy.retained}: {formatBytes(operation.retained_bytes)}</p>
    <details><summary>{copy.files} ({operation.artifacts.length})</summary>
      <p className="model-identity">{copy.source}: {operation.provenance.repository} · {operation.provenance.revision}</p>
      <ul className="package-files">{operation.artifacts.map(a => <li key={a.path}><span>{a.path}</span><span>{formatBytes(a.bytes_done)} / {formatBytes(a.bytes_total)} {a.verified ? '✓' : ''}</span></li>)}</ul>
    </details>
    <div className="model-actions">{active ? <button className="secondary-button" disabled={disabled || operation.state === 'cancelling'} onClick={() => control('cancel')}>{text.models.cancel}</button> : <>
      {operation.state !== 'complete' && <button className="primary-button" disabled={disabled} onClick={() => control('resume')}>{text.models.resume}</button>}
      <button className="secondary-button" disabled={disabled} onClick={discard}>{copy.discard}</button>
    </>}</div>
  </article>;
}
