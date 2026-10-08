import { refreshWorkspace } from './lib/workspace-refresh';
import { supportsChat } from './api/model-capabilities';
import { Component, useCallback, useEffect, useRef, useState, type ErrorInfo, type KeyboardEvent, type ReactNode } from 'react';
import { APIError, api, setAgentReplayScope, type Model, type PublicUser } from './api/client';
import { CommandPalette, type CommandAction } from './components/CommandPalette';
import { Icon } from './components/Icon';
import { ActivityPage } from './features/activity/ActivityPage';
import { AgentPage } from './features/agents/AgentPage';
import { ComputerActivity } from './features/agents/ComputerActivity';
import { ChatPage } from './features/chat/ChatPage';
import { LoginPage } from './features/auth/LoginPage';
import { KnowledgePage } from './features/knowledge/KnowledgePage';
import { ModelsPage } from './features/models/ModelsPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { useI18n } from './i18n';
import { WorkspaceHeader } from './components/shell/WorkspaceHeader';
import { WorkspaceNavigation, MobileNavigation } from './components/shell/WorkspaceNavigation';
import { navigationGroups, pageFromLocation, type Page, type ServiceHealth as Health } from './lib/navigation';
import { useTheme } from './theme';
import { readPreference, writePreference } from './lib/preferences';
import { WorkspaceContext, clearWorkspaceState } from './lib/workspace-context';
import { SetupHandoffProvider } from './lib/setup-handoff';
import { WorkspaceDraftProvider, DraftRecoveryNotice } from './lib/draft-scope';
import { ActiveWorkProvider } from './lib/active-work';

class PageBoundary extends Component<{ children: ReactNode; message: string; retry: string }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  componentDidCatch(error: Error, info: ErrorInfo) { console.error('Page render failed', error, info); }
  render() {
    if (this.state.failed) return <div className="page-failure" role="alert"><div><strong>{this.props.message}</strong><p>{this.props.retry}</p></div><button className="primary-button" onClick={() => this.setState({ failed: false })}>{this.props.retry}</button></div>;
    return this.props.children;
  }
}

