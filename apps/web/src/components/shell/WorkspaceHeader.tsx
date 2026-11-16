import { useEffect, useState } from 'react';
import { useI18n } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import { Icon } from '../Icon';
import { KnownWork } from './KnownWork';
import { workspaceHelp } from '../../i18n/workspace-help';
import { settingsNavigation } from '../../i18n/settings-navigation';

export function WorkspaceHeader({ title, shortcut, onCommands, onHelp, navigationOpen, onNavigation }: {
  title: string; shortcut: string; onCommands: () => void; onHelp:()=>void; navigationOpen: boolean; onNavigation: () => void;
}) {
  const { messages: text, locale } = useI18n();
  const [terminalAvailable, setTerminalAvailable] = useState(false);
  useEffect(() => {
    let current = true;
    const bridge = window.electron;
    if (!bridge?.openTerminal || !bridge.getTerminalStatus) return;
    void bridge.getTerminalStatus().then(status => { if (current) setTerminalAvailable(status.available); }).catch(() => { if (current) setTerminalAvailable(false); });
    return () => { current = false; };
  }, []);
  return <header className="topbar">
    <div className="topbar-leading"><button className="utility-button icon-button navigation-trigger" title={workspaceShell[locale].navigation} aria-label={workspaceShell[locale].navigation} aria-expanded={navigationOpen} aria-controls="workspace-sidebar" onClick={onNavigation}><Icon name="menu" size={20} /></button><h1>{title}</h1></div>
    <div className="topbar-actions"><KnownWork />{terminalAvailable && <button className="utility-button icon-button" title={`${settingsNavigation[locale].terminal} (Ctrl+\u0060)`} aria-label={settingsNavigation[locale].terminal} onClick={() => void window.electron?.openTerminal?.()}><Icon name="terminal" size={18} /></button>}<button className="utility-button icon-button help-trigger" onClick={onHelp} aria-label={workspaceHelp[locale].help} title={workspaceHelp[locale].help}><Icon name="help" size={18}/></button><button className="command-trigger" onClick={onCommands} aria-label={`${text.shell.quickActions} (${shortcut})`} title={`${text.shell.quickActions} (${shortcut})`}><Icon name="search" size={16} /><span>{text.shell.quickActions}</span><kbd>{shortcut}</kbd></button></div>
  </header>;
}
