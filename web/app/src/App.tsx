import { Component, useCallback, useEffect, useRef, useState, type ErrorInfo, type ReactNode } from 'react';
import { APIError, api, type Model, type PublicUser } from './api/client';
import { CommandPalette, type CommandAction } from './components/CommandPalette';
import { Icon } from './components/Icon';
import { ActivityPage } from './features/activity/ActivityPage';
import { AgentPage } from './features/agents/AgentPage';
import { ChatPage } from './features/chat/ChatPage';
import { LoginPage } from './features/auth/LoginPage';
import { KnowledgePage } from './features/knowledge/KnowledgePage';
import { ModelsPage } from './features/models/ModelsPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { useI18n, type LocaleCode } from './i18n';
import { useTheme } from './theme';

type Page = 'chat' | 'knowledge' | 'agents' | 'models' | 'activity' | 'settings';
type Health = 'checking' | 'ready' | 'offline';

const pages: Page[] = ['chat', 'knowledge', 'agents', 'models', 'activity', 'settings'];
const navigationGroups: { label: 'work' | 'library' | 'system'; items: Page[] }[] = [
  { label: 'work', items: ['chat', 'agents'] },
  { label: 'library', items: ['knowledge', 'models'] },
  { label: 'system', items: ['activity', 'settings'] }
];

