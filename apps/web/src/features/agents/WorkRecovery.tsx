import type { AgentRun } from '../../api/client';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { useI18n } from '../../i18n';
import { workRecoveryText } from '../../i18n/work-recovery';

/** Guidance comes from durable state, never from matching English error text. */
export function WorkRecovery({ run, readError, actionError, refresh }: {
  run: AgentRun; readError: string; actionError: string; refresh: () => void;
}) {
  const { locale } = useI18n(), copy = workRecoveryText(locale);
  const hint = run.status === 'interrupted'
    ? run.computer_session_expired ? run.last_access ? copy.access : copy.archived : run.resumable ? copy.resume : copy.archived
    : run.status === 'failed' ? copy.failed : run.status === 'cancelled' ? copy.cancelled : '';
  return <>
    {readError && <ScopedNotice kind="error" action={<button type="button" className="secondary-button" onClick={refresh}>{copy.connection}</button>}>{readError}</ScopedNotice>}
    {actionError && <ScopedNotice kind="error">{actionError}</ScopedNotice>}
    {(hint || run.error) && <section className="work-recovery-guidance" role={run.error ? 'alert' : 'status'}>
      {run.error && <p>{run.error}</p>}
      {hint && <p>{hint}</p>}
      {!!(run.output || run.artifacts?.length) && <p className="workspace-secondary">{copy.retained}</p>}
    </section>}
  </>;
}
