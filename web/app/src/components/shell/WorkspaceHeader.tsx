import { useId } from 'react';
import { useI18n, type LocaleCode } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import type { ThemeChoice } from '../../theme';
import { Icon } from '../Icon';
import { UtilityPopover } from '../UtilityPopover';
import { KnownWork } from './KnownWork';

export function WorkspaceHeader({ title, shortcut, refreshing, onRefresh, onCommands, theme, onThemeChange, username, onLogout }: {
  title: string; shortcut: string; refreshing: boolean; onRefresh: () => void; onCommands: () => void;
  theme: ThemeChoice; onThemeChange: (theme: ThemeChoice) => void; username?: string; onLogout: () => void;
}) {
  const { messages: text, locale, setLocale, available } = useI18n();
  const id = useId();
  return <header className="topbar">
    <h1>{title}</h1>
    <div className="topbar-actions">
      <KnownWork />
      <button className="command-trigger" onClick={onCommands} aria-label={`${text.shell.quickActions} (${shortcut})`} title={`${text.shell.quickActions} (${shortcut})`}><Icon name="search" size={16} /><span>{text.shell.quickActions}</span><kbd>{shortcut}</kbd></button>
      <button className="icon-button" disabled={refreshing} aria-busy={refreshing} onClick={onRefresh} aria-label={text.common.refresh} title={text.common.refresh}><Icon name="refresh" size={18} /></button>
      <UtilityPopover label={workspaceShell[locale].options} className="workspace-options" triggerClassName="icon-button" panelClassName="workspace-options-panel" align="end" trigger={<Icon name="settings" size={18} />}>
        <strong>{workspaceShell[locale].options}</strong>
        <div className="field locale-picker"><label htmlFor={`${id}-locale`}>{text.common.language}</label><select id={`${id}-locale`} value={locale} onChange={event => setLocale(event.target.value as LocaleCode)}>{available.map(item => <option key={item.code} value={item.code}>{item.label}</option>)}</select></div>
        <div className="field"><label htmlFor={`${id}-theme`}>{text.shell.appearance}</label><select id={`${id}-theme`} value={theme} onChange={event => onThemeChange(event.target.value as ThemeChoice)}>
          <option value="system">{text.shell.systemTheme}</option><option value="light">{text.shell.lightTheme}</option><option value="dark">{text.shell.darkTheme}</option>
        </select></div>
        {username !== undefined && <div className="workspace-account"><span>{username}</span><button className="text-button" onClick={onLogout}>{text.auth.signOut}</button></div>}
      </UtilityPopover>
    </div>
  </header>;
}
