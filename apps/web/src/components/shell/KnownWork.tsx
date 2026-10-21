import { useKnownWork } from '../../lib/active-work';
import { useI18n } from '../../i18n';
import { UtilityPopover } from '../UtilityPopover';
import { knownWorkText, attentionWorkText } from '../../i18n/known-work';
import { WorkLinks } from '../WorkLinks';

export function KnownWork() {
  const { entries, more } = useKnownWork();
  const { locale, messages } = useI18n(), copy = knownWorkText[locale];
  const relevant = entries.filter(entry => !['completed', 'failed', 'cancelled'].includes(entry.status));
  const running = relevant.filter(entry => !entry.stale && ['running', 'pending', 'waiting_for_children'].includes(entry.status)).length;
  if (!relevant.length) return null;
  return <UtilityPopover label={copy.title} triggerClassName="utility-button" trigger={<>{running ? messages.recovery.running : attentionWorkText[locale]} · {running || relevant.length}{more ? '+' : ''}</>} className="known-work" align="end">
    <p>{copy.scope}</p>
    <WorkLinks entries={entries} />
    {more && <a className="secondary-button" href="#/activity">{messages.agentRuntime.history}</a>}
  </UtilityPopover>;
}
