import { useI18n } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import { navigationGroups, type Page, type ServiceHealth } from '../../lib/navigation';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useFocusScope } from '../../lib/focus-scope';
import { Icon } from '../Icon';

function RailLink({ item, label, active, onActivate }: { item: Page; label: string; active: boolean; onActivate: () => void }) {
  const [visible, setVisible] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const dismiss = () => { clearTimeout(timer.current); setVisible(false); };
  useEffect(() => () => clearTimeout(timer.current), []);
  return <a href={`#/${item}`} className={active ? 'nav-link active' : 'nav-link'} aria-label={label} aria-current={active ? 'page' : undefined}
    data-label-open={visible || undefined}
    onPointerEnter={event => { if (event.pointerType !== 'touch') timer.current = setTimeout(() => setVisible(true), 450); }}
    onPointerLeave={dismiss} onBlur={dismiss}
    onFocus={event => { if (event.currentTarget.matches(':focus-visible')) setVisible(true); }}
    onKeyDown={event => { if (event.key === 'Escape' && visible) { dismiss(); event.stopPropagation(); } }}
    onClick={() => { dismiss(); onActivate(); }}>
    <Icon name={item} size={18} /><span role="tooltip" aria-hidden={!visible}>{label}</span>
  </a>;
}

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
  return <aside ref={panel} id="workspace-sidebar" data-page={page} className={`sidebar${open ? ' open' : ''}`} role={narrow && open ? 'dialog' : undefined} aria-modal={narrow && open || undefined} aria-label={labels.navigation}>
    <div className="sidebar-heading">
      <div className="brand"><img src="/ui/app-mark.svg" width="28" height="28" alt="" /><strong>{text.product}</strong></div>
      {!narrow && <button className="utility-button icon-button sidebar-collapse" type="button" aria-label={page === 'chat' ? collapsed ? labels.expand : labels.collapse : text.nav.chat} title={page === 'chat' ? collapsed ? labels.expand : labels.collapse : text.nav.chat} aria-expanded={page === 'chat' ? !collapsed : undefined} onClick={page === 'chat' ? onToggle : () => { window.location.hash = '#/chat'; onClose(); }}><Icon name="sidebar" size={18} /></button>}
      <button className="icon-button sidebar-toggle" type="button" aria-label={text.models.cancel} onClick={onClose}><Icon name="close" size={18} /></button>
    </div>
    <nav id="workspace-navigation" className="primary-nav" aria-label={labels.navigation}>
      {navigationGroups.filter(group => group.label !== 'system').map(group => <div className={`nav-section nav-section-${group.label}`} key={group.label}>
      <div className="nav-group" role="group" aria-label={text.shell[group.label]}>
        {group.items.map(item => <div key={item} className={item === 'chat' ? 'sidebar-chat-row' : 'sidebar-nav-row'}><RailLink item={item} label={text.nav[item]} active={page === item} onActivate={onClose} /></div>)}
      </div>
      </div>)}
    </nav>
    <div className="sidebar-conversations">
      <div className="sidebar-create" ref={actionRef}>{page !== 'chat' && <a className="new-chat-button" href="#/chat/new" onClick={onClose} title={text.chat.newChat} aria-label={text.chat.newChat}><Icon name="plus" size={18} /><span>{text.chat.newChat}</span></a>}</div>
      <div ref={historyRef} className="sidebar-context" hidden={page !== 'chat'} />
    </div>
    <div className="sidebar-foot">
      {footer}
      <div className={`service-state ${health}`} role="status" title={status}><i aria-hidden="true" /><span>{status}</span></div>
    </div>
  </aside>;
}
