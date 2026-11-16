import { workSurface } from '../../i18n/work-surface';
import { useI18n } from '../../i18n';
import { workspaceShell } from '../../i18n/workspace-shell';
import { navigationGroups, type Page, type ServiceHealth } from '../../lib/navigation';
import { useEffect, useRef, useState } from 'react';
import { useFocusScope } from '../../lib/focus-scope';
import { Icon } from '../Icon';
import { BrandMark } from '../BrandMark';

function RailLink({ item, label, active, onActivate }: { item: Page | 'home'; label: string; active: boolean; onActivate: () => void }) {
  const [visible, setVisible] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const dismiss = () => { clearTimeout(timer.current); setVisible(false); };
  useEffect(() => () => clearTimeout(timer.current), []);
  return <a href={`#/${item}`} className={active ? 'nav-link active' : 'nav-link'} aria-label={label} aria-current={active ? 'page' : undefined}
    data-label-open={visible || undefined}
    onPointerEnter={event => { if (event.pointerType !== 'touch') timer.current = setTimeout(() => setVisible(true), 450); }}
    onPointerLeave={dismiss} onBlur={dismiss}
    onFocus={event => { if (event.currentTarget.matches(':focus-visible')) setVisible(true); }}
    onKeyDown={event => { if (event.key === 'Escape' && visible) { dismiss(); if (!document.querySelector(':popover-open')) event.stopPropagation(); } }}
    onClick={() => { dismiss(); onActivate(); }}>
    <Icon name={item} size={18} /><span>{label}</span>
  </a>;
}

export function WorkspaceNavigation({ page, open, narrow, collapsed, onToggle, onClose, historyRef, actionRef }: {
  page: Page; health: ServiceHealth; open: boolean; narrow: boolean; onClose: () => void;
  collapsed: boolean; onToggle: () => void;
  historyRef: (element: HTMLDivElement | null) => void; actionRef: (element: HTMLDivElement | null) => void;
}) {
  const { messages: text, locale } = useI18n();
  const labels = workspaceShell[locale];
  const panel = useRef<HTMLElement>(null);
  useFocusScope(panel, narrow && open, onClose);
  const hasContext = page === 'chat' || page === 'settings';
  const showContext = narrow || (hasContext && !collapsed);
  const newChat = page !== 'chat' && <a className="new-chat-button" href="#/chat/new" onClick={onClose} title={text.chat.newChat} aria-label={text.chat.newChat}><Icon name="plus" size={18} /><span>{text.chat.newChat}</span></a>;
  return <aside ref={panel} id="workspace-sidebar" data-page={page} className={`sidebar${open ? ' open' : ''}`} role={narrow && open ? 'dialog' : undefined} aria-modal={narrow && open || undefined} aria-label={labels.navigation}>
    <div className="sidebar-rail">
      <a className="rail-brand" href="#/home" aria-label={text.product} onClick={onClose}><BrandMark /></a>
      {!narrow && hasContext && collapsed && <button className="utility-button icon-button sidebar-collapse" type="button" aria-label={labels.expand} title={labels.expand} aria-expanded={false} onClick={onToggle}><Icon name="sidebar" size={18} /></button>}
      {(!showContext || page !== 'chat') && <div className="sidebar-create" ref={actionRef}>{newChat}</div>}
    <nav id="workspace-navigation" className="primary-nav" aria-label={labels.navigation}>
      {navigationGroups.filter(group => group.label === 'work').map(group => <div className={`nav-section nav-section-${group.label}`} key={group.label}>
      <div className="nav-group" role="group" aria-label={text.shell[group.label]}>
        {group.items.map(item => <div key={item} className={item === 'chat' ? 'sidebar-chat-row' : 'sidebar-nav-row'}><RailLink item={item} label={item === 'knowledge' ? workSurface(locale).library : text.nav[item]} active={page === item && !(item==='chat'&&location.hash==='#/home')} onActivate={onClose} /></div>)}
      </div>
      </div>)}
    </nav>
    <div className="sidebar-foot">
      <RailLink item="settings" label={text.nav.settings} active={page === 'settings'} onActivate={onClose} />
    </div>
    </div>
    <div className="sidebar-detail" hidden={!showContext}>
      <div className="sidebar-heading">
        <strong>{page === 'settings' ? text.nav.settings : text.product}</strong>
        {!narrow && showContext && <button className="utility-button icon-button sidebar-collapse" type="button" aria-label={labels.collapse} title={labels.collapse} aria-expanded={true} onClick={onToggle}><Icon name="sidebar" size={18} /></button>}
        <button className="icon-button sidebar-toggle" type="button" aria-label={text.models.cancel} onClick={onClose}><Icon name="close" size={18} /></button>
      </div>
      {showContext && page === 'chat' && <div className="sidebar-create" ref={actionRef} />}
      <div className="sidebar-conversations">
        <div ref={historyRef} className="sidebar-context" hidden={!hasContext} />
      </div>
    </div>
  </aside>;
}
