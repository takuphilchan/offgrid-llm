import { useI18n } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import { navigationGroups, pages, type Page, type ServiceHealth } from '../../lib/navigation';
import { Icon } from '../Icon';

export function WorkspaceNavigation({ page, health, collapsed, onToggle }: {
  page: Page; health: ServiceHealth; collapsed: boolean; onToggle: () => void;
}) {
  const { messages: text, locale } = useI18n();
  const labels = workspaceShell[locale];
  const status = health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking;
  const toggle = collapsed ? labels.expand : labels.collapse;
  return <aside className="sidebar">
    <div className="sidebar-heading">
      <div className="brand"><div className="brand-mark" aria-hidden="true"><span /></div><strong>{text.product}</strong></div>
      <button className="icon-button sidebar-toggle" type="button" title={toggle} aria-label={toggle} aria-expanded={!collapsed} aria-controls="workspace-navigation" onClick={onToggle}><Icon name="menu" size={18} /></button>
    </div>
    <nav id="workspace-navigation" className="primary-nav" aria-label={labels.navigation}>
      {navigationGroups.map(group => <div className="nav-group" key={group.label}>
        <span className="nav-group-label">{text.shell[group.label]}</span>
        {group.items.map(item => <a key={item} href={`#/${item}`} className={page === item ? 'nav-link active' : 'nav-link'} aria-label={text.nav[item]} aria-current={page === item ? 'page' : undefined} title={text.nav[item]}>
          <Icon name={item} /><span>{text.nav[item]}</span>
        </a>)}
      </div>)}
    </nav>
    <div className="sidebar-foot">
      <div className={`service-state ${health}`} role="status" title={status}><i aria-hidden="true" /><span>{status}</span></div>
    </div>
  </aside>;
}

export function MobileNavigation({ page }: { page: Page }) {
  const { messages: text, locale } = useI18n();
  return <nav className="mobile-nav" aria-label={workspaceShell[locale].navigation}>{pages.map(item =>
    <a key={item} href={`#/${item}`} className={page === item ? 'active' : ''} aria-current={page === item ? 'page' : undefined}><Icon name={item} size={19} /><span>{text.nav[item]}</span></a>
  )}</nav>;
}
