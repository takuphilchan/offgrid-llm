import { useState } from 'react';
import { api, APIError } from '../../api/client';
import { HistoryDeleteDialog, type HistoryItem } from '../../components/HistoryDeleteDialog';
import { useI18n } from '../../i18n';
import { activityText } from '../../i18n/activity';
import { conversationWorkText } from '../../i18n/conversation-work';

/** Ordinary deletion needs no job policy. Only a typed service conflict reveals
 * the explicit choice; changing a choice never sends a stop or delete itself. */
export function ChatHistoryDelete({ items, onDeleted, onClose, onOpen }: {
  onOpen?: (name:string)=>void; items: HistoryItem[]; onDeleted: (ids: string[]) => void; onClose: () => void;
}) {
  const { locale, messages:text } = useI18n();
  const [code,setCode]=useState('');
  const recovery=activityText(locale);
  const recoverable=['conversation_active_turn','conversation_input_pending','conversation_busy','conversation_link_pending','task_stop_pending'].includes(code);
  const copy = conversationWorkText[locale];
  const [needsChoice, setNeedsChoice] = useState(false);
  const [choice, setChoice] = useState<'' | 'retain' | 'stop'>('');
  return <HistoryDeleteDialog items={items} kind="chats" onDeleted={onDeleted} onClose={onClose}
    remove={async name => {
      const item=items.find(item=>item.id===name);
      if(!item?.canonicalId)return api.deleteSession(name,choice||undefined);
      // Refresh the revision only after an explicit confirmation/retry, never
      // silently retry a racing mutation. The confirmed identity stays pinned.
      let revision=item.revision??1;
      try { revision=(await api.conversations.get(item.canonicalId)).revision; }
      catch(reason){if(!(reason instanceof APIError&&reason.status===404))throw reason;}
      // A tombstone may precede filesystem cleanup. Retry DELETE even when the
      // readable conversation is already gone; only DELETE confirms cleanup.
      return api.deleteSession(name,choice||undefined,{id:item.canonicalId,revision});
    }}
    confirmDisabled={needsChoice && !choice}
    onFailure={reason => {
      setCode(reason instanceof APIError ? reason.data?.error?.code || (reason.status===403?'ownership':'') : '');
      if (reason instanceof APIError && ['conversation_has_work', 'task_stop_pending'].includes(reason.data?.error?.code)) setNeedsChoice(true);
    }}
    options={busy => <>
      {(recoverable||code==='conversation_conflict'||code==='ownership')&&<div className="history-delete-recovery" role="status"><p>{code==='conversation_conflict'?recovery.changed:recovery.protection(code==='ownership'?'ownership':code==='task_stop_pending'?'unsettled_action':'active_work')}</p>{recoverable&&items.length===1&&onOpen&&<button className="secondary-button" disabled={busy} onClick={()=>onOpen(items[0].id)}>{text.nav.chat}</button>}</div>}
      {needsChoice && <fieldset className="history-work-choice" disabled={busy}>
      <legend>{copy.deleteChoice}</legend>
      {(['stop', 'retain'] as const).map(value => <label key={value}><input type="radio" name="delete-work" checked={choice === value} onChange={() => setChoice(value)} /><span>{copy[value]}</span></label>)}
      <p>{copy.deleteHint}</p>
    </fieldset>}</>} />;
}
