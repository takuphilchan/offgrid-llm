import { useEffect, useRef, useState } from 'react';
import { api, type ChatSession, type SessionTurn } from '../../api/client';
import { canonicalInputs, submitCanonicalSession, retainedCanonicalAnswer, type CanonicalInput, type CanonicalQuestion, type CanonicalInputMode } from './canonical-input';
import { messageText } from './chat-presentation';
import {clearSubmittedDraft, draftKey} from '../../lib/drafts';
import {useDraftScope} from '../../lib/draft-scope';
import type {useConversationConfiguration} from './agent-configuration';

export function useCanonicalConversation(name: string, id: string, handlers: {
  receive: (session: ChatSession, turn: SessionTurn | null) => void;
  answered: (answer: string) => void; unavailable: () => void;
}) {
  const [inputs, setInputs] = useState<CanonicalInput[]>([]);
  const [question, setQuestion] = useState<CanonicalQuestion | null>(null);
  const scope = useDraftScope(), requestKey = draftKey(scope, 'canonical-request', name);
  const [pendingAnswer, setPendingAnswer] = useState(false);
  const [inputMode, setInputMode] = useState<CanonicalInputMode>('auto');
  const [error, setError] = useState('');
  const receiver = useRef(handlers); receiver.current = handlers;
  const replyID = useRef('');
  const wake = useRef<() => void>(() => {});
  useEffect(() => {
    setInputs([]); setQuestion(null); setInputMode('auto'); setPendingAnswer(!!retainedCanonicalAnswer(requestKey, id)); setError('');
    if (!name || !id) return;
    let alive = true, pending = false;
    const controller = new AbortController();
    const refresh = async () => {
      if (pending || !alive) return;
      pending = true;
      try {
        const [session, entries, current] = await Promise.all([api.session(name), canonicalInputs(id, controller.signal), api.conversations.get(id, controller.signal)]);
        if (!alive) return;
        const unresolved = retainedCanonicalAnswer(requestKey, id);
        setInputs(entries); setPendingAnswer(!!unresolved); setQuestion(unresolved && unresolved.payload.question_id !== current.pending_question?.id ? null : current.pending_question ?? null); setError('');
        const turn = session.turn;
        receiver.current.receive(session, turn?.job_id && !entries.some(input => input.run_id === turn.job_id) ? turn : null);
        const reply = entries.find(input => input.id === replyID.current);
        if (reply?.run_status === 'completed' && !reply.context_unavailable) {
          const answer = session.messages.find(message => message.turn_id === reply.id && message.job_id === reply.run_id && message.role === 'assistant');
          if (answer) {replyID.current = ''; receiver.current.answered(messageText(answer.content));}
        } else if (reply && (reply.state === 'failed' || reply.stopped_at || reply.context_unavailable)) {
          replyID.current = ''; receiver.current.unavailable();
        }
      } catch (reason) { if (alive && reason instanceof Error) setError(reason.message); }
      finally { pending = false; }
    };
    wake.current = () => void refresh();
    void refresh(); const timer = window.setInterval(() => void refresh(), 1000);
    return () => {alive = false; controller.abort(); window.clearInterval(timer); wake.current = () => {};};
  }, [name, id, requestKey]);
  const control = async (input: CanonicalInput, kind: 'stop' | 'continue') => {
    const current = await api.conversations.get(id);
    await api.conversations.inputCommand(id, {request_id: crypto.randomUUID(), target_id: input.id, expected_revision: current.revision, kind, payload: {}});
    wake.current();
  };
  const send = async (options: Parameters<typeof submitCanonicalSession>[0], draftKey: string, submitted: string, valid: () => boolean) => {
    const input = await submitCanonicalSession({...options, question: inputMode === 'auto' ? question : null, intent: inputMode === 'auto' ? undefined : inputMode});
    clearSubmittedDraft(draftKey, submitted);
    if (valid()) {setInputMode('auto'); replyID.current = input.incorporation === 'worker_answer' ? '' : input.id; wake.current();}
  };
  return {inputs, question, inputMode, setInputMode, pendingAnswer, answeringQuestion: inputMode === 'auto' && !!question || pendingAnswer, error, control, replyID, send, refresh: () => wake.current()};
}

export async function canonicalConfiguration(id: string) {
  const [current, effective] = await Promise.all([api.conversations.get(id), api.conversations.configuration(id)]);
  const profile = current.preferences.profile_id ? await api.agentProfile(current.preferences.profile_id, current.preferences.profile_revision) : null;
  return {version: 1 as const, profile, model: effective.model ?? '', restrictions: current.preferences.restrictions ?? {}};
}

export function restoreCanonicalConfiguration(id: string, configuration: ReturnType<typeof useConversationConfiguration>, revision: number, valid: () => boolean, model: (value: string) => void, failed: (reason: unknown) => void, settled: () => void) {
  void canonicalConfiguration(id).then(saved => {
    if (!valid()) return;
    const restored = configuration.restore(saved, revision);
    if (restored) model(restored);
  }).catch(reason => {if (valid()) failed(reason);}).finally(() => {if (valid()) settled();});
}
