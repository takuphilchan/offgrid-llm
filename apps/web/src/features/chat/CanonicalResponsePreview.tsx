import {useDeferredValue, useEffect, useMemo, useRef} from 'react';
import {useI18n} from '../../i18n';
import {useTaskRun} from '../agents/useTaskRun';
import {MarkdownMessage} from '../../components/MarkdownMessage';
import type {CanonicalInput} from './canonical-input';
import {pendingCanonicalInputs} from './CanonicalInputActivity';

// Subscribe only to the active input's existing run. Reading never starts work.
// Source-checked server snapshots explicitly opt in; older services stay buffered.
export function CanonicalResponsePreview({inputs, messages, onChange}: {inputs:CanonicalInput[]; messages:readonly {role:string;job_id?:string}[]; onChange:()=>void}) {
  const input = pendingCanonicalInputs(inputs).find(item=>item.run_id && !item.context_unavailable);
  const id = input?.run_id ?? '';
  const {run} = useTaskRun(id);
  const {messages:text} = useI18n();
  const saved = messages.some(message=>message.role==='assistant' && message.job_id===id);
  const content = !saved && !input?.context_unavailable && run?.response_preview_allowed && run.status==='running' ? run.progress?.preview ?? '' : '';
  const preview = useMemo(()=>({id, content}),[id,content]);
  const deferred = useDeferredValue(preview), follow = useRef(onChange); follow.current=onChange;
  useEffect(()=>{if(content)follow.current();},[content]);
  // Immediately conceal revoked/settled text instead of retaining deferred data.
  if (!content) return null;
  return <article className="message assistant canonical-live-response"><div className="message-heading"><div className="message-label">{text.chat.assistant}</div><small>{text.agentLive.provisional}</small></div><div className="message-body"><MarkdownMessage content={deferred.id === id ? deferred.content : content} streaming /></div>{run?.progress?.truncated && <small>{text.agentLive.previewLimit}</small>}</article>;
}
