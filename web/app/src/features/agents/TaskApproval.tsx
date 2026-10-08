import { useEffect, useState } from 'react';
import type { AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { browserActionLabel, computerExperience } from '../../i18n/computer-experience';
import { taskPresentation } from '../../i18n/task-presentation';
import { BrowserActionSummary } from './BrowserActionSummary';

export function TaskApproval({ run, busy, refresh, act }: { run: AgentRun; busy: boolean; refresh: () => void; act: (action: 'approve' | 'deny') => void }) {
  const { locale, messages } = useI18n(), copy = taskPresentation(locale), details = computerExperience(locale);
  const approval = run.pending_approval!;
  const [now, setNow] = useState(Date.now());
  const expiry = Date.parse(approval.expires_at);
  const expired = !Number.isFinite(expiry) || expiry <= now;
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 1000); return () => clearInterval(timer); }, [approval.id]);
  const supported = !!run.computer_session && browserActionLabel(locale, approval.tool) !== approval.tool;
  return <section className="approval-card" aria-labelledby="task-approval-title">
    <h3 id="task-approval-title">{messages.agents.approvalTitle}</h3>
    {supported ? <BrowserActionSummary tool={approval.tool} args={approval.arguments} steps={run.steps} /> : <>
      <strong>{approval.tool}</strong><p>{copy.unknown}</p>
      {['path', 'url', 'destination', 'recipient'].map(key => typeof approval.arguments[key] === 'string' ? <p className="approval-destination" key={key}><span>{details.target}: </span>{String(approval.arguments[key])}</p> : null)}
    </>}
    {/* Exact canonical arguments preserve large numbers and consequential values.
        Unknown tools must show them, not conceal uncertainty under friendly copy. */}
    <details open={!supported}><summary>{copy.exact}</summary><pre>{approval.canonical_arguments ?? JSON.stringify(approval.arguments, null, 2)}</pre></details>
    {expired ? <p role="status">{copy.expired}</p> : <p>{copy.expires}: <time dateTime={approval.expires_at}>{new Date(expiry).toLocaleString(locale)}</time></p>}
    <div className="button-row">
      <button className="secondary-button" disabled={busy} onClick={() => act('deny')}>{messages.agents.deny}</button>
      {expired ? <button className="secondary-button" disabled={busy} onClick={refresh}>{messages.common.refresh}</button> : <button className="primary-button" disabled={busy} onClick={() => { if (expiry > Date.now()) act('approve'); else setNow(Date.now()); }}>{messages.agents.approve}</button>}
    </div>
  </section>;
}
