import { useEffect, useRef, useState, type HTMLAttributes } from 'react';

/** Keep following live output until the reader deliberately scrolls away. */
export function useConversationScroll() {
  const conversationPane = useRef<HTMLDivElement | null>(null);
  const conversationContent = useRef<HTMLDivElement | null>(null);
  const followOutput = useRef(true);
  const userScrollUntil = useRef(0);
  const touchY = useRef<number | undefined>(undefined);
  const [showLatest, setShowLatest] = useState(false);
  const followConversation = () => {
    const pane = conversationPane.current;
    if (pane && followOutput.current) pane.scrollTo({ top: pane.scrollHeight, behavior: 'instant' });
    else if (pane) setShowLatest(pane.scrollHeight - pane.scrollTop - pane.clientHeight > 2);
  };
  const userScroll = (away = false) => {
    userScrollUntil.current = performance.now() + 1200;
    if (away) followOutput.current = false;
  };
  const scrollEvents: HTMLAttributes<HTMLDivElement> = {
    onFocus: event => { if(event.target === event.currentTarget) event.currentTarget.dataset.keyboardFocus = String(event.currentTarget.matches(':focus-visible')); },
    onBlur: event => { delete event.currentTarget.dataset.keyboardFocus; },
    onWheel: event => { delete event.currentTarget.dataset.keyboardFocus; userScroll(event.deltaY < 0); },
    onTouchStart: event => { delete event.currentTarget.dataset.keyboardFocus; touchY.current = event.touches[0]?.clientY; userScroll(); },
    onTouchMove: event => {
      const nextY = event.touches[0]?.clientY;
      userScroll(nextY !== undefined && touchY.current !== undefined && nextY > touchY.current);
      touchY.current = nextY;
    },
    onTouchEnd: () => { touchY.current = undefined; },
    onPointerDown: event => { delete event.currentTarget.dataset.keyboardFocus; if (event.target === event.currentTarget) userScroll(true); },
    onKeyDown: event => {
      if (event.target instanceof Element && event.target.closest('input,textarea,select,[contenteditable=true]')) return;
      if (['ArrowUp','ArrowDown','PageUp','PageDown','Home','End',' '].includes(event.key)) {
        userScroll(['ArrowUp','PageUp','Home'].includes(event.key) || (event.key === ' ' && event.shiftKey));
        // Keep transcript endpoints immediate. A native animated Home scroll
        // can otherwise finish after Latest and undo that explicit action.
        if (event.target === event.currentTarget && (event.key === 'Home' || event.key === 'End')) {
          event.preventDefault();
          event.currentTarget.scrollTo({ top: event.key === 'Home' ? 0 : event.currentTarget.scrollHeight, behavior: 'instant' });
        }
      }
    },
    onScroll: event => {
      const pane = event.currentTarget;
      // A small upward gesture is still reading intent. Resume only at the
      // actual bottom, not within the old 100px zone that caught the reader.
      const atBottom = pane.scrollHeight - pane.scrollTop - pane.clientHeight <= 2;
      if (performance.now() <= userScrollUntil.current) followOutput.current = atBottom;
      setShowLatest(!atBottom && !followOutput.current);
    },
  };
  useEffect(() => {
    const content = conversationContent.current, pane = conversationPane.current;
    if (!content || !pane) return;
    const observer = new ResizeObserver(followConversation);
    observer.observe(content);
    observer.observe(pane);
    return () => observer.disconnect();
  }, []);
  return { conversationPane, conversationContent, followOutput, userScrollUntil, showLatest, setShowLatest, followConversation, scrollEvents };
}
