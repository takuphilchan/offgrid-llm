import { useWorkspace } from '../../lib/workspace-context';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { useI18n, type LanguageCode } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import type { DesktopPaths, DesktopBackend } from '../../platform';
import type { ThemeChoice } from '../../theme';
import { SectionHeading } from '../../components/WorkspacePresentation';
import { workspaceManagement } from '../../i18n/workspace-management';
import { desktopCompatibility } from '../../i18n/desktop-compatibility';
import { VoiceSettings } from '../../components/VoiceSettings';
import { ComposerResponseLimits } from '../chat/ComposerOptions';
import {useResponsePreferences} from '../chat/useResponsePreferences';
import {responseDisplay} from '../../i18n/response-display';
import { settingsNavigation, settingsSections, type SettingsSection as BaseSettingsSection } from '../../i18n/settings-navigation';
import { SearchField } from '../../components/SearchField';
import { computerStatus } from '../../i18n/computer-status';
import './settings.css';
import { AppearanceSettings, ThemeSample } from './AppearanceSettings';
import { palettes, paletteLabel } from '../../appearance';
import { personalization } from '../../i18n/personalization';
import {DesktopCommandSettings} from './DesktopCommandSettings';
import {desktopCommand} from '../../i18n/desktop-command';
import { SidebarSlot } from '../../components/shell/SidebarSlot';
import { Icon, type IconName } from '../../components/Icon';
import { ActionCard } from '../../components/ActionCard';
import { profileSelectionPrompt } from '../../i18n/agent-profiles';

type SettingsSection = BaseSettingsSection | 'appearance' | 'shortcuts' | 'storage';
const sectionIcons: Record<SettingsSection, IconName> = { general: 'settings', appearance: 'palette', shortcuts: 'quick-window', storage: 'drive', models: 'models', voice: 'mic', terminal: 'terminal', access: 'shield', advanced: 'info' };

type Health = 'checking' | 'ready' | 'offline';
type Details = {
  identity?: Awaited<ReturnType<typeof api.systemIdentity>>;
  config?: Awaited<ReturnType<typeof api.systemConfig>>;
  rag?: Awaited<ReturnType<typeof api.ragStatus>>;
  computer?: Awaited<ReturnType<typeof api.computerStatus>>;
  localComputer?: { installed: boolean };
};

