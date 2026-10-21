import { useId } from 'react';
import { useI18n, type LocaleCode } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import type { ThemeChoice } from '../../theme';
import { Icon } from '../Icon';
import { UtilityPopover } from '../UtilityPopover';
import { KnownWork } from './KnownWork';

export function WorkspaceHeader({ title, shortcut, onCommands, navigationOpen, onNavigation }: {
  title: string; shortcut: string; onCommands: () => void; navigationOpen: boolean; onNavigation: () => void;
}) {
  const { messages: text, locale } = useI18n();
  return <header className="topbar">
    <div className="topbar-leading"><button className="secondary-button navigation-trigger" aria-label={workspaceShell[locale].navigation} aria-expanded={navigationOpen} aria-controls="workspace-sidebar" onClick={onNavigation}><Icon name="menu" size={18} /><span>{workspaceShell[locale].navigation}</span></button><h1>{title}</h1></div>
    <div className="topbar-actions"><KnownWork /><button className="command-trigger" onClick={onCommands} aria-label={`${text.shell.quickActions} (${shortcut})`} title={`${text.shell.quickActions} (${shortcut})`}><Icon name="search" size={16} /><span>{text.shell.quickActions}</span><kbd>{shortcut}</kbd></button></div>
  </header>;
}

export function WorkspaceOptions({ refreshing, onRefresh, theme, onThemeChange, username, onLogout }: {
  refreshing: boolean; onRefresh: () => void;
  theme: ThemeChoice; onThemeChange: (theme: ThemeChoice) => void; username?: string; onLogout: () => void;
}) {
  const { messages: text, locale, setLocale, available } = useI18n();
  const id = useId();
  return <UtilityPopover label={workspaceShell[locale].options} className="workspace-options" triggerClassName="utility-button" panelClassName="workspace-options-panel" trigger={<><Icon name="settings" size={18} /><span>{username ?? text.product}</span><Icon name="more" size={16} /></>}>
        <strong>{workspaceShell[locale].options}</strong>
        <a className="secondary-button" href="#/settings" aria-current={location.hash === '#/settings' ? 'page' : undefined}>{text.nav.settings}</a>
        <div className="field locale-picker"><label htmlFor={`${id}-locale`}>{text.common.language}</label><select id={`${id}-locale`} value={locale} onChange={event => setLocale(event.target.value as LocaleCode)}>{available.map(item => <option key={item.code} value={item.code}>{item.label}</option>)}</select></div>
        <div className="field"><label htmlFor={`${id}-theme`}>{text.shell.appearance}</label><select id={`${id}-theme`} value={theme} onChange={event => onThemeChange(event.target.value as ThemeChoice)}>
          <option value="system">{text.shell.systemTheme}</option><option value="light">{text.shell.lightTheme}</option><option value="dark">{text.shell.darkTheme}</option>
        </select></div>
        <button className="secondary-button" disabled={refreshing} aria-busy={refreshing} onClick={onRefresh}><Icon name="refresh" size={16} />{text.common.refresh}</button>
        {username !== undefined && <div className="workspace-account"><span>{username}</span><button className="text-button" onClick={onLogout}>{text.auth.signOut}</button></div>}
      </UtilityPopover>;
}
