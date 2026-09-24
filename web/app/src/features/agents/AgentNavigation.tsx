import {useI18n} from '../../i18n';
import {computerExperience} from '../../i18n/computer-experience';

export function AgentNavigation({view = 'workspace'}: {view?: 'workspace'|'tools'|'connections'}) {
  const {messages: text, locale} = useI18n();
  const copy = computerExperience(locale);
  return <nav className="task-management" aria-label={text.nav.agents}>
    <a id="agent-workspace-tab" href="#/agents" aria-current={view === 'workspace' ? 'page' : undefined}>{text.shell.work}</a>
    <a id="agent-tools-tab" href="#/agents/tools" aria-current={view === 'tools' ? 'page' : undefined}>{text.agentRuntime.tools}</a>
    <a id="agent-connections-tab" href="#/agents/connections" aria-current={view === 'connections' ? 'page' : undefined}>{copy.connections}</a>
  </nav>;
}
