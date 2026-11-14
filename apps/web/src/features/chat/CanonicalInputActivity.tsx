import { useState } from 'react';
import { useI18n } from '../../i18n';
import { useTaskStatus } from '../agents/useTaskStatus';
import { conversationWorkText } from '../../i18n/conversation-work';
import type { CanonicalInput, CanonicalQuestion, CanonicalInputMode } from './canonical-input';
import { conversationInputModeText } from '../../i18n/conversation-input-mode';
import { Icon } from '../../components/Icon';

export function ConversationInputMode({mode, chooseMode, question, disabled}: {mode: CanonicalInputMode; chooseMode: (mode: CanonicalInputMode) => void; question?: CanonicalQuestion | null; disabled: boolean}) {
  const {locale} = useI18n(), copy = conversationInputModeText[locale];
  return <label className="composer-input-mode">{copy.label}<select value={mode} disabled={disabled} onChange={event => chooseMode(event.target.value as CanonicalInputMode)}>
    <option value="auto">{question ? copy.answer : copy.message}</option>
    {(question || mode === 'message') && <option value="message">{copy.message}</option>}
    <option value="new_work">{copy.newWork}</option>
  </select></label>;
}

export function CanonicalInputActivity({id, inputs, control, error, question, mode, chooseMode, disabled}: {id: string; inputs: CanonicalInput[]; control: (input: CanonicalInput, kind: 'stop'|'continue') => Promise<void>; error: string; question?: CanonicalQuestion | null; mode: CanonicalInputMode; chooseMode: (mode: CanonicalInputMode) => void; disabled: boolean}) {
  const {messages, locale} = useI18n(), status = useTaskStatus();
  const [busy, setBusy] = useState(''), [failure, setFailure] = useState('');
  const pending = inputs.filter(input => !input.applied_at && input.state !== 'stopped');
  const act = async (input: CanonicalInput, kind: 'stop'|'continue') => {
    setBusy(input.id); setFailure('');
    try { await control(input, kind); }
    catch (reason) { setFailure(reason instanceof Error ? reason.message : messages.common.error); }
    finally { setBusy(''); }
  };
  const copy = conversationInputModeText[locale];
  return <div className="canonical-input-controls">
    {mode !== 'auto' && <div className="composer-mode-override"><span>{mode === 'new_work' ? copy.newWork : copy.message}</span><button type="button" className="icon-button" disabled={disabled} aria-label={messages.models.cancel} onClick={() => chooseMode('auto')}><Icon name="close" size={16} /></button></div>}
    {(pending.length > 0 || error || failure || question) && <section className="conversation-work" aria-label={conversationWorkText[locale].activity}>
    {question && <p className="composer-question" role="status">{question.content}</p>}
    {pending.map(input => <div key={input.id}>
      <p>{input.input.content.length > 120 ? `${input.input.content.slice(0,120)}…` : input.input.content}</p>
      <span role="status">{status(input.state === 'accepted' ? 'pending' : input.state === 'stopping' ? 'stopping' : input.run_status ?? input.state)}</span>
      {input.delivery_issue && <p role="status">{input.delivery_issue.message}</p>}
      {input.decision_issue && <p role="status">{input.decision_issue.message}</p>}
      {!input.stopped_at && <button type="button" className="secondary-button" disabled={!!busy} onClick={() => void act(input,'stop')}>{messages.chat.stop}</button>}
      {input.run_status === 'interrupted' && !input.stopped_at && <button type="button" className="secondary-button" disabled={!!busy} onClick={() => void act(input,'continue')}>{messages.models.resume}</button>}
    </div>)}
    {(failure || error) && <p role="alert">{failure || error}</p>}
  </section>}</div>;
}
