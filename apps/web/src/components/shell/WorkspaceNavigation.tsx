import { useI18n } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import { navigationGroups, type Page, type ServiceHealth } from '../../lib/navigation';
import { useRef, type ReactNode } from 'react';
import { useFocusScope } from '../../lib/focus-scope';
import { Icon } from '../Icon';

export function WorkspaceNavigation({ page, health, open, narrow, collapsed, onToggle, onClose, historyRef, actionRef, footer }: {
  page: Page; health: ServiceHealth; open: boolean; narrow: boolean; onClose: () => void;
  collapsed: boolean; onToggle: () => void;
  historyRef: (element: HTMLDivElement | null) => void; actionRef: (element: HTMLDivElement | null) => void; footer: ReactNode;
}) {
  const { messages: text, locale } = useI18n();
  const labels = workspaceShell[locale];
  const status = health === 'ready' ? text.status.ready : health === 'offline' ? text.status.offline : text.status.checking;
  const panel = useRef<HTMLElement>(null);
  useFocusScope(panel, narrow && open, onClose);
  return <aside ref={panel} id="workspace-sidebar" className={`sidebar${open ? ' open' : ''}`} role={narrow && open ? 'dialog' : undefined} aria-modal={narrow && open || undefined} aria-label={labels.navigation}>
    <div className="sidebar-heading">
      <div className="brand"><img src="/ui/app-mark.svg" width="28" height="28" alt="" /><strong>{text.product}</strong></div>
      {!narrow && <button className="utility-button icon-button sidebar-collapse" type="button" aria-label={collapsed ? labels.expand : labels.collapse} title={collapsed ? labels.expand : labels.collapse} aria-expanded={!collapsed} onClick={onToggle}><Icon name="sidebar" size={18} /></button>}
      <button className="icon-button sidebar-toggle" type="button" aria-label={text.models.cancel} onClick={onClose}><Icon name="close" size={18} /></button>
    </div>
    <div className="sidebar-create" ref={actionRef}>{page !== 'chat' && <a className="new-chat-button" href="#/chat/new" onClick={onClose} title={text.chat.newChat} aria-label={text.chat.newChat}><Icon name="plus" size={18} /><span>{text.chat.newChat}</span></a>}</div>
    <nav id="workspace-navigation" className="primary-nav" aria-label={labels.navigation}>
      {navigationGroups.map(group => <div className="nav-group" key={group.label}>
        {group.items.filter(item => item !== 'settings').map(item => <a key={item} href={`#/${item}`} onClick={onClose} className={page === item ? 'nav-link active' : 'nav-link'} aria-label={text.nav[item]} aria-current={page === item ? 'page' : undefined} title={text.nav[item]}>
          <Icon name={item} size={18} /><span>{text.nav[item]}</span>
        </a>)}
      </div>)}
    </nav>
    <div ref={historyRef} className="sidebar-context" hidden={page !== 'chat'} />
    <div className="sidebar-foot">
      {footer}
      <div className={`service-state ${health}`} role="status" title={status}><i aria-hidden="true" /><span>{status}</span></div>
    </div>
  </aside>;
}
