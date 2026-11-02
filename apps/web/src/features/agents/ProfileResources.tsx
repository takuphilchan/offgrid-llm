import { useEffect, useRef, useState, type ReactNode } from 'react';

/** Empty optional capabilities stay out of the first-use form. Existing
 * selections are visible for review; disclosure never changes authority. */
export function ProfileResources({ title, count, children, onOpen }: { title: string; count: number; children: ReactNode; onOpen: () => void }) {
  const [open, setOpen] = useState(count > 0);
  const [visited, setVisited] = useState(count > 0);
  const load = useRef(onOpen); load.current = onOpen;
  useEffect(() => { if (open) { setVisited(true); load.current(); } }, [open]);
  return <details className="profile-resource-settings" open={open} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>{title}{count > 0 && <span className="profile-capability-count">{count}</span>}</summary>
    {visited && <fieldset className="profile-resources"><legend className="sr-only">{title}</legend>{children}</fieldset>}
  </details>;
}
