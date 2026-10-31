import { useLayoutEffect, useRef, type RefObject } from 'react';

// For responsive drawers which cannot use a permanently modal <dialog>.
export function useFocusScope(ref: RefObject<HTMLElement | null>, open: boolean, close: () => void) {
  const latestClose = useRef(close); latestClose.current = close;
  useLayoutEffect(() => {
    if (!open || !ref.current) return;
    const element = ref.current;
    const previous = document.activeElement as HTMLElement | null;
    const candidates = () => Array.from(element.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex="0"]')).filter(item => item.tabIndex >= 0 && item.getClientRects().length > 0 && getComputedStyle(item).visibility !== 'hidden');
    candidates()[0]?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.isComposing || event.keyCode === 229) return;
      // A modal confirmation owns its own focus scope. A nonmodal utility in
      // this drawer owns Escape, but its controls still belong to the drawer's
      // Tab order; otherwise the final utility control can tab into browser UI.
      const dialog = Array.from(document.querySelectorAll('dialog[open]')).find(item => item !== element);
      if (dialog) return;
      const utility = document.querySelector(':popover-open');
      if (utility && (event.key === 'Escape' || !element.contains(utility))) return;
      if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); latestClose.current(); }
      if (event.key !== 'Tab') return;
      const items = candidates(); const first = items[0], last = items.at(-1);
      if (!first) { event.preventDefault(); return; }
      if (!element.contains(document.activeElement) || (event.shiftKey ? document.activeElement === first : document.activeElement === last)) {
        event.preventDefault(); (event.shiftKey ? last : first)?.focus();
      }
    };
    document.addEventListener('keydown', keydown, true);
    return () => { document.removeEventListener('keydown', keydown, true); previous?.focus(); };
  }, [ref, open]);
}
