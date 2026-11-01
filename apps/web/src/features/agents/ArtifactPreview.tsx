import { useEffect, useId, useRef, useState } from 'react';
import { api, type AgentRun } from '../../api/client';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { useI18n } from '../../i18n';
import { artifactPreviewText } from '../../i18n/artifact-preview';

type Artifact = NonNullable<AgentRun['artifacts']>[number];
const formats = new Set(['txt', 'text', 'md', 'markdown', 'csv', 'json']);
export function canPreviewArtifact(artifact: Artifact) {
  return formats.has(artifact.format) && Number.isSafeInteger(artifact.bytes) && artifact.bytes >= 0 && artifact.bytes <= 64 * 1024;
}

/** Explicit authorized read; never trust the artifact's verification label alone. */
export function ArtifactPreview({ runID, artifact }: { runID: string; artifact: Artifact }) {
  const { locale, messages } = useI18n(), copy = artifactPreviewText(locale);
  const [open, setOpen] = useState(false), [content, setContent] = useState<string | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState('');
  const pending = useRef<AbortController | null>(null), regionID = useId();
  useEffect(() => () => pending.current?.abort(), []);

  const toggle = async () => {
    if (open) {
      pending.current?.abort(); pending.current = null;
      setOpen(false); setContent(null); setError(''); setBusy(false);
      return;
    }
    if (!canPreviewArtifact(artifact)) return;
    const controller = new AbortController(); pending.current = controller;
    setOpen(true); setBusy(true); setError(''); setContent(null);
    try {
      const blob = await api.taskArtifact(runID, artifact.sha256, controller.signal);
      const bytes = await blob.arrayBuffer();
      const digest = [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map(n => n.toString(16).padStart(2, '0')).join('');
      if (controller.signal.aborted) return;
      if (bytes.byteLength !== artifact.bytes || digest !== artifact.sha256) throw Error('integrity');
      const text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
      // Reject binary control bytes even when they happen to form valid UTF-8.
      if (/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(text)) throw Error('binary');
      setContent(text);
    } catch {
      if (!controller.signal.aborted) setError(copy.unavailable);
    } finally {
      if (pending.current === controller) pending.current = null;
      if (!controller.signal.aborted) setBusy(false);
    }
  };

  return <div className="artifact-preview">
    <button type="button" className="secondary-button" aria-label={`${open ? copy.close : copy.preview}: ${artifact.name}`} aria-expanded={open} aria-controls={regionID} onClick={() => void toggle()}>{open ? copy.close : copy.preview}</button>
    {open && <section id={regionID} aria-label={`${copy.preview}: ${artifact.name}`} aria-busy={busy} className="artifact-preview-content">
      {busy && <p role="status">{messages.common.loading}</p>}
      {error && <p role="alert">{error}</p>}
      {content !== null && <>
        {['md', 'markdown'].includes(artifact.format) ? <div className="message-body"><MarkdownMessage content={content} /></div> : <pre tabIndex={0} dir="auto">{content}</pre>}
      </>}
    </section>}
  </div>;
}