export function App() {
  const { messages: text, locale } = useI18n();
  const [collapsed, setCollapsed] = useState(() => readPreference('offgrid.sidebar.collapsed') === 'true');
  const toggleNavigation = () => setCollapsed(current => { writePreference('offgrid.sidebar.collapsed', String(!current)); return !current; });
  const theme = useTheme();
  useEffect(() => {
    if (window.electron?.setPresentation) void window.electron.setPresentation({ locale, theme: theme.choice }).catch(() => console.warn('Desktop appearance preferences could not be saved.'));
  }, [locale, theme.choice]);
  const [page, setPage] = useState<Page>(pageFromLocation);
  const [health, setHealth] = useState<Health>('checking');
  const [models, setModels] = useState<Model[]>([]);
  const [model, setModel] = useState(() => readPreference('offgrid.model') ?? '');
  const [access, setAccess] = useState<'checking' | 'ready' | 'login'>('checking');
  const [authUser, setAuthUser] = useState<PublicUser | null>(null);
  const [guest, setGuest] = useState(false);
  const [authRequired, setAuthRequired] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [workspaceID, setWorkspaceID] = useState('');
  const [taskFirst, setTaskFirst] = useState(false);
  const [onboardingPending, setOnboardingPending] = useState(() => readPreference('offgrid.onboarding.complete') !== 'true');
  const [showOnboarding, setShowOnboarding] = useState(() => readPreference('offgrid.onboarding.complete') !== 'true' && readPreference('offgrid.onboarding.stage') !== 'working');
  const [showCommands, setShowCommands] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const refreshLock = useRef(false);
  const accessRevision = useRef(0);
  useEffect(() => {
    const expired = () => { accessRevision.current++; clearWorkspaceState(); setAgentReplayScope('signed-out'); setAccess('login'); setAuthUser(null); };
    window.addEventListener('offgrid:unauthenticated', expired);
    return () => window.removeEventListener('offgrid:unauthenticated', expired);
  }, []);

  const refreshBase = useCallback(async () => {
    const revision = ++accessRevision.current;
    setLoadError('');
    const [healthResult, modelResult, userResult, identityResult] = await Promise.allSettled([api.health(), api.models(), api.currentUser(), api.systemIdentity()]);
    if (revision !== accessRevision.current) return;
    setWorkspaceID(identityResult.status === 'fulfilled' ? identityResult.value.workspace_id ?? '' : '');
    setTaskFirst(identityResult.status === 'fulfilled' && identityResult.value.capabilities?.includes('task-first-agents-v2') === true);
    setHealth(healthResult.status === 'fulfilled' ? 'ready' : 'offline');
    if (userResult.status === 'rejected') {
      if (userResult.reason instanceof APIError && userResult.reason.status === 401) { setAuthUser(null); setAccess('login'); }
      else { setLoadError(userResult.reason instanceof Error ? userResult.reason.message : text.common.error); setAccess('checking'); }
      return;
    }
    // Older services identify anonymous local access as a guest too. Resolve
    // enforcement explicitly; never infer management rights from guest alone.
    let enforced = userResult.value.auth_required;
    if (enforced === undefined) {
      if (userResult.value.guest) { try { enforced = (await api.systemConfig()).require_auth !== false; } catch { enforced = true; } }
      else enforced = userResult.value.authenticated;
    }
    if (revision !== accessRevision.current) return;
    setAuthRequired(enforced);
    setAgentReplayScope(JSON.stringify([userResult.value.user?.id ?? (enforced ? 'guest' : 'local'), userResult.value.user?.role, enforced, identityResult.status === 'fulfilled' ? identityResult.value.workspace_id : null]));
    setAuthUser(userResult.value.authenticated ? userResult.value.user : null);
    setGuest(userResult.value.guest === true);
    if (modelResult.status === 'fulfilled') {
      setAccess('ready');
      setModels(modelResult.value);
      setModel(current => {
        if (current && modelResult.value.some(item => item.id === current && supportsChat(item))) return current;
        return modelResult.value.find(supportsChat)?.id ?? '';
      });
    } else if (modelResult.reason instanceof APIError && modelResult.reason.status === 401) {
      setAccess('login');
      setAuthUser(null);
    } else {
      setAccess('ready');
      setLoadError(modelResult.reason instanceof Error ? modelResult.reason.message : text.common.error);
    }
  }, [text.common.error]);

  useEffect(() => {
    if (!window.location.hash) window.history.replaceState(null, '', '#/chat');
    const syncPage = () => setPage(pageFromLocation());
    window.addEventListener('hashchange', syncPage);
    window.addEventListener('popstate', syncPage);
    return () => { window.removeEventListener('hashchange', syncPage); window.removeEventListener('popstate', syncPage); };
  }, []);
  useEffect(() => { void refreshBase(); const timer = window.setInterval(() => void api.health().then(() => setHealth('ready')).catch(() => setHealth('offline')), 15_000); return () => clearInterval(timer); }, [refreshBase]);
  useEffect(() => { if (model) writePreference('offgrid.model', model); }, [model]);
  useEffect(() => {
    const toggleCommands = (event: globalThis.KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLocaleLowerCase() === 'k' && !showOnboarding) {
        event.preventDefault();
        setShowCommands(current => !current);
      }
    };
    window.addEventListener('keydown', toggleCommands);
    return () => window.removeEventListener('keydown', toggleCommands);
  }, [showOnboarding]);

  const finishOnboarding = useCallback(() => {
    writePreference('offgrid.onboarding.complete', 'true');
    writePreference('offgrid.onboarding.stage', null);
    setOnboardingPending(false);
    setShowOnboarding(false);
  }, []);

  const refreshCurrentWorkspace = async () => {
    if (refreshLock.current) return;
    refreshLock.current = true;
    setRefreshing(true);
    try { await Promise.all([refreshBase(), refreshWorkspace()]); }
    catch (reason) { setLoadError(reason instanceof Error ? reason.message : text.common.error); }
    finally { refreshLock.current = false; setRefreshing(false); }
  };

  const continueSetup = (target: 'chat' | 'models') => {
    if (onboardingPending) writePreference('offgrid.onboarding.stage', 'working');
    setShowOnboarding(false);
    window.location.hash = `#/${target}`;
  };
  const chatModels = models.filter(supportsChat);
  const commandActions: CommandAction[] = [
    ...navigationGroups.flatMap(group => group.items.map(item => ({ id: `page-${item}`, label: text.nav[item], group: text.shell[group.label], icon: item, run: () => { window.location.hash = `#/${item}`; } }))),
    { id: 'theme-system', label: text.shell.systemTheme, group: text.shell.appearance, icon: 'settings', keywords: text.shell.theme, run: () => theme.setChoice('system') },
    { id: 'theme-dark', label: text.shell.darkTheme, group: text.shell.appearance, icon: 'settings', keywords: text.shell.theme, run: () => theme.setChoice('dark') },
    { id: 'theme-light', label: text.shell.lightTheme, group: text.shell.appearance, icon: 'settings', keywords: text.shell.theme, run: () => theme.setChoice('light') }
  ];
  const commandShortcut = navigator.userAgent.includes('Mac') ? '⌘ K' : 'Ctrl K';

  if (access === 'checking') return <div className="boot-screen"><div className="orb"><div /></div><span>{loadError || text.status.checking}</span>{loadError && <button onClick={() => void refreshBase()}>{text.common.retry}</button>}</div>;
  if (access === 'login') return <LoginPage onAuthenticated={user => { setAuthUser(user); setAccess('ready'); void refreshBase(); }} />;

  const logout = async () => {
    accessRevision.current++;
    try {
      await api.logout(); clearWorkspaceState(); setAgentReplayScope('signed-out'); accessRevision.current++; setAccess('login'); setAuthUser(null);
    } catch (reason) { setLoadError(reason instanceof Error ? reason.message : text.common.error); }
  };

  const admin = !authRequired || authUser?.role === 'admin';
  const actor = authUser?.id ?? (authRequired && guest ? 'guest' : 'local');
  const workspaceScope = JSON.stringify([actor, workspaceID, admin, authUser?.role, taskFirst]);
  return <WorkspaceContext.Provider value={{ scope: actor, workspace: workspaceID, admin, knowledge: admin || authUser?.role === 'user' }}><WorkspaceDraftProvider key={workspaceScope}><ActiveWorkProvider taskFirst={taskFirst}><div className={`app-shell${collapsed ? ' navigation-collapsed' : ''}`}>
    <WorkspaceNavigation page={page} health={health} collapsed={collapsed} onToggle={toggleNavigation} />

    <main className="workspace">
      <WorkspaceHeader title={text[page].title} shortcut={commandShortcut} refreshing={refreshing}
        onCommands={() => setShowCommands(true)} onRefresh={() => void refreshCurrentWorkspace()}
        theme={theme.choice} onThemeChange={theme.setChoice} username={authUser?.username} onLogout={() => void logout()} />
      {admin && <ComputerActivity key={authUser?.id ?? 'local'} />}
      {loadError && <div className="error-banner" role="alert"><span>{loadError}</span><button onClick={() => void refreshBase()}>{text.common.retry}</button></div>}
      <section className="page-content">
        {(page === 'chat' || page === 'agents') && <DraftRecoveryNotice />}
        <SetupHandoffProvider key={workspaceScope} refresh={refreshBase}>
        <PageBoundary key={`${page}:${workspaceScope}`} message={text.common.error} retry={text.common.retry}>
          {page === 'chat' && <ChatPage scope={actor} models={models} model={model} setModel={setModel} onboardingPending={onboardingPending} onFirstResponse={finishOnboarding} onOpenModels={() => { window.location.hash = '#/models'; }} />}
          {page === 'knowledge' && <KnowledgePage models={models} onModelsChanged={refreshBase} onOpenModels={() => { window.location.hash = '#/models'; }} />}
          {page === 'agents' && <AgentPage scope={actor} models={models} model={model} setModel={setModel} />}
          {page === 'models' && <ModelsPage models={models} selected={model} setSelected={setModel} onRefresh={refreshBase} onboardingPending={onboardingPending} />}
          {page === 'activity' && <ActivityPage health={health} />}
          {page === 'settings' && <SettingsPage health={health} themeChoice={theme.choice} onThemeChange={theme.setChoice} onShowOnboarding={() => setShowOnboarding(true)} />}
        </PageBoundary>
        </SetupHandoffProvider>
      </section>
    </main>
    <MobileNavigation page={page} />
    {showCommands && !showOnboarding && <CommandPalette actions={commandActions} title={text.shell.quickActions} placeholder={text.shell.searchActions} empty={text.shell.noActions} closeLabel={text.models.cancel} onClose={() => setShowCommands(false)} />}
    {showOnboarding && <Onboarding health={health} chatModelCount={chatModels.length} pending={onboardingPending} onAction={() => continueSetup(chatModels.length > 0 ? 'chat' : 'models')} onRetry={refreshBase} onClose={() => setShowOnboarding(false)} />}
  </div></ActiveWorkProvider></WorkspaceDraftProvider></WorkspaceContext.Provider>;
}