export function SettingsPage({ health, themeChoice, onThemeChange, onShowOnboarding, inSidebar = false, username, onLogout }: {
  inSidebar?: boolean;
  username?: string;
  onLogout?: () => void;
  health: Health;
  themeChoice: ThemeChoice;
  onThemeChange: (choice: ThemeChoice) => void;
  onShowOnboarding: () => void;
}) {
  const { messages: text, locale, language, available, setLocale, translationNotice } = useI18n();
  const { admin, knowledge, scope } = useWorkspace();
  const copy = settingsNavigation[locale];
  const sectionKeys: SettingsSection[] = ['general','appearance','models','voice',...(window.electron?.setCommandShortcut ? ['shortcuts' as const] : []),'terminal','access',...(window.electron?.getPaths ? ['storage' as const] : []),'advanced'];
  const [focusTarget,setFocusTarget] = useState<string|null>(null);
  const [section, setSection] = useState<SettingsSection>('general');
  const [query, setQuery] = useState('');
  const [unsaved, setUnsaved] = useState<string[]>([]);
  useEffect(()=>{const saved=(event:Event)=>{const {key,saved}= (event as CustomEvent<{key:string;saved:boolean}>).detail;if(key==='offgrid.appearance.v1')return;setUnsaved(keys=>saved?keys.filter(item=>item!==key):[...new Set([...keys,key])]);};window.addEventListener('offgrid-preference-save',saved);return()=>window.removeEventListener('offgrid-preference-save',saved);},[]);
  const settingTargets = [
    {id:'language',section:'general',label:text.common.language,selector:'#settings-language'},
    {id:'theme',section:'appearance',label:text.shell.theme,selector:'#setting-theme button'},
    {id:'appearance',section:'appearance',label:personalization[locale][0],selector:'#setting-appearance'},
    {id:'text-size',section:'appearance',label:personalization[locale][3],selector:'#appearance-text-size'},
    {id:'motion',section:'appearance',label:personalization[locale][4],selector:'#setting-appearance input'},
    {id:'responses',section:'models',label:text.chatStreaming.responseLimit+' '+responseDisplay(locale).label,selector:'#model-settings-title'},
    {id:'voice',section:'voice',label:copy.sections[2],selector:'#voice-settings-title'},
    {id:'terminal',section:'terminal',label:copy.sections[3],selector:'#terminal-settings-title'},
    {id:'quick-window',section:'shortcuts',label:desktopCommand(locale).title,selector:'#desktop-command-shortcut'},
    {id:'access',section:'access',label:copy.sections[4],selector:'#access-settings-title'},
    {id:'service',section:'advanced',label:text.settings.service,selector:'#workspace-title'}
  ] as const;
  const searchTargets = query.trim() ? settingTargets.filter(item=>item.label.toLocaleLowerCase(language).includes(query.trim().toLocaleLowerCase(language))) : [];
  const revealSetting = (id: string) => {
    if(sectionKeys.includes(id as SettingsSection)){setQuery('');setFocusTarget(null);setSection(id as SettingsSection);return;}
    const item=settingTargets.find(item=>item.id===id);if(!item)return;
    setQuery('');setSection(item.section);setFocusTarget(item.selector);
  };
  useEffect(()=>{if(!focusTarget)return;const element=document.querySelector<HTMLElement>(focusTarget);if(element){if(!element.matches('input,select,button'))element.tabIndex=-1;element.focus();element.scrollIntoView({block:'nearest'});setFocusTarget(null);}},[focusTarget,section,query]);
  useEffect(()=>{const navigate=()=>{const parts=location.hash.split('/');if(parts[1]==='settings'&&parts[2])revealSetting(parts[2]);};navigate();window.addEventListener('hashchange',navigate);return()=>window.removeEventListener('hashchange',navigate);},[locale]);
  const preferencesKey = 'offgrid.chat.preferences:' + encodeURIComponent(scope);
  const [preferences, updatePreferences] = useResponsePreferences(preferencesKey);
  const terms = [text.common.language+' '+text.shell.theme+' '+text.settings.showGuide+' '+personalization[locale].join(' ')+' '+palettes.map(id=>paletteLabel(id,'light')+' '+paletteLabel(id,'dark')).join(' '), text.chatStreaming.profile+' '+text.chatStreaming.responseLimit+' '+text.nav.models, text.nav.models, 'Aa', text.settings.computerUse+' '+text.shell.storage, text.settings.service+' '+text.settings.version+' API'];
  const sectionTitle = (key: SettingsSection) => key === 'appearance' ? personalization[locale][0] : key === 'shortcuts' ? desktopCommand(locale).title : key === 'storage' ? text.shell.storage : copy.sections[settingsSections.indexOf(key)];
  const description = (key: SettingsSection) => key === 'appearance' ? personalization[locale][9] : key === 'shortcuts' ? desktopCommand(locale).background : key === 'storage' ? workspaceManagement(locale).desktopStorage : copy.descriptions[settingsSections.indexOf(key)];
  const matches = sectionKeys.filter(key => (sectionTitle(key)+' '+description(key)+' '+(key === 'appearance' ? terms[0] : key === 'shortcuts' ? desktopCommand(locale).shortcut : key === 'storage' ? text.shell.modelsPath+' '+text.shell.dataPath : terms[settingsSections.indexOf(key)])+' '+settingTargets.filter(item=>item.section===key).map(item=>item.label).join(' ')).toLocaleLowerCase(language).includes(query.trim().toLocaleLowerCase(language)));
  const shown = (key: SettingsSection) => query.trim() ? matches.includes(key) : key === section;
  const [details, setDetails] = useState<Details>({});
  const [error, setError] = useState('');
  const [desktopPaths, setDesktopPaths] = useState<DesktopPaths | null>(null);
  const [backend, setBackend] = useState<DesktopBackend | null>(null);
  const [loading, setLoading] = useState(true);
  const [stopping, setStopping] = useState(false);
  const stopPending = useRef(false);
  const refreshRevision = useRef(0);
  const management = workspaceManagement(locale);
  const compatibility = desktopCompatibility(locale);
  const unknown = loading ? text.common.loading : presentation[locale].unknown;

  const refresh = async () => {
    const revision = ++refreshRevision.current;
    setLoading(true);
    setError('');
    const [config, rag, computer, identity, localComputer] = await Promise.allSettled([api.systemConfig(), knowledge ? api.ragStatus() : Promise.resolve(undefined), admin ? api.computerStatus() : Promise.resolve(undefined), api.systemIdentity(), admin ? window.electron?.getComputerStatus?.() : Promise.resolve(undefined)]);
    if (revision !== refreshRevision.current) return;
    setDetails({
      identity: identity.status === 'fulfilled' ? identity.value : undefined,
      config: config.status === 'fulfilled' ? config.value : undefined,
      rag: rag.status === 'fulfilled' ? rag.value : undefined,
      computer: computer.status === 'fulfilled' ? computer.value : undefined,
      localComputer: localComputer.status === 'fulfilled' ? localComputer.value : undefined
    });
    const failed = [config, rag, computer, identity].find(item => item.status === 'rejected');
    if (failed?.status === 'rejected') setError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    setLoading(false);
  };
  useWorkspaceRefresh(refresh);

  useEffect(() => {
    let disposed = false;
    void refresh();
    if (window.electron?.getPaths) void window.electron.getPaths().then(value => { if (!disposed) setDesktopPaths(value); }).catch(() => { if (!disposed) setDesktopPaths(null); });
    if (window.electron?.getBackendInfo) void window.electron.getBackendInfo().then(value => { if (!disposed) setBackend(value); }).catch(() => { if (!disposed) setBackend(null); });
    return () => { disposed = true; refreshRevision.current++; };
  }, [admin, knowledge]);
  const stopComputer = async () => {
    if (!admin || stopPending.current) return;
    stopPending.current = true; setStopping(true); setError('');
    try { await api.emergencyStop(); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { stopPending.current = false; setStopping(false); }
  };

  const options: { choice: ThemeChoice; label: string }[] = [
    { choice: 'system', label: text.shell.systemTheme },
    { choice: 'light', label: text.shell.lightTheme },
    { choice: 'dark', label: text.shell.darkTheme }
  ];
  const computerCopy = computerStatus[locale];
  const computerDescription = !admin ? interaction[locale].adminOnly
    : details.computer?.active_sessions ? computerCopy.connected
    : details.localComputer ? (details.localComputer.installed ? computerCopy.installed : computerCopy.missing)
    : window.electron?.getComputerStatus || !details.computer ? unknown : computerCopy.idle;

  const navigation = <div className="settings-navigation">
      <SearchField className="settings-search" label={copy.search} value={query} onChange={setQuery} />
      <nav aria-label={text.nav.settings}>{sectionKeys.map(key => <button key={key} data-section={key} aria-current={!query.trim() && section === key ? 'page' : undefined} onClick={() => { setQuery(''); setSection(key);location.hash=`#/settings/${key}`; }}><Icon name={sectionIcons[key]} size={17} /><span>{sectionTitle(key)}</span></button>)}</nav>
    </div>;
  return <div className="settings-page settings-workspace" data-context-navigation={inSidebar || undefined}>
    {inSidebar ? <SidebarSlot>{navigation}</SidebarSlot> : navigation}
    <div className="settings-content">
    {!!unsaved.length && <p role="status">{personalization[locale][10]}</p>}
    {!!searchTargets.length && <nav className="settings-results" aria-label={copy.search}>{searchTargets.map(item=><a key={item.id} href={`#/settings/${item.id}`} onClick={()=>revealSetting(item.id)}>{item.label}</a>)}</nav>}
    {query.trim() && !matches.length && <p role="status">{copy.empty}</p>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {backend?.reason && <div className="inline-error" role="alert">{backend.reason}</div>}
    <section hidden={!shown('general')} className="settings-group" aria-labelledby="preferences-title">
      <SectionHeading title={sectionTitle('general')} description={description('general')} id="preferences-title" />
      <div className="settings-panel"><label htmlFor="settings-language">{text.common.language}</label><select id="settings-language" value={language} aria-describedby={translationNotice ? 'translation-notice' : undefined} onChange={event => setLocale(event.target.value as LanguageCode)}>{available.filter(item=>!item.preview).map(item => <option key={item.code} value={item.code}>{item.label}</option>)}<optgroup label={personalization[locale][5]}>{available.filter(item=>item.preview).map(item => <option key={item.code} value={item.code}>{item.label}</option>)}</optgroup></select></div>
      {translationNotice && <p id="translation-notice" role="status" lang={language}>{translationNotice}</p>}
      {username !== undefined && <div className="settings-panel settings-account"><strong>{username}</strong><button type="button" className="secondary-button" onClick={onLogout}>{text.auth.signOut}</button></div>}
      <ActionCard icon="help" title={text.settings.showGuide} description={text.onboarding.body} onClick={onShowOnboarding}/>
    </section>
    <section hidden={!shown('appearance')} className="settings-group settings-appearance-group" aria-labelledby="appearance-title">
      <SectionHeading title={sectionTitle('appearance')} id="appearance-title" />
      <div className="settings-card appearance-mode-row" id="setting-theme"><h3>{text.shell.theme}</h3><div className="segmented-control theme-mode-picker" role="group" aria-label={text.shell.theme}>{options.map(option => <button key={option.choice} type="button" aria-label={option.label} title={option.label} aria-pressed={themeChoice === option.choice} onClick={() => onThemeChange(option.choice)}>
        <span className={'mode-preview mode-preview-'+option.choice}><ThemeSample id="offgrid" mode={option.choice === 'dark' ? 'dark' : 'light'}/>{option.choice === 'system' && <ThemeSample id="offgrid" mode="dark"/>}</span>
      </button>)}</div></div>
      {shown('appearance') && <AppearanceSettings />}
    </section>
    <section hidden={!shown('models')} className="settings-group" aria-labelledby="model-settings-title">
      <SectionHeading title={sectionTitle('models')} id="model-settings-title" />
      <div className="action-card-grid">
        <ActionCard icon="models" title={text.nav.models} description={`${text.models.installed} · ${text.models.discover}`} href="#/models" />
        <ActionCard icon="agents" title={text.nav.agents} description={profileSelectionPrompt[locale]} href="#/agents" />
      </div>
      <div className="settings-response-section"><p className="settings-scope">{description('models')}</p>
        <ComposerResponseLimits preferences={preferences} busy={false} update={updatePreferences} presentation="settings" />
      </div>
    </section>
    {shown('voice') && <section className="settings-group" aria-labelledby="voice-settings-title">
      <SectionHeading title={sectionTitle('voice')} description={description('voice')} id="voice-settings-title" /><VoiceSettings inline />
    </section>}
    {shown('shortcuts') && <section className="settings-group" aria-labelledby="shortcuts-settings-title"><SectionHeading title={sectionTitle('shortcuts')} id="shortcuts-settings-title"/><DesktopCommandSettings /></section>}
    {shown('storage') && <section className="settings-group" aria-labelledby="storage-settings-title"><SectionHeading title={sectionTitle('storage')} description={description('storage')} id="storage-settings-title"/>{desktopPaths ? <div className="settings-card desktop-local-paths"><dl><div><dt>{text.shell.modelsPath}</dt><dd>{desktopPaths.models}</dd></div><div><dt>{text.shell.dataPath}</dt><dd>{desktopPaths.data}</dd></div></dl></div> : <p role="status">{unknown}</p>}</section>}
    <section hidden={!shown('terminal')} className="settings-group" aria-labelledby="terminal-settings-title">
      <SectionHeading title={sectionTitle('terminal')} description={description('terminal')} id="terminal-settings-title" />
      {window.electron?.openTerminal ? <ActionCard icon="terminal" title={copy.terminal} description="OffGrid CLI" onClick={() => void window.electron!.openTerminal!().catch(reason => setError(reason instanceof Error ? reason.message : text.common.error))} /> : <p>{copy.browser}</p>}
    </section>
    <section hidden={!shown('access')} className="settings-group" aria-labelledby="access-settings-title">
      <SectionHeading title={sectionTitle('access')} description={description('access')} id="access-settings-title" />
      <div className="settings-panel"><div><h3>{text.settings.computerUse}</h3><p>{computerDescription}</p></div>{admin && !!details.computer?.active_sessions && <div className="settings-actions"><span role="status" className={details.computer.emergency_stop ? 'status-pill danger' : 'status-pill'}>{details.computer.emergency_stop ? text.settings.stopped : `${details.computer.active_sessions} ${text.settings.sessions}`}</span><button className="danger-button" disabled={stopping} onClick={() => void stopComputer()}>{stopping ? text.common.loading : text.settings.emergencyStop}</button></div>}</div>
    </section>
    <section hidden={!shown('advanced')} className="settings-group settings-about" aria-labelledby="workspace-title">
      <SectionHeading title={sectionTitle('advanced')} id="workspace-title" />
      <div className="settings-product"><strong>OffGrid</strong><span>{backend?.desktopVersion ?? details.identity?.version ?? details.config?.version ?? unknown}</span></div>
      {health === 'offline' && <p role="status">{text.guidance.serviceOffline}</p>}
      <div className="settings-service-details desktop-storage"><dl>
      <div><dt>{text.settings.service}</dt><dd><code dir="ltr">{backend?.url ?? window.location.origin}</code></dd></div>
      <div><dt>{management.serviceState}</dt><dd>{health === 'ready' ? text.status.ready : health === 'checking' ? text.status.checking : text.status.offline}</dd></div>
      <div><dt>{text.settings.version}</dt><dd>{details.identity?.version ?? details.config?.version ?? unknown}</dd></div>

    </dl></div>

    <details className="settings-diagnostics desktop-storage"><summary>{management.diagnostics}</summary><p>{description('advanced')}</p><p>{text.guidance.connectionMeaning}</p><dl>
      <div><dt>API</dt><dd>{details.identity?.api_version ?? unknown}</dd></div>
      {backend && <><div><dt>{compatibility.title}</dt><dd>{backend.compatibilityBasis === 'declared-contract' ? compatibility.declared : backend.compatibilityBasis === 'reviewed-legacy' ? compatibility.legacy : unknown}{backend.bridgeProtocol ? ` · ${backend.bridgeProtocol}` : ''}</dd></div><div><dt>{text.settings.service}</dt><dd>{backend.managedByDesktop ? compatibility.owned : compatibility.external}</dd></div></>}
      <div><dt>UI SHA-256</dt><dd><code>{details.identity?.ui_build_id || unknown}</code></dd></div>
      <div><dt>{text.settings.inferenceSlots}</dt><dd>{details.config?.inference_slots ?? unknown}</dd></div>
      {knowledge && <div><dt>{text.settings.knowledge}</dt><dd>{!details.rag ? unknown : details.rag.enabled ? text.settings.enabled : text.settings.disabled}</dd></div>}
    </dl>

    </details>
    </section>
    </div>
  </div>;
}
