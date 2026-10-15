import { useEffect, useRef, useState } from 'react';
import { api, type Model } from '../../api/client';
import { useI18n } from '../../i18n';
import { interaction } from '../../i18n/interaction';
import { agentAvailability } from '../../i18n/agent-availability';
import { useWorkspace } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { TaskWorkspace } from './TaskWorkspace';
import { AgentNavigation } from './AgentNavigation';
import { AgentManagement } from './AgentManagement';
import { AgentProfiles } from './AgentProfiles';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { useTaskRun } from './useTaskRun';
import { ConversationWork } from '../chat/ConversationWork';

type AgentView = 'home' | 'workspace' | 'profiles' | 'tools' | 'connections';
function viewFromLocation(): AgentView {
  const candidate = window.location.hash.split('/')[2];
  return candidate === 'tools' || candidate === 'connections' || candidate === 'profiles' ? candidate : candidate ? 'workspace' : 'home';
}

function ProfileWorkInspector({ id }: { id: string }) {
  const { run, refresh, error, connection } = useTaskRun(id);
  const { messages } = useI18n();
  return <><AgentNavigation /><h2>{run?.prompt ?? messages.agents.task}</h2>{run
    ? <ConversationWork run={run} refresh={refresh} readError={error} connection={connection} outputSaved={false} recoveryNeeded={false} recover={async () => {}} />
    : <p role={error ? 'alert' : 'status'}>{error || messages.common.loading}</p>}</>;
}

export function AgentPage({ scope, models, model, setModel }: { scope: string; models: Model[]; model: string; setModel: (model: string) => void }) {
  const { admin } = useWorkspace();
  const { locale, messages } = useI18n();
  const copy = agentAvailability[locale];
  const [identity, setIdentity] = useState<{ workspace: string; supported: boolean; available: boolean; profiles: boolean } | null>(null);
  const [identityError, setIdentityError] = useState(false);
  const [view, setView] = useState(viewFromLocation);
  const request = useRef(0);
  const refresh = async () => {
    const revision = ++request.current;
    try {
      const next = await api.systemIdentity();
      if (revision !== request.current) return;
      const workspace = next.workspace_id ?? 'legacy';
      const available = next.capabilities?.includes('task-first-agents-v2') === true;
      // Compatibility is not health. Older services withdraw their capability
      // on storage failure; keep the current draft, never switch renderers.
      setIdentity(current => ({ workspace, available,
        profiles: next.capabilities?.includes('agent-profiles-v1') === true || (current?.workspace === workspace && current.profiles) === true,
        supported: available || (current?.workspace === workspace && current.supported) }));
      setIdentityError(false);
    } catch {
      if (revision === request.current) setIdentityError(true);
    }
  };
  useEffect(() => {
    const changed = () => setView(viewFromLocation());
    window.addEventListener('hashchange', changed);
    return () => window.removeEventListener('hashchange', changed);
  }, []);
  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 5000);
    return () => { request.current++; clearInterval(timer); };
  }, [admin]);
  useWorkspaceRefresh(refresh);
  if (view === 'profiles' || (view === 'home' && identity?.profiles)) return <><AgentNavigation view="profiles" />{identity?.profiles
    ? <AgentProfiles key={identity.workspace} models={models} />
    : <p role="status">{identityError ? copy.connection : identity ? agentProfilesText[locale].unsupported : messages.common.loading}</p>}</>;
  if (!admin && window.location.hash.startsWith('#/agents/task/')) return <ProfileWorkInspector key={window.location.hash} id={window.location.hash.split('/')[3] ?? ''} />;
  if (!admin) return <><AgentNavigation /><p className="permission-notice" role="status">{interaction[locale].adminOnly}</p></>;
  if (view === 'tools' || view === 'connections') return <AgentManagement model={model} view={view} />;
  const unavailable = identityError || (identity !== null && !identity.available);
  return <>
    {(!identity?.supported) && <AgentNavigation />}
    {unavailable && <div className="inline-error" role="alert">
      <p>{identityError ? copy.connection : copy.unsupported}</p>
      <button type="button" className="secondary-button" onClick={() => void refresh()}>{messages.common.retry}</button>
    </div>}
    {identity?.supported
      ? <TaskWorkspace key={identity.workspace} scope={scope} workspace={identity.workspace} />
      : !unavailable && <p role="status">{messages.common.loading}</p>}
  </>;
}
