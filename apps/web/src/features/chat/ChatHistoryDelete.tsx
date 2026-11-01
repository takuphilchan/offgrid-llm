import { useState } from 'react';
import { api, APIError } from '../../api/client';
import { HistoryDeleteDialog, type HistoryItem } from '../../components/HistoryDeleteDialog';
import { useI18n } from '../../i18n';
import { conversationWorkText } from '../../i18n/conversation-work';

/** Ordinary deletion needs no job policy. Only a typed service conflict reveals
 * the explicit choice; changing a choice never sends a stop or delete itself. */
export function ChatHistoryDelete({ items, onDeleted, onClose }: {
  items: HistoryItem[]; onDeleted: (ids: string[]) => void; onClose: () => void;
}) {
  const { locale } = useI18n();
  const copy = conversationWorkText[locale];
  const [needsChoice, setNeedsChoice] = useState(false);
  const [choice, setChoice] = useState<'' | 'retain' | 'stop'>('');
  return <HistoryDeleteDialog items={items} kind="chats" onDeleted={onDeleted} onClose={onClose}
    remove={name => api.deleteSession(name, choice || undefined)}
    confirmDisabled={needsChoice && !choice}
    onFailure={reason => {
      if (reason instanceof APIError && ['conversation_has_work', 'task_stop_pending'].includes(reason.data?.error?.code)) setNeedsChoice(true);
    }}
    options={busy => needsChoice && <fieldset className="history-work-choice" disabled={busy}>
      <legend>{copy.deleteChoice}</legend>
      {(['stop', 'retain'] as const).map(value => <label key={value}><input type="radio" name="delete-work" checked={choice === value} onChange={() => setChoice(value)} /><span>{copy[value]}</span></label>)}
      <p>{copy.deleteHint}</p>
    </fieldset>} />;
}
