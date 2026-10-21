import { useEffect, useRef, useState } from 'react';
import { api, type Model } from '../../api/client';
import { useI18n } from '../../i18n';
import { interaction } from '../../i18n/interaction';
import { agentAvailability } from '../../i18n/agent-availability';
import { useWorkspace } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { AgentNavigation } from './AgentNavigation';
import { AgentManagement } from './AgentManagement';
import { AgentProfiles } from './AgentProfiles';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { ScopedNotice } from '../../components/WorkspacePresentation';

type AgentView = 'home' | 'workspace' | 'profiles' | 'tools' | 'connections';
function viewFromLocation(): AgentView {
  const candidate = window.location.hash.split('/')[2];
  return candidate === 'tools' || candidate === 'connections' || candidate === 'profiles' ? candidate : candidate ? 'workspace' : 'home';
}

export function AgentPage({ models, model }: { models: Model[]; model: string }) {
  const { admin } = useWorkspace();
  const { locale, messages } = useI18n();
  const copy = agentAvailability[locale];
  const [identity, setIdentity] = useState<{ workspace: string; profiles: boolean } | null>(null);
  const [identityError, setIdentityError] = useState(false);
  const [view, setView] = useState(viewFromLocation);
  const request = useRef(0);
  const refresh = async () => {
    const revision = ++request.current;
    try {
      const next = await api.systemIdentity();
      if (revision !== request.current) return;
      const workspace = next.workspace_id ?? 'legacy';
      // Compatibility is not health. Older services withdraw their capability
      // on storage failure; keep the current draft, never switch renderers.
      setIdentity(current => ({ workspace,
        profiles: next.capabilities?.includes('agent-profiles-v1') === true || (current?.workspace === workspace && current.profiles) === true }));
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
  if (view === 'profiles' || view === 'home' || view === 'workspace') return <><AgentNavigation view="profiles" />{identity?.profiles
    ? <AgentProfiles key={identity.workspace} models={models} />
    : identityError || identity ? <ScopedNotice kind="error" action={<button className="secondary-button" onClick={() => void refresh()}>{messages.common.retry}</button>}>{identityError ? copy.connection : agentProfilesText[locale].unsupported}</ScopedNotice> : <p role="status">{messages.common.loading}</p>}</>;
  if (!admin) return <><AgentNavigation /><p className="permission-notice" role="status">{interaction[locale].adminOnly}</p></>;
  return <AgentManagement model={model} view={view} />;
}
