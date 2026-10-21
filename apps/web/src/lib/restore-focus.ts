// A modal opened from an auto popover closes that popover. Return to its
// visible trigger rather than attempting to focus an invisible action inside.
export function restoreFocus(previous: HTMLElement | null) {
  if (previous?.isConnected && previous.getClientRects().length && getComputedStyle(previous).visibility !== 'hidden') { previous.focus(); return; }
  const id = previous?.closest('[popover]')?.id;
  if (id) document.querySelector<HTMLElement>(`[aria-controls="${CSS.escape(id)}"]`)?.focus();
}
