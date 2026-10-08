import { useEffect, useRef, useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskPresentation } from '../../i18n/task-presentation';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { ScopedNotice } from '../../components/WorkspacePresentation';

type Artifact = NonNullable<AgentRun['artifacts']>[number];
function ArtifactRow({ id, artifact }: { id: string; artifact: Artifact }) {
  const { locale, messages } = useI18n(), copy = taskPresentation(locale);
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const download = useRef<AbortController | null>(null);
  useEffect(() => () => download.current?.abort(), []);
  const checked = artifact.verified === true && artifact.check === 'stored_bytes_sha256_and_format';
  const save = async () => {
    if (download.current) return;
    const controller = new AbortController(); download.current = controller;
    setBusy(true); setError('');
    try {
      const blob = await api.taskArtifact(id, artifact.sha256, controller.signal);
      const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', await blob.arrayBuffer()))].map(n => n.toString(16).padStart(2, '0')).join('');
      if (controller.signal.aborted) return;
      if (blob.size !== artifact.bytes || digest !== artifact.sha256) throw Error(copy.unavailable);
      const url = URL.createObjectURL(blob), anchor = document.createElement('a');
      anchor.href = url; anchor.download = artifact.name.replace(/[\\/]/g, '_'); anchor.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      if (!controller.signal.aborted) setError(copy.unavailable);
    } finally { if (download.current === controller) download.current = null; if (!controller.signal.aborted) setBusy(false); }
  };
  return <li className="task-artifact">
    <div><strong>{artifact.name}</strong><p>{artifact.format} · {artifact.bytes.toLocaleString(locale)} B · {checked ? copy.integrity : copy.unverified}</p><p>{copy.noPreview}</p></div>
    <button className="secondary-button" disabled={busy} onClick={() => void save()} aria-label={`${copy.download}: ${artifact.name}`}>{busy ? messages.common.loading : copy.download}</button>
    {error && <ScopedNotice kind="error">{error}</ScopedNotice>}
  </li>;
}

export function TaskResult({ run }: { run: AgentRun }) {
  const { locale, messages } = useI18n(), copy = taskPresentation(locale);
  if (!run.output && !run.artifacts?.length) return null;
  return <section className="task-result" aria-label={messages.agents.result}>
    {run.output && <><h3>{run.status === 'completed' ? copy.answer : messages.agents.result}</h3><div className="markdown-body"><MarkdownMessage content={run.output} /></div><p className="workspace-secondary">{copy.generated}</p></>}
    {!!run.artifacts?.length && <section className="task-artifacts" aria-label={copy.files}>
      <h3>{copy.files}</h3><p className="workspace-secondary">{copy.integrityHint}</p>
      <ul>{run.artifacts.map((artifact, index) => <ArtifactRow key={`${run.run_id}:${artifact.sha256}:${index}`} id={run.run_id} artifact={artifact} />)}</ul>
    </section>}
  </section>;
}
