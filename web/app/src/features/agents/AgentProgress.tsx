import { useEffect, useState } from 'react';
import type { AgentRun } from '../../api/client';
import { agentActive } from '../../api/agent-stream';
import { useI18n } from '../../i18n';

function duration(since: string | null | undefined, now: number) {
  const seconds = since ? Math.max(0, Math.floor((now - Date.parse(since)) / 1000)) : 0;
  return Number.isFinite(seconds) ? `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2,'0')}` : '0:00';
}

export function AgentProgress({ run, connection, showPreview, setShowPreview }: { run: AgentRun; connection: 'connecting' | 'live' | 'reconnecting'; showPreview: boolean; setShowPreview: (value: boolean) => void }) {
  const { messages: text } = useI18n();
  const [now, setNow] = useState(Date.now());
  const active = agentActive(run);
  useEffect(() => { if (!active) return; const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, [active]);
  const progress = run.progress;
  const phases: Record<string,string> = { queued:text.chatStreaming.queued, loading:text.chatStreaming.loading, processing:text.chatStreaming.processing, generating:text.chatStreaming.generating, preparing_tool:text.agentLive.preparingTool, tool:text.agentLive.tool, approval:text.recovery.waiting_for_approval };
  const incomplete = ['failed','cancelled','interrupted','uncertain'].includes(run.status);
  if (!active && !(incomplete && progress?.preview)) return null;
  return <div className="agent-progress">
    {active && <>
      <div className="agent-progress-heading" role="status"><span className="agent-progress-dot" aria-hidden="true" /><strong>{phases[progress?.phase ?? ''] ?? text.recovery.running}{progress?.tool ? ` · ${progress.tool}` : ''}</strong><small>{text.agentLive.step} {progress?.iteration ?? 1}</small></div>
      <div className="agent-progress-meta"><span>{text.agentLive.elapsed} {duration(run.started_at, now)}</span><span>{connection === 'live' ? text.agentLive.live : text.agentLive.reconnecting}</span>{progress?.updated_at && <span>{text.agentLive.lastUpdate} {duration(progress.updated_at, now)}</span>}</div>
      {connection !== 'live' && <p className="agent-connection-note" role="status">{text.agentLive.reconnectHint}</p>}
    </>}
    {(active || (incomplete && progress?.preview)) && <label className="agent-preview-toggle"><input type="checkbox" checked={showPreview} onChange={event => setShowPreview(event.target.checked)} />{text.agentLive.showPreview}</label>}
  </div>;
}

export function AgentPreview({ run, showPreview }: { run: AgentRun; showPreview: boolean }) {
  const { messages: text } = useI18n();
  const progress = run.progress;
  if (!showPreview || !progress?.preview || run.status === 'completed') return null;
  const incomplete = ['failed','cancelled','interrupted','uncertain'].includes(run.status);
  return <section className="agent-live-preview" aria-label={text.agentLive.showPreview}>
      <span className="eyebrow">{incomplete ? text.agentLive.incomplete : text.agentLive.provisional}</span>
      {/* Do not announce every token to screen readers or steal scroll/focus. */}
      <pre>{progress.preview}</pre>
      {progress.truncated && <small>{text.agentLive.previewLimit}</small>}
    </section>;
}
