import { useI18n } from '../i18n';
import { knownWorkText } from '../i18n/known-work';
import { useWorkActions, type KnownWork } from '../lib/active-work';

export function WorkLinks({ entries }: { entries: KnownWork[] }) {
  const actions = useWorkActions(), { locale, messages } = useI18n(), copy = knownWorkText[locale];
  return <ul className="known-work-list">{entries.map(work => <li key={`${work.kind}:${work.id}`}>
    {work.kind === 'task' ? <a className="secondary-button" href={`#/agents/task/${encodeURIComponent(work.id)}`}>{work.title}</a>
      : <button className="secondary-button" onClick={event => { actions.openChat(work.id); (event.currentTarget.closest('[popover]') as HTMLElement | null)?.hidePopover(); }}>{work.title}</button>}
    <small>{work.stale || work.status === 'unavailable' ? copy.stale : messages.recovery[work.status as keyof typeof messages.recovery] ?? work.status}</small>
  </li>)}</ul>;
}
