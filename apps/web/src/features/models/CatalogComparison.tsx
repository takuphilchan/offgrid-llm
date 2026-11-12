import type { CatalogModel } from '../../api/client';
import { useI18n } from '../../i18n';
import { modelComparison } from '../../i18n/model-comparison';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { modelLibrary } from '../../i18n/model-library';
import { formatBytes } from './model-format';

export function CatalogComparison({ models, clear }: { models: CatalogModel[]; clear: () => void }) {
  const { locale } = useI18n(), copy = modelComparison[locale], acquisition = modelAcquisition[locale], unknown = modelLibrary(locale).unknown;
  return <section className="catalog-comparison" data-empty={models.length === 0} aria-label={copy.title}>
    <div className="catalog-comparison-heading"><h3 className={!models.length ? 'sr-only' : undefined}>{copy.title}{models.length > 0 && ` · ${models.length}/3`}</h3>
      {models.length > 0 && <button type="button" className="text-button" onClick={clear}>{copy.clear}</button>}</div>
    <p className="workspace-secondary">{copy.hint}</p>
    {!!models.length && <div className="catalog-comparison-grid">{models.map(model => <article key={model.id}>
      <h4>{model.name || model.id}</h4><small dir="auto">{model.id}</small>
      <dl>
        <dt>{copy.parameters}</dt><dd>{model.parameters || unknown}</dd>
        <dt>{acquisition.transfer}</dt><dd>{model.size_bytes > 0 ? formatBytes(model.size_bytes, locale) : unknown}</dd>
        <dt>{copy.ram}</dt><dd>{model.min_ram_gb > 0 ? `${model.min_ram_gb} GB` : unknown}</dd>
        <dt>{copy.quant}</dt><dd>{model.quant || unknown}</dd>
        <dt>{acquisition.license}</dt><dd>{model.license || unknown}</dd>
      </dl>
    </article>)}</div>}
  </section>;
}
