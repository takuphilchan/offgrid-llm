import { useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react';

/** Nonmodal utilities only. Consequential confirmations keep their dialog contract. */
export function UtilityPopover({ label, trigger, children, className = '', triggerClassName = 'text-button', panelClassName = '', align = 'start', onOpenChange }: {
  label: string; trigger?: ReactNode; children: ReactNode; className?: string;
  triggerClassName?: string; panelClassName?: string; align?: 'start' | 'end';
  onOpenChange?: (open: boolean) => void;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const button = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const content = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!open) return;
    const position = () => {
      if (!button.current || !panel.current) return;
      const anchor = button.current.getBoundingClientRect();
      const viewport = window.visualViewport;
      const left = viewport?.offsetLeft ?? 0, top = viewport?.offsetTop ?? 0;
      const width = viewport?.width ?? window.innerWidth, height = viewport?.height ?? window.innerHeight;
      const margin = 12, gap = 8;
      if (anchor.bottom <= top || anchor.top >= top + height || anchor.right <= left || anchor.left >= left + width) {
        panel.current.hidePopover();
        return;
      }
      const panelWidth = Math.min(340, Math.max(0, width - 2 * margin));
      const above = Math.max(0, anchor.top - top - margin - gap);
      const below = Math.max(0, top + height - anchor.bottom - margin - gap);
      const upwards = below < Math.min(panel.current.scrollHeight, 360) && above > below;
      const space = Math.max(above, below);
      // At extreme zoom or short height, use the viewport rather than an unusable sliver.
      const overlay = space < 100;
      const rightAligned = (getComputedStyle(button.current).direction === 'rtl') !== (align === 'end');
      const start = rightAligned ? anchor.right - panelWidth : anchor.left;
      Object.assign(panel.current.style, {
        width: `${panelWidth}px`, left: `${Math.max(left + margin, Math.min(start, left + width - panelWidth - margin))}px`,
        top: overlay ? `${top + margin}px` : upwards ? 'auto' : `${anchor.bottom + gap}px`,
        bottom: !overlay && upwards ? `${window.innerHeight - anchor.top + gap}px` : 'auto',
        maxHeight: `${overlay ? Math.max(0, height - 2 * margin) : upwards ? above : below}px`,
      });
    };
    position();
    const firstFrame = requestAnimationFrame(position);
    const observer = new ResizeObserver(position);
    if (content.current) observer.observe(content.current);
    // Language/direction can change without changing the panel's dimensions.
    const direction = new MutationObserver(position);
    direction.observe(document.documentElement, { attributes: true, attributeFilter: ['dir'] });
    window.addEventListener('resize', position);
    // Scrolls from the panel/content must not recalculate placement. Otherwise
    // changing max-height/edges during wheel or keyboard scrolling can reset it.
    const scroll = (event: Event) => {
      if (event.target instanceof Node && panel.current?.contains(event.target)) return;
      position();
    };
    window.addEventListener('scroll', scroll, true);
    const dismiss = () => panel.current?.hidePopover();
    window.addEventListener('hashchange', dismiss);
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !panel.current?.matches(':popover-open')) return;
      event.preventDefault(); panel.current.hidePopover(); button.current?.focus();
    };
    window.addEventListener('keydown', escape);
    window.visualViewport?.addEventListener('resize', position);
    window.visualViewport?.addEventListener('scroll', position);
    return () => {
      cancelAnimationFrame(firstFrame);
      observer.disconnect(); direction.disconnect();
      window.removeEventListener('resize', position);
      window.removeEventListener('scroll', scroll, true);
      window.removeEventListener('hashchange', dismiss);
      window.removeEventListener('keydown', escape);
      window.visualViewport?.removeEventListener('resize', position);
      window.visualViewport?.removeEventListener('scroll', position);
    };
  }, [open, align]);

  return <div className={`utility-popover ${className}`}>
    <button ref={button} type="button" className={triggerClassName} title={label} aria-label={label}
      popoverTarget={id} aria-expanded={open} aria-controls={id}>{trigger ?? label}</button>
    <div ref={panel} id={id} className={`utility-popover-panel ${panelClassName}`} popover="auto" role="region" aria-label={label} tabIndex={0}
      onKeyDown={event => {
        // Chromium/Electron do not consistently give a focused generic region
        // native scroll keys. Only handle the region itself, never its controls.
        if (event.target !== event.currentTarget) return;
        const el = event.currentTarget, page = Math.max(40, el.clientHeight - 32);
        const top = ({ Home: 0, End: el.scrollHeight, PageUp: el.scrollTop - page,
          PageDown: el.scrollTop + page, ArrowUp: el.scrollTop - 40, ArrowDown: el.scrollTop + 40 } as Record<string, number>)[event.key];
        if (top !== undefined) { event.preventDefault(); el.scrollTop = top; }
      }}
      onBeforeToggle={event => setOpen(event.newState === 'open')}
      onClick={event => { if ((event.target as Element).closest('a[href]')) panel.current?.hidePopover(); }}
      onToggle={event => onOpenChange?.(event.newState === 'open')}>
      <div ref={content} className="utility-popover-content">{children}</div>
    </div>
  </div>;
}
