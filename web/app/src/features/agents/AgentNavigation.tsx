import {useI18n} from '../../i18n';
import {computerExperience} from '../../i18n/computer-experience';
import {agentProfilesText} from '../../i18n/agent-profiles';
import {useWorkspace} from '../../lib/workspace-context';

export function AgentNavigation({view = 'workspace'}: {view?: 'workspace'|'profiles'|'tools'|'connections'}) {
  const {messages: text, locale} = useI18n();
  const {admin} = useWorkspace();
  const copy = computerExperience(locale);
  return <nav className="task-management" aria-label={text.nav.agents}>
    <a id="agent-profiles-tab" href="#/agents/profiles" aria-current={view === 'profiles' ? 'page' : undefined}>{agentProfilesText[locale].profiles}</a>
    {admin && <>
    <a id="agent-tools-tab" href="#/agents/tools" aria-current={view === 'tools' ? 'page' : undefined}>{text.agentRuntime.tools}</a>
    <a id="agent-connections-tab" href="#/agents/connections" aria-current={view === 'connections' ? 'page' : undefined}>{copy.connections}</a>
    </>}
  </nav>;
}
