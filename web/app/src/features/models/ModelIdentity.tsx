import { useI18n } from '../../i18n';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { modelLibrary } from '../../i18n/model-library';
import { formatBytes } from './model-format';
import type { ModelDisplay } from './model-display';
import './model-library.css';

// One identity/size grammar. The caller still owns its exact operation API and
// package readiness; an installed file never becomes a runtime qualification.
export function ModelIdentity({ model, select, selected }: { model: ModelDisplay; select?: () => void; selected?: boolean }) {
  const { locale } = useI18n(), copy = modelLibrary(locale), acquisition = modelAcquisition[locale];
  const size = model.bytes === undefined ? copy.unknown : `${formatBytes(model.bytes)}${model.maximumBytes && model.maximumBytes !== model.bytes ? ` – ${formatBytes(model.maximumBytes)}` : ''}`;
  return <div className="model-display">
    {select ? <button className="secondary-button installed-model-main" aria-label={model.id} aria-pressed={selected} onClick={select}><strong>{model.name}</strong></button> : <h3>{model.name}</h3>}
    <div className="model-summary">
      <p className="model-meta"><span>{model.category ? acquisition[model.category] : copy.unknown}</span>{model.runtime && <span>{copy[model.runtime]}</span>}</p>
      <p className="model-size">{model.sizeMeaning === 'installed' ? copy.installedSize : acquisition.transfer}: {size}</p>
    </div>
    <details className="model-display-details"><summary>{copy.details}</summary>
      <p className="model-identity">{model.id}{model.revision ? ` · ${model.revision}` : ''}</p>
      <p>{acquisition.source}: {model.source || copy.unknown}</p>
      {model.quant && <p>{model.quant}</p>}
      <p>{acquisition.license}: {model.license || copy.unknown}</p>
    </details>
  </div>;
}