function Onboarding({ health, chatModelCount, pending, onAction, onRetry, onClose }: {
  health: Health;
  chatModelCount: number;
  pending: boolean;
  onAction: () => void;
  onRetry: () => Promise<void>;
  onClose: () => void;
}) {
  const { messages: text } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const element = dialog.current!;
    const previous = document.activeElement as HTMLElement | null;
    element.showModal();
    return () => { element.close(); previous?.focus(); };
  }, []);
  const containFocus = (event: KeyboardEvent<HTMLDialogElement>) => {
    if (event.key !== 'Tab') return;
    const focusable = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'));
    if (focusable.length === 0) { event.preventDefault(); event.currentTarget.focus(); return; }
    const first = focusable[0], last = focusable[focusable.length - 1], active = document.activeElement;
    if (event.shiftKey && (active === first || !event.currentTarget.contains(active))) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && (active === last || !event.currentTarget.contains(active))) { event.preventDefault(); first.focus(); }
  };
  const action = !pending ? text.onboarding.close : health !== 'ready' ? text.onboarding.retryService : chatModelCount > 0 ? text.onboarding.startChat : text.onboarding.chooseModel;
  return <dialog ref={dialog} className="onboarding" aria-labelledby="onboarding-title" onKeyDown={containFocus} onCancel={event => { event.preventDefault(); onClose(); }}>
    <button className="onboarding-close icon-button" onClick={onClose} aria-label={text.onboarding.close}><Icon name="close" size={18} /></button>
    <div className="onboarding-mark"><span /></div><span className="eyebrow">{text.privateWorkspace}</span>
    <h2 id="onboarding-title">{text.onboarding.title}</h2><p>{text.onboarding.body}</p>
    <div className="readiness-list"><div><i className={health === 'ready' ? 'ready' : ''} /><span>{text.onboarding.service}</span><strong>{health === 'ready' ? text.status.ready : text.status.offline}</strong></div><div><i className={chatModelCount > 0 ? 'ready' : ''} /><span>{text.onboarding.models}</span><strong>{chatModelCount}</strong></div><div><span aria-hidden="true" /><span>{text.onboarding.privacy}</span><strong>{text.onboarding.local}</strong></div></div>
    {pending && <p className="onboarding-next">{chatModelCount > 0 ? text.onboarding.firstReplyHint : text.onboarding.modelHint}</p>}
    <button autoFocus className="primary-button" onClick={!pending ? onClose : health !== 'ready' ? () => void onRetry() : onAction}>{action}</button>
  </dialog>;
}
