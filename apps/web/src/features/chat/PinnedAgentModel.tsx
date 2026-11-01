import type { AgentProfile } from '../../api/client';
import { Icon } from '../../components/Icon';
import { UtilityPopover } from '../../components/UtilityPopover';
import { useI18n } from '../../i18n';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { readableModelName } from '../models/model-display';

/** Inspection only: the profile revision, not this display name, selects inference. */
export function PinnedAgentModel({ profile }: { profile: AgentProfile }) {
  const { locale, messages } = useI18n(), copy = agentProfilesText[locale];
  return <UtilityPopover label={messages.chat.model} className="chat-agent-model" triggerClassName="utility-button pinned-model-trigger"
    trigger={<><Icon name="models" size={16} /><span>{readableModelName(profile.settings.model)}</span></>}>
    <strong>{profile.settings.name} · {copy.revision.replace('{revision}', String(profile.revision))}</strong>
    <p><code>{profile.settings.model}</code></p>
    <p>{copy.future}</p><a className="secondary-button" href="#/agents/profiles">{copy.profiles}</a>
  </UtilityPopover>;
}
