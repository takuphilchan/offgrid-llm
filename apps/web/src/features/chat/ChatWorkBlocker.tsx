import type { AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { conversationWorkText } from '../../i18n/conversation-work';
import { chatUncertainHint, chatWorkBlockerText } from '../../i18n/chat-work-blocker';
import { workRecoveryText } from '../../i18n/work-recovery';
import { useTaskStatus } from '../agents/useTaskStatus';

/** A read-only explanation of the canonical job, never a recovery command. */
export function ChatWorkBlocker({ run, review, startNew, refresh }: {
  run: AgentRun | null; review: () => void; startNew: () => void; refresh: () => void;
}) {
  const { locale, messages } = useI18n(), copy = chatWorkBlockerText[locale];
  const recovery = workRecoveryText(locale), work = conversationWorkText[locale];
  const status = useTaskStatus();
  const reason = !run ? copy[0]
    : run.status === 'uncertain' ? chatUncertainHint[locale]
    : run.status === 'waiting_for_input' ? copy[1]
    : run.status === 'waiting_for_approval' ? copy[2]
    : run.status === 'waiting_for_children' ? copy[3]
    : run.status === 'interrupted'
      ? run.computer_session_expired && run.last_access ? recovery.access : run.resumable ? recovery.resume : recovery.archived
      : copy[4];
  return <section className="chat-work-blocker">
    <div role="status"><strong>{run ? status(run.status, run.outcome) : messages.common.loading}</strong><p>{reason}</p></div>
    <div className="button-row">
      <button type="button" className="text-button" onClick={run ? review : refresh}>{run ? work.inspect : messages.common.refresh}</button>
      <button type="button" className="text-button" onClick={startNew}>{messages.chat.newChat}</button>
    </div>
  </section>;
}
