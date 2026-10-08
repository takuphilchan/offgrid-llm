import { useKnownWork } from '../../lib/active-work';
import { useI18n } from '../../i18n';
import { UtilityPopover } from '../UtilityPopover';
import { knownWorkText } from '../../i18n/known-work';
import { WorkLinks } from '../WorkLinks';

export function KnownWork() {
  const { entries, more } = useKnownWork();
  const { locale, messages } = useI18n(), copy = knownWorkText[locale];
  if (!entries.length) return null;
  return <UtilityPopover label={copy.title} trigger={<>{copy.title} · {entries.length}{more ? '+' : ''}</>} className="known-work" align="end">
    <p>{copy.scope}</p>
    <WorkLinks entries={entries} />
    {more && <a className="secondary-button" href="#/agents">{messages.agentRuntime.history}</a>}
  </UtilityPopover>;
}
