import type { AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskEvidenceText } from '../../i18n/task-evidence';
import { documentWorkflowText } from '../../i18n/document-workflow';

// Observation only: rendering evidence never issues a command or grants access.
export function TaskEvidence({ run }: { run: AgentRun }) {
 const { locale } = useI18n(), copy = taskEvidenceText(locale), evidence = run.verification;
 if (!evidence) return null;
 if (![1,2].includes(evidence.version) || !Array.isArray(evidence.checks)) return <section className="task-evidence" aria-label={copy.title}><h3>{copy.title}</h3><p>{copy.unavailable}</p></section>;
 const state = evidence.state === 'checks_passed' || evidence.state === 'partial' || evidence.state === 'pending' ? evidence.state : 'unverified';
 return <section className="task-evidence" aria-label={copy.title}>
  <h3>{copy.title}</h3><p className="task-evidence-state" role="status">{copy[state]}</p>
  <ul>{evidence.checks.map(check => <li key={`${evidence.revision}:${check.expectation.id}`}>
   <strong>{check.expectation.title}</strong><span>{check.state === 'verified' ? copy.verified : check.state === 'pending' ? copy.checkPending : copy.unavailable}</span>
   <small>{check.provenance === 'caller_explicit' ? copy.caller : copy.model}{evidence.version === 2 && check.expectation.kind.startsWith('document_') ? ` · ${documentWorkflowText(locale).verification}` : ''}</small>
  </li>)}</ul>
  <p className="workspace-secondary">{copy.scope}</p>
  <details><summary>{copy.details}</summary>{evidence.checks.map(check => <div key={check.expectation.id}><strong>{check.expectation.title}</strong>{check.reason && <p>{check.reason}</p>}<code>{check.expectation.kind}</code>{check.checked_at && Number.isFinite(Date.parse(check.checked_at)) && <p><time dateTime={check.checked_at}>{new Date(check.checked_at).toLocaleString(locale)}</time></p>}{!!check.evidence_steps?.length && <p>{check.evidence_steps.map(n => `#${n}`).join(', ')}</p>}</div>)}</details>
 </section>;
}
