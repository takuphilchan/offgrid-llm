import { useKnownWork } from '../../lib/active-work';
import { useI18n } from '../../i18n';
import { UtilityPopover } from '../UtilityPopover';
import { knownWorkText, attentionWorkText } from '../../i18n/known-work';
import { WorkLinks } from '../WorkLinks';
import { workBucket } from '../../lib/work-state';

export function KnownWork() {
  const { entries, more } = useKnownWork();
  const { locale, messages } = useI18n(), copy = knownWorkText[locale];
  const relevant = entries.filter(entry => !['completed', 'failed', 'cancelled'].includes(entry.status));
  const running = relevant.filter(entry => workBucket(entry.status, entry.stale) === 'running').length;
  const attention = relevant.filter(entry => workBucket(entry.status, entry.stale) === 'attention').length;
  if (!relevant.length) return null;
  return <UtilityPopover label={copy.title} triggerClassName="utility-button" trigger={<>{attention ? attentionWorkText[locale] : messages.recovery.running} · {attention || running || relevant.length}{more ? '+' : ''}</>} className="known-work" align="end" preferredHeight={440} panelTabIndex={-1}>
    <p>{copy.scope}</p>
    <WorkLinks entries={[...entries].sort((a,b) => ({attention:0,running:1,history:2}[workBucket(a.status,a.stale)] - {attention:0,running:1,history:2}[workBucket(b.status,b.stale)]))} />
    <a className="text-button" href={attention ? '#/activity/attention' : '#/activity/running'} onClick={event => event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover()}>{messages.nav.activity}</a>
  </UtilityPopover>;
}
