'use strict';
// Selected output belongs to the user's clipboard, not to the PTY input stream.
// Without a selection, Ctrl+C must still reach the shell as an interrupt.
function terminalShortcut(event, selection) {
  if (event.type !== 'keydown' || event.altKey || (!event.ctrlKey && !event.metaKey)) return '';
  if (event.code === 'KeyC' && (selection || event.shiftKey || event.metaKey)) return 'copy';
  if (event.code === 'KeyV') return 'paste';
  if (event.code === 'KeyF' && event.shiftKey) return 'search';
  return '';
}
if (typeof module !== 'undefined') module.exports = {terminalShortcut};
