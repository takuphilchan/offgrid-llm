import {useEffect, useRef, useState} from 'react';
import {api} from '../../api/client';
import {useI18n} from '../../i18n';

// Retrieve on expansion, one bounded page at a time. A failed authorization
// refresh clears already displayed excerpts instead of retaining stale content.
export function SavedResult({reference}: {reference: string}) {
  const {messages} = useI18n();
  const [content, setContent] = useState(''), [offset, setOffset] = useState(0);
  const [complete, setComplete] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const active = useRef<AbortController | null>(null);
  useEffect(() => () => active.current?.abort(), [reference]);
  const load = async () => {
    if (active.current || complete) return;
    const controller = new AbortController(); active.current = controller; setBusy(true); setError('');
    try {
      const page = await api.conversations.result(reference, offset, 16384, controller.signal);
      if (page.id !== reference || page.offset !== offset || page.next_offset < offset || (!page.complete && page.next_offset <= offset)) throw new Error(messages.common.error);
      if (!controller.signal.aborted) {setContent(value => value + page.content); setOffset(page.next_offset); setComplete(page.complete);}
    } catch (reason) {
      if (!controller.signal.aborted) {setContent(''); setOffset(0); setError(reason instanceof Error ? reason.message : messages.common.error);}
    } finally {if (active.current === controller) active.current = null; if (!controller.signal.aborted) setBusy(false);}
  };
  return <details onToggle={event => {if (event.currentTarget.open && !content && !error) void load();}}>
    <summary>{messages.agents.result}</summary>
    {content && <pre className="saved-result-content" style={{whiteSpace: 'pre-wrap', overflowWrap: 'anywhere'}}>{content}</pre>}
    {error && <p role="alert">{error}</p>}
    {!complete && <button type="button" className="text-button" disabled={busy} onClick={() => void load()}>{busy ? messages.common.loading : error ? messages.common.retry : messages.history.showMore}</button>}
  </details>;
}
