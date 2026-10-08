import { useId, type ReactNode } from 'react';

/** Presentation only: these components never fetch, submit, or own work. */
export function ActionGroup({ children, label, className = '' }: { children: ReactNode; label?: string; className?: string }) {
  return <div className={`action-group ${className}`} role={label ? 'group' : undefined} aria-label={label}>{children}</div>;
}

export function SectionHeading({ title, description, actions, id }: { title: string; description?: ReactNode; actions?: ReactNode; id?: string }) {
  return <header className="workspace-section-heading"><div><h2 id={id}>{title}</h2>{description && <p>{description}</p>}</div>{actions && <ActionGroup>{actions}</ActionGroup>}</header>;
}

export function EmptyState({ title, description, action, className = '' }: { title: string; description?: ReactNode; action?: ReactNode; className?: string }) {
  return <div className={`workspace-empty ${className}`}><p>{title}</p>{description && <p className="workspace-secondary">{description}</p>}{action && <ActionGroup>{action}</ActionGroup>}</div>;
}

export function ScopedNotice({ children, action, kind = 'status', className = '' }: { children: ReactNode; action?: ReactNode; kind?: 'status' | 'error'; className?: string }) {
  return <div className={`workspace-notice ${kind === 'error' ? 'inline-error' : ''} ${className}`} role={kind === 'error' ? 'alert' : 'status'}><div>{children}</div>{action && <ActionGroup>{action}</ActionGroup>}</div>;
}

export function ListSearch({ label, value, onChange, className = '' }: { label: string; value: string; onChange: (value: string) => void; className?: string }) {
  const id = useId();
  return <label className={`workspace-list-search field ${className}`} htmlFor={id}><span>{label}</span><input id={id} type="search" value={value} onChange={event => onChange(event.target.value)} /></label>;
}
