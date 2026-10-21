import type { AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { documentWorkflowText } from '../../i18n/document-workflow';
import { browserActionLabel } from '../../i18n/computer-experience';

const reads = new Set(['computer_observe', 'computer_verify_application', 'computer_documents', 'computer_read_text']);
const changes = new Set(['computer_create_document', 'computer_insert_text', 'computer_replace_selection', 'computer_replace_text']);

// Display only committed state. Never infer permission, replay or success from
// application text, tool names alone, or a streamed model preview.
export function DocumentProgress({ run }: { run: AgentRun }) {
 const { locale } = useI18n(), copy = documentWorkflowText(locale);
 const last = [...run.steps].reverse().find(step => step.type === 'action' || step.type === 'error');
 if (!last?.tool_name) return null;
 if (run.status === 'interrupted' && !run.uncertain_call_id && reads.has(last.tool_name) && last.type === 'error') {
  let failure: { effects?:unknown; code?:unknown; verified?:unknown } = {};
  try { failure = JSON.parse(last.tool_result ?? '{}'); } catch { return null; }
  if (failure && failure.effects === 'none' && failure.verified === false && typeof failure.code === 'string' && failure.code.startsWith('computer_')) {
   return <p className="workspace-secondary native-document-progress" role="status"><strong>{browserActionLabel(locale,last.tool_name)}</strong> · {copy.readFailed}</p>;
  }
 }
 const evidence = run.verification;
 if (evidence?.version === 2 && changes.has(last.tool_name) && last.type === 'action' && !['uncertain','failed','cancelled'].includes(run.status) && evidence.checks.some(check => check.expectation.kind.startsWith('document_') && check.state === 'pending')) {
  return <p className="workspace-secondary native-document-progress" role="status">{copy.dispatched}</p>;
 }
 return null;
}
