import { useI18n } from '../../i18n';
import { taskWorkspaceText } from '../../i18n/task-workspace';
import { taskControls } from '../../i18n/task-controls';
import { taskAccessText } from '../../i18n/task-access';

/** Display projection only; never infer a completed outcome from a label. */
export function useTaskStatus() {
  const { locale, messages } = useI18n();
  return (status: string, outcome?: string) => {
    if (status === 'waiting_for_input') return taskWorkspaceText(locale).needsAccess;
    if (status === 'waiting_for_children') return taskControls(locale).waiting;
    if (status === 'completed' && outcome === 'answer_only') return taskAccessText(locale).answered;
    return messages.recovery[status as keyof typeof messages.recovery] ?? status;
  };
}
