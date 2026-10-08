import { useI18n } from '../../i18n';
import { modelAcquisition } from '../../i18n/model-acquisition';
import { modelLibrary } from '../../i18n/model-library';
import { formatBytes } from './model-format';

// Legacy acquisition has no package preflight/immutable revision in its public
// contract. Preserve its authority and say what is unknown, rather than deriving
// disk capacity from a different service filesystem or inventing a SHA identity.
export function LegacyDownloadReview({ repository, file, bytes, license }: { repository: string; file: string; bytes?: number; license?: string }) {
  const { locale, messages: text } = useI18n(), copy = modelAcquisition[locale], library = modelLibrary(locale);
  return <div className="legacy-download-review">
    <dl className="package-facts">
      <dt>{copy.transfer}</dt><dd>{bytes && bytes > 0 ? formatBytes(bytes) : library.unknown}</dd>
      <dt>{copy.space}</dt><dd>{library.unknown}</dd>
      <dt>{copy.license}</dt><dd>{license || library.unknown}</dd>
      <dt>{copy.source}</dt><dd>{repository} · {file} · {library.unpinned}</dd>
    </dl>
    <p>{library.legacyPreflight}</p>
    <details><summary>{library.details}</summary><p>{text.modelSearch.compatibility}</p></details>
    <a className="secondary-button" href={`https://huggingface.co/${encodeURI(repository)}`} target="_blank" rel="noreferrer">{text.modelSearch.modelCard}</a>
  </div>;
}
