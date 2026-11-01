import { useState, type ReactNode } from 'react';

/** Disclosure state is presentation only; opening never selects or grants resources. */
export function ProfileCapability({ title, count, children }: { title: string; count: number; children: ReactNode }) {
  const [open, setOpen] = useState(count > 0);
  return <details className="profile-capability" open={open} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary><span>{title}</span><span className="profile-capability-count">{count}</span></summary>
    <div className="profile-capability-body">{children}</div>
  </details>;
}