function pageFromLocation(): Page {
  const candidate = window.location.hash.replace(/^#\/?/, '').split('/')[0];
  return pages.includes(candidate as Page) ? candidate as Page : 'chat';
}

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
  const { messages: text, locale, setLocale, available } = useI18n();
  const theme = useTheme();
  const [page, setPage] = useState<Page>(pageFromLocation);
  const [health, setHealth] = useState<Health>('checking');
  const [models, setModels] = useState<Model[]>([]);
  const [model, setModel] = useState(localStorage.getItem('offgrid.model') ?? '');
  const [access, setAccess] = useState<'checking' | 'ready' | 'login'>('checking');
  const [authUser, setAuthUser] = useState<PublicUser | null>(null);
  const [loadError, setLoadError] = useState('');
  const [onboardingPending, setOnboardingPending] = useState(() => localStorage.getItem('offgrid.onboarding.complete') !== 'true');
  const [showOnboarding, setShowOnboarding] = useState(() => localStorage.getItem('offgrid.onboarding.complete') !== 'true' && localStorage.getItem('offgrid.onboarding.stage') !== 'working');
  const [showCommands, setShowCommands] = useState(false);
  const accessRevision = useRef(0);

  const refreshBase = useCallback(async () => {
    const revision = ++accessRevision.current;
    setLoadError('');
    const [healthResult, modelResult, userResult] = await Promise.allSettled([api.health(), api.models(), api.currentUser()]);
    if (revision !== accessRevision.current) return;
    setHealth(healthResult.status === 'fulfilled' ? 'ready' : 'offline');
    if (userResult.status === 'rejected') {
      if (userResult.reason instanceof APIError && userResult.reason.status === 401) { setAuthUser(null); setAccess('login'); }
      else { setLoadError(userResult.reason instanceof Error ? userResult.reason.message : text.common.error); setAccess('checking'); }
      return;
    }
    setAuthUser(userResult.value.authenticated ? userResult.value.user : null);
    if (modelResult.status === 'fulfilled') {
      setAccess('ready');
      setModels(modelResult.value);
      setModel(current => {
        if (current && modelResult.value.some(item => item.id === current && item.type !== 'embedding')) return current;
        return modelResult.value.find(item => item.type !== 'embedding')?.id ?? '';
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
  useEffect(() => { if (model) localStorage.setItem('offgrid.model', model); }, [model]);
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
    localStorage.setItem('offgrid.onboarding.complete', 'true');
    localStorage.removeItem('offgrid.onboarding.stage');
    setOnboardingPending(false);
    setShowOnboarding(false);
  }, []);

  const continueSetup = (target: 'chat' | 'models') => {
    if (onboardingPending) localStorage.setItem('offgrid.onboarding.stage', 'working');
    setShowOnboarding(false);
    window.location.hash = `#/${target}`;
  };
  const chatModels = models.filter(item => item.type !== 'embedding');
  const currentGroup = navigationGroups.find(group => group.items.includes(page))?.label ?? 'work';
  const commandActions: CommandAction[] = [
    ...navigationGroups.flatMap(group => group.items.map(item => ({ id: `page-${item}`, label: text.nav[item], group: text.shell[group.label], icon: item, run: () => { window.location.hash = `#/${item}`; } }))),
    { id: 'theme-system', label: text.shell.systemTheme, group: text.shell.appearance, icon: 'settings', keywords: text.shell.theme, run: () => theme.setChoice('system') },
    { id: 'theme-dark', label: text.shell.darkTheme, group: text.shell.appearance, icon: 'settings', keywords: text.shell.theme, run: () => theme.setChoice('dark') },
    { id: 'theme-light', label: text.shell.lightTheme, group: text.shell.appearance, icon: 'settings', keywords: text.shell.theme, run: () => theme.setChoice('light') }
  ];
  const commandShortcut = navigator.userAgent.includes('Mac') ? '⌘ K' : 'Ctrl K';

  const titles = {
    chat: [text.chat.title, text.chat.subtitle], knowledge: [text.knowledge.title, text.knowledge.subtitle],
    agents: [text.agents.title, text.agents.subtitle], models: [text.models.title, text.models.subtitle],
    activity: [text.activity.title, text.activity.subtitle], settings: [text.settings.title, text.settings.subtitle]
  };

  if (access === 'checking') return <div className="boot-screen"><div className="orb"><div /></div><span>{loadError || text.status.checking}</span>{loadError && <button onClick={() => void refreshBase()}>{text.common.retry}</button>}</div>;
  if (access === 'login') return <LoginPage onAuthenticated={user => { setAuthUser(user); setAccess('ready'); void refreshBase(); }} />;

  const logout = async () => {
    accessRevision.current++;
    try {
      await api.logout(); accessRevision.current++; setAccess('login'); setAuthUser(null);
    } catch (reason) { setLoadError(reason instanceof Error ? reason.message : text.common.error); }
  };

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark"><span /></div><div><strong>{text.product}</strong><small>{text.privateWorkspace}</small></div></div>
      <nav className="primary-nav" aria-label="Primary">
        {navigationGroups.map(group => <div className="nav-group" key={group.label}>
          <span className="nav-group-label">{text.shell[group.label]}</span>
          {group.items.map(item => <a key={item} href={`#/${item}`} className={page === item ? 'nav-link active' : 'nav-link'} aria-current={page === item ? 'page' : undefined} title={text.nav[item]}>
            <Icon name={item} /><span>{text.nav[item]}</span>
          </a>)}
        </div>)}
      </nav>
      <div className="sidebar-foot">
        <div className={`service-state ${health}`}><i /> <span>{health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking}</span></div>
        <div className="privacy-note"><span>100%</span> {text.common.localProcessing}</div>
      </div>
    </aside>

    <main className="workspace">
      <header className="topbar">
        <div><span className="workspace-kicker">{text.shell[currentGroup]}</span><h1>{titles[page][0]}</h1><p>{titles[page][1]}</p></div>
        <div className="topbar-actions">
          <button className="command-trigger" onClick={() => setShowCommands(true)} aria-label={`${text.shell.quickActions} (${commandShortcut})`}><Icon name="search" size={16} /><span>{text.shell.quickActions}</span><kbd>{commandShortcut}</kbd></button>
          <label className="locale-picker"><span>{text.common.language}</span><select value={locale} onChange={event => setLocale(event.target.value as LocaleCode)}>{available.map(item => <option key={item.code} value={item.code}>{item.label}</option>)}</select></label>
          <button className="icon-button" onClick={() => void refreshBase()} aria-label={text.common.refresh}><Icon name="refresh" size={18} /></button>
          {authUser && <button className="text-button" onClick={() => void logout()}>{text.auth.signOut}</button>}
        </div>
      </header>
      {loadError && <div className="error-banner" role="alert"><span>{loadError}</span><button onClick={() => void refreshBase()}>{text.common.retry}</button></div>}
      <section className="page-content">
        <PageBoundary key={`${page}:${authUser?.id ?? 'local'}`} message={text.common.error} retry={text.common.retry}>
          {page === 'chat' && <ChatPage scope={authUser?.id ?? 'local'} models={models} model={model} setModel={setModel} onboardingPending={onboardingPending} onFirstResponse={finishOnboarding} onOpenModels={() => { window.location.hash = '#/models'; }} />}
          {page === 'knowledge' && <KnowledgePage models={models} onModelsChanged={refreshBase} onOpenModels={() => { window.location.hash = '#/models'; }} />}
          {page === 'agents' && <AgentPage scope={authUser?.id ?? 'local'} models={models} model={model} setModel={setModel} />}
          {page === 'models' && <ModelsPage models={models} selected={model} setSelected={setModel} onRefresh={refreshBase} onboardingPending={onboardingPending} />}
          {page === 'activity' && <ActivityPage health={health} />}
          {page === 'settings' && <SettingsPage health={health} themeChoice={theme.choice} onThemeChange={theme.setChoice} onShowOnboarding={() => setShowOnboarding(true)} />}
        </PageBoundary>
      </section>
    </main>
    <nav className="mobile-nav" aria-label="Primary">{pages.map(item => <a key={item} href={`#/${item}`} className={page === item ? 'active' : ''} aria-current={page === item ? 'page' : undefined}><Icon name={item} size={19} /><span>{text.nav[item]}</span></a>)}</nav>
    {showCommands && !showOnboarding && <CommandPalette actions={commandActions} title={text.shell.quickActions} placeholder={text.shell.searchActions} empty={text.shell.noActions} onClose={() => setShowCommands(false)} />}
    {showOnboarding && <Onboarding health={health} chatModelCount={chatModels.length} pending={onboardingPending} onAction={() => continueSetup(chatModels.length > 0 ? 'chat' : 'models')} onRetry={refreshBase} onClose={() => setShowOnboarding(false)} />}
  </div>;
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
  useEffect(() => {
    const closeOnEscape = (event: globalThis.KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [onClose]);
  const action = !pending ? text.onboarding.close : health !== 'ready' ? text.onboarding.retryService : chatModelCount > 0 ? text.onboarding.startChat : text.onboarding.chooseModel;
  return <div className="modal-backdrop" role="presentation"><section className="onboarding" role="dialog" aria-modal="true" aria-labelledby="onboarding-title">
    <button className="onboarding-close icon-button" onClick={onClose} aria-label={text.onboarding.close}><Icon name="close" size={18} /></button>
    <div className="onboarding-mark"><span /></div><span className="eyebrow">{text.privateWorkspace}</span>
    <h2 id="onboarding-title">{text.onboarding.title}</h2><p>{text.onboarding.body}</p>
    <div className="readiness-list"><div><i className={health === 'ready' ? 'ready' : ''} /><span>{text.onboarding.service}</span><strong>{health === 'ready' ? text.status.ready : text.status.offline}</strong></div><div><i className={chatModelCount > 0 ? 'ready' : ''} /><span>{text.onboarding.models}</span><strong>{chatModelCount}</strong></div><div><i className="ready" /><span>{text.onboarding.privacy}</span><strong>{text.onboarding.local}</strong></div></div>
    {pending && <p className="onboarding-next">{chatModelCount > 0 ? text.onboarding.firstReplyHint : text.onboarding.modelHint}</p>}
    <button autoFocus className="primary-button" onClick={!pending ? onClose : health !== 'ready' ? () => void onRetry() : onAction}>{action}</button>
  </section></div>;
}
