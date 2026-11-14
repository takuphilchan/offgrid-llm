import {useEffect, useRef, useState} from 'react';
import {api} from '../../api/client';
import type {components} from '../../api/schema.generated';
import {useI18n} from '../../i18n';
import {workChangesText} from '../../i18n/work-changes';
import {useDraftScope} from '../../lib/draft-scope';
import {WorkChangeDetails} from './WorkChangeDetails';
import './work-changes.css';

type Change = components['schemas']['WorkChange'];

export function WorkChanges({id, revision}: {id: string; revision?: string}) {
  const scope = useDraftScope();
  return <ScopedWorkChanges key={`${scope}:${id}`} id={id} revision={revision} />;
}

function ScopedWorkChanges({id, revision}: {id: string; revision?: string}) {
  const {locale,messages} = useI18n(), copy = workChangesText(locale);
  const [open,setOpen] = useState(false), [items,setItems] = useState<Change[]>([]);
  const [cursor,setCursor] = useState(''), [more,setMore] = useState(false), [busy,setBusy] = useState(false), [error,setError] = useState('');
  const pending = useRef<AbortController | null>(null);
  const load = async (after = '') => {
    pending.current?.abort();
    const controller = new AbortController(); pending.current = controller;
    setBusy(true); setError('');
    try {
      const collected: Change[] = []; let next = after, hasMore = false;
      do {
        const page = await api.conversations.workChanges(id,next,controller.signal);
        if (controller.signal.aborted) return;
        if (page.items.some(item => item.work_id !== id) || page.has_more && page.next_cursor <= next) throw new Error(messages.common.error);
        collected.push(...page.items); next = page.next_cursor; hasMore = page.has_more;
      } while (!after && hasMore && collected.length < Math.max(1,items.length));
      setItems(old => after ? [...old,...collected] : collected); setCursor(next); setMore(hasMore);
    } catch (reason) {
      if (!controller.signal.aborted) {setItems([]); setMore(false); setError(reason instanceof Error ? reason.message : messages.common.error);}
    } finally {if (!controller.signal.aborted) setBusy(false);}
  };
  useEffect(() => {
    if (open) void load();
    else {setItems([]); setCursor(''); setMore(false);}
    return () => pending.current?.abort();
  }, [id,open,revision]);
  return <details className="work-changes" onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>{copy.title}</summary>
    {open && <div className="work-changes-body" aria-busy={busy}>
      <p className="workspace-secondary">{copy.proof}</p>
      <button type="button" className="secondary-button" disabled={busy} onClick={() => void load()}>{copy.refresh}</button>
      {error && <p role="alert">{error}</p>}
      {!busy && !error && items.length === 0 && <p>{copy.empty}</p>}
      {items.map(item => <WorkChangeDetails key={item.id} item={item} />)}
      {more && <button type="button" className="secondary-button" disabled={busy} onClick={() => void load(cursor)}>{copy.more}</button>}
    </div>}
  </details>;
}
