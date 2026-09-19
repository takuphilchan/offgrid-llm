import { useEffect, useRef, type RefObject } from 'react';

// For responsive drawers which cannot use a permanently modal <dialog>.
export function useFocusScope(ref: RefObject<HTMLElement | null>, open: boolean, close: () => void) {
  const latestClose = useRef(close); latestClose.current = close;
  useEffect(() => {
    if (!open || !ref.current) return;
    const element = ref.current;
    const previous = document.activeElement as HTMLElement | null;
    const candidates = () => Array.from(element.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex="0"]')).filter(item => item.getClientRects().length > 0);
    candidates()[0]?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.isComposing || event.keyCode === 229) return;
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
