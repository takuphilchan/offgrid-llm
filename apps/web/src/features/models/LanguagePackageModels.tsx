import { useEffect, useRef, useState } from 'react';
import { api, type InstalledTypedModel } from '../../api/client';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { useI18n } from '../../i18n';
import { modelProviderDetails } from '../../i18n/model-library';
import { workflow } from '../../i18n/workflow';
import { useWorkspace } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { ModelIdentity } from './ModelIdentity';
import { packageDisplay } from './model-display';
import { SpeechPackages } from './SpeechPackages';

// Package imports remain metadata-only until a supervised provider and this
// exact model/runtime revision have been qualified. Never offer Select here.
export function LanguagePackageModels() {
  const { messages: text, locale } = useI18n();
  const details = modelProviderDetails(locale);
  const { admin } = useWorkspace();
  const [packages, setPackages] = useState<InstalledTypedModel[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const locked = useRef(false);
  const [remove, setRemove] = useState<InstalledTypedModel | null>(null);
  const load = async (signal?: AbortSignal) => {
    try {
      const items = await api.typedModels('language', signal);
      if (!signal?.aborted) { setPackages(items.filter(item => item.kind === 'package')); setError(''); }
    } catch (reason) {
      if (!signal?.aborted) setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  };
  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, []);
  useWorkspaceRefresh(load);
  const perform = async (action: () => Promise<unknown>) => {
    if (locked.current) return;
    locked.current = true;
    setBusy(true); setError('');
    try { await action(); await load(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); throw reason; }
    finally { locked.current = false; setBusy(false); }
  };
  return <section className="model-section" aria-busy={loading}>
    {error && <div className="inline-error" role="alert">{error}<button className="secondary-button" onClick={() => void load()}>{text.common.retry}</button></div>}
    {packages.length > 0 && <div className="catalog-grid">{packages.map(item => <article className="catalog-card package-card" key={`${item.id}/${item.revision}`}>
      <ModelIdentity model={packageDisplay(item)} />
      <p className="model-runtime-meta">{[item.engine, item.format, item.backend].filter(Boolean).join(' · ')}</p>
      <p className="model-readiness">{details.qualified}: {item.readiness.qualified ? details.yes : details.no}</p>
      <details><summary>{details.title}</summary>
        <dl className="package-facts">
          <dt>{text.models.installed}</dt><dd>{item.readiness.installed ? details.yes : details.no}</dd>
          <dt>{details.ready}</dt><dd>{item.readiness.runtime_ready ? details.yes : details.no}</dd>
          <dt>{details.smokeTested}</dt><dd>{item.readiness.smoke_tested ? details.yes : details.no}</dd>
          <dt>{details.qualified}</dt><dd>{item.readiness.qualified ? details.yes : details.no}</dd>
          {item.package?.manifest?.context_length && <><dt>{details.context}</dt><dd>{item.package.manifest.context_length}</dd></>}
          {item.package?.manifest?.runtime && <><dt>{details.runtime}</dt><dd>{item.package.manifest.runtime.adapter} · {item.package.manifest.runtime.revision}</dd></>}
        </dl>
        {item.readiness.issue && <p>{item.readiness.issue}</p>}
      </details>
      <div className="model-actions">
        <button className="secondary-button" disabled={!admin || busy || !!item.package?.in_use} onClick={() => void perform(() => api.verifyModelPackage(item.id, item.revision!, new AbortController().signal)).catch(() => {})}>{text.models.verify}</button>
        <button className="danger-button" disabled={!admin || busy || !!item.package?.in_use} onClick={() => setRemove(item)}>{text.models.delete}</button>
      </div>
    </article>)}</div>}
    <SpeechPackages importOnly language onChanged={() => load()} />
    {remove && <ConfirmDialog title={remove.name} body={workflow[locale].deleteModel} close={() => setRemove(null)} confirm={async () => { await perform(() => api.removeModelPackage(remove.id, remove.revision!, new AbortController().signal)); setRemove(null); }} />}
  </section>;
}
