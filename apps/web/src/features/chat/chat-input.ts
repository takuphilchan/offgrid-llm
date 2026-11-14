import type { KeyboardEvent } from 'react';

export function inspectConversationWork(target: EventTarget, follow: {current: boolean}, scrollUntil: {current: number}) {
  // Opening/focusing work evidence is reading intent too. Otherwise resize
  // following pulls expanded controls away before the next click or Tab.
  if (target instanceof Element && target.closest('.conversation-work')) {
    follow.current = false;
    scrollUntil.current = 0;
  }
}

export function reviewConversationWork(content: HTMLElement | null, follow: { current: boolean }, refresh: () => void) {
  const panel = content?.querySelector<HTMLElement>('.conversation-work');
  if (!panel) { refresh(); return; }
  follow.current = false;
  panel.focus({ preventScroll: true });
  panel.scrollIntoView({ block: 'start', behavior: 'instant' });
}

export function submitComposerInput(event: KeyboardEvent<HTMLTextAreaElement>, send: () => Promise<void>) {
  // Enter may confirm an IME candidate, including WebKit composition-end events.
  if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault();
    void send();
  }
}
