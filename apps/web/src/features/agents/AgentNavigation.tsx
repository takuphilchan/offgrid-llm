import {Icon} from '../../components/Icon';
import './agent-management-polish.css';
import {useI18n} from '../../i18n';
import {agentManagement} from '../../i18n/agent-management';
import {agentProfilesText} from '../../i18n/agent-profiles';
import {useWorkspace} from '../../lib/workspace-context';

export function AgentNavigation({view = 'workspace'}: {view?: 'workspace'|'profiles'|'tools'|'connections'|'external'}) {
  const {messages: text, locale} = useI18n();
  const {admin} = useWorkspace();
  const copy = agentManagement[locale];
  return <nav className="task-management agent-navigation" aria-label={text.nav.agents}>
    <a id="agent-profiles-tab" href="#/agents/profiles" aria-current={view === 'profiles' ? 'page' : undefined}><Icon name="agents" size={17}/>{agentProfilesText[locale].profiles}</a>
    {admin && <>
    <a id="agent-tools-tab" href="#/agents/tools" aria-current={view === 'tools' || view === 'connections' ? 'page' : undefined}><Icon name="plug" size={17}/>{copy.tools}</a>
    <a id="agent-external-tab" href="#/agents/external" aria-current={view === 'external' ? 'page' : undefined}><Icon name="external" size={17}/>{copy.external}</a>
    </>}
  </nav>;
}
